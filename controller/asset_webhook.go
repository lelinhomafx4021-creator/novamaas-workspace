package controller

import (
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	webhookService "github.com/QuantumNous/new-api/service/asyncwebhook"

	"github.com/gin-gonic/gin"
)

func ListAssetWebhookEndpoints(c *gin.Context) {
	endpoints, err := webhookService.ListWebhookEndpoints(c.GetInt("id"))
	if err != nil {
		assetLibraryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": endpoints})
}

func CreateAssetWebhookEndpoint(c *gin.Context) {
	var input webhookService.WebhookEndpointInput
	if err := common.DecodeJson(c.Request.Body, &input); err != nil {
		assetLibraryError(c, &webhookService.RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("invalid webhook endpoint request")})
		return
	}
	endpoint, err := webhookService.CreateWebhookEndpoint(c.GetInt("id"), input)
	if err != nil {
		assetLibraryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": endpoint})
}

func DeleteAssetWebhookEndpoint(c *gin.Context) {
	if err := webhookService.DeleteWebhookEndpoint(c.GetInt("id"), c.Param("id")); err != nil {
		assetLibraryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UpdateWebhookEndpoint(c *gin.Context) {
	var input webhookService.WebhookEndpointInput
	if err := common.DecodeJson(c.Request.Body, &input); err != nil {
		assetLibraryError(c, &webhookService.RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("invalid webhook endpoint request")})
		return
	}
	endpoint, err := webhookService.UpdateWebhookEndpoint(c.GetInt("id"), c.Param("id"), input)
	if err != nil {
		assetLibraryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": endpoint})
}

func TestAssetWebhookEndpoint(c *gin.Context) {
	result, err := service.ProbeUserWebhook(c.Request.Context(), c.GetInt("id"), c.Param("id"), strings.HasPrefix(c.FullPath(), "/api/asset-library/"))
	if err != nil {
		assetLibraryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}
