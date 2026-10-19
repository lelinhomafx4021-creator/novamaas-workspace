package suppliertest

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// VideoExchange is one actual call, shared by generation and official verification.
type VideoExchange struct {
	Method    string `json:"method"`
	URL       string `json:"url"`
	Headers   string `json:"headers"`
	Request   string `json:"request"`
	Response  string `json:"response"`
	Status    int    `json:"status"`
	RequestID string `json:"request_id"`
	ElapsedMS int64  `json:"elapsed_ms"`
	Error     string `json:"error,omitempty"`
}

// SafeVideoJSON retains original JSON number tokens and unknown fields. Binary
// input is summarized for the report, retained only in the administrator's attachment.
func SafeVideoJSON(raw string, summarize bool) string {
	result := raw
	var visit func(gjson.Result, string)
	visit = func(value gjson.Result, path string) {
		if value.IsObject() || value.IsArray() {
			value.ForEach(func(key, child gjson.Result) bool {
				part := strings.ReplaceAll(strings.ReplaceAll(key.String(), `\`, `\\`), `.`, `\.`)
				if path != "" {
					part = path + "." + part
				}
				name := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(key.String()))
				if value.IsObject() && (strings.Contains(name, "secret") || strings.Contains(name, "authorization") || strings.Contains(name, "cookie") || name == "apikey" || name == "token" || name == "password" || name == "accesskeyid") {
					result, _ = sjson.Set(result, part, "[REDACTED]")
				} else {
					visit(child, part)
				}
				return true
			})
		} else if value.Type == gjson.String {
			text, replacement := value.String(), value.String()
			if summarize && strings.HasPrefix(text, "data:") && strings.Contains(text, ";base64,") {
				parts := strings.SplitN(text, ",", 2)
				data, err := base64.StdEncoding.DecodeString(parts[1])
				replacement = "[invalid Base64]"
				if err == nil {
					replacement = fmt.Sprintf("[%s; bytes=%d; sha256=%x]", strings.SplitN(parts[0], ";", 2)[0], len(data), sha256.Sum256(data))
				}
			} else if parsed, err := url.Parse(text); err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") {
				q, changed := parsed.Query(), false
				if parsed.User != nil {
					parsed.User = url.User("[REDACTED]")
					changed = true
				}
				for key := range q {
					name := strings.ToLower(key)
					if strings.Contains(name, "signature") || strings.Contains(name, "credential") || strings.Contains(name, "token") || strings.Contains(name, "accesskey") || name == "sig" || name == "api_key" {
						q.Set(key, "[REDACTED]")
						changed = true
					}
				}
				if changed {
					parsed.RawQuery = q.Encode()
					replacement = parsed.String()
				}
			}
			if replacement != text {
				if path == "" {
					body, _ := common.Marshal(replacement)
					result = string(body)
				} else {
					result, _ = sjson.Set(result, path, replacement)
				}
			}
		}
	}
	if !gjson.Valid(raw) {
		return raw
	}
	visit(gjson.Parse(raw), "")
	if formatted, err := common.IndentJson([]byte(result)); err == nil {
		return string(formatted)
	}
	return result
}

func videoHTTP(client *http.Client, request *http.Request) (exchange VideoExchange, raw []byte, err error) {
	exchange.Method, exchange.URL = request.Method, request.URL.String()
	headers := request.Header.Clone()
	for key := range headers {
		name := strings.ToLower(key)
		if strings.Contains(name, "authorization") || strings.Contains(name, "cookie") || strings.Contains(name, "key") || strings.Contains(name, "token") {
			headers.Set(key, "[REDACTED]")
		}
	}
	headers.Set("Host", request.URL.Host)
	if request.Body != nil && request.ContentLength >= 0 {
		headers.Set("Content-Length", fmt.Sprint(request.ContentLength))
	}
	var buffer bytes.Buffer
	_ = headers.Write(&buffer)
	exchange.Headers = strings.TrimSpace(buffer.String())
	if request.GetBody != nil {
		body, bodyErr := request.GetBody()
		if bodyErr != nil {
			return exchange, nil, bodyErr
		}
		payload, readErr := io.ReadAll(io.LimitReader(body, 64<<20+1))
		body.Close()
		if readErr != nil || len(payload) > 64<<20 {
			return exchange, nil, fmt.Errorf("video request exceeds capture limit")
		}
		exchange.Request = SafeVideoJSON(string(payload), true)
	}
	started := time.Now()
	defer func() {
		exchange.ElapsedMS = time.Since(started).Milliseconds()
		if err != nil {
			exchange.Error = "No complete response received"
		}
	}()
	response, err := client.Do(request)
	if err != nil {
		return exchange, nil, err
	}
	defer response.Body.Close()
	exchange.Status = response.StatusCode
	exchange.RequestID = firstNonEmpty(response.Header.Get("X-Request-Id"), response.Header.Get("X-Tt-Logid"))
	raw, err = io.ReadAll(io.LimitReader(response.Body, 4<<20+1))
	if err != nil || len(raw) > 4<<20 {
		return exchange, nil, fmt.Errorf("No complete response received")
	}
	exchange.Response = SafeVideoJSON(string(raw), true)
	if exchange.RequestID == "" {
		exchange.RequestID = gjson.GetBytes(raw, "ResponseMetadata.RequestId").String()
	}
	return exchange, raw, nil
}

func VideoTerminal(state string) bool {
	switch strings.ToLower(state) {
	case "succeeded", "success", "failed", "failure", "cancelled", "expired":
		return true
	}
	return false
}

func validateVideoInputs(raw []byte) error {
	duration := gjson.GetBytes(raw, "duration")
	if duration.Exists() && (duration.Type != gjson.Number || math.IsNaN(duration.Num) || math.IsInf(duration.Num, 0) || duration.Num < -1 || duration.Num > relaycommon.MaxTaskDurationSeconds || math.Trunc(duration.Num) != duration.Num) {
		return fmt.Errorf("Video duration is outside the supported limit")
	}
	counts := map[string]int{}
	for _, item := range gjson.GetBytes(raw, "content").Array() {
		if item.Get("type").String() != "image_url" {
			continue
		}
		if item.Get("image_url.url").String() == "" {
			return fmt.Errorf("Each material requires an image URL")
		}
		role := item.Get("role").String()
		if role == "" {
			role = "first_frame"
		}
		if role != "first_frame" && role != "last_frame" && role != "reference_image" {
			return fmt.Errorf("Unsupported image role")
		}
		counts[role]++
	}
	maximum := 30
	if strings.Contains(gjson.GetBytes(raw, "model").String(), "seedance-2-0") {
		maximum = 9
	}
	if counts["first_frame"] > 1 || counts["last_frame"] > 1 || counts["reference_image"] > maximum || counts["last_frame"] > 0 && counts["first_frame"] != 1 || counts["reference_image"] > 0 && counts["first_frame"]+counts["last_frame"] > 0 {
		return fmt.Errorf("Image roles or count do not match the generation scenario")
	}
	return nil
}
