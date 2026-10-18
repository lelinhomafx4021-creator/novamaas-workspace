package suppliertest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service/assetlibrary"
	"github.com/tidwall/gjson"
	"golang.org/x/time/rate"
)

const officialEndpoint = "https://ark.cn-beijing.volcengineapi.com/"

type OfficialCredentials struct {
	AccessKeyID string
	Secret      string
}

var officialLimiters sync.Map

func callOfficial(ctx context.Context, client *http.Client, credential *OfficialCredentials, action string, payload any) StageEvidence {
	stage := StageEvidence{ID: "verification_result", Purpose: "Retrieve the final official verification result", Method: "POST", Path: "/?Action=" + action + "&Version=2024-01-01", Status: "fail"}
	if action == "CreateArkOfficialResultQuery" {
		stage.ID = "verification_submit"
		stage.Purpose = "Submit the original video for official verification"
	}
	body, err := common.Marshal(payload)
	if err != nil {
		stage.Conclusion = err.Error()
		return stage
	}
	stage.RequestBody = string(body)
	started := time.Now()
	key := sha256.Sum256([]byte(credential.AccessKeyID))
	limiter, _ := officialLimiters.LoadOrStore(key, rate.NewLimiter(rate.Every(time.Second), 1))
	if err = limiter.(*rate.Limiter).Wait(ctx); err != nil {
		stage.Conclusion = err.Error()
		return stage
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(officialEndpoint, "/")+stage.Path, bytes.NewReader(body))
	if err != nil {
		stage.Conclusion = err.Error()
		return stage
	}
	request.Header.Set("Content-Type", "application/json")
	stage.Endpoint = request.URL.String()
	assetlibrary.SignArkRequest(request, body, credential.AccessKeyID, credential.Secret, "cn-beijing", "ark", time.Now())
	stage.RequestHeaders = ReportRequestHeaders(request)
	// Official credentials must never follow a redirect to another origin.
	clone := *client
	clone.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := clone.Do(request)
	stage.ElapsedMS = time.Since(started).Milliseconds()
	if err != nil {
		stage.Conclusion = "Official verification request failed"
		return stage
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 4<<20+1))
	stage.ElapsedMS = time.Since(started).Milliseconds()
	stage.HTTPStatus = response.StatusCode
	if err != nil || len(raw) > 4<<20 {
		stage.Conclusion = "Official response could not be captured completely"
		return stage
	}
	stage.ResponseBody = string(raw)
	stage.RequestID = gjson.GetBytes(raw, "ResponseMetadata.RequestId").String()
	if response.StatusCode != 200 || gjson.GetBytes(raw, "ResponseMetadata.Error").Exists() {
		stage.Conclusion = ExtractAPIError(raw, "Official verification request failed")
		return stage
	}
	stage.Status = "pass"
	stage.Conclusion = "Official verification request completed"
	return stage
}

