package suppliertest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestVideoURLs(t *testing.T) {
	tests := []struct {
		input          string
		customEndpoint string
		expected       string
		expectError    bool
	}{
		{
			input:    "https://ark.cn-beijing.volces.com",
			expected: "https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks",
		},
		{
			input:    "https://ark.cn-beijing.volces.com/api/v3",
			expected: "https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks",
		},
		{
			input:    "https://ark.cn-beijing.volces.com/api/v3/contents",
			expected: "https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks",
		},
		{
			input:    "https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks",
			expected: "https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks",
		},
		{
			input:    "https://api.myproxy.com/v1",
			expected: "https://api.myproxy.com/api/v3/contents/generations/tasks",
		},
		{
			// Vendor without /api/v3, isolating path to /contents
			input:    "https://custom-vendor.com/contents",
			expected: "https://custom-vendor.com/contents/generations/tasks",
		},
		{
			// Vendor without /api/v3, complete path in Base URL
			input:    "https://custom-vendor.com/contents/generations/tasks",
			expected: "https://custom-vendor.com/contents/generations/tasks",
		},
		{
			// Vendor with custom path override
			input:          "https://custom-vendor.com",
			customEndpoint: "/contents/generations/tasks",
			expected:       "https://custom-vendor.com/contents/generations/tasks",
		},
		{
			// Vendor with /v1 custom path override
			input:          "https://custom-vendor.com/v1",
			customEndpoint: "/contents/generations/tasks",
			expected:       "https://custom-vendor.com/v1/contents/generations/tasks",
		},
	}

	for _, tt := range tests {
		var got string
		var err error
		if tt.customEndpoint != "" {
			got, err = VideoTasksURL(tt.input, tt.customEndpoint)
		} else {
			got, err = VideoTasksURL(tt.input)
		}
		if tt.expectError {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		}
	}

	getURL, err := VideoTaskGetURL("https://ark.cn-beijing.volces.com", "cgt-123456")
	require.NoError(t, err)
	assert.Equal(t, "https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks/cgt-123456", getURL)

	customGetURL, err := VideoTaskGetURL("https://custom-vendor.com/contents/generations/tasks", "cgt-custom-999")
	require.NoError(t, err)
	assert.Equal(t, "https://custom-vendor.com/contents/generations/tasks/cgt-custom-999", customGetURL)
}

func TestRunVideo_PublicURL(t *testing.T) {
	pollCount := 0
	var receivedSubmitBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "contents/generations/tasks") {
			bodyBytes := make([]byte, 10240)
			n, _ := r.Body.Read(bodyBytes)
			receivedSubmitBody = string(bodyBytes[:n])
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":"cgt-test-video-task","status":"queued"}`))
			return
		}
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "cgt-test-video-task") {
			pollCount++
			w.WriteHeader(http.StatusOK)
			if pollCount == 1 {
				w.Write([]byte(`{"id":"cgt-test-video-task","status":"running"}`))
			} else {
				w.Write([]byte(`{"id":"cgt-test-video-task","status":"succeeded","content":{"video_url":"https://tos.volces.com/video/result.mp4"}}`))
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	res := "1080p"
	ratio := "16:9"
	duration := 5
	watermark := false

	req := RunRequest{
		BaseURL: server.URL,
		APIKey:  "test-api-key",
		Model:   "doubao-seedance-2-0-260128",
		Modules: []string{ModuleVideo},
		Video: VideoConfig{
			Prompt:     "一只在雨夜奔跑的猫",
			UploadMode: "url",
			ImageURL:   "https://example.com/test.jpg",
			Resolution: &res,
			Ratio:      &ratio,
			Duration:   &duration,
			Watermark:  &watermark,
		},
	}

	var events []Event
	var mu sync.Mutex
	emit := func(e Event) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, e)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err := Run(ctx, server.Client(), req, emit)
	require.NoError(t, err)

	// Verify request JSON structure
	require.NotEmpty(t, receivedSubmitBody)
	assert.Equal(t, "doubao-seedance-2-0-260128", gjson.Get(receivedSubmitBody, "model").String())
	assert.Equal(t, "1080p", gjson.Get(receivedSubmitBody, "resolution").String())
	assert.Equal(t, "16:9", gjson.Get(receivedSubmitBody, "ratio").String())
	assert.Equal(t, int64(5), gjson.Get(receivedSubmitBody, "duration").Int())
	assert.False(t, gjson.Get(receivedSubmitBody, "watermark").Bool())

	// Verify content array
	content := gjson.Get(receivedSubmitBody, "content").Array()
	require.Len(t, content, 2)
	assert.Equal(t, "text", content[0].Get("type").String())
	assert.Equal(t, "一只在雨夜奔跑的猫", content[0].Get("text").String())
	assert.Equal(t, "image_url", content[1].Get("type").String())
	assert.Equal(t, "https://example.com/test.jpg", content[1].Get("image_url.url").String())

	// Verify event lifecycle
	var submitEvent, resultEvent *Event
	for i := range events {
		e := &events[i]
		if e.CheckID == CheckVideoSubmit && e.Status == "pass" {
			submitEvent = e
		}
		if e.CheckID == CheckVideoResult && e.Status == "pass" {
			resultEvent = e
		}
	}
	require.NotNil(t, submitEvent)
	require.NotNil(t, resultEvent)
	require.NotNil(t, resultEvent.Video)
	assert.Equal(t, "https://tos.volces.com/video/result.mp4", resultEvent.Video.VideoURL)
	assert.Equal(t, "succeeded", resultEvent.Video.Status)
	assert.NotEmpty(t, resultEvent.Video.RawRequestJSON)
	assert.NotEmpty(t, resultEvent.Video.RawSubmitResponseJSON)
	assert.NotEmpty(t, resultEvent.Video.RawPollResponseJSON)
}

