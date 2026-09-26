package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLogsSelfStatSeparatesRequestsFromBillingRecords(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	now := time.Now().Unix()
	require.NoError(t, db.Create(&[]model.Log{
		{UserId: 7, Type: model.LogTypeConsume, Quota: 500000, CreatedAt: now},
		{UserId: 7, Type: model.LogTypeRefund, Quota: 100000, CreatedAt: now},
		{UserId: 8, Type: model.LogTypeConsume, Quota: 900000, CreatedAt: now},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 7)
	ctx.Set("role", common.RoleCommonUser)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/log/self/stat", nil)

	GetLogsSelfStat(ctx)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Quota    int64 `json:"quota"`
			Records  int64 `json:"records"`
			Requests int64 `json:"requests"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	assert.Equal(t, int64(500000), response.Data.Quota)
	assert.Equal(t, int64(2), response.Data.Records)
	assert.Equal(t, int64(1), response.Data.Requests)
}
