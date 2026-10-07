package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommonUserConfiguresOwnWebhookWithDashboardSession(t *testing.T) {
	db := setupManageUserTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AssetWebhookEndpoint{}))
	require.NoError(t, db.Create(&model.AssetWebhookEndpoint{PublicID: model.SystemAssetWebhookEndpointID, OwnerUserID: 0, Status: model.AssetWebhookEndpointStatusEnabled, EventTypes: `["asset.failed"]`}).Error)
	previousSecret := common.SessionSecret
	common.SessionSecret = "asset-webhook-dashboard-test-secret"
	t.Cleanup(func() { common.SessionSecret = previousSecret })

	owner := model.User{Username: "webhook-owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "webhook-owner"}
	other := model.User{Username: "webhook-other", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "webhook-other"}
	require.NoError(t, db.Create(&owner).Error)
	require.NoError(t, db.Create(&other).Error)
	ownerSession, err := service.CreateLoginSession(owner.Id, "password", "127.0.0.1", "test")
	require.NoError(t, err)
	otherSession, err := service.CreateLoginSession(other.Id, "password", "127.0.0.1", "test")
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	webhookRoutes := router.Group("/api/asset-library/webhook-endpoints")
	webhookRoutes.Use(middleware.UserAuth())
	webhookRoutes.POST("", CreateAssetWebhookEndpoint)
	webhookRoutes.GET("", ListAssetWebhookEndpoints)
	webhookRoutes.PUT("/:id", UpdateWebhookEndpoint)
	webhookRoutes.POST("/:id/test", TestAssetWebhookEndpoint)
	webhookRoutes.DELETE("/:id", DeleteAssetWebhookEndpoint)

	createRequest := httptest.NewRequest(http.MethodPost, "/api/asset-library/webhook-endpoints", strings.NewReader(`{"name":"production","url":"https://customer.example.com/assets","event_types":["asset.failed"]}`))
	createRequest.Header.Set("Authorization", "Bearer "+ownerSession.AccessToken)
	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, createRequest)
	require.Equal(t, http.StatusOK, createRecorder.Code, createRecorder.Body.String())
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(createRecorder.Body.Bytes(), &created))
	require.NotEmpty(t, created.Data.ID)
	var endpoint model.AssetWebhookEndpoint
	require.NoError(t, db.Where("public_id = ?", created.Data.ID).First(&endpoint).Error)
	assert.Equal(t, owner.Id, endpoint.OwnerUserID)

	listRequest := httptest.NewRequest(http.MethodGet, "/api/asset-library/webhook-endpoints", nil)
	listRequest.Header.Set("Authorization", "Bearer "+otherSession.AccessToken)
	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, listRequest)
	require.Equal(t, http.StatusOK, listRecorder.Code)
	assert.NotContains(t, listRecorder.Body.String(), created.Data.ID)

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/asset-library/webhook-endpoints/"+created.Data.ID, nil)
	deleteRequest.Header.Set("Authorization", "Bearer "+otherSession.AccessToken)
	deleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRecorder, deleteRequest)
	assert.Equal(t, http.StatusNotFound, deleteRecorder.Code)

	unsignedRecorder := httptest.NewRecorder()
	router.ServeHTTP(unsignedRecorder, httptest.NewRequest(http.MethodPost, "/api/asset-library/webhook-endpoints", nil))
	assert.Equal(t, http.StatusUnauthorized, unsignedRecorder.Code)

	require.NoError(t, db.Model(&model.AssetWebhookEndpoint{}).Where("public_id = ?", model.SystemAssetWebhookEndpointID).Update("status", model.AssetWebhookEndpointStatusDisabled).Error)
	for _, route := range []struct{ method, suffix string }{{http.MethodPost, ""}, {http.MethodPut, "/" + created.Data.ID}, {http.MethodPost, "/" + created.Data.ID + "/test"}} {
		request := httptest.NewRequest(route.method, "/api/asset-library/webhook-endpoints"+route.suffix, strings.NewReader(`{"name":"production","url":"https://customer.example.com/assets","event_types":["asset.failed"]}`))
		request.Header.Set("Authorization", "Bearer "+ownerSession.AccessToken)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	}
}
