package common

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordVideoTaskUpstreamRequestBodyRejectsOversizedSnapshotBeforeSend(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	body := bytes.Repeat([]byte(" "), constant.MaxVideoTaskRequestBodyBytes+1)

	err := RecordVideoTaskUpstreamRequestBody(c, body)

	require.Error(t, err)
	var statusError interface{ HTTPStatusCode() int }
	require.ErrorAs(t, err, &statusError)
	assert.Equal(t, http.StatusRequestEntityTooLarge, statusError.HTTPStatusCode())
	assert.Empty(t, rootcommon.GetContextKeyString(c, constant.ContextKeyVideoTaskUpstreamRequestBody))
}

func TestExternalSnapshotRecorderAcceptsLargeBodyWithoutEmbeddingItInContext(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	body := append([]byte(`{"prompt":"`), bytes.Repeat([]byte("x"), constant.MaxVideoTaskRequestBodyBytes)...)
	body = append(body, []byte(`"}`)...)
	rootcommon.SetContextKey(c, constant.ContextKeyVideoTaskExternalSnapshots, true)
	var recorded []byte
	rootcommon.SetContextKey(c, constant.ContextKeyVideoTaskSnapshotRecorder, func(payload []byte) {
		recorded = payload
	})

	require.NoError(t, RecordVideoTaskUpstreamRequestBody(c, body))
	assert.Equal(t, body, recorded)
	assert.Empty(t, rootcommon.GetContextKeyString(c, constant.ContextKeyVideoTaskUpstreamRequestBody))
	size, exists := rootcommon.GetContextKeyType[int64](c, constant.ContextKeyVideoTaskUpstreamBodySize)
	assert.True(t, exists)
	assert.Equal(t, int64(len(body)), size)
}
