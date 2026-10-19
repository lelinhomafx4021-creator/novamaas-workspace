package suppliertest

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service/assetlibrary"
	"github.com/tidwall/gjson"
)

type VideoVerificationRequest struct {
	Action    string `json:"action"`
	VideoURL  string `json:"video_url"`
	QueryID   string `json:"query_id"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

// Each invocation performs one call. Creating an accepted or ambiguous query is
// never retried here; subsequent invocations must supply its QueryID.
func VerifyVideo(ctx context.Context, client *http.Client, input VideoVerificationRequest) (VideoExchange, error) {
	if input.AccessKey == "" || input.SecretKey == "" || len(input.AccessKey) > 256 || len(input.SecretKey) > 512 {
		return VideoExchange{}, fmt.Errorf("Official AK/SK is required")
	}
	action := "GetArkOfficialResult"
	payload := map[string]string{"QueryID": input.QueryID}
	switch input.Action {
	case "create":
		media, err := url.Parse(input.VideoURL)
		if err != nil || media.Host == "" || media.User != nil || (media.Scheme != "https" && media.Scheme != "http") || input.QueryID != "" {
			return VideoExchange{}, fmt.Errorf("An original video URL is required")
		}
		action = "CreateArkOfficialResultQuery"
		payload = map[string]string{"Type": "content_url", "ContentURL": input.VideoURL}
	case "query":
		if input.QueryID == "" || len(input.QueryID) > 256 {
			return VideoExchange{}, fmt.Errorf("QueryID is required")
		}
	default:
		return VideoExchange{}, fmt.Errorf("Invalid verification action")
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return VideoExchange{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://ark.cn-beijing.volcengineapi.com/?Action="+action+"&Version=2024-01-01", bytes.NewReader(body))
	if err != nil {
		return VideoExchange{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	assetlibrary.SignArkRequest(request, body, input.AccessKey, input.SecretKey, "cn-beijing", "ark", time.Now())
	clone := *client
	clone.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	exchange, raw, err := videoHTTP(&clone, request)
	// A transport error may follow an accepted creation. Return its evidence and
	// mark it ambiguous so the caller cannot silently create a second query.
	if err != nil {
		exchange.Error = "Official verification response is unavailable"
	}
	if exchange.Status != 0 && (exchange.Status != 200 || gjson.GetBytes(raw, "ResponseMetadata.Error").Exists()) {
		exchange.Error = "Official verification request failed"
	}
	exchange.Response = strings.ReplaceAll(strings.ReplaceAll(exchange.Response, input.AccessKey, "[REDACTED]"), input.SecretKey, "[REDACTED]")
	return exchange, nil
}
