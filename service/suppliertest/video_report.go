package suppliertest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type StageEvidence struct {
	ID             string `json:"id"`
	Purpose        string `json:"purpose"`
	Status         string `json:"status"`
	Method         string `json:"method,omitempty"`
	Path           string `json:"path,omitempty"`
	Endpoint       string `json:"endpoint,omitempty"`
	RequestHeaders string `json:"request_headers,omitempty"`
	RequestBody    string `json:"request_body"`
	ResponseBody   string `json:"response_body"`
	HTTPStatus     int    `json:"http_status,omitempty"`
	RequestID      string `json:"request_id,omitempty"`
	ElapsedMS      int64  `json:"elapsed_ms"`
	Conclusion     string `json:"conclusion"`
	Final          bool   `json:"final"`
}

// ReportRequestHeaders captures configured outgoing headers without retaining
// authentication values, even in the private evidence attachment.
func ReportRequestHeaders(request *http.Request) string {
	headers := request.Header.Clone()
	if headers == nil {
		headers = make(http.Header)
	}
	headers.Set("Host", firstNonEmpty(request.Host, request.URL.Host))
	if request.Body != nil && request.ContentLength >= 0 {
		headers.Set("Content-Length", fmt.Sprint(request.ContentLength))
	}
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var result strings.Builder
	for _, key := range keys {
		normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", ""), "_", ""))
		sensitive := normalized == "key" || normalized == "xkey"
		for _, fragment := range []string{"authorization", "cookie", "token", "secret", "signature", "credential", "apikey", "accesskey"} {
			if strings.Contains(normalized, fragment) {
				sensitive = true
				break
			}
		}
		for _, value := range headers[key] {
			if sensitive {
				value = "[REDACTED]"
			} else {
				value = ReportURL(value)
			}
			fmt.Fprintf(&result, "%s: %s\n", key, value)
		}
	}
	return strings.TrimSuffix(result.String(), "\n")
}

