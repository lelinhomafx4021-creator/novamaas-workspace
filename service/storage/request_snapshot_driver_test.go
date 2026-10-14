package storage

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func snapshotDriverCredential(t *testing.T, profileID int, authType, accessKey string) *model.StorageCredential {
	t.Helper()
	credential := &model.StorageCredential{StorageProfileID: profileID, Version: 1,
		AuthType: authType, Status: model.StorageCredentialStatusActive, KeyVersion: credentialEncryptionVersion}
	if authType == model.StorageCredentialAuthStatic {
		payload, err := encryptCredential(CredentialSecret{AccessKeyID: accessKey, AccessKeySecret: "snapshot-test-secret"})
		require.NoError(t, err)
		credential.EncryptedPayload = payload
	}
	require.NoError(t, model.DB.Create(credential).Error)
	return credential
}

func snapshotDriverSigningKey(t *testing.T, driver ObjectDriver) string {
	t.Helper()
	signed, err := driver.PresignGet(context.Background(), "snapshots/test.json", time.Minute)
	require.NoError(t, err)
	parsed, err := url.Parse(signed)
	require.NoError(t, err)
	key, _, found := strings.Cut(parsed.Query().Get("x-oss-credential"), "/")
	require.True(t, found, "signed snapshot URL must identify its credential")
	return key
}

func TestSnapshotDriverReusesConnectionsAndImmediatelyObservesCredentialRotation(t *testing.T) {
	t.Setenv("STORAGE_CREDENTIAL_ENCRYPTION_KEY", "snapshot-driver-test-master")
	profile := setupSnapshotDatabase(t)
	credential := snapshotDriverCredential(t, profile.ID, model.StorageCredentialAuthStatic, "snapshot-old-key")
	_, first, err := openSnapshotDriver(context.Background(), profile.ID)
	require.NoError(t, err)
	_, reused, err := openSnapshotDriver(context.Background(), profile.ID)
	require.NoError(t, err)
	assert.Same(t, first, reused, "different objects in one profile must share their connection pool")
	assert.Equal(t, "snapshot-old-key", snapshotDriverSigningKey(t, reused))

	// Same-second in-place changes must also invalidate the cache; timestamps
	// alone do not identify the credential that signs the next object operation.
	payload, err := encryptCredential(CredentialSecret{AccessKeyID: "snapshot-new-key", AccessKeySecret: "new-test-secret"})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(credential).Update("encrypted_payload", payload).Error)
	_, rotated, err := openSnapshotDriver(context.Background(), profile.ID)
	require.NoError(t, err)
	assert.NotSame(t, first, rotated)
	assert.Equal(t, "snapshot-new-key", snapshotDriverSigningKey(t, rotated))

	require.NoError(t, model.DB.Model(credential).Update("status", model.StorageCredentialStatusInactive).Error)
	_, unavailable, err := openSnapshotDriver(context.Background(), profile.ID)
	require.ErrorContains(t, err, "credential is required")
	assert.Nil(t, unavailable, "a cached credential must not survive database revocation")
}

func TestSnapshotDriverCacheDoesNotMaskCredentialDecryptionFailure(t *testing.T) {
	t.Setenv("STORAGE_CREDENTIAL_ENCRYPTION_KEY", "snapshot-original-master-key")
	profile := setupSnapshotDatabase(t)
	snapshotDriverCredential(t, profile.ID, model.StorageCredentialAuthStatic, "snapshot-signing-key")
	_, _, err := openSnapshotDriver(context.Background(), profile.ID)
	require.NoError(t, err)
	t.Setenv("STORAGE_CREDENTIAL_ENCRYPTION_KEY", "snapshot-wrong-master-key")
	_, driver, err := openSnapshotDriver(context.Background(), profile.ID)
	require.ErrorContains(t, err, "cannot be decrypted")
	assert.Nil(t, driver)
}

