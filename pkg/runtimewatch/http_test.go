package runtimewatch

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type observedRoundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip observedRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func trafficRowForTest(t *testing.T, scope, category string) TrafficRow {
	t.Helper()
	for _, row := range Snapshot(1).Traffic {
		if row.Scope == scope && row.Category == category {
			return row
		}
	}
	return TrafficRow{Scope: scope, Category: category}
}

func TestTransportCountsConsumedBodiesAndCompletesAtEOF(t *testing.T) {
	before := trafficRowForTest(t, "outbound", "video")
	transport := WrapTransport(observedRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		data, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		assert.Equal(t, "request", string(data))
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("response")), ContentLength: 999999}, nil
	}), "video")
	request, err := http.NewRequest(http.MethodPost, "https://provider.invalid/opaque", strings.NewReader("request"))
	require.NoError(t, err)
	request.ContentLength = 999999
	response, err := transport.RoundTrip(request)
	require.NoError(t, err)
	during := trafficRowForTest(t, "outbound", "video")
	assert.Equal(t, before.Active+1, during.Active)
	assert.Equal(t, before.SentBytes+7, during.SentBytes)
	assert.Equal(t, before.ReceivedBytes, during.ReceivedBytes)
	buffer := make([]byte, 3)
	_, err = io.ReadFull(response.Body, buffer)
	require.NoError(t, err)
	assert.Equal(t, before.ReceivedBytes+3, trafficRowForTest(t, "outbound", "video").ReceivedBytes)
	_, err = io.Copy(io.Discard, response.Body)
	require.NoError(t, err)
	afterEOF := trafficRowForTest(t, "outbound", "video")
	assert.Equal(t, before.Active, afterEOF.Active)
	assert.Equal(t, before.Completed+1, afterEOF.Completed)
	assert.Equal(t, before.ReceivedBytes+8, afterEOF.ReceivedBytes)
	require.NoError(t, response.Body.Close())
	assert.Equal(t, afterEOF.Completed, trafficRowForTest(t, "outbound", "video").Completed)
}

func TestTransportClosesUnreadBodyAndRecordsHTTPError(t *testing.T) {
	before := trafficRowForTest(t, "outbound", "storage")
	transport := WrapTransport(observedRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("unread"))}, nil
	}), "storage")
	request, err := http.NewRequest(http.MethodGet, "https://provider.invalid/object", nil)
	require.NoError(t, err)
	response, err := transport.RoundTrip(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	after := trafficRowForTest(t, "outbound", "storage")
	assert.Equal(t, before.Requests+1, after.Requests)
	assert.Equal(t, before.Completed+1, after.Completed)
	assert.Equal(t, before.Errors+1, after.Errors)
	assert.Equal(t, before.Active, after.Active)
	assert.Equal(t, before.ReceivedBytes, after.ReceivedBytes)
}

type failingTrafficBody struct{}

func (failingTrafficBody) Read(buffer []byte) (int, error) {
	return copy(buffer, "ab"), io.ErrUnexpectedEOF
}
func (failingTrafficBody) Close() error { return nil }

func TestTransportRecordsPartialResponseAndReadFailure(t *testing.T) {
	before := trafficRowForTest(t, "outbound", "assets")
	transport := WrapTransport(observedRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: failingTrafficBody{}}, nil
	}), "assets")
	request, err := http.NewRequest(http.MethodGet, "https://provider.invalid/object", nil)
	require.NoError(t, err)
	response, err := transport.RoundTrip(request)
	require.NoError(t, err)
	_, err = io.ReadAll(response.Body)
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	require.NoError(t, response.Body.Close())
	after := trafficRowForTest(t, "outbound", "assets")
	assert.Equal(t, before.ReceivedBytes+2, after.ReceivedBytes)
	assert.Equal(t, before.Errors+1, after.Errors)
	assert.Equal(t, before.Completed+1, after.Completed)
	assert.Equal(t, before.Active, after.Active)
}

func TestTransportRecordsFailureAndContextCategory(t *testing.T) {
	before := trafficRowForTest(t, "outbound", "webhook")
	wantErr := errors.New("dial failed")
	transport := WrapTransport(observedRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, wantErr
	}), "")
	request, err := http.NewRequestWithContext(WithCategory(context.Background(), "webhook"), http.MethodPost, "https://provider.invalid/opaque", nil)
	require.NoError(t, err)
	_, err = transport.RoundTrip(request)
	require.ErrorIs(t, err, wantErr)
	after := trafficRowForTest(t, "outbound", "webhook")
	assert.Equal(t, before.Errors+1, after.Errors)
	assert.Equal(t, before.Completed+1, after.Completed)
	assert.Equal(t, before.Active, after.Active)
}

