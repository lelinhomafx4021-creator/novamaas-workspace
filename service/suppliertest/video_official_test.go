package suppliertest

import (
	"context"
	"crypto/sha256"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

type officialTestTransport func(*http.Request) (*http.Response, error)

func (transport officialTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestOfficialResumeKeepsQueryIDAndParsesFalseWithoutInferringModelMismatch(t *testing.T) {
	credential := &OfficialCredentials{AccessKeyID: "test-resume-key", Secret: "test-secret"}
	key := sha256.Sum256([]byte(credential.AccessKeyID))
	officialLimiters.Store(key, rate.NewLimiter(rate.Inf, 1))
	t.Cleanup(func() { officialLimiters.Delete(key) })
	calls := 0
	client := &http.Client{Transport: officialTestTransport(func(request *http.Request) (*http.Response, error) {
		calls++
		assert.Equal(t, "GetArkOfficialResult", request.URL.Query().Get("Action"))
		assert.Contains(t, request.Header.Get("Authorization"), "HMAC-SHA256 Credential=test-resume-key/")
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"QueryID":"existing-query"}`, string(body))
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"ResponseMetadata":{"RequestId":"official-request"},"Result":{"Status":"Success","IsOfficial":"false","ModelName":"doubao-seedance-2-0-260128","ResourceType":"video","extra":9007199254740993}}`))}, nil
	})}
	report := &VideoRunReport{ID: "svr_resume", Model: "doubao-seedance-2-0", QueryID: "existing-query", Stages: []StageEvidence{}}
	VerifyVideoArtifact(context.Background(), client, report, credential, func(*VideoRunReport) {})
	require.Equal(t, 1, calls)
	require.NotNil(t, report.Verification.IsOfficial)
	assert.False(t, *report.Verification.IsOfficial)
	assert.Equal(t, "not_detected", report.Verification.Status)
	assert.Equal(t, "unknown", report.Verification.ModelMatch)
	require.Len(t, report.Stages, 1)
	assert.Contains(t, report.Stages[0].ResponseBody, "9007199254740993")
	assert.True(t, report.Stages[0].Final)
}

func TestVideoOfficialCreationAndResultPreserveBothExchanges(t *testing.T) {
	credential := &OfficialCredentials{AccessKeyID: "creation-key", Secret: "creation-secret"}
	key := sha256.Sum256([]byte(credential.AccessKeyID))
	officialLimiters.Store(key, rate.NewLimiter(rate.Inf, 1))
	t.Cleanup(func() { officialLimiters.Delete(key) })
	creates, queries := 0, 0
	client := &http.Client{Transport: officialTestTransport(func(request *http.Request) (*http.Response, error) {
		response := `{"ResponseMetadata":{"RequestId":"create-request"},"Result":{"QueryID":"created-query"}}`
		switch request.URL.Query().Get("Action") {
		case "CreateArkOfficialResultQuery":
			creates++
			raw, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			assert.JSONEq(t, `{"Type":"ContentURL","ContentURL":"https://cdn.example/original.mp4"}`, string(raw))
		case "GetArkOfficialResult":
			queries++
			response = `{"ResponseMetadata":{"RequestId":"result-request"},"Result":{"Status":"succeeded","IsOfficial":"true","ModelName":"doubao-seedance-2-0-260128","ResourceType":"video","Resolution":"1920x1080","Size":"42","unknown":"retained"}}`
		default:
			t.Fatalf("unexpected action: %s", request.URL.Query().Get("Action"))
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(response))}, nil
	})}
	report := &VideoRunReport{ID: "svr_created", TaskID: "video-task", Model: "doubao-seedance-2-0-260128", OriginalVideoURL: "https://cdn.example/original.mp4", Stages: []StageEvidence{}}
	VerifyVideoArtifact(context.Background(), client, report, credential, func(*VideoRunReport) {})
	require.Equal(t, 1, creates)
	require.Equal(t, 1, queries)
	assert.Equal(t, "created-query", report.QueryID)
	require.Len(t, report.Stages, 2)
	assert.Contains(t, report.Stages[0].RequestBody, "ContentURL")
	assert.Contains(t, report.Stages[0].ResponseBody, "created-query")
	assert.Contains(t, report.Stages[1].RequestBody, "created-query")
	assert.Contains(t, report.Stages[1].ResponseBody, "retained")
	assert.Equal(t, 1, report.VerificationPolling.Count)
	assert.Equal(t, "matched", report.Verification.ModelMatch)
	// Re-verifying an existing record performs only a result query.
	VerifyVideoArtifact(context.Background(), client, report, credential, func(*VideoRunReport) {})
	assert.Equal(t, 1, creates)
	assert.Equal(t, 2, queries)
}

