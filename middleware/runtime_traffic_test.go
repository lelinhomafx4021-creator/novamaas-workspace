package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/pkg/runtimewatch"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runtimeTrafficRow(t *testing.T, category string) runtimewatch.TrafficRow {
	t.Helper()
	for _, row := range runtimewatch.Snapshot(1).Traffic {
		if row.Scope == "inbound" && row.Category == category {
			return row
		}
	}
	return runtimewatch.TrafficRow{Scope: "inbound", Category: category}
}

func TestRuntimeTrafficCountsConsumedBodyAndStreamingWrites(t *testing.T) {
	before := runtimeTrafficRow(t, "relay")
	engine := gin.New()
	engine.Use(RuntimeTraffic())
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		buffer := make([]byte, 3)
		_, err := io.ReadFull(c.Request.Body, buffer)
		require.NoError(t, err)
		_, err = c.Writer.WriteString("data: hello\n\n")
		require.NoError(t, err)
		c.Writer.Flush()
		_, err = c.Writer.(io.ReaderFrom).ReadFrom(strings.NewReader("data: bye\n\n"))
		require.NoError(t, err)
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("unread-tail"))
	request.ContentLength = 9000
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	assert.True(t, response.Flushed)
	assert.Equal(t, "data: hello\n\ndata: bye\n\n", response.Body.String())
	after := runtimeTrafficRow(t, "relay")
	assert.Equal(t, before.Requests+1, after.Requests)
	assert.Equal(t, before.Completed+1, after.Completed)
	assert.Equal(t, before.ReceivedBytes+3, after.ReceivedBytes)
	assert.Equal(t, before.SentBytes+uint64(response.Body.Len()), after.SentBytes)
	assert.Equal(t, before.Active, after.Active)
}

func TestRuntimeTrafficExcludesMonitorAndCountsRedirect(t *testing.T) {
	beforeAdmin := runtimeTrafficRow(t, "admin")
	beforeVideo := runtimeTrafficRow(t, "video")
	engine := gin.New()
	engine.Use(RuntimeTraffic())
	engine.GET("/api/system-info/monitor", func(c *gin.Context) { c.String(200, "monitor") })
	engine.GET("/v1/videos/task/content", func(c *gin.Context) { c.Redirect(302, "https://cdn.invalid/video.mp4") })
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/system-info/monitor", nil))
	assert.Equal(t, beforeAdmin, runtimeTrafficRow(t, "admin"))
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/videos/task/content", nil))
	assert.Equal(t, 302, response.Code)
	after := runtimeTrafficRow(t, "video")
	assert.Equal(t, beforeVideo.Redirects+1, after.Redirects)
	assert.Equal(t, beforeVideo.SentBytes+uint64(response.Body.Len()), after.SentBytes)
	assert.Equal(t, beforeVideo.Active, after.Active)
}

type runtimeReaderFromWriter struct{ gin.ResponseWriter }

func (writer *runtimeReaderFromWriter) ReadFrom(reader io.Reader) (int64, error) {
	return io.Copy(struct{ io.Writer }{writer.ResponseWriter}, reader)
}

func TestRuntimeTrafficCountsDelegatedReaderFromExactlyOnce(t *testing.T) {
	before := runtimeTrafficRow(t, "video")
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Writer = &runtimeReaderFromWriter{ResponseWriter: c.Writer}
		c.Next()
	})
	engine.Use(RuntimeTraffic())
	engine.GET("/v1/videos/task/content", func(c *gin.Context) {
		n, err := c.Writer.(io.ReaderFrom).ReadFrom(strings.NewReader("streamed video"))
		require.NoError(t, err)
		assert.EqualValues(t, len("streamed video"), n)
	})
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/videos/task/content", nil))
	assert.Equal(t, "streamed video", response.Body.String())
	after := runtimeTrafficRow(t, "video")
	assert.Equal(t, before.SentBytes+uint64(response.Body.Len()), after.SentBytes)
	assert.Equal(t, before.Completed+1, after.Completed)
	assert.Equal(t, before.Errors, after.Errors)
}

func TestDisabledRuntimeTrafficPreservesRequestAndWriter(t *testing.T) {
	previousEnabled := runtimewatch.Enabled()
	runtimewatch.SetEnabled(false)
	t.Cleanup(func() { runtimewatch.SetEnabled(previousEnabled) })
	before := runtimewatch.Snapshot(1).Traffic
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("original request"))
	request.Header.Set("X-Client-Header", "preserved")
	body := &struct{ io.ReadCloser }{request.Body}
	request.Body = body
	var originalWriter gin.ResponseWriter
	engine := gin.New()
	engine.Use(func(c *gin.Context) { originalWriter = c.Writer; c.Next() })
	engine.Use(RuntimeTraffic())
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		assert.Same(t, request, c.Request)
		assert.Same(t, body, c.Request.Body)
		assert.Same(t, originalWriter, c.Writer)
		assert.Equal(t, "preserved", c.Request.Header.Get("X-Client-Header"))
		data, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		assert.Equal(t, "original request", string(data))
		c.Header("X-Response-Header", "preserved")
		c.String(http.StatusCreated, "original response")
	})
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	assert.Equal(t, http.StatusCreated, response.Code)
	assert.Equal(t, "preserved", response.Header().Get("X-Response-Header"))
	assert.Equal(t, "original response", response.Body.String())
	assert.Equal(t, before, runtimewatch.Snapshot(1).Traffic)
}
