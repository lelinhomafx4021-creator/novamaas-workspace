package asyncwebhook

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSystemSettingsArePersistentAtomicAndKeepEndpointIdentity(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.AssetWebhookEndpoint{}, &model.Option{}))
	original := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = original; assert.NoError(t, sqlDB.Close()) })
	defaults, err := GetSystemWebhookSettings()
	require.NoError(t, err)
	assert.False(t, defaults.AssetLibrary.Enabled)
	assert.False(t, defaults.MediaTasks.Enabled)
	assert.False(t, defaults.ManualEnabled)
	personalOnly := SystemWebhookSettings{AssetLibrary: SystemWebhookTarget{Enabled: true}, MediaTasks: SystemWebhookTarget{Enabled: true}}
	_, err = SaveSystemWebhookSettings(personalOnly)
	require.NoError(t, err, "category switches can enable personal callbacks without system addresses")
	stored, err := GetSystemWebhookSettings()
	require.NoError(t, err)
	assert.Equal(t, personalOnly, *stored)
	capabilities, err := GetWebhookCapabilities()
	require.NoError(t, err)
	assert.True(t, capabilities.AssetLibraryEnabled)
	assert.True(t, capabilities.MediaTasksEnabled)
	input := SystemWebhookSettings{AssetLibrary: SystemWebhookTarget{Enabled: true, URL: "https://operator.example/assets"}, MediaTasks: SystemWebhookTarget{Enabled: true, URL: "https://operator.example/tasks"}, ManualEnabled: true}
	_, err = SaveSystemWebhookSettings(input)
	require.NoError(t, err)
	var before model.AssetWebhookEndpoint
	require.NoError(t, db.Where("public_id = ?", model.SystemTaskWebhookEndpointID).First(&before).Error)
	invalid := input
	invalid.AssetLibrary.URL = "https://changed.example/assets"
	invalid.MediaTasks.URL = "http://operator.example/tasks"
	invalid.ManualEnabled = false
	_, err = SaveSystemWebhookSettings(invalid)
	require.Error(t, err)
	stored, err = GetSystemWebhookSettings()
	require.NoError(t, err)
	assert.Equal(t, input, *stored)
	input.MediaTasks.Enabled = false
	input.ManualEnabled = false
	_, err = SaveSystemWebhookSettings(input)
	require.NoError(t, err)
	stored, err = GetSystemWebhookSettings()
	require.NoError(t, err)
	assert.Equal(t, input, *stored)
	var after model.AssetWebhookEndpoint
	require.NoError(t, db.Where("public_id = ?", model.SystemTaskWebhookEndpointID).First(&after).Error)
	assert.Equal(t, before.ID, after.ID, "pending deliveries keep their endpoint reference")
	personal, err := ListWebhookEndpoints(7)
	require.NoError(t, err)
	assert.Empty(t, personal, "system addresses are private to root")
	// A failure persisting the manual policy must roll back both callback settings too.
	require.NoError(t, db.Migrator().DropTable(&model.Option{}))
	input.AssetLibrary.URL = "https://changed.example/assets"
	input.MediaTasks.URL = "https://changed.example/tasks"
	input.ManualEnabled = true
	_, err = SaveSystemWebhookSettings(input)
	require.Error(t, err)
	require.NoError(t, db.Where("public_id = ?", model.SystemTaskWebhookEndpointID).First(&after).Error)
	assert.Equal(t, before.URL, after.URL)
	var asset model.AssetWebhookEndpoint
	require.NoError(t, db.Where("public_id = ?", model.SystemAssetWebhookEndpointID).First(&asset).Error)
	assert.Equal(t, "https://operator.example/assets", asset.URL)
}