func TestOfficialResultSeparatesOriginModelAndProtocolErrors(t *testing.T) {
	for _, fixture := range []struct {
		name, body, expectedMatch, expectedStatus string
		platform                                  bool
	}{
		{"platform alias has no mapping", `{"Result":{"Status":"Success","IsOfficial":"true","ModelName":"doubao-seedance-2-0-260128","ResourceType":"video"}}`, "unknown", "matched", true},
		{"known canonical mismatch", `{"Result":{"Status":"Success","IsOfficial":"true","ModelName":"doubao-seedance-1-5-pro-251215","ResourceType":"video"}}`, "mismatch", "matched", false},
		{"missing fields", `{"Result":{"Status":"Success"}}`, "unknown", "unknown", false},
		{"wrong query ID", `{"Result":{"QueryID":"another-query","Status":"Success","IsOfficial":"true"}}`, "unknown", "failed", false},
		{"HTTP 200 provider error", `{"ResponseMetadata":{"Error":{"Code":"AccessDenied","Message":"not enabled"}}}`, "unknown", "failed", false},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			credential := &OfficialCredentials{AccessKeyID: fixture.name, Secret: "fixture"}
			key := sha256.Sum256([]byte(credential.AccessKeyID))
			officialLimiters.Store(key, rate.NewLimiter(rate.Inf, 1))
			t.Cleanup(func() { officialLimiters.Delete(key) })
			client := &http.Client{Transport: officialTestTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(fixture.body))}, nil
			})}
			report := &VideoRunReport{Model: "doubao-seedance-2-0-260128", PlatformMode: fixture.platform, QueryID: "existing"}
			VerifyVideoArtifact(context.Background(), client, report, credential, func(*VideoRunReport) {})
			require.NotNil(t, report.Verification)
			assert.Equal(t, fixture.expectedMatch, report.Verification.ModelMatch)
			assert.Equal(t, fixture.expectedStatus, report.Verification.Status)
			assert.Equal(t, "existing", report.QueryID)
		})
	}
}

func TestMissingCredentialsDoNotEraseAcceptedOfficialEvidence(t *testing.T) {
	detected := true
	report := &VideoRunReport{QueryID: "accepted-query", Verification: &OfficialVerification{Status: "matched", IsOfficial: &detected, ModelMatch: "matched"}}
	report.SetStage(StageEvidence{ID: "verification_submit", Method: "POST", ResponseBody: `{"Result":{"QueryID":"accepted-query"}}`, Status: "pass", Final: true})
	report.SetStage(StageEvidence{ID: "verification_result", Method: "POST", ResponseBody: `{"Result":{"Status":"Success","IsOfficial":"true"}}`, Status: "pass", Final: true})
	VerifyVideoArtifact(context.Background(), nil, report, nil, func(*VideoRunReport) {})
	require.Len(t, report.Stages, 2)
	assert.Contains(t, report.Stages[0].ResponseBody, "accepted-query")
	assert.Contains(t, report.Stages[1].ResponseBody, "IsOfficial")
	assert.Equal(t, "matched", report.Verification.Status)
	assert.True(t, *report.Verification.IsOfficial)
}
