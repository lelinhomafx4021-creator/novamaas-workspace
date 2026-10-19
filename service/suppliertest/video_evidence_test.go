package suppliertest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVideoEvidencePreservesActualNativeRequestAndTerminalResponse(t *testing.T) {
	requestJSON := `{"model":"doubao-seedance-2-0-260128","seed":0,"watermark":false,"unknown":9007199254740993,"content":[{"type":"image_url","role":"reference_image","image_url":{"url":"data:image/png;base64,aGVsbG8="}}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer private-generation-key", r.Header.Get("Authorization"))
		w.Header().Set("X-Request-Id", "upstream-request")
		if r.Method == http.MethodPost {
			raw, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			assert.Equal(t, requestJSON, string(raw))
			_, _ = io.WriteString(w, `{"id":"task-42","extra":false}`)
			return
		}
		_, _ = io.WriteString(w, `{"id":"task-42","status":"succeeded","content":{"video_url":"https://cdn.example/original.mp4?signature=private"},"usage":{"completion_tokens":0},"large":9007199254740993}`)
	}))
	defer server.Close()
	var metrics *VideoMetrics
	require.NoError(t, Run(context.Background(), server.Client(), RunRequest{BaseURL: server.URL, APIKey: "private-generation-key", Model: "model", Modules: []string{ModuleVideo}, Video: VideoConfig{RawPayload: requestJSON}}, func(event Event) {
		if event.Video != nil {
			value := *event.Video
			metrics = &value
		}
	}))
	require.NotNil(t, metrics)
	require.NotNil(t, metrics.Submit)
	require.NotNil(t, metrics.Final)
	assert.Equal(t, "task-42", metrics.TaskID)
	assert.Equal(t, 1, metrics.PollCount)
	assert.Contains(t, metrics.RawRequestJSON, "aGVsbG8=")
	assert.Contains(t, metrics.Submit.Request, "sha256=")
	assert.Contains(t, metrics.Submit.Request, "9007199254740993")
	assert.Contains(t, metrics.Submit.Request, `"watermark": false`)
	assert.NotContains(t, metrics.Submit.Headers, "private-generation-key")
	assert.Contains(t, metrics.Submit.Headers, "Authorization: [REDACTED]")
	assert.Equal(t, "upstream-request", metrics.Final.RequestID)
	assert.Empty(t, metrics.Final.Request)
	assert.Contains(t, metrics.Final.Response, "9007199254740993")
	assert.NotContains(t, metrics.Final.Response, "signature=private")
	assert.Contains(t, metrics.VideoURL, "signature=private") // Only the in-memory original URL goes to official verification.
}

func TestVideoCancellationRetainsFirstProgressWithoutInventingTerminalResponse(t *testing.T) {
	for _, state := range []string{"queued", "running"} {
		t.Run(state, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					_, _ = io.WriteString(w, `{"id":"pending"}`)
				} else {
					_, _ = io.WriteString(w, `{"id":"pending","status":"`+state+`"}`)
				}
			}))
			defer server.Close()
			var metrics *VideoMetrics
			require.NoError(t, Run(ctx, server.Client(), RunRequest{BaseURL: server.URL, Model: "model", Modules: []string{ModuleVideo}, Video: VideoConfig{Prompt: "animate"}}, func(event Event) {
				if event.Video != nil {
					metrics = event.Video
				}
				if event.Type == "progress" {
					cancel()
				}
			}))
			require.NotNil(t, metrics)
			assert.Nil(t, metrics.Final)
			assert.Equal(t, 1, metrics.PollCount)
			if state == "queued" {
				require.NotNil(t, metrics.Queued)
			} else {
				require.NotNil(t, metrics.Running)
			}
		})
	}
}

func TestQueryErrorCannotSupplyAnotherTasksVideo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":"another","status":"succeeded","content":{"video_url":"https://cdn.example/wrong.mp4"}}`)
	}))
	defer server.Close()
	exchange, _, err := GetVideoTask(context.Background(), server.Client(), server.URL, "", "", "expected")
	require.Error(t, err)
	assert.Contains(t, exchange.Error, "does not match")
}

func TestVideoInputValidationRejectsInvalidCountsAndDuration(t *testing.T) {
	for _, raw := range []string{`{"duration":3601}`, `{"duration":18446744073686646784}`, `{"content":[{"type":"image_url","role":"last_frame","image_url":{"url":"https://cdn.example/a.png"}}]}`} {
		require.Error(t, validateVideoInputs([]byte(raw)))
	}
	require.NoError(t, validateVideoInputs([]byte(`{"duration":0,"watermark":false}`)))
	assert.Contains(t, SafeVideoJSON(`{"authorization":"secret","nested":{"api_key":"secret"},"value":0}`, false), `"value": 0`)
	assert.NotContains(t, SafeVideoJSON(`{"authorization":"secret","nested":{"api_key":"secret"}}`, false), "secret")
	assert.Equal(t, "provider error", SafeVideoJSON("provider error", true))
}

func TestTruncatedVideoResponseIsRecordedAsIncomplete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "999")
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()
	exchange, _, err := GetVideoTask(context.Background(), server.Client(), server.URL, "", "", "task")
	require.Error(t, err)
	assert.Equal(t, 200, exchange.Status)
	assert.Empty(t, exchange.Response)
	assert.Contains(t, exchange.Error, "complete")
	assert.False(t, VideoTerminal(strings.ToLower("Running")))
}

func TestVideoFailuresKeepTheActualFailedStage(t *testing.T) {
	for _, stage := range []string{"submission", "generation"} {
		t.Run(stage, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if stage == "submission" {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = io.WriteString(w, `{"error":{"message":"unsupported model"},"unknown":false,"value":0}`)
					return
				}
				if r.Method == http.MethodPost {
					_, _ = io.WriteString(w, `{"id":"failed-task"}`)
					return
				}
				_, _ = io.WriteString(w, `{"id":"failed-task","status":"failed","error":{"message":"generation failed"},"unknown":false,"value":0}`)
			}))
			defer server.Close()
			var metrics *VideoMetrics
			require.NoError(t, Run(context.Background(), server.Client(), RunRequest{BaseURL: server.URL, Model: "model", Modules: []string{ModuleVideo}, Video: VideoConfig{Prompt: "animate"}}, func(event Event) {
				if event.Video != nil {
					metrics = event.Video
				}
			}))
			require.NotNil(t, metrics)
			require.NotNil(t, metrics.Submit)
			assert.Empty(t, metrics.VideoURL)
			if stage == "submission" {
				assert.Equal(t, http.StatusBadRequest, metrics.Submit.Status)
				assert.Contains(t, metrics.Submit.Response, `"unknown": false`)
				assert.Nil(t, metrics.Final)
				assert.Empty(t, metrics.TaskID)
			} else {
				require.NotNil(t, metrics.Final)
				assert.Equal(t, "failed-task", metrics.TaskID)
				assert.Equal(t, "failed", metrics.Status)
				assert.Contains(t, metrics.Final.Response, `"value": 0`)
			}
		})
	}
}
