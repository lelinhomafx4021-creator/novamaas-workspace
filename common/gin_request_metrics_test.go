package common

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestBodyRecordsReadMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("POST", "/v1/videos", strings.NewReader(`{"model":"video-model"}`))

	storage, err := GetBodyStorage(context)
	require.NoError(t, err)
	t.Cleanup(func() { _ = storage.Close() })

	bodyBytes, ok := GetContextKeyType[int64](context, constant.ContextKeyRequestBodyBytes)
	require.True(t, ok)
	assert.Equal(t, storage.Size(), bodyBytes)
	readMilliseconds, ok := GetContextKeyType[int64](context, constant.ContextKeyRequestBodyReadMilliseconds)
	require.True(t, ok)
	assert.GreaterOrEqual(t, readMilliseconds, int64(0))
}
