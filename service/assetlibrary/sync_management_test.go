package assetlibrary

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssetViewsDoNotExposeUpstreamSynchronizationState(t *testing.T) {
	views := assetViews([]model.MediaAsset{{
		PublicID: "asset-20260922120000-public", GroupID: 4, Name: "campaign", AssetType: model.AssetTypeImage,
		ContentType: "image/png", Size: 128, SHA256: "checksum", Status: model.AssetStatusReady,
	}}, map[int64]string{4: "group-20260922120000-public"})

	require.Len(t, views, 1)
	payload, err := common.Marshal(views[0])
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "replica")
	assert.NotContains(t, string(payload), "channel")
}

func TestSyncManagementListsJobsAndRetriesOnlyFailures(t *testing.T) {
	db := setupAssetLibraryTestDB(t)
	owner := model.User{Id: 42, Username: "asset-owner", Password: "password", DisplayName: "Asset Owner"}
	require.NoError(t, db.Create(&owner).Error)
	channel := model.Channel{Id: 9, Name: "YooFang assets", Type: 50, Status: 1}
	require.NoError(t, db.Create(&channel).Error)
	group := model.AssetGroup{PublicID: "group-20260922120000-admin", OwnerUserID: owner.Id, Name: "Campaign", Status: model.AssetStatusReady}
	require.NoError(t, db.Create(&group).Error)
	asset := model.MediaAsset{
		PublicID: "asset-20260922120000-admin", OwnerUserID: owner.Id, GroupID: group.ID, StorageObjectID: 5001,
		Name: "Hero", AssetType: model.AssetTypeImage, ContentType: "image/png", Size: 256, Status: model.AssetStatusReady,
	}
	require.NoError(t, db.Create(&asset).Error)
	failed := model.AssetReplica{
		AssetID: asset.ID, ChannelID: channel.Id, Operation: model.AssetReplicaOperationSync,
		Status: model.AssetReplicaStatusFailed, Progress: 50, Attempts: 3, LastError: "upstream unavailable",
	}
	require.NoError(t, db.Create(&failed).Error)

	result, err := ListSyncJobs(1, 20, channel.Id, "")
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.EqualValues(t, 1, result.Summary.Total)
	assert.EqualValues(t, 1, result.Summary.Failed)
	assert.Equal(t, asset.PublicID, result.Items[0].AssetID)
	assert.Equal(t, group.Name, result.Items[0].GroupName)
	assert.Equal(t, owner.DisplayName, result.Items[0].OwnerName)
	assert.Equal(t, channel.Name, result.Items[0].ChannelName)

	require.NoError(t, RetrySyncJob(failed.ID))
	var retried model.AssetReplica
	require.NoError(t, db.First(&retried, failed.ID).Error)
	assert.Equal(t, model.AssetReplicaStatusPending, retried.Status)
	assert.Equal(t, model.AssetReplicaOperationSync, retried.Operation)
	assert.Zero(t, retried.Attempts)
	assert.Empty(t, retried.LastError)

	err = RetrySyncJob(failed.ID)
	require.Error(t, err)
	var requestErr *RequestError
	require.True(t, errors.As(err, &requestErr))
	assert.Equal(t, http.StatusConflict, requestErr.HTTPStatusCode())
}
