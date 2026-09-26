package taskcommon

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	storageService "github.com/QuantumNous/new-api/service/storage"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaterializeVideoTaskBase64LeavesStorageTimingEmptyWithoutInlineMedia(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	info := &relaycommon.RelayInfo{
		UserId:        1,
		RequestId:     "request-no-inline-media",
		TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "task_no_inline_media"},
	}
	body := []byte(`{"model":"video-model","content":[{"type":"text","text":"hello"}]}`)

	materialized, convertedCount, err := MaterializeVideoTaskBase64(
		c,
		body,
		info,
		model.StoragePolicyRelayMediaTemp,
		storageService.Base64StagingSourceDoubaoVideo,
	)

	require.NoError(t, err)
	assert.Equal(t, body, materialized)
	assert.Zero(t, convertedCount)
	_, recorded := common.GetContextKeyType[int64](c, constant.ContextKeyTemporaryMediaMilliseconds)
	assert.False(t, recorded)
}
