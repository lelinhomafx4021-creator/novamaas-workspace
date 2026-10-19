package suppliertest

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type verificationTransport func(*http.Request) (*http.Response, error)

func (transport verificationTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestOfficialVerificationUsesFixedOriginAndExistingSignerWithoutRetry(t *testing.T) {
	for _, action := range []string{"create", "query"} {
		t.Run(action, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: verificationTransport(func(request *http.Request) (*http.Response, error) {
				calls++
				assert.Equal(t, "ark.cn-beijing.volcengineapi.com", request.URL.Host)
				assert.Contains(t, request.Header.Get("Authorization"), "HMAC-SHA256 Credential=official-key/")
				body, err := io.ReadAll(request.Body)
				require.NoError(t, err)
				if action == "create" {
					assert.Equal(t, "CreateArkOfficialResultQuery", request.URL.Query().Get("Action"))
					assert.JSONEq(t, `{"Type":"content_url","ContentURL":"https://cdn.example/original.mp4"}`, string(body))
				} else {
					assert.Equal(t, "GetArkOfficialResult", request.URL.Query().Get("Action"))
					assert.JSONEq(t, `{"QueryID":"accepted-query"}`, string(body))
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"ResponseMetadata":{"RequestId":"official-request"},"Result":{"QueryID":"accepted-query","Status":"Success","IsOfficial":null,"unknown":9007199254740993}}`))}, nil
			})}
			input := VideoVerificationRequest{Action: action, AccessKey: "official-key", SecretKey: "official-secret"}
			if action == "create" {
				input.VideoURL = "https://cdn.example/original.mp4"
			} else {
				input.QueryID = "accepted-query"
			}
			exchange, err := VerifyVideo(context.Background(), client, input)
			require.NoError(t, err)
			assert.Equal(t, 1, calls)
			assert.Contains(t, exchange.Response, "9007199254740993")
			assert.Contains(t, exchange.Response, `"IsOfficial": null`)
			assert.Contains(t, exchange.Headers, "Authorization: [REDACTED]")
			assert.NotContains(t, exchange.Headers, "official-key")
			assert.NotContains(t, exchange.Headers, "official-secret")
			assert.Equal(t, "official-request", exchange.RequestID)
		})
	}
}

func TestOfficialCreationDoesNotRetryAnAmbiguousTransportFailure(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: verificationTransport(func(request *http.Request) (*http.Response, error) { calls++; return nil, io.ErrUnexpectedEOF })}
	exchange, err := VerifyVideo(context.Background(), client, VideoVerificationRequest{Action: "create", VideoURL: "https://cdn.example/original.mp4", AccessKey: "official-key", SecretKey: "secret"})
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	assert.NotEmpty(t, exchange.Error)
	assert.Contains(t, exchange.Request, "content_url")
	_, err = VerifyVideo(context.Background(), client, VideoVerificationRequest{Action: "create", QueryID: "already-accepted", VideoURL: "https://cdn.example/original.mp4", AccessKey: "key", SecretKey: "secret"})
	require.Error(t, err)
	assert.Equal(t, 1, calls)
}
