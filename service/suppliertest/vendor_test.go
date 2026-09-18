package suppliertest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveVendor(t *testing.T) {
	t.Parallel()
	got, err := ResolveVendor("")
	require.NoError(t, err)
	assert.Equal(t, VendorGeneric, got)

	got, err = ResolveVendor("GLM")
	require.NoError(t, err)
	assert.Equal(t, VendorGLM, got)

	got, err = ResolveVendor("kimi")
	require.NoError(t, err)
	assert.Equal(t, VendorKimi, got)

	_, err = ResolveVendor("claude")
	require.Error(t, err)
}

func TestThinkingRequired(t *testing.T) {
	t.Parallel()
	assert.True(t, thinkingRequired(VendorGLM, "glm-5.3"))
	assert.False(t, thinkingRequired(VendorGLM, "glm-4-flash"))
	assert.True(t, thinkingRequired(VendorKimi, "kimi-k3"))
	assert.False(t, thinkingRequired(VendorGeneric, "glm-5.3"))
}

func TestApplyThinkingKimiK3OmitsThinkingObject(t *testing.T) {
	t.Parallel()
	got := applyThinking(chatRequest{Model: "kimi-k3"}, VendorKimi, "kimi-k3")
	assert.Nil(t, got.Thinking)
	assert.Equal(t, "low", got.ReasoningEffort)

	got = applyThinking(chatRequest{Model: "glm-5.3"}, VendorGLM, "glm-5.3")
	require.NotNil(t, got.Thinking)
	assert.Equal(t, "enabled", got.Thinking["type"])
}

func TestCacheMessagesSystemPrefix(t *testing.T) {
	t.Parallel()
	profile := profileFor(VendorGLM)
	warm := cacheMessages(profile, "long-prefix", "pong", true)
	require.Len(t, warm, 2)
	assert.Equal(t, "system", warm[0].Role)
	assert.Equal(t, "long-prefix", warm[0].Content)

	generic := cacheMessages(profileFor(VendorGeneric), "long-prefix", "pong", true)
	require.Len(t, generic, 1)
	assert.Equal(t, "user", generic[0].Role)
}

func TestJoinOpenAIPathV4(t *testing.T) {
	t.Parallel()
	got, err := ChatCompletionsURL("https://open.bigmodel.cn/api/paas/v4")
	require.NoError(t, err)
	assert.Equal(t, "https://open.bigmodel.cn/api/paas/v4/chat/completions", got)
}

func TestGLMThinkingMissingReasoningFails(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"id\":\"c1\",\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":2}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	var events []Event
	err := Run(context.Background(), server.Client(), RunRequest{
		BaseURL: server.URL,
		APIKey:  "any",
		Model:   "glm-5.3",
		Vendor:  VendorGLM,
		Modules: []string{ModuleBasic},
		Basic:   BasicConfig{Checks: []string{CheckThinking}},
	}, func(event Event) {
		events = append(events, event)
	})
	require.NoError(t, err)
	statusByCheck := map[string]string{}
	for _, event := range events {
		if event.Type == "check" && event.CheckID != "" {
			statusByCheck[event.CheckID] = event.Status
		}
	}
	assert.Equal(t, "fail", statusByCheck[CheckThinking])
}

func TestKimiK3ThinkingSendsReasoningEffort(t *testing.T) {
	t.Parallel()
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"step\",\"content\":\"323\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":4,\"cached_tokens\":0}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	var events []Event
	err := Run(context.Background(), server.Client(), RunRequest{
		BaseURL: server.URL,
		APIKey:  "any",
		Model:   "kimi-k3",
		Vendor:  VendorKimi,
		Modules: []string{ModuleBasic},
		Basic:   BasicConfig{Checks: []string{CheckThinking}},
	}, func(event Event) {
		events = append(events, event)
	})
	require.NoError(t, err)
	assert.Contains(t, body, `"reasoning_effort":"low"`)
	assert.NotContains(t, body, `"thinking"`)
	statusByCheck := map[string]string{}
	for _, event := range events {
		if event.Type == "check" && event.CheckID != "" {
			statusByCheck[event.CheckID] = event.Status
		}
	}
	assert.Equal(t, "pass", statusByCheck[CheckThinking])
}

func TestGLMCacheUsesSystemPrefix(t *testing.T) {
	t.Parallel()
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if body == "" {
			body = string(raw)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":20,\"completion_tokens\":1}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	err := Run(context.Background(), server.Client(), RunRequest{
		BaseURL: server.URL,
		APIKey:  "any",
		Model:   "glm-5.3",
		Vendor:  VendorGLM,
		Modules: []string{ModuleCache},
		Cache:   CacheConfig{Prompt: "glm-prefix", FollowUp: "pong", WaitSeconds: 0, MaxTokens: 8, Rounds: 1},
	}, func(Event) {})
	require.NoError(t, err)
	assert.Contains(t, body, `"role":"system"`)
	assert.Contains(t, body, "glm-prefix")
}
