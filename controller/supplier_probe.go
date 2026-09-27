package controller

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/suppliertest"

	"github.com/gin-gonic/gin"
)

func ListSupplierTestModels(c *gin.Context) {
	var req struct {
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request body"})
		return
	}
	if _, err := suppliertest.ModelsURL(req.BaseURL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	models, err := suppliertest.ListModels(c.Request.Context(), suppliertest.NewHTTPClient(service.GetHttpClient()), req.BaseURL, req.APIKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    models,
	})
}

func RunSupplierTest(c *gin.Context) {
	var req suppliertest.RunRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request body"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	flusher, _ := c.Writer.(http.Flusher)
	var writeMu sync.Mutex
	runCtx, cancelRun := context.WithCancel(c.Request.Context())
	defer cancelRun()
	writeFrame := func(frame string) bool {
		writeMu.Lock()
		defer writeMu.Unlock()
		if _, err := fmt.Fprint(c.Writer, frame); err != nil {
			cancelRun()
			return false
		}
		if flusher != nil {
			flusher.Flush()
		}
		return true
	}
	if !writeFrame(": connected\n\n") {
		return
	}
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				if !writeFrame(": keep-alive\n\n") {
					return
				}
			}
		}
	}()
	var stopHeartbeatOnce sync.Once
	stopHeartbeat := func() {
		stopHeartbeatOnce.Do(func() {
			cancelRun()
			<-heartbeatDone
		})
	}
	defer stopHeartbeat()

	emit := func(event suppliertest.Event) {
		payload, err := common.Marshal(event)
		if err != nil {
			return
		}
		writeFrame(fmt.Sprintf("data: %s\n\n", payload))
	}

	if err := suppliertest.NormalizeRunRequest(&req); err != nil {
		emit(suppliertest.Event{Type: "error", Message: err.Error()})
		stopHeartbeat()
		writeFrame("data: [DONE]\n\n")
		return
	}
	hasChatModule := false
	for _, m := range req.Modules {
		if m == suppliertest.ModuleBasic || m == suppliertest.ModuleStress || m == suppliertest.ModuleCache {
			hasChatModule = true
			break
		}
	}
	if hasChatModule {
		if _, err := suppliertest.ChatCompletionsURL(req.BaseURL); err != nil {
			emit(suppliertest.Event{Type: "error", Message: err.Error()})
			stopHeartbeat()
			writeFrame("data: [DONE]\n\n")
			return
		}
	}

	err := suppliertest.Run(runCtx, suppliertest.NewHTTPClient(service.GetHttpClient()), req, emit)
	if err != nil && runCtx.Err() == nil {
		emit(suppliertest.Event{Type: "error", Message: err.Error()})
	}
	stopHeartbeat()
	writeFrame("data: [DONE]\n\n")
}

func QuerySupplierTestVideoTask(c *gin.Context) {
	var req struct {
		BaseURL    string `json:"base_url"`
		APIKey     string `json:"api_key"`
		TaskID     string `json:"task_id"`
		CustomPath string `json:"custom_path"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request body"})
		return
	}
	if req.BaseURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "base_url is required"})
		return
	}
	if req.TaskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "task_id is required"})
		return
	}

	httpClient := suppliertest.NewHTTPClient(service.GetHttpClient())
	status, rawResp, err := suppliertest.GetVideoTaskRaw(c.Request.Context(), httpClient, req.BaseURL, req.CustomPath, req.APIKey, req.TaskID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success":      false,
			"message":      err.Error(),
			"raw_response": string(rawResp),
		})
		return
	}

	if status != http.StatusOK {
		errMsg := suppliertest.ExtractAPIError(rawResp, fmt.Sprintf("HTTP %d", status))
		c.JSON(http.StatusOK, gin.H{
			"success":      false,
			"status_code":  status,
			"task_id":      req.TaskID,
			"message":      errMsg,
			"raw_response": string(rawResp),
		})
		return
	}

	taskState, videoURL, failReason := suppliertest.ParseVideoTaskState(rawResp)
	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"status_code":  status,
		"task_id":      req.TaskID,
		"status":       taskState,
		"video_url":    videoURL,
		"fail_reason":  failReason,
		"raw_response": string(rawResp),
	})
}