func TestRunVideo_Base64(t *testing.T) {
	var receivedSubmitBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "contents/generations/tasks") {
			bodyBytes := make([]byte, 10240)
			n, _ := r.Body.Read(bodyBytes)
			receivedSubmitBody = string(bodyBytes[:n])
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":"cgt-test-base64-task","status":"succeeded","content":{"video_url":"https://tos.volces.com/video/b64.mp4"}}`))
			return
		}
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":"cgt-test-base64-task","status":"succeeded","content":{"video_url":"https://tos.volces.com/video/b64.mp4"}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	testBase64 := "data:image/jpeg;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

	req := RunRequest{
		BaseURL: server.URL,
		APIKey:  "test-api-key",
		Model:   "doubao-seedance-2-5",
		Modules: []string{ModuleVideo},
		Video: VideoConfig{
			Prompt:     "测试 Base64 图片生成视频",
			UploadMode: "base64",
			Base64Data: testBase64,
		},
	}

	var events []Event
	emit := func(e Event) {
		events = append(events, e)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := Run(ctx, server.Client(), req, emit)
	require.NoError(t, err)

	require.NotEmpty(t, receivedSubmitBody)
	content := gjson.Get(receivedSubmitBody, "content").Array()
	require.Len(t, content, 2)
	assert.Equal(t, testBase64, content[1].Get("image_url.url").String())

	// Notice that resolution/ratio/duration are not in the payload because they were not specified
	assert.False(t, gjson.Get(receivedSubmitBody, "resolution").Exists())
	assert.False(t, gjson.Get(receivedSubmitBody, "ratio").Exists())
	assert.False(t, gjson.Get(receivedSubmitBody, "duration").Exists())
}

func TestRunVideo_FirstLastFrameAndAudio(t *testing.T) {
	var receivedSubmitBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "contents/generations/tasks") {
			bodyBytes := make([]byte, 10240)
			n, _ := r.Body.Read(bodyBytes)
			receivedSubmitBody = string(bodyBytes[:n])
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":"cgt-first-last-task","status":"succeeded","content":{"video_url":"https://tos.volces.com/video/dual.mp4","last_frame_url":"https://tos.volces.com/last.jpg"}}`))
			return
		}
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":"cgt-first-last-task","status":"succeeded","content":{"video_url":"https://tos.volces.com/video/dual.mp4","last_frame_url":"https://tos.volces.com/last.jpg"}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	role := "first_frame"
	audio := true
	returnLast := true

	req := RunRequest{
		BaseURL: server.URL,
		APIKey:  "test-api-key",
		Model:   "doubao-seedance-2-5",
		Modules: []string{ModuleVideo},
		Video: VideoConfig{
			Prompt:          "镜头从白天逐渐过渡到黑夜",
			UploadMode:      "url",
			ImageURL:        "https://example.com/start.png",
			Role:            &role,
			LastFrameMode:   "url",
			LastFrameURL:    "https://example.com/end.png",
			GenerateAudio:   &audio,
			ReturnLastFrame: &returnLast,
			CustomJSON:      `{"draft":false,"camera_movement":"pan_right"}`,
		},
	}

	var events []Event
	emit := func(e Event) {
		events = append(events, e)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := Run(ctx, server.Client(), req, emit)
	require.NoError(t, err)

	require.NotEmpty(t, receivedSubmitBody)
	content := gjson.Get(receivedSubmitBody, "content").Array()
	require.Len(t, content, 3) // text + first_frame + last_frame
	assert.Equal(t, "text", content[0].Get("type").String())
	assert.Equal(t, "first_frame", content[1].Get("role").String())
	assert.Equal(t, "https://example.com/start.png", content[1].Get("image_url.url").String())
	assert.Equal(t, "last_frame", content[2].Get("role").String())
	assert.Equal(t, "https://example.com/end.png", content[2].Get("image_url.url").String())

	assert.True(t, gjson.Get(receivedSubmitBody, "generate_audio").Bool())
	assert.True(t, gjson.Get(receivedSubmitBody, "return_last_frame").Bool())
	assert.False(t, gjson.Get(receivedSubmitBody, "draft").Bool())
	assert.Equal(t, "pan_right", gjson.Get(receivedSubmitBody, "camera_movement").String())
}

func TestRunVideo_CustomEndpointPath(t *testing.T) {
	var submitPath string
	var pollPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			submitPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":"cgt-custom-path-task","status":"succeeded","content":{"video_url":"https://example.com/custom.mp4"}}`))
			return
		}
		if r.Method == http.MethodGet {
			pollPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":"cgt-custom-path-task","status":"succeeded","content":{"video_url":"https://example.com/custom.mp4"}}`))
			return
		}
	}))
	defer server.Close()

	req := RunRequest{
		BaseURL: server.URL,
		APIKey:  "test-api-key",
		Model:   "doubao-seedance-2-0",
		Modules: []string{ModuleVideo},
		Video: VideoConfig{
			Prompt:     "测试自定义隔离路径",
			CustomPath: "/isolated/vendor/generations",
		},
	}

	var events []Event
	emit := func(e Event) {
		events = append(events, e)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := Run(ctx, server.Client(), req, emit)
	require.NoError(t, err)

	assert.Equal(t, "/isolated/vendor/generations", submitPath)
	assert.Equal(t, "/isolated/vendor/generations/cgt-custom-path-task", pollPath)
}
