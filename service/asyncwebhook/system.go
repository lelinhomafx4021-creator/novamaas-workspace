package asyncwebhook

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const webhookManualEnabledOption = "WebhookManualEnabled"

type WebhookCapabilities struct {
	model.WebhookFeatureAvailability
	ManualEnabled bool `json:"manual_enabled"`
}

type SystemWebhookTarget struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url"`
}

type SystemWebhookSettings struct {
	AssetLibrary  SystemWebhookTarget `json:"asset_library"`
	MediaTasks    SystemWebhookTarget `json:"media_tasks"`
	ManualEnabled bool                `json:"manual_enabled"`
}

func GetWebhookCapabilities() (*WebhookCapabilities, error) {
	availability, err := model.GetWebhookFeatureAvailability(model.DB)
	if err != nil {
		return nil, err
	}
	var option model.Option
	// Read the persisted policy on every request so disabling applies across nodes immediately.
	if err := model.DB.Where(&model.Option{Key: webhookManualEnabledOption}).Limit(1).Find(&option).Error; err != nil {
		return nil, err
	}
	return &WebhookCapabilities{WebhookFeatureAvailability: *availability, ManualEnabled: option.Value == "true"}, nil
}

func GetSystemWebhookSettings() (*SystemWebhookSettings, error) {
	capabilities, err := GetWebhookCapabilities()
	if err != nil {
		return nil, err
	}
	var endpoints []model.AssetWebhookEndpoint
	if err := model.DB.Where("owner_user_id = 0 AND public_id IN ?", []string{model.SystemAssetWebhookEndpointID, model.SystemTaskWebhookEndpointID}).Find(&endpoints).Error; err != nil {
		return nil, err
	}
	settings := &SystemWebhookSettings{ManualEnabled: capabilities.ManualEnabled}
	for _, endpoint := range endpoints {
		target := SystemWebhookTarget{Enabled: endpoint.Status == model.AssetWebhookEndpointStatusEnabled, URL: endpoint.URL}
		if endpoint.PublicID == model.SystemAssetWebhookEndpointID {
			settings.AssetLibrary = target
		} else {
			settings.MediaTasks = target
		}
	}
	return settings, nil
}

func SaveSystemWebhookSettings(settings SystemWebhookSettings) (*SystemWebhookSettings, error) {
	for _, target := range []*SystemWebhookTarget{&settings.AssetLibrary, &settings.MediaTasks} {
		target.URL = strings.TrimSpace(target.URL)
		if target.URL != "" {
			url, err := ValidateCallbackURL(target.URL)
			if err != nil {
				return nil, err
			}
			target.URL = url
		}
	}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		for _, entry := range []struct {
			ID, Name, Events string
			Target           SystemWebhookTarget
		}{
			{model.SystemAssetWebhookEndpointID, "System asset library webhook", `["asset.active","asset.failed"]`, settings.AssetLibrary},
			{model.SystemTaskWebhookEndpointID, "System media task webhook", `["task.status_changed"]`, settings.MediaTasks},
		} {
			status := model.AssetWebhookEndpointStatusDisabled
			if entry.Target.Enabled {
				status = model.AssetWebhookEndpointStatusEnabled
			}
			endpoint := model.AssetWebhookEndpoint{PublicID: entry.ID, OwnerUserID: 0, Name: entry.Name, URL: entry.Target.URL, EventTypes: entry.Events, Status: status, UpdatedAt: common.GetTimestamp()}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "public_id"}}, DoUpdates: clause.AssignmentColumns([]string{"url", "status", "updated_at"})}).Create(&endpoint).Error; err != nil {
				return err
			}
		}
		option := model.Option{Key: webhookManualEnabledOption, Value: strconv.FormatBool(settings.ManualEnabled)}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoUpdates: clause.AssignmentColumns([]string{"value"})}).Create(&option).Error
	})
	return &settings, err
}