type VideoAssetEvidence struct {
	Index   int    `json:"index"`
	Role    string `json:"role"`
	Source  string `json:"source"`
	URL     string `json:"url"`
	AssetID string `json:"asset_id,omitempty"`
	Name    string `json:"name,omitempty"`
	Status  string `json:"status,omitempty"`
	Size    int64  `json:"size,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
	Reused  bool   `json:"reused"`
}

type OfficialVerification struct {
	Status       string `json:"status"`
	IsOfficial   *bool  `json:"is_official,omitempty"`
	ModelName    string `json:"model_name,omitempty"`
	ModelMatch   string `json:"model_match"`
	ResourceType string `json:"resource_type,omitempty"`
	Resolution   string `json:"resolution,omitempty"`
	Size         string `json:"size,omitempty"`
	Message      string `json:"message,omitempty"`
}

type VideoPollingSummary struct {
	Count     int   `json:"count"`
	ElapsedMS int64 `json:"elapsed_ms"`
}

type VideoRunReport struct {
	VideoPolling        VideoPollingSummary      `json:"video_polling"`
	VerificationPolling VideoPollingSummary      `json:"verification_polling"`
	ExpectedModel       string                   `json:"expected_model,omitempty"`
	ID                  string                   `json:"id"`
	Model               string                   `json:"model"`
	BaseURL             string                   `json:"base_url"`
	PlatformMode        bool                     `json:"platform_mode"`
	TaskID              string                   `json:"task_id,omitempty"`
	QueryID             string                   `json:"query_id,omitempty"`
	Status              string                   `json:"status"`
	StartedAt           int64                    `json:"started_at"`
	CompletedAt         int64                    `json:"completed_at,omitempty"`
	Stages              []StageEvidence          `json:"stages"`
	Assets              []VideoAssetEvidence     `json:"assets"`
	Verification        *OfficialVerification    `json:"verification,omitempty"`
	VideoURL            string                   `json:"video_url,omitempty"`
	UsageJSON           string                   `json:"usage_json,omitempty"`
	EvidenceAvailable   bool                     `json:"evidence_available"`
	EvidenceError       string                   `json:"evidence_error,omitempty"`
	RawEvidence         map[string]StageEvidence `json:"-"`
	OriginalVideoURL    string                   `json:"-"`
}

// ReportBody redacts binary values and signed links using JSON paths rather
// than float64 decoding, so unknown fields and large integer tokens survive.
func ReportBody(raw string) string {
	if !gjson.Valid(raw) {
		return raw
	}
	result := raw
	var visit func(gjson.Result, string)
	visit = func(value gjson.Result, path string) {
		if value.IsObject() || value.IsArray() {
			value.ForEach(func(key, child gjson.Result) bool {
				part := key.String()
				if value.IsObject() {
					part = strings.ReplaceAll(strings.ReplaceAll(part, `\`, `\\`), `.`, `\.`)
				}
				childPath := part
				if path != "" {
					childPath = path + "." + part
				}
				lower := strings.ToLower(key.String())
				if value.IsObject() && (lower == "api_key" || lower == "authorization" || lower == "secret_access_key" || lower == "access_key_secret" || lower == "access_key_id" || lower == "accesskeyid" || lower == "password" || lower == "token") {
					result, _ = sjson.Set(result, childPath, "[REDACTED]")
				} else {
					visit(child, childPath)
				}
				return true
			})
			return
		}
		if value.Type != gjson.String {
			return
		}
		text := value.String()
		replacement := text
		if strings.HasPrefix(text, "data:") && strings.Contains(text, ";base64,") {
			parts := strings.SplitN(text, ",", 2)
			data, err := base64.StdEncoding.DecodeString(parts[1])
			if err != nil {
				replacement = "[invalid Base64]"
			} else {
				hash := sha256.Sum256(data)
				replacement = fmt.Sprintf("[%s; bytes=%d; sha256=%s]", strings.SplitN(parts[0], ";", 2)[0], len(data), hex.EncodeToString(hash[:]))
			}
		} else if parsed, err := url.Parse(text); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
			query := parsed.Query()
			changed := false
			if parsed.User != nil {
				parsed.User = url.User("[REDACTED]")
				changed = true
			}
			for key := range query {
				lower := strings.ToLower(key)
				if strings.Contains(lower, "signature") || strings.Contains(lower, "credential") || strings.Contains(lower, "token") || strings.Contains(lower, "accesskey") || strings.Contains(lower, "access_key") || lower == "sig" || lower == "authorization" || lower == "api_key" {
					query.Set(key, "[REDACTED]")
					changed = true
				}
			}
			if changed {
				parsed.RawQuery = query.Encode()
				replacement = parsed.String()
			}
		}
		if replacement != text {
			if path == "" {
				encoded, _ := common.Marshal(replacement)
				result = string(encoded)
			} else {
				result, _ = sjson.Set(result, path, replacement)
			}
		}
	}
	visit(gjson.Parse(raw), "")
	return result
}

func (report *VideoRunReport) SetStage(stage StageEvidence) {
	if report.RawEvidence == nil {
		report.RawEvidence = map[string]StageEvidence{}
	}
	report.RawEvidence[stage.ID] = stage
	if strings.Contains(stage.Path, "?") {
		if parsed, err := url.Parse(ReportURL("https://report.invalid" + stage.Path)); err == nil {
			stage.Path = parsed.RequestURI()
		}
	}
	stage.RequestBody = ReportBody(stage.RequestBody)
	stage.ResponseBody = ReportBody(stage.ResponseBody)
	stage.Endpoint = ReportURL(stage.Endpoint)
	for i := range report.Stages {
		if report.Stages[i].ID == stage.ID {
			report.Stages[i] = stage
			return
		}
	}
	report.Stages = append(report.Stages, stage)
	order := map[string]int{"video_submit": 20, "video_queued": 21, "video_running": 22, "video_result": 30, "verification_submit": 50, "verification_result": 60}
	sort.SliceStable(report.Stages, func(i, j int) bool { return order[report.Stages[i].ID] < order[report.Stages[j].ID] })
}

type videoEvidenceTransport struct {
	base     http.RoundTripper
	endpoint string
	report   *VideoRunReport
	publish  func(*VideoRunReport)
}

func (transport *videoEvidenceTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	submit := request.Method == http.MethodPost && request.URL.String() == transport.endpoint
	query := request.Method == http.MethodGet && strings.HasPrefix(request.URL.String(), strings.TrimRight(transport.endpoint, "/")+"/")
	if !submit && !query {
		return transport.base.RoundTrip(request)
	}
	stage := StageEvidence{ID: "video_result", Purpose: "Retrieve the final video result", Method: request.Method, Path: request.URL.RequestURI(), Endpoint: request.URL.String(), RequestHeaders: ReportRequestHeaders(request), Status: "fail"}
	if submit {
		stage.ID = "video_submit"
		stage.Purpose = "Submit the video task"
	} else if transport.report.TaskID == "" {
		transport.report.TaskID, _ = url.PathUnescape(request.URL.EscapedPath()[strings.LastIndex(request.URL.EscapedPath(), "/")+1:])
	}
	if request.Body != nil {
		raw, err := io.ReadAll(io.LimitReader(request.Body, 64<<20+1))
		if err != nil {
			return nil, err
		}
		if len(raw) > 64<<20 {
			return nil, fmt.Errorf("video request exceeds 64 MiB")
		}
		request.Body = io.NopCloser(bytes.NewReader(raw))
		stage.RequestBody = string(raw)
	}
	started := time.Now()
	if query {
		transport.report.VideoPolling.Count++
		defer func() { transport.report.VideoPolling.ElapsedMS += time.Since(started).Milliseconds() }()
	}
	response, err := transport.base.RoundTrip(request)
	stage.ElapsedMS = time.Since(started).Milliseconds()
	if err != nil {
		stage.Conclusion = "Video request failed before a response was received"
		transport.report.SetStage(stage)
		transport.publish(transport.report)
		return response, err
	}
	stage.HTTPStatus = response.StatusCode
	stage.RequestID = firstNonEmpty(response.Header.Get("X-Request-Id"), response.Header.Get("X-Tt-Logid"))
	raw, readErr := io.ReadAll(io.LimitReader(response.Body, 4<<20+1))
	stage.ElapsedMS = time.Since(started).Milliseconds()
	response.Body.Close()
	response.Body = io.NopCloser(bytes.NewReader(raw))
	if readErr != nil || len(raw) > 4<<20 {
		stage.Conclusion = "Response could not be captured completely"
		transport.report.SetStage(stage)
		transport.publish(transport.report)
		return nil, fmt.Errorf("video response exceeds capture limit or is incomplete")
	}
	stage.ResponseBody = string(raw)
	if stage.RequestID == "" {
		stage.RequestID = gjson.GetBytes(raw, "ResponseMetadata.RequestId").String()
	}
	if submit {
		if actual := gjson.Get(stage.RequestBody, "model").String(); actual != "" {
			transport.report.Model = actual
		}
		transport.report.TaskID = extractTaskID(raw)
		stage.Final = true
		stage.Conclusion = "Video task submission failed"
		if response.StatusCode >= 200 && response.StatusCode <= 202 && transport.report.TaskID != "" {
			stage.Status = "pass"
			stage.Conclusion = "Task accepted"
		}
	} else {
		if err := ValidateVideoTaskResponse(transport.report.TaskID, raw); err != nil {
			stage.Conclusion = err.Error()
			transport.report.SetStage(stage)
			transport.publish(transport.report)
			return response, nil
		}
		if response.StatusCode != http.StatusOK {
			stage.Conclusion = "Video result query failed"
			transport.report.SetStage(stage)
			transport.publish(transport.report)
			return response, nil
		}
		state, videoURL, _ := ParseVideoTaskState(raw)
		switch strings.ToLower(state) {
		case "succeeded", "success":
			stage.Final = true
			stage.Status = "pass"
			stage.Conclusion = "Video generated"
			transport.report.VideoURL = ReportURL(videoURL)
			if videoURL == "" {
				stage.Status = "fail"
				stage.Conclusion = "Task succeeded but no video URL returned"
			}
			transport.report.OriginalVideoURL = videoURL
			transport.report.UsageJSON = ReportBody(gjson.GetBytes(raw, "usage").Raw)
		case "failed", "failure", "cancelled", "expired":
			stage.Final = true
			stage.Conclusion = "Video task failed"
		default:
			if response.StatusCode == 200 {
				switch strings.ToLower(state) {
				case "queued", "pending":
					stage.ID = "video_queued"
					stage.Purpose = "First queued response"
				case "running", "processing", "in_progress":
					stage.ID = "video_running"
					stage.Purpose = "First running response"
				default:
					return response, nil
				}
				for _, existing := range transport.report.Stages {
					if existing.ID == stage.ID {
						return response, nil
					}
				}
				stage.Status = "pass"
				stage.Conclusion = stage.Purpose
				transport.report.SetStage(stage)
				transport.publish(transport.report)
				return response, nil
			}
			stage.Conclusion = "Video result query failed"
		}
	}
	transport.report.SetStage(stage)
	transport.publish(transport.report)
	return response, nil
}

func ValidateVideoTaskResponse(expectedID string, raw []byte) error {
	if returnedID := extractTaskID(raw); returnedID != "" && returnedID != expectedID {
		return fmt.Errorf("Returned task ID does not match this video run")
	}
	return nil
}

// RunVideoReport records submission, the first queued/running observations, and
// the final response. Repeated polling contributes only to one summary.
func RunVideoReport(ctx context.Context, client, officialClient *http.Client, req RunRequest, runID string, previous *VideoRunReport, emit Emitter) *VideoRunReport {
	report := &VideoRunReport{ID: runID, Model: req.Model, BaseURL: req.BaseURL, PlatformMode: req.Video.PlatformMode, Status: "running", StartedAt: time.Now().UnixMilli(), Stages: []StageEvidence{}, Assets: []VideoAssetEvidence{}}
	if previous != nil {
		report = previous
		report.Status = "running"
		report.CompletedAt = 0
	}
	publish := func(report *VideoRunReport) {
		if emit != nil {
			emit(Event{Type: "evidence", Module: ModuleVideo, VideoReport: report})
		}
	}
	prepared := map[string]bool{}
	for _, stage := range req.PreparedStages {
		if prepared[stage.ID] {
			continue
		}
		prepared[stage.ID] = true
		report.SetStage(stage)
	}
	publish(report)
	endpoint, err := VideoTasksURL(req.BaseURL, req.Video.CustomPath)
	if err != nil {
		report.SetStage(StageEvidence{ID: "video_submit", Purpose: "Submit the video task", Status: "fail", Conclusion: err.Error()})
		report.Status = "failed"
		report.MarkUnexecutedStages(req.Video.VerifyOfficial)
		publish(report)
		return report
	}
	if err = prepareVideoAssets(ctx, client, req, report, publish); err != nil {
		report.Status = "failed"
		report.CompletedAt = time.Now().UnixMilli()
		report.MarkUnexecutedStages(req.Video.VerifyOfficial)
		publish(report)
		return report
	}
	clone := *client
	base := clone.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	recorder := &videoEvidenceTransport{base: base, endpoint: endpoint, report: report, publish: publish}
	clone.Transport = recorder
	// A resumed official query does not submit or poll another generation task.
	if previous == nil || report.QueryID == "" && req.Video.ResumeRunID == "" {
		runErr := Run(ctx, &clone, req, func(event Event) {
			if event.Type == "check" && event.Status == "fail" {
				found := false
				for _, stage := range report.Stages {
					if stage.ID == event.CheckID || event.CheckID == CheckVideoPoll && stage.ID == "video_result" {
						found = true
					}
				}
				if !found {
					stage := StageEvidence{ID: event.CheckID, Purpose: event.Title, Status: "fail", Conclusion: event.Message}
					if event.CheckID == CheckVideoPoll {
						stage.ID = "video_result"
						stage.Purpose = "Retrieve the final video result"
					}
					report.SetStage(stage)
				}
			}
			if event.Type == "done" {
				return
			}
			if event.Video != nil {
				if event.Video.TaskID != "" {
					report.TaskID = event.Video.TaskID
				}
				metrics := *event.Video
				metrics.RawRequestJSON = ""
				metrics.RawSubmitResponseJSON = ""
				metrics.RawPollResponseJSON = ""
				event.Video = &metrics
			}
			if emit != nil {
				emit(event)
			}
		})
		if runErr != nil {
			report.SetStage(StageEvidence{ID: "video_submit", Purpose: "Submit the video task", Status: "fail", Conclusion: runErr.Error()})
		}
	}
	if req.Video.VerifyOfficial && report.VideoURL != "" && ctx.Err() == nil {
		VerifyVideoArtifact(ctx, officialClient, report, req.VerificationCredentials, publish)
	}
	report.Status = "completed"
	for _, stage := range report.Stages {
		if stage.Status == "fail" {
			report.Status = "failed"
		}
	}
	if ctx.Err() != nil {
		report.Status = "cancelled"
	}
	report.CompletedAt = time.Now().UnixMilli()
	report.MarkUnexecutedStages(req.Video.VerifyOfficial)
	publish(report)
	return report
}

// MarkUnexecutedStages keeps partial reports explicit without inventing HTTP
// exchanges for work that never ran.
func (report *VideoRunReport) MarkUnexecutedStages(verificationRequested bool) {
	for _, expected := range []StageEvidence{
		{ID: "video_submit", Purpose: "Submit the video task"},
		{ID: "video_result", Purpose: "Retrieve the final video result"},
		{ID: "verification_submit", Purpose: "Submit the original video for official verification"},
		{ID: "verification_result", Purpose: "Retrieve the final official verification result"},
	} {
		found := false
		for _, stage := range report.Stages {
			if stage.ID == expected.ID {
				found = true
				break
			}
		}
		if found {
			continue
		}
		expected.Status = "skip"
		expected.Conclusion = "This stage was not executed"
		if strings.HasPrefix(expected.ID, "verification_") && !verificationRequested {
			expected.Conclusion = "Official verification was not requested"
		}
		report.SetStage(expected)
	}
}

func ReportURL(value string) string {
	raw, _ := common.Marshal(value)
	var sanitized string
	if err := common.UnmarshalJsonStr(ReportBody(string(raw)), &sanitized); err != nil {
		return ""
	}
	return sanitized
}