// VerifyVideoArtifact resumes an existing QueryID instead of creating another
// billable query. It always uses the unmodified original media.
func VerifyVideoArtifact(ctx context.Context, client *http.Client, report *VideoRunReport, credential *OfficialCredentials, publish func(*VideoRunReport)) {
	if credential == nil || credential.AccessKeyID == "" || credential.Secret == "" {
		if report.Verification == nil {
			report.Verification = &OfficialVerification{Status: "unverified", ModelMatch: "unknown"}
		}
		report.Verification.Message = "Configure official verification credentials before verification"
		attempted := report.QueryID != ""
		for _, stage := range report.Stages {
			if stage.ID == "verification_submit" && stage.Method != "" {
				attempted = true
			}
		}
		if !attempted {
			report.SetStage(StageEvidence{ID: "verification_submit", Purpose: "Submit the original video for official verification", Status: "skip", Conclusion: report.Verification.Message})
		}
		return
	}
	verifyCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if report.QueryID == "" {
		report.Verification = &OfficialVerification{Status: "unverified", ModelMatch: "unknown", Message: "Original video must be available before official verification"}
		// Once a creation attempt exists, an ambiguous submission must not be retried.
		for _, stage := range report.Stages {
			if stage.ID == "verification_submit" && stage.Method != "" {
				report.Verification = &OfficialVerification{Status: "unknown", ModelMatch: "unknown", Message: "Verification submission already attempted; check its receipt before creating another query"}
				return
			}
		}
		artifactURL := report.OriginalVideoURL
		if artifactURL == "" {
			artifactURL = report.VideoURL
		}
		parsed, err := url.Parse(artifactURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
			report.SetStage(StageEvidence{ID: "verification_submit", Purpose: "Submit the original video for official verification", Status: "skip", Conclusion: "Original video must use an absolute HTTP or HTTPS URL"})
			return
		}
		stage := callOfficial(verifyCtx, client, credential, "CreateArkOfficialResultQuery", map[string]string{"Type": "ContentURL", "ContentURL": artifactURL})
		report.QueryID = gjson.Get(stage.ResponseBody, "Result.QueryID").String()
		if stage.Status == "pass" && report.QueryID != "" {
			stage.Final = true
			stage.Conclusion = "Verification accepted"
		} else {
			stage.Status = "fail"
			stage.Conclusion = "Official verification was not accepted or returned no QueryID"
		}
		report.SetStage(stage)
		publish(report)
		if stage.Status != "pass" {
			report.Verification = &OfficialVerification{Status: "unknown", ModelMatch: "unknown", Message: stage.Conclusion}
			return
		}
	}
	for {
		stage := callOfficial(verifyCtx, client, credential, "GetArkOfficialResult", map[string]string{"QueryID": report.QueryID})
		report.VerificationPolling.Count++
		report.VerificationPolling.ElapsedMS += stage.ElapsedMS
		if returnedID := gjson.Get(stage.ResponseBody, "Result.QueryID").String(); returnedID != "" && returnedID != report.QueryID {
			stage.Status = "fail"
			stage.Conclusion = "Returned QueryID does not match this verification query"
			report.Verification = &OfficialVerification{Status: "failed", ModelMatch: "unknown", Message: stage.Conclusion}
			report.SetStage(stage)
			publish(report)
			return
		}
		state := strings.ToLower(gjson.Get(stage.ResponseBody, "Result.Status").String())
		if stage.Status == "pass" && (state == "success" || state == "succeeded") {
			stage.Final = true
			stage.Conclusion = "Official verification completed"
			result := gjson.Get(stage.ResponseBody, "Result")
			verification := &OfficialVerification{Status: "unknown", ModelMatch: "unknown", ModelName: result.Get("ModelName").String(), ResourceType: result.Get("ResourceType").String(), Resolution: result.Get("Resolution").String(), Size: result.Get("Size").String(), Message: result.Get("Message").String()}
			switch strings.ToLower(result.Get("IsOfficial").String()) {
			case "true":
				v := true
				verification.IsOfficial = &v
				verification.Status = "matched"
			case "false":
				v := false
				verification.IsOfficial = &v
				verification.Status = "not_detected"
			}
			expected := report.ExpectedModel
			if expected == "" && !report.PlatformMode && regexp.MustCompile(`^doubao-seedance-[a-z0-9-]+-[0-9]{6}$`).MatchString(report.Model) {
				expected = report.Model
			}
			if verification.IsOfficial != nil && *verification.IsOfficial && verification.ModelName != "" && expected != "" {
				report.ExpectedModel = expected
				if verification.ModelName == expected {
					verification.ModelMatch = "matched"
				} else {
					verification.ModelMatch = "mismatch"
				}
			}

			if verification.ResourceType != "" && !strings.EqualFold(verification.ResourceType, "video") {
				stage.Status = "fail"
				stage.Conclusion = "Official result resource type does not match video"
				verification.Status = "failed"
				verification.ModelMatch = "unknown"
				verification.Message = stage.Conclusion
			}
			if verification.ModelMatch == "mismatch" {
				stage.Status = "fail"
				stage.Conclusion = "Official model differs from the requested model"
			}
			report.Verification = verification
			report.SetStage(stage)
			publish(report)
			return
		}
		if state == "failed" || state == "failure" || state == "error" {
			stage.Status = "fail"
			stage.Final = true
			stage.Conclusion = "Official verification failed"
			report.SetStage(stage)
			report.Verification = &OfficialVerification{Status: "failed", ModelMatch: "unknown", Message: stage.Conclusion}
			publish(report)
			return
		}
		providerError := stage.HTTPStatus == http.StatusOK && gjson.Get(stage.ResponseBody, "ResponseMetadata.Error").Exists()
		clientError := stage.HTTPStatus >= 400 && stage.HTTPStatus < 500 && stage.HTTPStatus != 429 && stage.HTTPStatus != 408
		if stage.Status == "fail" && (providerError || clientError) {
			report.SetStage(stage)
			report.Verification = &OfficialVerification{Status: "failed", ModelMatch: "unknown", Message: stage.Conclusion}
			publish(report)
			return
		}
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-verifyCtx.Done():
			timer.Stop()
			stage.Status = "fail"
			stage.Final = false
			stage.Conclusion = "Official verification did not reach a final result before cancellation or timeout"
			report.SetStage(stage)
			report.Verification = &OfficialVerification{Status: "unknown", ModelMatch: "unknown", Message: stage.Conclusion}
			publish(report)
			return
		case <-timer.C:
		}
	}
}
