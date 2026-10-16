package runtimewatch

import (
	"net"
	"sync"
	"syscall"
)

type measuredConn struct {
	net.Conn
	scope      string
	collector  *Collector
	closeOnce  sync.Once
	closeError error
	bytes      ByteCounter
}

type measuredSyscallConn struct {
	*measuredConn
	raw syscall.Conn
}

func (connection *measuredSyscallConn) SyscallConn() (syscall.RawConn, error) {
	return connection.raw.SyscallConn()
}

func (connection *measuredConn) Read(buffer []byte) (int, error) {
	count, err := connection.Conn.Read(buffer)
	connection.bytes.Add(int64(count), 0)
	return count, err
}

func (connection *measuredConn) Write(buffer []byte) (int, error) {
	count, err := connection.Conn.Write(buffer)
	connection.bytes.Add(0, int64(count))
	return count, err
}

func (connection *measuredConn) Close() error {
	connection.closeOnce.Do(func() {
		connection.closeError = connection.Conn.Close()
		connection.collector.TrackConnection(connection.scope, -1)
	})
	return connection.closeError
}

// WrapConn measures actual socket reads/writes, including protocol overhead.
// HTTP body accounting should use its transport wrapper instead to avoid
// counting the same bytes twice. A nil connection remains nil.
func (collector *Collector) WrapConn(scope string, connection net.Conn) net.Conn {
	if connection == nil {
		return nil
	}
	collector.TrackConnection(scope, 1)
	measured := &measuredConn{Conn: connection, scope: scope, collector: collector, bytes: collector.NewByteCounter(scope, CategoryOther)}
	if raw, ok := connection.(syscall.Conn); ok {
		return &measuredSyscallConn{measuredConn: measured, raw: raw}
	}
	return measured
}

func WrapConn(scope string, connection net.Conn) net.Conn {
	if !Enabled() {
		return connection
	}
	return defaultCollector.WrapConn(scope, connection)
}

// WrapConnection tracks a socket's lifetime without counting bytes. Use this
// with HTTP transports whose logical body bytes are measured separately.
func (collector *Collector) WrapConnection(scope string, connection net.Conn) net.Conn {
	if connection == nil {
		return nil
	}
	collector.TrackConnection(scope, 1)
	measured := &measuredConn{Conn: connection, scope: scope, collector: collector}
	if raw, ok := connection.(syscall.Conn); ok {
		return &measuredSyscallConn{measuredConn: measured, raw: raw}
	}
	return measured
}

func WrapConnection(scope string, connection net.Conn) net.Conn {
	if !Enabled() {
		return connection
	}
	return defaultCollector.WrapConnection(scope, connection)
}
