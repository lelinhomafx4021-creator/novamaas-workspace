package asyncwebhook

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTaskSubscriptionUpdatesKeepEndpointIdentityAndOwnership(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.AssetWebhookEndpoint{}, &model.AssetWebhookDelivery{}))
	require.NoError(t, db.Create(&[]model.AssetWebhookEndpoint{
		{PublicID: model.SystemAssetWebhookEndpointID, OwnerUserID: 0, Status: model.AssetWebhookEndpointStatusEnabled, EventTypes: `["asset.failed"]`},
		{PublicID: model.SystemTaskWebhookEndpointID, OwnerUserID: 0, Status: model.AssetWebhookEndpointStatusEnabled, EventTypes: `["task.status_changed"]`},
	}).Error)
	original := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = original; assert.NoError(t, sqlDB.Close()) })
	endpoint := model.AssetWebhookEndpoint{PublicID: "we_existing", OwnerUserID: 7, Name: "assets", URL: "https://customer.example/assets",
		EventTypes: `["asset.active","asset.failed"]`, Status: model.AssetWebhookEndpointStatusEnabled}
	require.NoError(t, db.Create(&endpoint).Error)
	input := WebhookEndpointInput{Name: "unified events", URL: "https://customer.example/events", EventTypes: []string{model.WebhookEventTaskStatusChanged, model.WebhookEventAssetFailed, model.WebhookEventTaskStatusChanged}}
	_, err = UpdateWebhookEndpoint(8, endpoint.PublicID, input)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = QueueWebhookEndpointTest(8, endpoint.PublicID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.ErrorIs(t, DeleteWebhookEndpoint(8, endpoint.PublicID), gorm.ErrRecordNotFound)
	view, err := UpdateWebhookEndpoint(7, endpoint.PublicID, input)
	require.NoError(t, err)
	assert.Equal(t, endpoint.PublicID, view.ID)
	assert.Equal(t, "unified events", view.Name)
	assert.Equal(t, []string{model.WebhookEventAssetFailed, model.WebhookEventTaskStatusChanged}, view.EventTypes)
	eventID, err := QueueWebhookEndpointTest(7, endpoint.PublicID)
	require.NoError(t, err)
	var delivery model.AssetWebhookDelivery
	require.NoError(t, db.First(&delivery).Error)
	assert.Equal(t, eventID, delivery.EventID)
	assert.Equal(t, endpoint.ID, delivery.EndpointID)
	assert.Equal(t, "webhook.test", delivery.EventType)
	legacyEventID, err := QueueAssetWebhookEndpointTest(7, endpoint.PublicID)
	require.NoError(t, err)
	var legacyDelivery model.AssetWebhookDelivery
	require.NoError(t, db.Where("event_id = ?", legacyEventID).First(&legacyDelivery).Error)
	assert.Equal(t, "asset.webhook_test", legacyDelivery.EventType)
	require.NoError(t, DeleteWebhookEndpoint(7, endpoint.PublicID))
	_, err = UpdateWebhookEndpoint(7, endpoint.PublicID, input)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestTaskSubscriptionRejectsInvalidCallbackAndEvents(t *testing.T) {
	for _, input := range []WebhookEndpointInput{
		{Name: "callback", URL: "http://customer.example/events", EventTypes: []string{model.WebhookEventTaskStatusChanged}},
		{Name: "callback", URL: "https://user:password@customer.example/events", EventTypes: []string{model.WebhookEventTaskStatusChanged}},
		{Name: "callback", URL: "https://customer.example/events#fragment", EventTypes: []string{model.WebhookEventTaskStatusChanged}},
		{Name: "callback", URL: "https://customer.example/events", EventTypes: []string{"task.unknown"}},
		{Name: "callback", URL: "https://customer.example/events", EventTypes: nil},
	} {
		_, err := CreateWebhookEndpoint(7, input)
		var requestErr *RequestError
		require.ErrorAs(t, err, &requestErr)
		assert.Equal(t, 400, requestErr.HTTPStatusCode())
	}
}
