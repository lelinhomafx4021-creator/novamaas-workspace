package controller

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/asyncwebhook"
	"github.com/gin-gonic/gin"
)

func GetSystemWebhookSettings(c *gin.Context) {
	settings, err := asyncwebhook.GetSystemWebhookSettings()
	if err != nil {
		assetLibraryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": settings})
}

func SaveSystemWebhookSettings(c *gin.Context) {
	var input struct {
		AssetLibrary  *asyncwebhook.SystemWebhookTarget `json:"asset_library"`
		MediaTasks    *asyncwebhook.SystemWebhookTarget `json:"media_tasks"`
		ManualEnabled *bool                             `json:"manual_enabled"`
	}
	if err := common.DecodeJson(c.Request.Body, &input); err != nil {
		assetLibraryError(c, &asyncwebhook.RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("invalid system webhook settings")})
		return
	}
	if input.AssetLibrary == nil || input.MediaTasks == nil || input.ManualEnabled == nil {
		assetLibraryError(c, &asyncwebhook.RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("asset_library, media_tasks and manual_enabled settings are required")})
		return
	}
	settings, err := asyncwebhook.SaveSystemWebhookSettings(asyncwebhook.SystemWebhookSettings{AssetLibrary: *input.AssetLibrary, MediaTasks: *input.MediaTasks, ManualEnabled: *input.ManualEnabled})
	if err != nil {
		assetLibraryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": settings})
}

func TestSystemWebhook(c *gin.Context) {
	var input struct {
		URL string `json:"url"`
	}
	if err := common.DecodeJson(c.Request.Body, &input); err != nil {
		assetLibraryError(c, &asyncwebhook.RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("invalid webhook test request")})
		return
	}
	result, err := service.ProbeSystemWebhook(c.Request.Context(), c.Param("topic"), input.URL)
	if err != nil {
		assetLibraryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

func GetWebhookCapabilities(c *gin.Context) {
	capabilities, err := asyncwebhook.GetWebhookCapabilities()
	if err != nil {
		assetLibraryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": capabilities})
}

func DownloadWebhookManual(c *gin.Context) {
	capabilities, err := asyncwebhook.GetWebhookCapabilities()
	if err != nil {
		assetLibraryError(c, err)
		return
	}
	if !capabilities.ManualEnabled {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "webhook manual download is disabled by the administrator"})
		return
	}
	scope := service.WebhookManualScope(c.Query("scope"))
	if !scope.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "scope must be asset_library or media_tasks"})
		return
	}
	pdf, err := service.RenderWebhookManualPDF(c.Request.Context(), scope)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "webhook manual branding could not be rendered; check document branding settings"})
		return
	}
	c.Header("Cache-Control", "no-store")
	filename := "asset-library-webhook-api-manual.pdf"
	if scope == service.WebhookManualTasks {
		filename = "media-task-webhook-api-manual.pdf"
	}
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "application/pdf", pdf)
}
