package runtimewatch

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

// ObserveHTTPConnections adds only a live socket gauge. Payload bytes remain
// counted by WrapTransport, avoiding duplicate HTTP wire/body measurements.
func ObserveHTTPConnections(transport *http.Transport) {
	if !Enabled() {
		return
	}
	dial := transport.DialContext
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dial(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return WrapConnection("outbound", conn), nil
	}
}

// HTTPCategory keeps traffic labels bounded; URLs, providers and task IDs are
// never retained. An explicit context category handles opaque media/CDN paths.
func HTTPCategory(method, path string) string {
	switch {
	case strings.HasPrefix(path, "/v1/videos"), strings.HasPrefix(path, "/v1/video/"),
		strings.HasPrefix(path, "/videos/"), strings.HasPrefix(path, "/kling/"),
		strings.HasPrefix(path, "/api/v3/contents/generations/"), path == "/api/v3/contents/generations/tasks":
		return "video"
	case strings.HasPrefix(path, "/api/assets"), strings.HasPrefix(path, "/api/asset-library"), strings.HasPrefix(path, "/v1/assets"):
		return "assets"
	case strings.HasPrefix(path, "/api/storage"), strings.HasPrefix(path, "/api/relay-media"):
		return "storage"
	case strings.HasPrefix(path, "/v1/"), strings.HasPrefix(path, "/v1beta"), strings.HasPrefix(path, "/mj/"), strings.HasPrefix(path, "/suno/"):
		return "relay"
	case strings.HasPrefix(path, "/api/"):
		return "admin"
	default:
		return "other"
	}
}

func HTTPRequestCategory(request *http.Request) string {
	if request.URL == nil {
		return "other"
	}
	if request.Method == http.MethodPost && (request.URL.Path == "/" || request.URL.Path == "/api/v3/") {
		switch request.URL.Query().Get("Action") {
		case "CreateAssetGroup", "ListAssetGroups", "GetAssetGroup", "UpdateAssetGroup", "DeleteAssetGroup",
			"CreateAsset", "ListAssets", "GetAsset", "UpdateAsset", "DeleteAsset":
			return "assets"
		}
	}
	return HTTPCategory(request.Method, request.URL.Path)
}

type trafficRoundTripper struct {
	delegate http.RoundTripper
	category string
}

// WrapTransport counts streamed payload bytes, not headers, TLS or NIC traffic.
// Existing transport policies, proxies, pools and retry behavior stay delegated.
func WrapTransport(delegate http.RoundTripper, category string) http.RoundTripper {
	if delegate == nil {
		delegate = http.DefaultTransport
	}
	if !Enabled() {
		return UnwrapTransport(delegate)
	}
	if wrapped, ok := delegate.(*trafficRoundTripper); ok {
		if category == "" || category == wrapped.category {
			return wrapped
		}
		delegate = wrapped.delegate
	}
	return &trafficRoundTripper{delegate: delegate, category: category}
}

// UnwrapTransport lets callers clone the original HTTP transport configuration.
func UnwrapTransport(transport http.RoundTripper) http.RoundTripper {
	if wrapped, ok := transport.(*trafficRoundTripper); ok {
		return wrapped.delegate
	}
	return transport
}

func (transport *trafficRoundTripper) CloseIdleConnections() {
	if closer, ok := transport.delegate.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

func (transport *trafficRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if !Enabled() {
		return transport.delegate.RoundTrip(request)
	}
	category := transport.category
	if category == "" {
		category = Category(request.Context())
		if category == "other" && request.URL != nil {
			category = HTTPRequestCategory(request)
		}
	}
	complete := Begin("outbound", category)
	bytes := NewByteCounter("outbound", category)
	if (request.Body != nil && request.Body != http.NoBody) || request.GetBody != nil {
		// RoundTrippers may consume Body but must not mutate request metadata.
		// Copy only fields replaced by the observer; sharing immutable headers and
		// URL preserves the caller's request without their deep-copy allocations.
		observed := *request
		request = &observed
		if request.Body != nil && request.Body != http.NoBody {
			request.Body = &trafficRequestBody{ReadCloser: request.Body, bytes: bytes}
		}
		if getBody := request.GetBody; getBody != nil {
			request.GetBody = func() (io.ReadCloser, error) {
				body, err := getBody()
				if err != nil || body == nil || body == http.NoBody {
					return body, err
				}
				return &trafficRequestBody{ReadCloser: body, bytes: bytes}, nil
			}
		}
	}
	response, err := transport.delegate.RoundTrip(request)
	if err != nil {
		complete(-1)
		return response, err
	}
	if response.StatusCode >= 300 && response.StatusCode < 400 {
		AddRedirect("outbound", category)
	}
	if response.Body == nil || response.Body == http.NoBody {
		complete(response.StatusCode)
		return response, nil
	}
	body := &trafficResponseBody{ReadCloser: response.Body, bytes: bytes, status: response.StatusCode, complete: complete}
	if writer, ok := response.Body.(io.Writer); ok {
		response.Body = &trafficDuplexBody{trafficResponseBody: body, writer: writer}
	} else {
		response.Body = body
	}
	return response, nil
}

type trafficRequestBody struct {
	io.ReadCloser
	bytes ByteCounter
}

func (body *trafficRequestBody) Read(buffer []byte) (int, error) {
	n, err := body.ReadCloser.Read(buffer)
	body.bytes.Add(0, int64(n))
	return n, err
}

type trafficResponseBody struct {
	io.ReadCloser
	bytes    ByteCounter
	status   int
	complete func(int)
	once     sync.Once
}

func (body *trafficResponseBody) finish(status int) {
	body.once.Do(func() { body.complete(status) })
}

func (body *trafficResponseBody) Read(buffer []byte) (int, error) {
	n, err := body.ReadCloser.Read(buffer)
	body.bytes.Add(int64(n), 0)
	if err == io.EOF {
		body.finish(body.status)
	} else if err != nil {
		body.finish(-1)
	}
	return n, err
}

func (body *trafficResponseBody) Close() error {
	err := body.ReadCloser.Close()
	if err != nil {
		body.finish(-1)
	} else {
		body.finish(body.status)
	}
	return err
}

type trafficDuplexBody struct {
	*trafficResponseBody
	writer io.Writer
}

func (body *trafficDuplexBody) Write(buffer []byte) (int, error) {
	n, err := body.writer.Write(buffer)
	body.bytes.Add(0, int64(n))
	if err != nil {
		body.finish(-1)
	}
	return n, err
}
