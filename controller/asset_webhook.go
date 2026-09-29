package controller

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	assetService "github.com/QuantumNous/new-api/service/assetlibrary"

	"github.com/gin-gonic/gin"
)

func ListAssetWebhookEndpoints(c *gin.Context) {
	endpoints, err := assetService.ListWebhookEndpoints(c.GetInt("id"))
	if err != nil {
		assetLibraryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": endpoints})
}

func CreateAssetWebhookEndpoint(c *gin.Context) {
	var input assetService.WebhookEndpointInput
	if err := common.DecodeJson(c.Request.Body, &input); err != nil {
		assetLibraryError(c, &assetService.RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("invalid asset webhook endpoint request")})
		return
	}
	endpoint, err := assetService.CreateWebhookEndpoint(c.GetInt("id"), input)
	if err != nil {
		assetLibraryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": endpoint})
}

func UpdateAssetWebhookEndpoint(c *gin.Context) {
	var input assetService.WebhookEndpointInput
	if err := common.DecodeJson(c.Request.Body, &input); err != nil {
		assetLibraryError(c, &assetService.RequestError{StatusCode: http.StatusBadRequest, Err: errors.New("invalid asset webhook endpoint request")})
		return
	}
	endpoint, err := assetService.UpdateWebhookEndpoint(c.GetInt("id"), c.Param("id"), input)
	if err != nil {
		assetLibraryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": endpoint})
}

func DeleteAssetWebhookEndpoint(c *gin.Context) {
	if err := assetService.DeleteWebhookEndpoint(c.GetInt("id"), c.Param("id")); err != nil {
		assetLibraryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func TestAssetWebhookEndpoint(c *gin.Context) {
	eventID, err := assetService.QueueWebhookEndpointTest(c.GetInt("id"), c.Param("id"))
	if err != nil {
		assetLibraryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"event_id": eventID}})
}