func TestTransportCountsReplayedRequestBodyAndRedirect(t *testing.T) {
	before := trafficRowForTest(t, "outbound", "relay")
	transport := WrapTransport(observedRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		_, err := io.Copy(io.Discard, request.Body)
		require.NoError(t, err)
		body, err := request.GetBody()
		require.NoError(t, err)
		_, err = io.Copy(io.Discard, body)
		require.NoError(t, err)
		require.NoError(t, body.Close())
		return &http.Response{StatusCode: 302, Body: http.NoBody}, nil
	}), "")
	request, err := http.NewRequest(http.MethodPost, "https://provider.invalid/v1/responses", strings.NewReader("abc"))
	require.NoError(t, err)
	_, err = transport.RoundTrip(request)
	require.NoError(t, err)
	after := trafficRowForTest(t, "outbound", "relay")
	assert.Equal(t, before.SentBytes+6, after.SentBytes)
	assert.Equal(t, before.Redirects+1, after.Redirects)
	assert.Equal(t, before.Requests+1, after.Requests)
	assert.Equal(t, before.Active, after.Active)
}

func TestTransportPreservesCallerRequestAndCountsOnlyObservedReads(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Minute))
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://provider.invalid/v1/responses?part=1", strings.NewReader("abc"))
	require.NoError(t, err)
	request.Header = http.Header{"Authorization": {"Bearer fixture"}, "X-Feature": {"first", "second"}}
	request.Trailer = http.Header{"X-Checksum": {"fixture"}}
	request.ContentLength = -1
	request.TransferEncoding = []string{"chunked"}
	originalBody := request.Body
	originalURL := request.URL
	originalHeaders := request.Header.Clone()
	originalTrailers := request.Trailer.Clone()
	deadline, _ := ctx.Deadline()
	before := trafficRowForTest(t, "outbound", "relay")
	transport := WrapTransport(observedRoundTripFunc(func(forwarded *http.Request) (*http.Response, error) {
		assert.NotSame(t, request, forwarded)
		assert.Same(t, originalURL, forwarded.URL)
		assert.Same(t, ctx, forwarded.Context())
		forwardedDeadline, ok := forwarded.Context().Deadline()
		require.True(t, ok)
		assert.Equal(t, deadline, forwardedDeadline)
		assert.Equal(t, originalHeaders, forwarded.Header)
		assert.Equal(t, originalTrailers, forwarded.Trailer)
		data, readErr := io.ReadAll(forwarded.Body)
		require.NoError(t, readErr)
		assert.Equal(t, "abc", string(data))
		replayed, replayErr := forwarded.GetBody()
		require.NoError(t, replayErr)
		data, readErr = io.ReadAll(replayed)
		require.NoError(t, readErr)
		require.NoError(t, replayed.Close())
		assert.Equal(t, "abc", string(data))
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("reply"))}, nil
	}), "")
	response, err := transport.RoundTrip(request)
	require.NoError(t, err)
	assert.Equal(t, originalBody, request.Body)
	assert.Same(t, originalURL, request.URL)
	assert.Equal(t, originalHeaders, request.Header)
	assert.Equal(t, originalTrailers, request.Trailer)
	assert.Equal(t, int64(-1), request.ContentLength)
	assert.Equal(t, []string{"chunked"}, request.TransferEncoding)
	// The caller's GetBody must still return an unobserved reader. A read through
	// it after forwarding must not add another request-body charge to the metric.
	callerBody, err := request.GetBody()
	require.NoError(t, err)
	data, err := io.ReadAll(callerBody)
	require.NoError(t, err)
	require.NoError(t, callerBody.Close())
	assert.Equal(t, "abc", string(data))
	buffer := make([]byte, 2)
	_, err = io.ReadFull(response.Body, buffer)
	require.NoError(t, err)
	during := trafficRowForTest(t, "outbound", "relay")
	assert.Equal(t, before.SentBytes+6, during.SentBytes)
	assert.Equal(t, before.ReceivedBytes+2, during.ReceivedBytes)
	assert.Equal(t, before.Active+1, during.Active)
	_, err = io.Copy(io.Discard, response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	after := trafficRowForTest(t, "outbound", "relay")
	assert.Equal(t, before.SentBytes+6, after.SentBytes)
	assert.Equal(t, before.ReceivedBytes+5, after.ReceivedBytes)
	assert.Equal(t, before.Completed+1, after.Completed)
	assert.Equal(t, before.Active, after.Active)
}

func TestTransportDisabledRetainsNativeTransportAndDialer(t *testing.T) {
	previous := Enabled()
	t.Cleanup(func() { SetEnabled(previous) })
	native := &http.Transport{}
	SetEnabled(true)
	wrapped := WrapTransport(native, "video")
	require.NotSame(t, native, wrapped)
	SetEnabled(false)
	assert.Same(t, native, WrapTransport(native, "video"))
	assert.Same(t, native, WrapTransport(wrapped, "relay"))
	assert.Same(t, UnwrapTransport(http.DefaultTransport), WrapTransport(nil, ""))
	dial := func(context.Context, string, string) (net.Conn, error) { return nil, errors.New("fixture dial") }
	native.DialContext = dial
	ObserveHTTPConnections(native)
	assert.Equal(t, reflect.ValueOf(dial).Pointer(), reflect.ValueOf(native.DialContext).Pointer())
}