func TestSnapshotDriverCachePreservesProfileIdentityAndArchiveChecks(t *testing.T) {
	profile := setupSnapshotDatabase(t)
	snapshotDriverCredential(t, profile.ID, model.StorageCredentialAuthEnvironment, "")
	_, first, err := openSnapshotDriver(context.Background(), profile.ID)
	require.NoError(t, err)
	// A profile without an object ledger may still change destination. Opening
	// its next driver must not use a cached bucket from the previous identity.
	require.NoError(t, model.DB.Model(profile).Update("bucket", "snapshot-replacement-bucket").Error)
	current, replacement, err := openSnapshotDriver(context.Background(), profile.ID)
	require.NoError(t, err)
	assert.Equal(t, "snapshot-replacement-bucket", current.Bucket)
	assert.Equal(t, current.Bucket, replacement.(*aliyunOSSDriver).bucket)
	assert.NotSame(t, first, replacement)
	require.NoError(t, model.DB.Model(profile).Update("status", model.StorageProfileStatusArchived).Error)
	_, driver, err := openSnapshotDriver(context.Background(), profile.ID)
	require.ErrorContains(t, err, "profile is unavailable")
	assert.Nil(t, driver)
}

func TestSnapshotDriverReusedEnvironmentCredentialsRemainLive(t *testing.T) {
	profile := setupSnapshotDatabase(t)
	snapshotDriverCredential(t, profile.ID, model.StorageCredentialAuthEnvironment, "")
	t.Setenv("OSS_ACCESS_KEY_ID", "snapshot-env-first-key")
	t.Setenv("OSS_ACCESS_KEY_SECRET", "snapshot-env-first-secret")
	_, first, err := openSnapshotDriver(context.Background(), profile.ID)
	require.NoError(t, err)
	assert.Equal(t, "snapshot-env-first-key", snapshotDriverSigningKey(t, first))
	t.Setenv("OSS_ACCESS_KEY_ID", "snapshot-env-rotated-key")
	t.Setenv("OSS_ACCESS_KEY_SECRET", "snapshot-env-rotated-secret")
	_, reused, err := openSnapshotDriver(context.Background(), profile.ID)
	require.NoError(t, err)
	assert.Same(t, first, reused)
	assert.Equal(t, "snapshot-env-rotated-key", snapshotDriverSigningKey(t, reused))
}

func TestSnapshotConcurrentDriverOpensShareOneProfileConnectionPool(t *testing.T) {
	profile := setupSnapshotDatabase(t)
	snapshotDriverCredential(t, profile.ID, model.StorageCredentialAuthEnvironment, "")
	database, err := model.DB.DB()
	require.NoError(t, err)
	database.SetMaxOpenConns(1) // SQLite :memory: must use the same connection.
	type result struct {
		driver ObjectDriver
		err    error
	}
	results := make(chan result, 8)
	var group sync.WaitGroup
	for range cap(results) {
		group.Add(1)
		go func() {
			defer group.Done()
			_, driver, err := openSnapshotDriver(context.Background(), profile.ID)
			results <- result{driver: driver, err: err}
		}()
	}
	group.Wait()
	close(results)
	var shared ObjectDriver
	for opened := range results {
		require.NoError(t, opened.err)
		if shared == nil {
			shared = opened.driver
		}
		assert.Same(t, shared, opened.driver)
	}
}

func TestSnapshotDriverCacheDoesNotCrossDatabaseIdentities(t *testing.T) {
	firstProfile := setupSnapshotDatabase(t)
	snapshotDriverCredential(t, firstProfile.ID, model.StorageCredentialAuthEnvironment, "")
	_, first, err := openSnapshotDriver(context.Background(), firstProfile.ID)
	require.NoError(t, err)
	secondProfile := setupSnapshotDatabase(t)
	snapshotDriverCredential(t, secondProfile.ID, model.StorageCredentialAuthEnvironment, "")
	require.Equal(t, firstProfile.ID, secondProfile.ID)
	_, second, err := openSnapshotDriver(context.Background(), secondProfile.ID)
	require.NoError(t, err)
	assert.NotSame(t, first, second, "identical numeric IDs in another database must not reuse the old storage identity")
}
