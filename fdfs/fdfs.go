package fdfs

type Conn interface {
	// Close closes the connection.
	Close() error

	// Err returns a non-nil value if the connection is broken. The returned
	// value is either the first non-nil value returned from the underlying
	// network connection or a protocol parsing error. Applications should
	// close broken connections.
	Err() error

	// Receive receives a single reply from the Redis server
	Receive() (reply []byte, err error)

	// Send writes the command to the client's output buffer.
	Send(bytesStream []byte) error

	CheckConn() bool
}