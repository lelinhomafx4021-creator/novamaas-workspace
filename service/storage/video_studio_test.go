package storage

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memoryObjectDriver struct{ puts *int }

func (driver memoryObjectDriver) PutObject(_ context.Context, _ string, _ string, _ int64, body io.Reader) (string, error) {
	*driver.puts++
	_, err := io.Copy(io.Discard, body)
	return "etag", err
}

func (driver memoryObjectDriver) PresignGet(_ context.Context, objectKey string, _ time.Duration, _ ...ObjectGetOptions) (string, error) {
	return "https://oss.test/" + objectKey, nil
}

func (driver memoryObjectDriver) DeleteObject(context.Context, string) error { return nil }

func setupVideoStudioStorage(t *testing.T, maxTotalBytes int64) *int {
	t.Helper()
	setupStorageDatabase(t)
	require.NoError(t, model.DB.AutoMigrate(&model.MediaAsset{}))
	t.Setenv("STORAGE_CREDENTIAL_ENCRYPTION_KEY", "video-studio-storage-test-key")
	profile, err := SaveProfile(0, ProfileInput{
		Name: "Studio OSS", ProviderType: model.StorageProviderAliyunOSS, Status: model.StorageProfileStatusEnabled,
		Endpoint: "https://oss-cn-hangzhou.aliyuncs.com", Region: "cn-hangzhou", Bucket: "studio-bucket",
		AuthType: model.StorageCredentialAuthStatic, AccessKeyID: "test-id", AccessKeySecret: "test-secret",
	})
	require.NoError(t, err)
	policy := model.DefaultVideoStudioStoragePolicy()
	policy.StorageProfileID = profile.ID
	policy.Enabled = true
	policy.MaxTotalBytes = maxTotalBytes
	require.NoError(t, model.SaveStoragePolicy(policy))

	puts := 0
	savedDriver := driverForProfile
	driverForProfile = func(*model.StorageProfile, *model.StorageCredential) (ObjectDriver, error) {
		return memoryObjectDriver{puts: &puts}, nil
	}
	t.Cleanup(func() { driverForProfile = savedDriver })
	return &puts
}

func pngBytes(t *testing.T, width int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	require.NoError(t, png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, width, 1))))
	return buffer.Bytes()
}

func TestVideoStudioReferenceIsTemporaryAndDeduplicatedPerUser(t *testing.T) {
	puts := setupVideoStudioStorage(t, 512*1024*1024)
	payload := pngBytes(t, 4)

	first, err := UploadVideoStudioReference(context.Background(), 7, "../cat.png", bytes.NewReader(payload))
	require.NoError(t, err)
	assert.False(t, first.Saved)
	assert.Greater(t, first.ExpiresAt, common.GetTimestamp())
	assert.Equal(t, "cat.png", first.FileName)
	assert.Equal(t, "image/png", first.ContentType)

	again, err := UploadVideoStudioReference(context.Background(), 7, "cat.png", bytes.NewReader(payload))
	require.NoError(t, err)
	assert.Equal(t, first.ID, again.ID, "same user and bytes must reuse the stored file")

	other, err := UploadVideoStudioReference(context.Background(), 8, "cat.png", bytes.NewReader(payload))
	require.NoError(t, err)
	assert.NotEqual(t, first.ID, other.ID, "deduplication must not cross users")
	assert.Equal(t, 2, *puts)

	library, err := ListVideoStudioReferences(7, false, 1, 20)
	require.NoError(t, err)
	assert.Empty(t, library.Items, "temporary uploads stay out of the library")

	_, err = FindVideoStudioReference(8, first.ID)
	require.Error(t, err, "references are owner scoped")
}

func TestVideoStudioReferenceRejectsSpoofedAndOversizedFiles(t *testing.T) {
	setupVideoStudioStorage(t, 512*1024*1024)

	_, err := UploadVideoStudioReference(context.Background(), 7, "fake.png", bytes.NewReader([]byte("<html>not an image</html>")))
	var requestErr *RequestError
	require.ErrorAs(t, err, &requestErr)
	assert.Equal(t, http.StatusUnsupportedMediaType, requestErr.StatusCode)

	_, err = UploadVideoStudioReference(context.Background(), 7, "big.png", bytes.NewReader(make([]byte, 10*1024*1024+1)))
	require.ErrorAs(t, err, &requestErr)
	assert.Equal(t, http.StatusRequestEntityTooLarge, requestErr.StatusCode)
}

func TestVideoStudioReferenceQuotaCountsLiveFilesOnly(t *testing.T) {
	first := pngBytes(t, 4)
	setupVideoStudioStorage(t, int64(len(first))+10)

	reference, err := UploadVideoStudioReference(context.Background(), 7, "a.png", bytes.NewReader(first))
	require.NoError(t, err)
	_, err = UploadVideoStudioReference(context.Background(), 7, "b.png", bytes.NewReader(pngBytes(t, 64)))
	var requestErr *RequestError
	require.ErrorAs(t, err, &requestErr)
	assert.Equal(t, http.StatusInsufficientStorage, requestErr.StatusCode)

	require.NoError(t, model.DB.Model(&model.StorageObject{}).Where("object_id = ?", reference.ID).Update("delete_after", common.GetTimestamp()-1).Error)
	_, err = FindVideoStudioReference(7, reference.ID)
	require.Error(t, err, "expired temporary files are not addressable before cleanup runs")
	_, err = UploadVideoStudioReference(context.Background(), 7, "b.png", bytes.NewReader(pngBytes(t, 64)))
	require.NoError(t, err, "expired files no longer count toward the quota")
}

func TestSavedVideoStudioReferenceSurvivesTaskAndAssetCleanup(t *testing.T) {
	setupVideoStudioStorage(t, 512*1024*1024)
	reference, err := UploadVideoStudioReference(context.Background(), 7, "a.png", bytes.NewReader(pngBytes(t, 4)))
	require.NoError(t, err)

	saved, err := SaveVideoStudioReference(7, reference.ID)
	require.NoError(t, err)
	assert.True(t, saved.Saved)
	assert.Zero(t, saved.ExpiresAt)

	object, err := FindVideoStudioReference(7, reference.ID)
	require.NoError(t, err)
	require.NoError(t, DeleteAssetObject(object.ID, "asset deleted"))
	_, err = FindVideoStudioReference(7, reference.ID)
	require.NoError(t, err, "deleting a Seedance asset must not remove the user's library file")

	claimed, err := model.ClaimStorageObjectsForDeletion(common.GetTimestamp()+365*24*3600, common.GetTimestamp()+60, "test", 50)
	require.NoError(t, err)
	assert.Empty(t, claimed, "saved references are never claimed by the cleanup runner")

	require.NoError(t, model.DB.Create(&model.MediaAsset{PublicID: "asset-1", OwnerUserID: 7, StorageObjectID: object.ID, Status: model.AssetStatusReady}).Error)
	err = DeleteVideoStudioReference(7, reference.ID)
	var requestErr *RequestError
	require.ErrorAs(t, err, &requestErr)
	assert.Equal(t, http.StatusConflict, requestErr.StatusCode)
}