func TestExistingTransportDisabledDelegatesOriginalRequestAndBody(t *testing.T) {
	previous := Enabled()
	t.Cleanup(func() { SetEnabled(previous) })
	SetEnabled(true)
	request, err := http.NewRequest(http.MethodPost, "https://provider.invalid/v1/responses", strings.NewReader("request"))
	require.NoError(t, err)
	before := trafficRowForTest(t, "outbound", "relay")
	rawBody := io.NopCloser(strings.NewReader("response"))
	transport := WrapTransport(observedRoundTripFunc(func(forwarded *http.Request) (*http.Response, error) {
		assert.Same(t, request, forwarded)
		_, readErr := io.Copy(io.Discard, forwarded.Body)
		require.NoError(t, readErr)
		return &http.Response{StatusCode: 200, Body: rawBody}, nil
	}), "")
	SetEnabled(false)
	response, err := transport.RoundTrip(request)
	require.NoError(t, err)
	assert.Equal(t, rawBody, response.Body)
	_, err = io.Copy(io.Discard, response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	assert.Equal(t, before, trafficRowForTest(t, "outbound", "relay"))
}

type duplexTrafficBody struct {
	reader  *strings.Reader
	written bytes.Buffer
}

func (body *duplexTrafficBody) Read(buffer []byte) (int, error)  { return body.reader.Read(buffer) }
func (body *duplexTrafficBody) Write(buffer []byte) (int, error) { return body.written.Write(buffer) }
func (*duplexTrafficBody) Close() error                          { return nil }

func TestTransportPreservesDuplexResponseAndCountsWritesOnce(t *testing.T) {
	before := trafficRowForTest(t, "outbound", "other")
	rawBody := &duplexTrafficBody{reader: strings.NewReader("read")}
	transport := WrapTransport(observedRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 101, Body: rawBody}, nil
	}), "other")
	request, err := http.NewRequest(http.MethodGet, "https://provider.invalid/duplex", nil)
	require.NoError(t, err)
	response, err := transport.RoundTrip(request)
	require.NoError(t, err)
	duplex, ok := response.Body.(io.ReadWriteCloser)
	require.True(t, ok)
	n, err := duplex.Write([]byte("write"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "write", rawBody.written.String())
	data, err := io.ReadAll(duplex)
	require.NoError(t, err)
	assert.Equal(t, "read", string(data))
	require.NoError(t, duplex.Close())
	after := trafficRowForTest(t, "outbound", "other")
	assert.Equal(t, before.SentBytes+5, after.SentBytes)
	assert.Equal(t, before.ReceivedBytes+4, after.ReceivedBytes)
	assert.Equal(t, before.Completed+1, after.Completed)
	assert.Equal(t, before.Active, after.Active)
}

type failingCloseTrafficBody struct{ err error }

func (failingCloseTrafficBody) Read([]byte) (int, error) { return 0, io.EOF }
func (body failingCloseTrafficBody) Close() error        { return body.err }

func TestTransportPreservesCloseFailureWithoutDuplicateCompletion(t *testing.T) {
	before := trafficRowForTest(t, "outbound", "storage")
	wantErr := errors.New("fixture close failed")
	transport := WrapTransport(observedRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: failingCloseTrafficBody{err: wantErr}}, nil
	}), "storage")
	request, err := http.NewRequest(http.MethodGet, "https://provider.invalid/object", nil)
	require.NoError(t, err)
	response, err := transport.RoundTrip(request)
	require.NoError(t, err)
	require.ErrorIs(t, response.Body.Close(), wantErr)
	require.ErrorIs(t, response.Body.Close(), wantErr)
	after := trafficRowForTest(t, "outbound", "storage")
	assert.Equal(t, before.Completed+1, after.Completed)
	assert.Equal(t, before.Errors+1, after.Errors)
	assert.Equal(t, before.ReceivedBytes, after.ReceivedBytes)
	assert.Equal(t, before.Active, after.Active)
}

type idleTrafficTransport struct{ closed bool }

func (*idleTrafficTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
}
func (transport *idleTrafficTransport) CloseIdleConnections() { transport.closed = true }

func TestTransportPreservesClientIdleConnectionCleanup(t *testing.T) {
	raw := &idleTrafficTransport{}
	client := &http.Client{Transport: WrapTransport(raw, "relay")}
	client.CloseIdleConnections()
	assert.True(t, raw.closed)
}
