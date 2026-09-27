package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePlaygroundChatImageCapability(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	previous := common.OptionMap[model_setting.MiniAppPlaygroundMediaOptionKey]
	common.OptionMap[model_setting.MiniAppPlaygroundMediaOptionKey] = `{"image_input_models":["vision"]}`
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap[model_setting.MiniAppPlaygroundMediaOptionKey] = previous
		common.OptionMapRWMutex.Unlock()
	})

	cases := []struct {
		name   string
		model  string
		url    string
		status int
	}{
		{name: "configured image model", model: "vision", url: "data:image/jpeg;base64,/9j/", status: http.StatusOK},
		{name: "unconfigured image model", model: "text-only", url: "data:image/jpeg;base64,/9j/", status: http.StatusForbidden},
		{name: "remote image denied", model: "vision", url: "https://example.com/image.jpg", status: http.StatusBadRequest},
		{name: "oversized image denied", model: "vision", url: "data:image/jpeg;base64," + strings.Repeat("A", 1_398_104), status: http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, err := common.Marshal(gin.H{"model": tc.model, "messages": []any{gin.H{"role": "user", "content": []any{gin.H{"type": "image_url", "image_url": gin.H{"url": tc.url}}}}}})
			require.NoError(t, err)
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			context.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", strings.NewReader(string(body)))
			context.Request.Header.Set("Content-Type", "application/json")
			ValidatePlaygroundChat(context)
			assert.Equal(t, tc.status, response.Code)
			common.CleanupBodyStorage(context)
		})
	}
}
