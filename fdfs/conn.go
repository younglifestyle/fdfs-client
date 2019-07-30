package fdfs

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"net"
	"sync"
	"time"
)

type conn struct {
	// Shared
	mu   sync.Mutex
	err  error
	conn net.Conn

	// Read & Write
	readTimeout  time.Duration
	writeTimeout time.Duration
	rw           *bufio.ReadWriter
}


/*
 * Functional Options Patter
 */
// DialOption specifies an option for dialing a Redis server.
type DialOption struct {
	f func(*dialOptions)
}

type dialOptions struct {
	readTimeout  time.Duration
	writeTimeout time.Duration
	dial         func(network, addr string) (net.Conn, error)
}

// DialReadTimeout specifies the timeout for reading a single command reply.
func DialReadTimeout(d time.Duration) DialOption {
	return DialOption{func(do *dialOptions) {
		do.readTimeout = d
	}}
}

// DialWriteTimeout specifies the timeout for writing a single command.
func DialWriteTimeout(d time.Duration) DialOption {
	return DialOption{func(do *dialOptions) {
		do.writeTimeout = d
	}}
}

// DialConnectTimeout specifies the timeout for connecting to the Redis server.
func DialConnectTimeout(d time.Duration) DialOption {
	return DialOption{func(do *dialOptions) {
		dialer := net.Dialer{Timeout: d}
		do.dial = dialer.Dial
	}}
}

// Dial connects to the Redis server at the given network and
// address using the specified options.
func Dial(network, address string, options ...DialOption) (Conn, error) {
	do := dialOptions{
		dial: net.Dial,
	}
	for _, option := range options {
		option.f(&do)
	}

	netConn, err := do.dial(network, address)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return NewConn(netConn, do.readTimeout, do.writeTimeout), nil
}

// NewConn new a Tracker/Storage conn.
func NewConn(netConn net.Conn, readTimeout, writeTimeout time.Duration) Conn {
	if writeTimeout <= 0 || readTimeout <= 0 {
		panic("must config fdfs timeout")
	}
	cn := &conn{
		conn: netConn,
		rw: bufio.NewReadWriter(bufio.NewReader(netConn),
			bufio.NewWriter(netConn)),
		readTimeout:  readTimeout,
		writeTimeout: writeTimeout,
	}

	return cn
}

func (c *conn) Close() error {
	c.mu.Lock()
	err := c.err
	if err != nil {
		c.err = errors.New("fsds-go: closed")
		err = c.conn.Close()
	}
	c.mu.Unlock()
	return err
}

func (c *conn) Err() error {
	c.mu.Lock()
	err := c.err
	c.mu.Unlock()
	return err
}

func (c *conn) fatal(err error) error {
	c.mu.Lock()
	if c.err == nil {
		c.err = errors.WithStack(err)
		// Close connection to force errors on subsequent calls and to unblock
		// other reader or writer.
		c.conn.Close()
	}
	c.mu.Unlock()
	return c.err
}

func (c *conn) Receive() (reply []byte, err error) {
	if c.readTimeout != 0 {
		c.conn.SetReadDeadline(time.Now().Add(c.readTimeout))
	}

	var header = &pkgHeader{}
	buf := make([]byte, 10)
	if _, err := c.rw.Read(buf); err != nil {
		return nil, err
	}

	buffer := bytes.NewBuffer(buf)
	if err := binary.Read(buffer, binary.BigEndian, &header.pkgLen); err != nil {
		return nil, err
	}

	bodyBuf := make([]byte, header.pkgLen)
	if _, err := io.ReadFull(c.rw, bodyBuf); err != nil {
		return nil, c.fatal(err)
	}

	// bytes.Buffer not support seek
	allBuffer := bytes.NewBuffer(buf)
	_, err = allBuffer.Write(bodyBuf)
	if err != nil {
		return nil, c.fatal(err)
	}
	return allBuffer.Bytes(), nil
}

func (c *conn) Send(bytesStream []byte) error {
	if c.writeTimeout != 0 {
		c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	}

	c.rw.Write(bytesStream)
	if err := c.rw.Flush(); err != nil {
		return c.fatal(err)
	}

	return nil
}

func (c *conn) CheckConn() bool {
	header := &header{
		cmd: FDFS_PROTO_CMD_ACTIVE_TEST,
	}
	c.SendHeader(header)
	c.RecvHeader(header)
	if header.cmd == TRACKER_PROTO_CMD_RESP && header.status == 0 {
		return true
	}
	return false
}

func (c *conn) SendHeader(hd *header) error {
	buffer := new(bytes.Buffer)
	if err := binary.Write(buffer, binary.BigEndian, hd.pkgLen); err != nil {
		return err
	}
	buffer.WriteByte(byte(hd.cmd))
	buffer.WriteByte(byte(hd.status))

	if err := c.Send(buffer.Bytes()); err != nil {
		return err
	}
	return nil
}

func (c *conn) RecvHeader(hd *header) error {
	if c.readTimeout != 0 {
		c.conn.SetReadDeadline(time.Now().Add(c.readTimeout))
	}

	buf := make([]byte, 10)
	if _, err := c.conn.Read(buf); err != nil {
		return err
	}

	buffer := bytes.NewBuffer(buf)

	if err := binary.Read(buffer, binary.BigEndian, &hd.pkgLen); err != nil {
		return err
	}
	cmd, err := buffer.ReadByte()
	if err != nil {
		return err
	}
	status, err := buffer.ReadByte()
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("recv resp status %d != 0", status)
	}
	hd.cmd = int8(cmd)
	hd.status = int8(status)
	return nil
}
