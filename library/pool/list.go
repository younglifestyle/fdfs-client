package pool

import (
	"container/list"
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

var _ Pool = &List{}

// List .
type List struct {
	// New is an application supplied function for creating and configuring a
	// item.
	//
	// The item returned from new must not be in a special state
	// (subscribed to pubsub channel, transaction started, ...).
	New func(ctx context.Context) (io.Closer, error)

	// mu protects fields defined below.
	mu     sync.Mutex
	cond   chan struct{}
	closed bool
	active int32
	// clean stale items
	cleanerCh chan struct{}

	// Stack of item with most recently used at the front.
	idles list.List

	// Config pool configuration
	conf *Config

	ping func(interface{}) bool
}

// NewList creates a new pool.
func NewList(c *Config) *List {
	// check Config
	if c == nil || int(c.Active) < c.Idle {
		panic("config nil or Idle Must <= Active")
	}
	// new pool
	p := &List{conf: c}
	p.cond = make(chan struct{})
	p.ping = c.Ping
	p.startCleanerLocked(time.Duration(c.IdleTimeout))
	return p
}

// Ping 检查单条连接是否有效
func (p *List) Ping(conn interface{}) bool {
	if conn == nil {
		return false
	}
	return p.ping(conn)
}

// Reload reload config.
func (p *List) Reload(c *Config) error {
	p.mu.Lock()
	p.startCleanerLocked(time.Duration(c.IdleTimeout))
	p.conf = c
	p.mu.Unlock()
	return nil
}

// startCleanerLocked
func (p *List) startCleanerLocked(d time.Duration) {
	if d <= 0 {
		// if set 0, staleCleaner() will return directly
		return
	}
	if d < time.Duration(p.conf.IdleTimeout) && p.cleanerCh != nil {
		select {
		case p.cleanerCh <- struct{}{}:
		default:
		}
	}
	// run only one, clean stale items.
	if p.cleanerCh == nil {
		p.cleanerCh = make(chan struct{}, 1)
		go p.staleCleaner()
	}
}

// staleCleaner clean stale items proc.
func (p *List) staleCleaner() {
	ticker := time.NewTicker(100 * time.Millisecond)
	for {
		select {
		case <-ticker.C:
		case <-p.cleanerCh: // maxLifetime was changed or db was closed.
		}
		p.mu.Lock()
		if p.closed || p.conf.IdleTimeout <= 0 {
			p.mu.Unlock()
			return
		}
		for i, n := 0, p.idles.Len(); i < n; i++ {
			e := p.idles.Back()
			if e == nil {
				// no possible
				break
			}
			ic := e.Value.(item)
			if !ic.expired(time.Duration(p.conf.IdleTimeout)) {
				// not need continue.
				break
			}
			p.idles.Remove(e)
			p.release()
			p.mu.Unlock()
			ic.c.Close()
			p.mu.Lock()
		}
		p.mu.Unlock()
	}
}

// Get returns a item from the idles List or
// get a new item.
func (p *List) Get(ctx context.Context) (io.Closer, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, ErrPoolClosed
	}

	fmt.Println("active :", p.active)

	for {
		// get idles item.
		for i, n := 0, p.idles.Len(); i < n; i++ {
			e := p.idles.Front()
			if e == nil {
				break
			}
			ic := e.Value.(item)
			p.idles.Remove(e)
			p.mu.Unlock()

			if !ic.expired(time.Duration(p.conf.IdleTimeout)) {
				if p.ping != nil {
					//判断是否失效，失效则丢弃，如果用户没有设定 ping 方法，就不检查
					if err := p.Ping(ic.c); !err {
						fmt.Println("conn is not able to be connected: ", err)
					} else {
						return ic.c, nil
					}
				}

				// ping函数无设置
				return ic.c, nil
			} else {
				fmt.Println("ping n12...")
			}

			ic.c.Close()
			p.mu.Lock()
			p.release()
		}
		// Check for pool closed before dialing a new item.
		if p.closed {
			p.mu.Unlock()
			return nil, ErrPoolClosed
		}
		// new item if under limit.
		if p.conf.Active == 0 || p.active < p.conf.Active {
			newItem := p.New
			p.active++
			p.mu.Unlock()
			c, err := newItem(ctx)
			if err != nil {
				p.mu.Lock()
				p.release()
				p.mu.Unlock()
				c = nil
			}
			return c, err
		}

		// 若设置WaitTimeout,则等待一段时间,
		// 等待能有用户调用`Put`归还connect
		if p.conf.WaitTimeout == 0 && !p.conf.Wait {
			p.mu.Unlock()
			return nil, ErrPoolExhausted
		}
		wt := p.conf.WaitTimeout
		p.mu.Unlock()

		// slowpath: reset context timeout
		nctx := ctx
		cancel := func() {}
		if wt > 0 {
			// Shrink, 根据WaitTimeout,设置一个timeout context
			_, nctx, cancel = wt.Shrink(ctx)
		}
		select {
		case <-nctx.Done():
			cancel()
			return nil, nctx.Err()
		case <-p.cond:
		}
		cancel()
		p.mu.Lock()
	}
}

// Put put item into pool.
func (p *List) Put(ctx context.Context, c io.Closer, forceClose bool) error {
	p.mu.Lock()
	if !p.closed && !forceClose {
		p.idles.PushFront(item{createdAt: nowFunc(), c: c})
		if p.idles.Len() > p.conf.Idle {
			c = p.idles.Remove(p.idles.Back()).(item).c
		} else {
			c = nil
		}
	}
	if c == nil {
		p.signal()
		p.mu.Unlock()
		return nil
	}
	p.release()
	p.mu.Unlock()
	return c.Close()
}

// Close releases the resources used by the pool.
func (p *List) Close() error {
	p.mu.Lock()
	idles := p.idles
	p.idles.Init()
	p.closed = true
	p.active -= int32(idles.Len())
	p.mu.Unlock()
	for e := idles.Front(); e != nil; e = e.Next() {
		e.Value.(item).c.Close()
	}
	return nil
}

// release decrements the active count and signals waiters. The caller must
// hold p.mu during the call.
func (p *List) release() {
	p.active--
	p.signal()
}

func (p *List) signal() {
	select {
	default:
	case p.cond <- struct{}{}:
	}
}

//func (p *List) ReleaseOne() {
//	atomic.AddInt32(&p.active, -1)
//}
