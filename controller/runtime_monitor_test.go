package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/runtimewatch"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeMonitorReadsWithoutBusinessOrLogDatabase(t *testing.T) {
	previousDB, previousLogDB := model.DB, model.LOG_DB
	model.DB, model.LOG_DB = nil, nil
	t.Cleanup(func() { model.DB, model.LOG_DB = previousDB, previousLogDB })

	gin.SetMode(gin.TestMode)
	for _, query := range []struct {
		name    string
		query   string
		minutes int
	}{
		{name: "default hour", minutes: 60},
		{name: "one minute", query: "?minutes=1", minutes: 1},
		{name: "seven days capped by memory retention", query: "?minutes=10080", minutes: 1440},
	} {
		t.Run(query.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/system-info/monitor"+query.query, nil)
			require.NotPanics(t, func() { GetRuntimeMonitor(c) })
			require.Equal(t, http.StatusOK, recorder.Code)
			var response struct {
				Success bool                `json:"success"`
				Data    runtimewatch.Report `json:"data"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			assert.True(t, response.Success)
			assert.Equal(t, query.minutes, response.Data.Meta.RequestedMinutes)
			assert.Equal(t, runtimewatch.StorageStatus{Mode: "memory", CapacityPoints: 1440}, response.Data.Meta.Storage)
			assert.LessOrEqual(t, len(response.Data.History), 360)
			assert.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
		})
	}
}

func TestRuntimeMonitorRejectsInvalidOrUnboundedWindows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, query := range []string{"0", "-1", "10081", "999999999999999999999999", "1.5", "all"} {
		t.Run(query, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/system-info/monitor?minutes="+query, nil)
			GetRuntimeMonitor(c)
			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.NotContains(t, recorder.Body.String(), "hostname")
		})
	}
}
