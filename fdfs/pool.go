package fdfs

import (
	"context"
	"errors"
	"io"
	"myClient/library/pool"
	"time"

	xtime "myClient/library/time"
)

var ErrClosed = errors.New("pool is closed")

// Config memcache config.
type Config struct {
	*pool.Config

	Proto        string
	Addr         string
	DialTimeout  xtime.Duration
	ReadTimeout  xtime.Duration
	WriteTimeout xtime.Duration
}

// Pool memcache connection pool struct.
type Pool struct {
	p pool.Pool
	c *Config
}

// NewPool new a memcache conn pool.
func NewPool(c *Config) (p *Pool) {

	if c.DialTimeout <= 0 || c.ReadTimeout <= 0 || c.WriteTimeout <= 0 {
		panic("must config fdfs timeout")
	}
	c.Ping = checkConn
	p1 := pool.NewList(c.Config)
	cnop := DialConnectTimeout(time.Duration(c.DialTimeout))
	rdop := DialReadTimeout(time.Duration(c.ReadTimeout))
	wrop := DialWriteTimeout(time.Duration(c.WriteTimeout))
	p1.New = func(ctx context.Context) (io.Closer, error) {
		conn, err := Dial(c.Proto, c.Addr, cnop, rdop, wrop)
		return conn, err
	}
	p = &Pool{p: p1, c: c}
	return
}

func (p *Pool) Get(ctx context.Context) Conn {
	c, err := p.p.Get(ctx)
	if err != nil {
		return errorConnection{err}
	}
	c1, _ := c.(Conn)
	return &pooledConnection{p: p, c: c1}
}

// Close release the resources used by the pool.
func (p *Pool) Close() error {
	return p.p.Close()
}

func checkConn(someConn interface{}) bool {
	conn,_ := someConn.(Conn)

	return conn.CheckConn()
}

type pooledConnection struct {
	p   *Pool
	c   Conn
}

func (pc *pooledConnection) CheckConn() bool {
	return pc.c.CheckConn()
}

func (pc *pooledConnection) Receive() (reply []byte, err error) {
	return pc.c.Receive()
}

func (pc *pooledConnection) Send(bytesStream []byte) error {
	return pc.c.Send(bytesStream)
}

func (pc *pooledConnection) Close() error {
	c := pc.c
	if _, ok := c.(errorConnection); ok {
		return nil
	}
	pc.c = errorConnection{ErrConnClosed}
	pc.p.p.Put(context.Background(), c, c.Err() != nil)
	return nil
}

func (pc *pooledConnection) Err() error {
	return pc.c.Err()
}

type errorConnection struct{ err error }

func (ec errorConnection) CheckConn() bool {
	return false
}
func (ec errorConnection) Receive() (reply []byte, err error) {
	return nil, ec.err
}
func (ec errorConnection) Send(bytesStream []byte) error {return ec.err}
func (ec errorConnection) Close() error { return ec.err }
func (ec errorConnection) Err() error   { return ec.err }
