package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRequestSnapshotPermanentLinksReturnOriginalAndUpstreamFiles(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Task{}, &model.TaskRequestBody{}))
	previous := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previous })
	original, upstream := []byte(`{"model":"client"}`), []byte(`{"model":"provider"}`)
	require.NoError(t, model.SaveTaskRequestSnapshots("task-files", "request-files", original, upstream))

	response := runTaskRequestBodyHandler(1, "task-files", GetTaskRequestSnapshots)
	require.Equal(t, http.StatusOK, response.Code)
	var payload struct {
		Data struct {
			OriginalURL string `json:"original_url"`
			UpstreamURL string `json:"upstream_url"`
		} `json:"data"`
	}
	require.NoError(t, common.DecodeJson(response.Body, &payload))
	assert.Equal(t, "/api/task/task-files/request-snapshots/original", payload.Data.OriginalURL)
	assert.Equal(t, "/api/task/task-files/request-snapshots/upstream", payload.Data.UpstreamURL)

	for _, test := range []struct {
		kind string
		body []byte
	}{{"original", original}, {"upstream", upstream}} {
		t.Run(test.kind, func(t *testing.T) {
			for attempt := 0; attempt < 2; attempt++ {
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequest(http.MethodGet, "/api/task/task-files/request-snapshots/"+test.kind, nil)
				c.Params = gin.Params{{Key: "task_id", Value: "task-files"}, {Key: "kind", Value: test.kind}}
				DownloadRequestSnapshot(c)
				require.Equal(t, http.StatusOK, recorder.Code)
				assert.JSONEq(t, string(test.body), recorder.Body.String())
				assert.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
				assert.Contains(t, recorder.Header().Get("Content-Disposition"), test.kind+"-request.json")
			}
		})
	}
}

func TestRequestSnapshotDownloadRejectsUnsupportedKind(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/task/task-files/request-snapshots/secret", nil)
	c.Params = gin.Params{{Key: "task_id", Value: "task-files"}, {Key: "kind", Value: "secret"}}
	DownloadRequestSnapshot(c)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestSnapshotMaintenanceRejectsUnboundedBatchSizes(t *testing.T) {
	for _, batch := range []int{0, -1, 101} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		body, err := common.Marshal(map[string]int{"batch_size": batch})
		require.NoError(t, err)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/storage/request-snapshots/migrate", bytes.NewReader(body))
		StartRequestSnapshotMigration(c)
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	}
}

func TestRequestSnapshotsReportPendingAcrossNodesUntilBothKindsArePublished(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Task{}, &model.TaskRequestBody{}))
	previous := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previous })
	require.NoError(t, db.Create(&model.Task{
		TaskID: "task-remote-node", RequestBodyAvailable: true,
		Properties: model.Properties{SnapshotKinds: []string{"original", "upstream"}},
	}).Error)

	for _, stage := range []struct {
		kind    string
		status  int
		pending bool
	}{
		{status: http.StatusAccepted, pending: true},
		{kind: "original", status: http.StatusAccepted, pending: true},
		{kind: "upstream", status: http.StatusOK},
	} {
		if stage.kind == "original" {
			require.NoError(t, model.SaveTaskRequestBody("task-remote-node", "", []byte(`{"model":"client"}`)))
		} else if stage.kind == "upstream" {
			require.NoError(t, model.SaveTaskRequestSnapshots("task-remote-node", "", []byte(`{"model":"client"}`), []byte(`{"model":"provider"}`)))
		}
		response := runTaskRequestBodyHandler(1, "task-remote-node", GetTaskRequestSnapshots)
		require.Equal(t, stage.status, response.Code)
		var payload struct {
			Success bool `json:"success"`
			Data    struct {
				Pending  bool            `json:"pending"`
				Original json.RawMessage `json:"original"`
				Upstream json.RawMessage `json:"upstream"`
			} `json:"data"`
		}
		require.NoError(t, common.DecodeJson(response.Body, &payload))
		assert.True(t, payload.Success)
		assert.Equal(t, stage.pending, payload.Data.Pending)
		if stage.kind != "" {
			assert.JSONEq(t, `{"model":"client"}`, string(payload.Data.Original))
		}
		if stage.kind == "upstream" {
			assert.JSONEq(t, `{"model":"provider"}`, string(payload.Data.Upstream))
		}
	}
}

func TestTaskAndLogDownloadLinksPreserveTheirRespectiveSourceBytes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Task{}, &model.TaskRequestBody{}))
	previous := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previous })
	canonical := []byte(`{"model":"video","prompt":"hello"}`)
	logSource := []byte(`{ "prompt": "hello", "model": "video" }`)
	require.NoError(t, db.Create(&model.Task{TaskID: "task-source", Properties: model.Properties{RequestID: "request-source"}}).Error)
	require.NoError(t, model.SaveTaskRequestBody("task-source", "request-source", canonical))
	require.NoError(t, model.SaveTaskRequestBody("", "request-source", logSource))
	require.NoError(t, model.BindTaskRequestSnapshots("task-source", "request-source"))

	for _, source := range []struct {
		url  string
		body []byte
	}{
		{snapshotDownloadURL("task-source", "", "original"), canonical},
		{snapshotDownloadURL("task-source", "request-source", "original"), logSource},
	} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, source.url, nil)
		c.Params = gin.Params{{Key: "kind", Value: "original"}}
		if c.Query("request_id") == "" {
			c.Params = append(c.Params, gin.Param{Key: "task_id", Value: "task-source"})
		}
		DownloadRequestSnapshot(c)
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, string(source.body), recorder.Body.String())
	}
}
