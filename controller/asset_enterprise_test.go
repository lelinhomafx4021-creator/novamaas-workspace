package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	assetService "github.com/QuantumNous/new-api/service/assetlibrary"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Exercise the public signed handler rather than injecting an authenticated user.
func requestSignedAssetAction(t *testing.T, key *assetService.CreatedAccessKeyView, action, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "https://gateway.example/api/v3/?Action="+action+"&Version=2024-01-01", strings.NewReader(body))
	now := time.Now().UTC()
	date, stamp := now.Format("20060102"), now.Format("20060102T150405Z")
	hash := sha256.Sum256([]byte(body))
	payloadHash := hex.EncodeToString(hash[:])
	headers := "host;x-content-sha256;x-date"
	canonical := strings.Join([]string{"POST", "/api/v3/", request.URL.RawQuery,
		"host:gateway.example\nx-content-sha256:" + payloadHash + "\nx-date:" + stamp + "\n", headers, payloadHash}, "\n")
	canonicalHash := sha256.Sum256([]byte(canonical))
	scope := date + "/cn-beijing/ark/request"
	signingKey := []byte(key.SecretAccessKey)
	for _, component := range []string{date, "cn-beijing", "ark", "request"} {
		mac := hmac.New(sha256.New, signingKey)
		_, _ = mac.Write([]byte(component))
		signingKey = mac.Sum(nil)
	}
	mac := hmac.New(sha256.New, signingKey)
	_, _ = mac.Write([]byte("HMAC-SHA256\n" + stamp + "\n" + scope + "\n" + hex.EncodeToString(canonicalHash[:])))
	request.Header.Set("X-Date", stamp)
	request.Header.Set("X-Content-Sha256", payloadHash)
	request.Header.Set("Authorization", "HMAC-SHA256 Credential="+key.AccessKeyID+"/"+scope+", SignedHeaders="+headers+", Signature="+hex.EncodeToString(mac.Sum(nil)))
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = request
	HandleVolcAssetAction(ctx)
	return recorder
}

func TestEnterpriseSignedAssetQueries(t *testing.T) {
	db := setupManageUserTestDB(t)
	t.Setenv("STORAGE_CREDENTIAL_ENCRYPTION_KEY", "enterprise-api-test-key")
	require.NoError(t, db.AutoMigrate(&model.AssetAccessKey{},
		&model.MediaAsset{}, &model.AssetGroup{}, &model.StoragePolicy{}, &model.StorageObject{}))
	owner := model.User{Username: "enterprise-owner", Status: common.UserStatusEnabled, Group: "default", AffCode: "enterprise-owner"}
	other := model.User{Username: "enterprise-other", Status: common.UserStatusEnabled, Group: "default", AffCode: "enterprise-other"}
	require.NoError(t, db.Create(&owner).Error)
	require.NoError(t, db.Create(&other).Error)
	key, err := assetService.CreateAccessKey(owner.Id, "integration")
	require.NoError(t, err)
	otherKey, err := assetService.CreateAccessKey(other.Id, "integration")
	require.NoError(t, err)

	group := model.AssetGroup{PublicID: "group-enterprise", OwnerUserID: owner.Id, Status: model.AssetStatusReady}
	require.NoError(t, db.Create(&group).Error)
	asset := model.MediaAsset{PublicID: "asset-enterprise", OwnerUserID: owner.Id, GroupID: group.ID, StorageObjectID: 999,
		AssetType: model.AssetTypeImage, Status: model.AssetStatusUnavailable, UnavailableReason: model.AssetUnavailableSensitiveContent}
	require.NoError(t, db.Create(&asset).Error)
	// The storage object is missing: metadata and verdict must still be returned.
	for _, action := range []string{"GetAsset", "ListAssets"} {
		response := requestSignedAssetAction(t, key, action, `{"Id":"asset-enterprise"}`)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Contains(t, response.Body.String(), `"Status":"Failed"`)
		assert.Contains(t, response.Body.String(), `"FailureReason":"sensitive_content"`)
		assert.Contains(t, response.Body.String(), `"URL":""`)
		assert.Contains(t, response.Body.String(), `"PreviewStatus":"Unavailable"`)
		assert.Contains(t, response.Body.String(), `"PreviewErrorCode":"preview_unavailable"`)
	}
	assert.Equal(t, http.StatusNotFound, requestSignedAssetAction(t, otherKey, "GetAsset", `{"Id":"asset-enterprise"}`).Code)
	for _, state := range []struct{ local, public string }{
		{model.AssetStatusProcessing, "Processing"}, {model.AssetStatusReady, "Active"},
	} {
		require.NoError(t, db.Model(&asset).Updates(map[string]any{"status": state.local, "unavailable_reason": ""}).Error)
		response := requestSignedAssetAction(t, key, "GetAsset", `{"Id":"asset-enterprise"}`)
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `"Status":"`+state.public+`"`)
		assert.NotContains(t, response.Body.String(), "FailureReason")
	}
	for _, action := range []string{
		"CreateAssetWebhookEndpoint", "ListAssetWebhookEndpoints", "UpdateAssetWebhookEndpoint",
		"DeleteAssetWebhookEndpoint", "TestAssetWebhookEndpoint",
	} {
		response := requestSignedAssetAction(t, key, action, `{}`)
		assert.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
		assert.Contains(t, response.Body.String(), `"Code":"InvalidAction"`)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v3/?Action=GetAsset&Version=2024-01-01", strings.NewReader(`{"Id":"asset-enterprise"}`))
	HandleVolcAssetAction(ctx)
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}
