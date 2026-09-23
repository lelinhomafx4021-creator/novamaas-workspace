package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	assetService "github.com/QuantumNous/new-api/service/assetlibrary"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type assetGroupListResponse struct {
	Success bool                     `json:"success"`
	Data    []assetService.GroupView `json:"data"`
}

func requestAssetGroups(t *testing.T, userID int, role int) assetGroupListResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", userID)
	ctx.Set("role", role)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/asset-library/groups?scope=all", nil)

	ListAssetGroups(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response assetGroupListResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	return response
}

func TestListAssetGroupsEnforcesOwnerScopeUnlessRequesterIsAdmin(t *testing.T) {
	db := setupManageUserTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AssetGroup{}))
	alice := model.User{Username: "alice-assets", Password: "password", Status: common.UserStatusEnabled, Group: "default", AffCode: "asset-groups-alice"}
	bob := model.User{Username: "bob-assets", Password: "password", Status: common.UserStatusEnabled, Group: "default", AffCode: "asset-groups-bob"}
	require.NoError(t, db.Create(&alice).Error)
	require.NoError(t, db.Create(&bob).Error)
	require.NoError(t, db.Create(&[]model.AssetGroup{
		{PublicID: "group-alice", OwnerUserID: alice.Id, Name: "Campaign", Status: model.AssetStatusReady},
		{PublicID: "group-bob", OwnerUserID: bob.Id, Name: "Campaign", Status: model.AssetStatusReady},
	}).Error)

	memberResponse := requestAssetGroups(t, alice.Id, common.RoleCommonUser)
	require.Len(t, memberResponse.Data, 1)
	assert.Equal(t, "group-alice", memberResponse.Data[0].ID)

	adminResponse := requestAssetGroups(t, alice.Id, common.RoleAdminUser)
	require.Len(t, adminResponse.Data, 2)
	creators := make(map[string]string, len(adminResponse.Data))
	for _, group := range adminResponse.Data {
		creators[group.ID] = group.OwnerName
	}
	assert.Equal(t, map[string]string{
		"group-alice": "alice-assets",
		"group-bob":   "bob-assets",
	}, creators)
}
