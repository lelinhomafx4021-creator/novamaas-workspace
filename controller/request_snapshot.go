package controller

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	storageService "github.com/QuantumNous/new-api/service/storage"
	"github.com/gin-gonic/gin"
)

func GetRequestSnapshotStoragePolicy(c *gin.Context) {
	policy, err := storageService.GetRequestSnapshotPolicy()
	if err != nil {
		storageAPIError(c, http.StatusInternalServerError, err)
		return
	}
	common.ApiSuccess(c, policy)
}

func UpdateRequestSnapshotStoragePolicy(c *gin.Context) {
	var input storageService.PolicyInput
	if err := common.DecodeJson(c.Request.Body, &input); err != nil {
		storageAPIError(c, http.StatusBadRequest, errors.New("invalid snapshot policy request"))
		return
	}
	policy, err := storageService.SaveRequestSnapshotPolicy(input)
	if err != nil {
		storageAPIError(c, http.StatusBadRequest, err)
		return
	}
	recordManageAudit(c, "storage.snapshot_policy_update", map[string]interface{}{"profile_id": policy.StorageProfileID, "enabled": policy.Enabled, "prefix": policy.ObjectPrefix})
	common.ApiSuccess(c, policy)
}

func GetRequestSnapshotStatus(c *gin.Context) {
	status := storageService.SnapshotWorkerStatus()
	common.ApiSuccess(c, gin.H{
		"node_name": status.NodeName, "spool_directory": status.SpoolDirectory,
		"workers": status.Workers, "pending_files": status.PendingFiles,
		"pending_bytes": status.PendingBytes, "last_error": status.LastError,
		"migration": service.RequestSnapshotMaintenanceStatus(),
	})
}

func StartRequestSnapshotMigration(c *gin.Context) {
	startRequestSnapshotMaintenance(c, "migrate")
}

func StartRequestSnapshotDatabaseCleanup(c *gin.Context) {
	startRequestSnapshotMaintenance(c, "cleanup")
}

func startRequestSnapshotMaintenance(c *gin.Context, mode string) {
	var input service.RequestSnapshotMaintenanceInput
	if err := common.DecodeJson(c.Request.Body, &input); err != nil {
		storageAPIError(c, http.StatusBadRequest, errors.New("invalid snapshot maintenance request"))
		return
	}
	state, err := service.StartRequestSnapshotMaintenance(mode, input)
	if err != nil {
		storageAPIError(c, http.StatusBadRequest, err)
		return
	}
	recordManageAudit(c, "storage.snapshot_"+mode, map[string]interface{}{"batch_size": input.BatchSize, "clear_database_body": input.ClearDatabaseBody, "node_name": common.NodeName})
	common.ApiSuccess(c, state)
}

// This application URL has no expiry. Every access still requires AdminAuth;
// the provider's private object URL and credentials never reach the browser.
func DownloadRequestSnapshot(c *gin.Context) {
	kind := c.Param("kind")
	if kind != "original" && kind != "upstream" {
		storageAPIError(c, http.StatusBadRequest, errors.New("invalid snapshot kind"))
		return
	}
	taskID, requestID := c.Param("task_id"), c.Query("request_id")
	if taskID == "" {
		taskID = c.Query("task_id")
	}
	if taskID == "" && requestID == "" {
		storageAPIError(c, http.StatusBadRequest, errors.New("task id or request id is required"))
		return
	}
	body, exists, err := storageService.ReadArchivedRequestSnapshot(c.Request.Context(), taskID, requestID, kind)
	if err != nil {
		storageAPIError(c, http.StatusBadGateway, err)
		return
	}
	if !exists && kind == "original" && taskID != "" {
		task, found, taskErr := model.GetByTaskIdForAdmin(taskID)
		if taskErr != nil {
			storageAPIError(c, http.StatusInternalServerError, taskErr)
			return
		}
		if found && task != nil && len(task.Properties.RequestBody) > 0 {
			body, exists = task.Properties.RequestBody, true
		}
	}
	if !exists || len(body) == 0 {
		storageAPIError(c, http.StatusNotFound, errors.New("request snapshot not found"))
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", "attachment; filename="+strconv.Quote(kind+"-request.json"))
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

func snapshotDownloadURL(taskID, requestID, kind string) string {
	if requestID != "" {
		query := url.Values{"request_id": {requestID}}
		if taskID != "" {
			query.Set("task_id", taskID)
		}
		return "/api/log/request-snapshots/" + kind + "?" + query.Encode()
	}
	return "/api/task/" + url.PathEscape(taskID) + "/request-snapshots/" + kind
}
