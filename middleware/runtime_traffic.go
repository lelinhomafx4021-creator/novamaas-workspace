package middleware

import (
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/pkg/runtimewatch"
	"github.com/gin-gonic/gin"
)

// RuntimeTraffic measures consumed request and written response payloads without
// buffering. Keep it outside compression so compressed response writes are seen.
func RuntimeTraffic() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !runtimewatch.Enabled() || c.Request.URL.Path == "/api/system-info/monitor" {
			c.Next()
			return
		}
		category := runtimewatch.HTTPRequestCategory(c.Request)
		complete := runtimewatch.Begin("inbound", category)
		counter := runtimewatch.NewByteCounter("inbound", category)
		writer := &runtimeTrafficWriter{ResponseWriter: c.Writer, counter: counter}
		c.Writer = writer
		if c.Request.Body != nil && c.Request.Body != http.NoBody {
			c.Request.Body = &runtimeTrafficBody{ReadCloser: c.Request.Body, counter: counter}
		}
		returned := false
		defer func() {
			status := writer.Status()
			if !returned || writer.failed {
				status = -1
			}
			if status >= 300 && status < 400 {
				runtimewatch.AddRedirect("inbound", category)
			}
			complete(status)
		}()
		c.Next()
		returned = true
	}
}

type runtimeTrafficBody struct {
	io.ReadCloser
	counter runtimewatch.ByteCounter
}

func (body *runtimeTrafficBody) Read(buffer []byte) (int, error) {
	n, err := body.ReadCloser.Read(buffer)
	body.counter.Add(int64(n), 0)
	return n, err
}

type runtimeTrafficWriter struct {
	gin.ResponseWriter
	counter runtimewatch.ByteCounter
	failed  bool
}

func (writer *runtimeTrafficWriter) Unwrap() http.ResponseWriter { return writer.ResponseWriter }

func (writer *runtimeTrafficWriter) Write(buffer []byte) (int, error) {
	n, err := writer.ResponseWriter.Write(buffer)
	writer.counter.Add(0, int64(n))
	writer.failed = writer.failed || err != nil
	return n, err
}

func (writer *runtimeTrafficWriter) WriteString(value string) (int, error) {
	n, err := writer.ResponseWriter.WriteString(value)
	writer.counter.Add(0, int64(n))
	writer.failed = writer.failed || err != nil
	return n, err
}

func (writer *runtimeTrafficWriter) ReadFrom(reader io.Reader) (int64, error) {
	if readerFrom, ok := writer.ResponseWriter.(io.ReaderFrom); ok {
		n, err := readerFrom.ReadFrom(reader)
		writer.counter.Add(0, n)
		writer.failed = writer.failed || err != nil
		return n, err
	}
	// Hide ReadFrom to avoid recursion while retaining a source WriterTo fast path.
	return io.Copy(struct{ io.Writer }{writer}, reader)
}
