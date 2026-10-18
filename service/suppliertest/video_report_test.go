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

func TestVideoReportCapturesSubmitAndFinalWithoutProgressBodies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer generation-private-key", r.Header.Get("Authorization"))
		if r.Method == http.MethodPost {
			_, err := io.WriteString(w, `{"id":"task-report","custom":"receipt"}`)
			require.NoError(t, err)
			return
		}
		_, err := io.WriteString(w, `{"status":"succeeded","content":{"video_url":"https://cdn.example/result.mp4"},"custom":"terminal"}`)
		require.NoError(t, err)
	}))
	defer server.Close()
	req := RunRequest{BaseURL: server.URL, APIKey: "generation-private-key", Model: "actual-model", Modules: []string{ModuleVideo}, Video: VideoConfig{RawPayload: `{"model":"actual-model","watermark":false,"seed":0,"content":[{"type":"text","text":"animate"}]}`}}
	report := RunVideoReport(context.Background(), server.Client(), server.Client(), req, "run-report", nil, nil)
	require.Len(t, report.Stages, 4)
	assert.Equal(t, "task-report", report.TaskID)
	assert.Contains(t, report.Stages[0].RequestBody, `"watermark":false`)
	assert.Contains(t, report.Stages[0].ResponseBody, `"custom":"receipt"`)
	assert.Contains(t, report.Stages[1].ResponseBody, `"custom":"terminal"`)
	assert.Empty(t, report.Stages[1].RequestBody)
	assert.Equal(t, "pass", report.Stages[1].Status)
	assert.Contains(t, report.Stages[0].RequestHeaders, "Content-Type: application/json")
	assert.Contains(t, report.Stages[0].RequestHeaders, "Authorization: [REDACTED]")
	assert.Contains(t, report.Stages[1].RequestHeaders, "Accept: application/json")
	assert.NotContains(t, report.RawEvidence["video_submit"].RequestHeaders, "generation-private-key")
}

func TestRequestHeaderEvidencePreservesDiagnosticFieldsAndNeverPersistsCredentials(t *testing.T) {
	request, err := http.NewRequest(http.MethodPost, "https://gateway.example/v3/tasks", strings.NewReader("request-body"))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer private-bearer")
	request.Header.Set("X-Api-Key", "private-api-key")
	request.Header.Set("Cookie", "session=private-cookie")
	request.Header.Set("X-Security-Token", "private-token")
	request.Header.Set("X-Date", "20261018T120000Z")
	request.Header.Set("Idempotency-Key", "task-attempt-one")
	request.Header.Add("X-Feature", "first")
	request.Header.Add("X-Feature", "second")
	request.Header.Set("Referer", "https://gateway.example/page?signature=private-signature")
	headers := ReportRequestHeaders(request)
	for _, field := range []string{"Host: gateway.example", "Content-Length: 12", "Content-Type: application/json", "X-Date: 20261018T120000Z", "Idempotency-Key: task-attempt-one", "X-Feature: first", "X-Feature: second", "Authorization: [REDACTED]"} {
		assert.Contains(t, headers, field)
	}
	for _, secret := range []string{"private-bearer", "private-api-key", "private-cookie", "private-token", "private-signature"} {
		assert.NotContains(t, headers, secret)
	}
	assert.Equal(t, "Bearer private-bearer", request.Header.Get("Authorization"))
}

func TestReportBodyRedactsBase64AndSignaturesWithoutChangingJSONNumbers(t *testing.T) {
	raw := `{"n":9007199254740993,"watermark":false,"url":"data:image/png;base64,aGVsbG8=","media":"https://cdn.example/a?X-Tos-Signature=secret&x=1"}`
	result := ReportBody(raw)
	assert.Contains(t, result, "9007199254740993")
	assert.Contains(t, result, `"watermark":false`)
	assert.Contains(t, result, "sha256")
	assert.NotContains(t, result, "aGVsbG8=")
	assert.NotContains(t, result, "Signature=secret")
}

func TestFinalVideoResponseWithoutURLCannotPass(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = io.WriteString(w, `{"id":"missing-url"}`)
			return
		}
		_, _ = io.WriteString(w, `{"status":"succeeded","extra":"preserved"}`)
	}))
	defer server.Close()
	req := RunRequest{BaseURL: server.URL, Model: "test", Modules: []string{ModuleVideo}, Video: VideoConfig{RawPayload: `{"content":[{"type":"text","text":"animate"}]}`}}
	report := RunVideoReport(context.Background(), server.Client(), server.Client(), req, "svr_missing", nil, nil)
	var result *StageEvidence
	for i := range report.Stages {
		if report.Stages[i].ID == "video_result" {
			result = &report.Stages[i]
		}
	}
	require.NotNil(t, result)
	assert.Equal(t, "fail", result.Status)
	assert.Equal(t, "failed", report.Status)
	assert.Contains(t, result.ResponseBody, "preserved")
}

