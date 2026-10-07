package asyncwebhook

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCategorySwitchGatesPersonalConfigurationAndTests(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AssetWebhookEndpoint{}, &model.AssetWebhookDelivery{}))
	original := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = original; assert.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Create(&model.User{Id: 7, Username: "policy-user", AffCode: "policy-user"}).Error)
	input := WebhookEndpointInput{Name: "callback", URL: "https://customer.example/events", EventTypes: []string{model.WebhookEventAssetFailed}}
	for _, events := range [][]string{{model.WebhookEventAssetFailed}, {model.WebhookEventTaskStatusChanged}} {
		input.EventTypes = events
		_, err = CreateWebhookEndpoint(7, input)
		var requestErr *RequestError
		require.ErrorAs(t, err, &requestErr)
		assert.Equal(t, 403, requestErr.HTTPStatusCode())
	}
	flag := model.AssetWebhookEndpoint{PublicID: model.SystemAssetWebhookEndpointID, OwnerUserID: 0, Status: model.AssetWebhookEndpointStatusEnabled, EventTypes: `["asset.active","asset.failed"]`}
	require.NoError(t, db.Create(&flag).Error)
	input.EventTypes = []string{model.WebhookEventAssetFailed}
	endpoint, err := CreateWebhookEndpoint(7, input)
	require.NoError(t, err, "enabling a category without an operator URL allows personal callbacks")
	input.EventTypes = []string{model.WebhookEventAssetFailed, model.WebhookEventTaskStatusChanged}
	_, err = UpdateWebhookEndpoint(7, endpoint.ID, input)
	var requestErr *RequestError
	require.ErrorAs(t, err, &requestErr)
	assert.Equal(t, 403, requestErr.HTTPStatusCode(), "a mixed subscription cannot include a disabled category")
	_, err = QueueWebhookEndpointTest(7, endpoint.ID)
	require.NoError(t, err)
	require.NoError(t, db.Model(&flag).Update("status", model.AssetWebhookEndpointStatusDisabled).Error)
	input.EventTypes = []string{model.WebhookEventAssetFailed}
	_, err = UpdateWebhookEndpoint(7, endpoint.ID, input)
	require.ErrorAs(t, err, &requestErr)
	assert.Equal(t, 403, requestErr.HTTPStatusCode())
	_, err = QueueWebhookEndpointTest(7, endpoint.ID)
	require.ErrorAs(t, err, &requestErr)
	assert.Equal(t, 403, requestErr.HTTPStatusCode())
	require.NoError(t, DeleteWebhookEndpoint(7, endpoint.ID), "users can still remove paused endpoints")
}
