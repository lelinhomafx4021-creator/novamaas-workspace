package suppliertest

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatRequest struct {
	Model          string           `json:"model"`
	Messages       []chatMessage    `json:"messages,omitempty"`
	Stream         bool             `json:"stream"`
	MaxTokens      *int             `json:"max_tokens,omitempty"`
	Temperature    *float64         `json:"temperature,omitempty"`
	TopP           *float64         `json:"top_p,omitempty"`
	StreamOptions  *streamOptions   `json:"stream_options,omitempty"`
	Tools          []map[string]any `json:"tools,omitempty"`
	ResponseFormat map[string]any   `json:"response_format,omitempty"`
	Thinking       map[string]any   `json:"thinking,omitempty"`
}

type usageFields struct {
	PromptTokens         *float64 `json:"prompt_tokens"`
	CompletionTokens     *float64 `json:"completion_tokens"`
	CachedTokens         *float64 `json:"cached_tokens"`
	PromptCacheHitTokens *float64 `json:"prompt_cache_hit_tokens"`
	CacheReadInputTokens *float64 `json:"cache_read_input_tokens"`
	PromptTokensDetails  *struct {
		CachedTokens *float64 `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
}

type streamChunk struct {
	ID    string `json:"id"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error"`
	Usage   *usageFields `json:"usage"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Delta        struct {
			Content          string `json:"content"`
			Reasoning        string `json:"reasoning"`
			ReasoningContent string `json:"reasoning_content"`
			ToolCalls        []struct {
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		Message *struct {
			Content          string `json:"content"`
			Reasoning        string `json:"reasoning"`
			ReasoningContent string `json:"reasoning_content"`
			ToolCalls        []struct {
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

type StreamDelta struct {
	Content   string
	Reasoning string
}

type StreamResult struct {
	StatusCode       int
	Header           http.Header
	ID               string
	Content          string
	Reasoning        string
	FinishReason     string
	ToolName         string
	ToolArgs         string
	PromptTokens     int
	CompletionTokens int
	CachedTokens     int
	HasUsage         bool
	HasCachedTokens  bool
	TTFT             time.Duration
	TPOT             time.Duration
	Elapsed          time.Duration
	ErrorMessage     string
	SSE              bool
}

func NewHTTPClient(base *http.Client) *http.Client {
	if base == nil {
		return &http.Client{Timeout: 0}
	}
	clone := *base
	clone.Timeout = 0
	clone.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		return nil
	}
	return &clone
}

func ChatCompletionsURL(raw string) (string, error) {
	return joinOpenAIPath(raw, "chat/completions")
}

func ModelsURL(raw string) (string, error) {
	return joinOpenAIPath(raw, "models")
}

func parseSupplierBase(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("base URL is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("base URL must use http or https")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("base URL host is required")
	}
	parsed.Fragment = ""
	parsed.RawQuery = ""
	return parsed, nil
}

func joinOpenAIPath(raw, leaf string) (string, error) {
	parsed, err := parseSupplierBase(raw)
	if err != nil {
		return "", err
	}
	path := strings.TrimRight(parsed.Path, "/")
	for _, suffix := range []string{"/supplier-test", "/console", "/dashboard"} {
		path = strings.TrimSuffix(path, suffix)
	}
	path = strings.TrimRight(path, "/")
	switch {
	case strings.HasSuffix(path, "/"+leaf):
		parsed.Path = path
	case strings.HasSuffix(path, "/v1"):
		parsed.Path = path + "/" + leaf
	default:
		parsed.Path = path + "/v1/" + leaf
	}
	return parsed.String(), nil
}

func ListModels(ctx context.Context, httpClient *http.Client, baseURL, apiKey string) ([]string, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("http client is required")
	}
	endpoint, err := ModelsURL(baseURL)
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s", extractAPIError(raw, fmt.Sprintf("HTTP %d", resp.StatusCode)))
	}
	ids, err := parseModelIDs(raw)
	if err != nil {
		snippet := strings.ToLower(strings.TrimSpace(string(raw)))
		if strings.HasPrefix(snippet, "<!doctype") || strings.HasPrefix(snippet, "<html") {
			return nil, fmt.Errorf("base URL should be the API origin, not the web console")
		}
		return nil, err
	}
	return ids, nil
}

func parseModelIDs(raw []byte) ([]string, error) {
	var payload struct {
		Data   []any `json:"data"`
		Models []any `json:"models"`
	}
	if err := common.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("upstream model list is not JSON")
	}
	items := payload.Data
	if len(items) == 0 {
		items = payload.Models
	}
	ids := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		id := modelIDFromItem(item)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, nil
}

func modelIDFromItem(item any) string {
	switch value := item.(type) {
	case string:
		return strings.TrimSpace(value)
	case map[string]any:
		for _, key := range []string{"id", "model", "name"} {
			text, _ := value[key].(string)
			text = strings.TrimSpace(text)
			if text != "" {
				return text
			}
		}
	}
	return ""
}

func streamChat(
	ctx context.Context,
	httpClient *http.Client,
	endpoint string,
	apiKey string,
	req chatRequest,
	timeout time.Duration,
	onDelta func(StreamDelta),
) StreamResult {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body, err := common.Marshal(req)
	if err != nil {
		return StreamResult{ErrorMessage: err.Error()}
	}

	httpReq, err := http.NewRequestWithContext(callCtx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return StreamResult{ErrorMessage: err.Error()}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if strings.TrimSpace(apiKey) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	}

	started := time.Now()
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return StreamResult{ErrorMessage: err.Error(), Elapsed: time.Since(started)}
	}
	defer resp.Body.Close()

	result := StreamResult{
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Elapsed:    time.Since(started),
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		result.ErrorMessage = extractAPIError(raw, resp.Status)
		return result
	}

	if strings.Contains(contentType, "text/event-stream") || req.Stream {
		result.SSE = true
		parseSSE(resp.Body, started, &result, onDelta)
		result.Elapsed = time.Since(started)
		return result
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}
	applyChunk(raw, started, &result, onDelta)
	result.Elapsed = time.Since(started)
	return result
}

func parseSSE(body io.Reader, started time.Time, result *StreamResult, onDelta func(StreamDelta)) {
	reader := bufio.NewReaderSize(body, 1<<20)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "data:") {
				payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
				if payload == "[DONE]" {
					return
				}
				applyChunk([]byte(payload), started, result, onDelta)
			}
		}
		if err != nil {
			if err != io.EOF && result.ErrorMessage == "" {
				result.ErrorMessage = err.Error()
			}
			return
		}
	}
}

func applyChunk(raw []byte, started time.Time, result *StreamResult, onDelta func(StreamDelta)) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return
	}
	var chunk streamChunk
	if err := common.Unmarshal(raw, &chunk); err != nil {
		return
	}
	if chunk.ID != "" && result.ID == "" {
		result.ID = chunk.ID
	}
	if chunk.Error != nil && chunk.Error.Message != "" && result.ErrorMessage == "" {
		result.ErrorMessage = chunk.Error.Message
	}
	applyUsage(chunk.Usage, result)
	delta := StreamDelta{}
	for _, choice := range chunk.Choices {
		if choice.FinishReason != "" {
			result.FinishReason = choice.FinishReason
		}
		content := choice.Delta.Content
		reasoning := choice.Delta.Reasoning
		if reasoning == "" {
			reasoning = choice.Delta.ReasoningContent
		}
		if choice.Message != nil {
			if content == "" {
				content = choice.Message.Content
			}
			if reasoning == "" {
				reasoning = choice.Message.Reasoning
			}
			if reasoning == "" {
				reasoning = choice.Message.ReasoningContent
			}
			if result.ToolName == "" {
				for _, call := range choice.Message.ToolCalls {
					if call.Function.Name != "" {
						result.ToolName = call.Function.Name
						result.ToolArgs += call.Function.Arguments
					}
				}
			}
		}
		for _, call := range choice.Delta.ToolCalls {
			if call.Function.Name != "" {
				result.ToolName = call.Function.Name
			}
			result.ToolArgs += call.Function.Arguments
		}
		if content != "" {
			if result.Content == "" && result.Reasoning == "" {
				result.TTFT = time.Since(started)
			}
			result.Content += content
			delta.Content += content
		}
		if reasoning != "" {
			if result.Content == "" && result.Reasoning == "" {
				result.TTFT = time.Since(started)
			}
			result.Reasoning += reasoning
			delta.Reasoning += reasoning
		}
	}
	if (delta.Content != "" || delta.Reasoning != "") && onDelta != nil {
		onDelta(delta)
	}
	tokens := result.CompletionTokens
	if tokens <= 0 && result.Content != "" {
		tokens = max(1, len([]rune(result.Content))/2)
	}
	if tokens > 1 && result.TTFT > 0 {
		remain := time.Since(started) - result.TTFT
		if remain > 0 {
			result.TPOT = remain / time.Duration(tokens-1)
		}
	}
}

func applyUsage(usage *usageFields, result *StreamResult) {
	if usage == nil {
		return
	}
	result.HasUsage = true
	if usage.PromptTokens != nil {
		result.PromptTokens = int(*usage.PromptTokens)
	}
	if usage.CompletionTokens != nil {
		result.CompletionTokens = int(*usage.CompletionTokens)
	}
	cached := usage.CachedTokens
	if cached == nil && usage.PromptTokensDetails != nil {
		cached = usage.PromptTokensDetails.CachedTokens
	}
	if cached == nil {
		cached = usage.PromptCacheHitTokens
	}
	if cached == nil {
		cached = usage.CacheReadInputTokens
	}
	if cached != nil {
		result.CachedTokens = int(*cached)
		result.HasCachedTokens = true
	}
}

func extractAPIError(raw []byte, fallback string) string {
	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if err := common.Unmarshal(raw, &payload); err == nil {
		if payload.Error.Message != "" {
			return payload.Error.Message
		}
		if payload.Message != "" {
			return payload.Message
		}
	}
	text := strings.TrimSpace(string(raw))
	if text != "" {
		if len(text) > 300 {
			return text[:300]
		}
		return text
	}
	return fallback
}

func requestIDFrom(result StreamResult) string {
	if result.ID != "" {
		return result.ID
	}
	if result.Header == nil {
		return ""
	}
	keys := []string{
		"X-Request-Id",
		"X-Request-ID",
		"Request-Id",
		"Request-ID",
		"X-Dashscope-Request-Id",
		"Cf-Ray",
	}
	for _, key := range keys {
		if value := strings.TrimSpace(result.Header.Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func fillPrompt(tokenTarget int) string {
	return padPrompt("", tokenTarget)
}

func padPrompt(base string, tokenTarget int) string {
	base = strings.TrimSpace(base)
	if tokenTarget <= 0 {
		if base == "" {
			return DefaultCachePrefix
		}
		return base
	}
	if tokenTarget < 8 {
		tokenTarget = 8
	}
	need := tokenTarget * 5
	if len(base) >= need {
		return base
	}
	const block = "alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu nu xi omicron pi rho sigma tau "
	var builder strings.Builder
	builder.Grow(need + 8)
	if base != "" {
		builder.WriteString(base)
		builder.WriteString("\n\n")
	}
	for builder.Len() < need {
		builder.WriteString(block)
	}
	return builder.String()
}

func ptrInt(v int) *int { return &v }

func ptrBool(v bool) *bool { return &v }

func boolVal(v *bool, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return *v
}