func TestRepeatedVideoPollsKeepFirstQueuedAndRunningOncePlusFinal(t *testing.T) {
	responses := []string{
		`{"status":"queued","progress":0,"marker":"first-queued"}`,
		`{"status":"queued","progress":0,"marker":"repeated-queued"}`,
		`{"status":"running","progress":20,"marker":"first-running"}`,
		`{"status":"running","progress":50,"marker":"repeated-running"}`,
		`{"status":"succeeded","content":{"video_url":"https://cdn.example/final.mp4"},"usage":{"completion_tokens":0}}`,
	}
	index := 0
	report := &VideoRunReport{}
	transport := &videoEvidenceTransport{endpoint: "https://gateway.example/api/v3/contents/generations/tasks", report: report, publish: func(*VideoRunReport) {}, base: officialTestTransport(func(*http.Request) (*http.Response, error) {
		response := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(responses[index]))}
		index++
		return response, nil
	})}
	client := &http.Client{Transport: transport}
	for range responses {
		response, err := client.Get(transport.endpoint + "/task-one")
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
	}
	require.Len(t, report.Stages, 3)
	assert.Equal(t, "video_queued", report.Stages[0].ID)
	assert.Contains(t, report.Stages[0].ResponseBody, "first-queued")
	assert.Equal(t, "video_running", report.Stages[1].ID)
	assert.Contains(t, report.Stages[1].ResponseBody, "first-running")
	assert.True(t, report.Stages[2].Final)
	assert.NotContains(t, report.Stages[2].ResponseBody, "progress")
	assert.Equal(t, 5, report.VideoPolling.Count)
	assert.JSONEq(t, `{"completion_tokens":0}`, report.UsageJSON)
}

func TestCancellationKeepsQueuedSnapshotOnceAndDoesNotInventFinalResponse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := &http.Client{Transport: officialTestTransport(func(request *http.Request) (*http.Response, error) {
		body := `{"id":"cancelled-task"}`
		if request.Method == http.MethodGet {
			body = `{"id":"cancelled-task","status":"queued","marker":"queued-once"}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	req := RunRequest{BaseURL: "https://gateway.example", Model: "test", Modules: []string{ModuleVideo}, Video: VideoConfig{RawPayload: `{"content":[{"type":"text","text":"animate"}]}`}}
	report := RunVideoReport(ctx, client, client, req, "svr_cancelled", nil, func(event Event) {
		if event.Type == "progress" {
			cancel()
		}
	})
	assert.Equal(t, "cancelled", report.Status)
	assert.Equal(t, 1, report.VideoPolling.Count)
	var queued, result *StageEvidence
	for i := range report.Stages {
		switch report.Stages[i].ID {
		case "video_queued":
			queued = &report.Stages[i]
		case "video_result":
			result = &report.Stages[i]
		}
	}
	require.NotNil(t, queued)
	assert.Equal(t, "pass", queued.Status)
	assert.Contains(t, queued.ResponseBody, "queued-once")
	require.NotNil(t, result)
	assert.False(t, result.Final)
	assert.Empty(t, result.ResponseBody)
	assert.Contains(t, result.Conclusion, "cancelled")
}

func TestAnotherTaskResponseCannotSupplyThisRunsVideo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = io.WriteString(w, `{"id":"task-requested"}`)
			return
		}
		_, _ = io.WriteString(w, `{"id":"task-other","status":"succeeded","content":{"video_url":"https://cdn.example/wrong.mp4"}}`)
	}))
	defer server.Close()
	req := RunRequest{BaseURL: server.URL, Model: "test", Modules: []string{ModuleVideo}, Video: VideoConfig{RawPayload: `{"content":[{"type":"text","text":"animate"}]}`}}
	report := RunVideoReport(context.Background(), server.Client(), server.Client(), req, "svr_mismatch", nil, nil)
	assert.Equal(t, "task-requested", report.TaskID)
	assert.Empty(t, report.VideoURL)
	assert.Equal(t, "failed", report.Status)
}

func TestLegacyVideoFieldsPreserveExplicitZeroAndFalse(t *testing.T) {
	zero, disabled := 0, false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		raw, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		assert.Contains(t, string(raw), `"duration":0`)
		assert.Contains(t, string(raw), `"watermark":false`)
		assert.Contains(t, string(raw), `"seed":0`)
		_, _ = io.WriteString(w, `{"id":"legacy-zero"}`)
	}))
	defer server.Close()
	req := RunRequest{BaseURL: server.URL, Model: "test", Modules: []string{ModuleVideo}, Video: VideoConfig{Prompt: "animate", Checks: []string{CheckVideoSubmit}, Duration: &zero, Watermark: &disabled, Seed: &zero}}
	require.NoError(t, Run(context.Background(), server.Client(), req, nil))
}

func TestHTTPQueryErrorCannotBecomeSuccessfulVideoEvidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = io.WriteString(w, `{"id":"query-http-error"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"status":"succeeded","content":{"video_url":"https://cdn.example/misleading.mp4"},"error":"not found"}`)
	}))
	defer server.Close()
	req := RunRequest{BaseURL: server.URL, Model: "test", Modules: []string{ModuleVideo}, Video: VideoConfig{RawPayload: `{"content":[{"type":"text","text":"animate"}]}`}}
	report := RunVideoReport(context.Background(), server.Client(), server.Client(), req, "svr_http_error", nil, nil)
	assert.Equal(t, "failed", report.Status)
	assert.Empty(t, report.VideoURL)
	for _, stage := range report.Stages {
		if stage.ID == "video_result" {
			assert.Equal(t, "fail", stage.Status)
			assert.False(t, stage.Final)
			assert.Equal(t, http.StatusNotFound, stage.HTTPStatus)
			assert.Contains(t, stage.ResponseBody, "not found")
		}
	}
}
