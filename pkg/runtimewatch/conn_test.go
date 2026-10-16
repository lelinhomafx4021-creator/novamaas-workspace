package runtimewatch

import (
	"context"
	"io"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixtureConn struct{ closes int }

func (*fixtureConn) Read(buffer []byte) (int, error)  { return copy(buffer, "abc"), io.EOF }
func (*fixtureConn) Write([]byte) (int, error)        { return 2, io.ErrUnexpectedEOF }
func (connection *fixtureConn) Close() error          { connection.closes++; return nil }
func (*fixtureConn) LocalAddr() net.Addr              { return nil }
func (*fixtureConn) RemoteAddr() net.Addr             { return nil }
func (*fixtureConn) SetDeadline(time.Time) error      { return nil }
func (*fixtureConn) SetReadDeadline(time.Time) error  { return nil }
func (*fixtureConn) SetWriteDeadline(time.Time) error { return nil }

type fixtureSyscallConn struct{ fixtureConn }

func (*fixtureSyscallConn) SyscallConn() (syscall.RawConn, error) { return nil, syscall.EBADF }

func TestMeasuredConnectionTracksPartialIOAndClosesOnce(t *testing.T) {
	collector := NewCollector()
	underlying := &fixtureConn{}
	connection := collector.WrapConn(ScopeDatabase, underlying)
	assert.Equal(t, int64(1), trafficRow(t, collector.Snapshot(60), ScopeDatabase, CategoryOther).Connections)
	buffer := make([]byte, 8)
	count, err := connection.Read(buffer)
	assert.Equal(t, 3, count)
	assert.ErrorIs(t, err, io.EOF)
	assert.Equal(t, uint64(3), trafficRow(t, collector.Snapshot(60), ScopeDatabase, CategoryOther).ReceivedBytes)
	count, err = connection.Write([]byte("longer payload"))
	assert.Equal(t, 2, count)
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
	assert.Equal(t, uint64(2), trafficRow(t, collector.Snapshot(60), ScopeDatabase, CategoryOther).SentBytes,
		"partial writes must be observable before closing the connection")
	require.NoError(t, connection.Close())
	require.NoError(t, connection.Close())
	assert.Equal(t, 1, underlying.closes)
	row := trafficRow(t, collector.Snapshot(60), ScopeDatabase, CategoryOther)
	assert.Zero(t, row.Connections)
	assert.Equal(t, uint64(3), row.ReceivedBytes)
	assert.Equal(t, uint64(2), row.SentBytes)
	_, supportsRaw := connection.(syscall.Conn)
	assert.False(t, supportsRaw, "unsupported syscall.Conn must stay unsupported")
}

func TestConnectionOnlyWrapperDoesNotDoubleCountHTTPBodyBytes(t *testing.T) {
	collector := NewCollector()
	connection := collector.WrapConnection(ScopeOutbound, &fixtureSyscallConn{})
	_, supportsRaw := connection.(syscall.Conn)
	require.True(t, supportsRaw)
	_, err := connection.(syscall.Conn).SyscallConn()
	assert.ErrorIs(t, err, syscall.EBADF)
	_, _ = connection.Read(make([]byte, 4))
	_, _ = connection.Write([]byte("request"))
	row := trafficRow(t, collector.Snapshot(60), ScopeOutbound, CategoryOther)
	assert.Equal(t, int64(1), row.Connections)
	assert.Zero(t, row.ReceivedBytes)
	assert.Zero(t, row.SentBytes)
	require.NoError(t, connection.Close())
	assert.Empty(t, collector.Snapshot(60).Traffic)
	assert.Nil(t, collector.WrapConn(ScopeCache, nil))
}

func TestContextCategoryIsFixedAndDoesNotRetainRequestLabels(t *testing.T) {
	ctx := WithCategory(context.Background(), CategoryVideo)
	assert.Equal(t, CategoryVideo, Category(ctx))
	assert.Equal(t, CategoryOther, Category(context.Background()))
	assert.Equal(t, CategoryOther, Category(WithCategory(ctx, "https://private/request")))
}

func TestDisabledCategoryPreservesOriginalContext(t *testing.T) {
	previousEnabled := Enabled()
	SetEnabled(false)
	t.Cleanup(func() { SetEnabled(previousEnabled) })
	type requestContextKey struct{}
	ctx := context.WithValue(context.Background(), requestContextKey{}, "original value")
	observed := WithCategory(ctx, CategoryVideo)
	assert.Same(t, ctx, observed)
	assert.Equal(t, "original value", observed.Value(requestContextKey{}))
	assert.Equal(t, CategoryOther, Category(observed))
}
