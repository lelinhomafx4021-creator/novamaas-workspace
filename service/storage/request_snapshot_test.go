package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func snapshotTestSpool(t *testing.T, dir string) *snapshotSpool {
	t.Helper()
	spool, err := openSnapshotSpool(dir, 2)
	require.NoError(t, err)
	t.Cleanup(func() {
		spool.cancel()
		_ = unlockSnapshotSpool(spool.lock)
		_ = spool.lock.Close()
	})
	return spool
}

func setupSnapshotDatabase(t *testing.T) *model.StorageProfile {
	t.Helper()
	savedDB := model.DB
	t.Cleanup(func() { model.DB = savedDB })
	setupStorageDatabase(t)
	require.NoError(t, model.DB.AutoMigrate(&model.TaskRequestBody{}, &model.Task{}))
	profile := &model.StorageProfile{Name: "snapshot-test", ProviderType: model.StorageProviderAliyunOSS,
		Status: model.StorageProfileStatusEnabled, Endpoint: "https://oss-cn-hangzhou.aliyuncs.com", Region: "cn-hangzhou", Bucket: "snapshot-test-bucket"}
	require.NoError(t, model.DB.Create(profile).Error)
	return profile
}

func TestSnapshotSpoolCapturesLargeBodiesAndRecoversWithoutRequestReader(t *testing.T) {
	dir := t.TempDir()
	spool := snapshotTestSpool(t, dir)
	payload := []byte(`{"model":"video","prompt":"` + strings.Repeat("a", 3*1024*1024) + `"}`)
	policy := model.DefaultRequestSnapshotStoragePolicy()
	profile := &model.StorageProfile{ID: 7}
	reader := io.NopCloser(bytes.NewReader(payload))
	require.NoError(t, spool.capture(SnapshotCapture{RequestID: "req-large", Kind: "original"}, reader, int64(len(payload)), policy, profile))
	require.NoError(t, reader.Close())
	require.Len(t, spool.jobs, 1)
	var job *snapshotSpoolJob
	for _, pending := range spool.jobs {
		job = pending
	}
	require.NotNil(t, job)
	assert.EqualValues(t, len(payload), spool.bytes)
	assert.Equal(t, fmt.Sprintf("%x", sha256.Sum256(payload)), job.SHA256)
	assert.Equal(t, model.TaskRequestEvidenceVersion, job.Evidence.Version)
	info, err := os.Stat(filepath.Join(dir, job.ID+".body"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	dirInfo, err := os.Stat(dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0700), dirInfo.Mode().Perm())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "migration-state.json"), []byte(`{"running":false}`), 0600))
	require.NoError(t, unlockSnapshotSpool(spool.lock))
	require.NoError(t, spool.lock.Close())
	recovered := snapshotTestSpool(t, dir)
	require.Len(t, recovered.jobs, 1)
	file, err := os.Open(filepath.Join(dir, job.ID+".body"))
	require.NoError(t, err)
	defer file.Close()
	require.NoError(t, copyVerifiedSnapshot(file, io.Discard, job.Size, job.SHA256))
	assert.Equal(t, job.RequestID, recovered.jobs[job.ID].RequestID)
	assert.Equal(t, spool.bytes, recovered.bytes)
	_, err = os.Stat(filepath.Join(dir, "migration-state.json"))
	require.NoError(t, err, "worker recovery must preserve administrator job checkpoints")
}

type snapshotFailingReader struct{}

func (snapshotFailingReader) Read(buffer []byte) (int, error) {
	return copy(buffer, "partial"), errors.New("input stream interrupted")
}

func TestSnapshotSpoolRejectsCapacityAndIncompleteCaptureWithoutCommittedJobs(t *testing.T) {
	spool := snapshotTestSpool(t, t.TempDir())
	payload := []byte(`{"prompt":"hello"}`)
	policy := model.DefaultRequestSnapshotStoragePolicy()
	policy.MaxFileBytes, policy.MaxTotalBytes = int64(len(payload)), int64(len(payload))
	metadata := SnapshotCapture{RequestID: "req-capacity", Kind: "original"}
	profile := &model.StorageProfile{ID: 7}
	require.ErrorContains(t, spool.capture(metadata, snapshotFailingReader{}, int64(len(payload)), policy, profile), "input stream interrupted")
	assert.Contains(t, spool.lastError, "input stream interrupted")
	assert.Zero(t, spool.bytes)
	assert.Empty(t, spool.jobs)
	entries, err := os.ReadDir(spool.dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "failed capture must leave only the owner lock")
	require.NoError(t, spool.capture(metadata, bytes.NewReader(payload), int64(len(payload)), policy, profile))
	require.ErrorContains(t, spool.capture(metadata, bytes.NewReader(payload), int64(len(payload)), policy, profile), "capacity")
	assert.Contains(t, spool.lastError, "capacity")
	assert.EqualValues(t, len(payload), spool.bytes)
	assert.Len(t, spool.jobs, 1)
	require.ErrorContains(t, spool.capture(metadata, bytes.NewReader(payload), int64(len(payload)+1), policy, profile), "file limit")
}

func TestSnapshotSpoolHasOnlyOneProcessOwner(t *testing.T) {
	dir := t.TempDir()
	first := snapshotTestSpool(t, dir)
	_, err := openSnapshotSpool(dir, 2)
	require.ErrorContains(t, err, "already owned")
	require.NoError(t, unlockSnapshotSpool(first.lock))
	require.NoError(t, first.lock.Close())
	second := snapshotTestSpool(t, dir)
	assert.Empty(t, second.jobs)
}

type snapshotMemoryDriver struct {
	objects map[string][]byte
	puts    int
}

func (driver *snapshotMemoryDriver) PutObject(_ context.Context, key, _ string, _ int64, body io.Reader) (string, error) {
	driver.puts++
	if _, exists := driver.objects[key]; exists {
		return "", errors.New("immutable object already exists")
	}
	payload, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	driver.objects[key] = payload
	return "test-etag", nil
}

func (driver *snapshotMemoryDriver) GetObject(_ context.Context, key string) (io.ReadCloser, error) {
	payload, exists := driver.objects[key]
	if !exists {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(payload)), nil
}

func (driver *snapshotMemoryDriver) PresignGet(context.Context, string, time.Duration, ...ObjectGetOptions) (string, error) {
	return "", errors.New("snapshot workers should not need signed URLs")
}

func (driver *snapshotMemoryDriver) DeleteObject(_ context.Context, key string) error {
	delete(driver.objects, key)
	return nil
}

func TestSnapshotUploadRetainsSpoolWhenDatabasePublishFailsAndReusesVerifiedObject(t *testing.T) {
	profile := setupSnapshotDatabase(t)
	spool := snapshotTestSpool(t, t.TempDir())
	payload := []byte(`{"model":"doubao-seedance","content":[{"type":"text","text":"a bird"}]}`)
	policy := model.DefaultRequestSnapshotStoragePolicy()
	metadata := SnapshotCapture{RequestID: "req-db-retry", Kind: "upstream", Evidence: model.ExtractTaskRequestEvidence(payload)}
	require.NoError(t, spool.capture(metadata, bytes.NewReader(payload), int64(len(payload)), policy, profile))
	job := spool.claimDueJob(time.Now().Unix())
	require.NotNil(t, job)
	driver := &snapshotMemoryDriver{objects: make(map[string][]byte)}
	store := snapshotObjectStore{driver: driver}
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register("snapshot:test_publish_failure", func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "TaskRequestBody" {
			tx.AddError(errors.New("snapshot index unavailable"))
		}
	}))
	err := store.put(context.Background(), job, bytes.NewReader(payload))
	require.ErrorContains(t, err, "snapshot index unavailable")
	spool.finishJob(job, err)
	assert.Len(t, spool.jobs, 1)
	_, err = os.Stat(filepath.Join(spool.dir, job.ID+".json"))
	require.NoError(t, err)
	assert.Equal(t, 1, driver.puts)
	var object model.StorageObject
	require.NoError(t, model.DB.Where("object_key = ?", snapshotObjectKey(job)).First(&object).Error)
	assert.Equal(t, model.StorageObjectStatusUploaded, object.Status)
	assert.Zero(t, object.DeleteAfter, "failed publication must not allow retention cleanup to remove retry data")
	require.NoError(t, model.DB.Callback().Create().Remove("snapshot:test_publish_failure"))
	require.NoError(t, store.put(context.Background(), job, bytes.NewReader(payload)))
	assert.Equal(t, 1, driver.puts, "retry must reuse the immutable verified object")
	records, err := model.GetSnapshotRecords("", job.RequestID)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Empty(t, records[0].Body)
	assert.Equal(t, job.SHA256, records[0].BodySHA256)
	assert.Equal(t, model.TaskRequestSnapshotReady, records[0].StorageState)
	require.NoError(t, model.DB.Where("id = ?", object.ID).First(&object).Error)
	assert.Zero(t, object.DeleteAfter, "the default permanent policy must remain permanent after publication")
	expired, err := model.ClaimStorageObjectsForDeletion(time.Now().AddDate(10, 0, 0).Unix(), time.Now().AddDate(10, 0, 0).Unix()+120, "snapshot-retention-check", 10)
	require.NoError(t, err)
	assert.Empty(t, expired, "permanent snapshots must not enter retention deletion")
	spool.finishJob(job, nil)
	assert.Empty(t, spool.jobs)
	assert.Zero(t, spool.bytes)
	_, err = os.Stat(filepath.Join(spool.dir, job.ID+".body"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestSnapshotIntegrityRejectsChangedLocalAndRemoteBodiesBeforePublishing(t *testing.T) {
	profile := setupSnapshotDatabase(t)
	payload := []byte(`{"prompt":"original"}`)
	job := &snapshotSpoolJob{SnapshotCapture: SnapshotCapture{RequestID: "req-integrity", Kind: "original", Evidence: model.TaskRequestEvidence{Version: 1}},
		Profile: *profile, Prefix: "snapshots/requests", Size: int64(len(payload)), SHA256: fmt.Sprintf("%x", sha256.Sum256(payload))}
	driver := &snapshotMemoryDriver{objects: make(map[string][]byte)}
	store := snapshotObjectStore{driver: driver}
	require.ErrorContains(t, store.put(context.Background(), job, bytes.NewReader([]byte(`{"prompt":"modified"}`))), "integrity")
	assert.Zero(t, driver.puts)
	driver.objects[snapshotObjectKey(job)] = []byte(`{"prompt":"modified"}`)
	require.Error(t, store.put(context.Background(), job, bytes.NewReader(payload)))
	var count int64
	require.NoError(t, model.DB.Model(&model.TaskRequestBody{}).Count(&count).Error)
	assert.Zero(t, count, "a corrupt immutable object cannot become a ready snapshot")
	for _, corrupt := range [][]byte{payload[:3], append(append([]byte(nil), payload...), 1), []byte(`{"prompt":"modified"}`)} {
		require.ErrorContains(t, copyVerifiedSnapshot(bytes.NewReader(corrupt), io.Discard, job.Size, job.SHA256), "integrity")
	}
}

func TestSnapshotRegistersAndFreezesProfileBeforeInitializingDriver(t *testing.T) {
	profile := setupSnapshotDatabase(t)
	payload := []byte(`{"prompt":"freeze the archive location"}`)
	job := &snapshotSpoolJob{SnapshotCapture: SnapshotCapture{RequestID: "req-profile-freeze", Kind: "original", Evidence: model.TaskRequestEvidence{Version: 1}},
		Profile: *profile, Prefix: "snapshots/requests", Size: int64(len(payload)), SHA256: fmt.Sprintf("%x", sha256.Sum256(payload))}
	driver := &snapshotMemoryDriver{objects: make(map[string][]byte)}
	opened := false
	store := snapshotObjectStore{openDriver: func(context.Context, int) (*model.StorageProfile, ObjectDriver, error) {
		opened = true
		// Driver construction is the boundary at which an administrator could
		// otherwise change B -> A -> B before publication. Its ledger must have
		// already frozen the bucket, even before any remote object exists.
		_, err := SaveProfile(profile.ID, ProfileInput{Name: profile.Name, ProviderType: profile.ProviderType,
			Status: profile.Status, Endpoint: profile.Endpoint, Region: profile.Region,
			Bucket: "another-snapshot-bucket", AuthType: model.StorageCredentialAuthEnvironment})
		require.ErrorContains(t, err, "cannot change provider, endpoint, region, or bucket")
		return profile, driver, nil
	}}
	require.NoError(t, store.put(context.Background(), job, bytes.NewReader(payload)))
	assert.True(t, opened)
	stored, err := model.GetStorageProfileByID(profile.ID)
	require.NoError(t, err)
	assert.Equal(t, profile.Bucket, stored.Bucket)
	records, err := model.GetSnapshotRecords("", job.RequestID)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, profile.ID, records[0].StorageProfileID)
	assert.Equal(t, driver.objects[snapshotObjectKey(job)], payload)
}

type snapshotOSSRequestTransport struct {
	headers http.Header
}

func (transport *snapshotOSSRequestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.headers = request.Header.Clone()
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"ETag": []string{"test-etag"}},
		Body: io.NopCloser(strings.NewReader("")), ContentLength: 0, Request: request}, nil
}

func TestSnapshotJSONUploadForcesPrivateObjectACL(t *testing.T) {
	transport := &snapshotOSSRequestTransport{}
	config := oss.LoadDefaultConfig().WithRegion("cn-hangzhou").WithEndpoint("https://oss-cn-hangzhou.aliyuncs.com").
		WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test-id", "test-secret")).
		WithHttpClient(&http.Client{Transport: transport})
	driver := &aliyunOSSDriver{client: oss.NewClient(config), bucket: "test-bucket"}
	payload := []byte(`{"prompt":"administrator only"}`)
	_, err := driver.PutObject(context.Background(), "snapshots/private.json", "application/json", int64(len(payload)), bytes.NewReader(payload))
	require.NoError(t, err)
	assert.Equal(t, "private", transport.headers.Get("x-oss-object-acl"))
	_, err = driver.PutObject(context.Background(), "assets/image.png", "image/png", 4, strings.NewReader("test"))
	require.NoError(t, err)
	assert.Empty(t, transport.headers.Get("x-oss-object-acl"), "the existing media upload ACL contract is preserved")
}

func TestSnapshotSpoolRejectsBodyCacheAndItsSymlinkAlias(t *testing.T) {
	base := t.TempDir()
	savedConfig := common.GetDiskCacheConfig()
	t.Cleanup(func() { common.SetDiskCacheConfig(savedConfig) })
	common.SetDiskCacheConfig(common.DiskCacheConfig{Path: base})
	cacheDir := common.GetDiskCacheDir()
	require.NoError(t, os.MkdirAll(cacheDir, 0755))
	alias := filepath.Join(base, "cache-alias")
	require.NoError(t, os.Symlink(cacheDir, alias))
	for _, path := range []string{cacheDir, alias, filepath.Join(os.TempDir(), "new-api-body-cache")} {
		t.Setenv("SNAPSHOT_SPOOL_DIR", path)
		require.ErrorContains(t, ValidateRequestSnapshotSpoolDirectory(), "must not share")
		_, err := openSnapshotSpool(path, 2)
		require.ErrorContains(t, err, "must not share")
	}
	t.Setenv("SNAPSHOT_SPOOL_DIR", filepath.Join(cacheDir, "durable-snapshots"))
	require.NoError(t, ValidateRequestSnapshotSpoolDirectory(), "the request cache cleanup does not recurse into independent subdirectories")
}

func TestSnapshotWorkersSerializeAttemptsForOneRequestButAllowOtherRequests(t *testing.T) {
	spool := snapshotTestSpool(t, t.TempDir())
	payload := []byte(`{"prompt":"hello"}`)
	policy, profile := model.DefaultRequestSnapshotStoragePolicy(), &model.StorageProfile{ID: 1}
	for _, metadata := range []SnapshotCapture{
		{RequestID: "one", Kind: "upstream", CapturedAt: 1},
		{RequestID: "one", TaskID: "later-task-id", Kind: "upstream", CapturedAt: 2},
		{RequestID: "two", Kind: "upstream", CapturedAt: 3},
	} {
		require.NoError(t, spool.capture(metadata, bytes.NewReader(payload), int64(len(payload)), policy, profile))
	}
	first := spool.claimDueJob(time.Now().Unix())
	require.NotNil(t, first)
	assert.EqualValues(t, 1, first.CapturedAt)
	second := spool.claimDueJob(time.Now().Unix())
	require.NotNil(t, second)
	assert.Equal(t, "two", second.RequestID)
	assert.Nil(t, spool.claimDueJob(time.Now().Unix()))
	spool.finishJob(first, nil)
	third := spool.claimDueJob(time.Now().Unix())
	require.NotNil(t, third)
	assert.EqualValues(t, 2, third.CapturedAt)
}

func TestRequestSnapshotPolicySupportsPermanentRetentionAndLargeSpool(t *testing.T) {
	profile := setupSnapshotDatabase(t)
	policy, err := SaveRequestSnapshotPolicy(PolicyInput{StorageProfileID: profile.ID, ObjectPrefix: "/snapshots/requests/",
		MaxFileBytes: 128 * 1024 * 1024, MaxTotalBytes: 4 * 1024 * 1024 * 1024, RetentionSeconds: 0, Enabled: true})
	require.NoError(t, err)
	assert.Equal(t, "snapshots/requests", policy.ObjectPrefix)
	assert.Zero(t, policy.RetentionSeconds)
	assert.Zero(t, policy.SignedURLTTLSeconds)
	assert.Equal(t, "application/json", policy.AllowedMIMETypes)
	for _, bad := range []PolicyInput{
		{ObjectPrefix: "snapshots", MaxFileBytes: 513 * 1024 * 1024, MaxTotalBytes: 1024 * 1024 * 1024},
		{ObjectPrefix: "snapshots", MaxFileBytes: 1, MaxTotalBytes: 65 * 1024 * 1024 * 1024},
		{ObjectPrefix: "snapshots", MaxFileBytes: 1, MaxTotalBytes: 1, RetentionSeconds: -1},
		{ObjectPrefix: strings.Repeat("a", 256), MaxFileBytes: 1, MaxTotalBytes: 1},
	} {
		_, err := SaveRequestSnapshotPolicy(bad)
		require.Error(t, err)
	}
	policy, err = SaveRequestSnapshotPolicy(PolicyInput{StorageProfileID: profile.ID, ObjectPrefix: strings.Repeat("a", 255),
		MaxFileBytes: 128 * 1024 * 1024, MaxTotalBytes: 4 * 1024 * 1024 * 1024, Enabled: true})
	require.NoError(t, err)
	assert.Len(t, policy.ObjectPrefix, 255)
	// SQLite does not enforce varchar bounds on older/directly written rows.
	// Refresh must reject such configuration before it can poison recovery.
	policy.ObjectPrefix = strings.Repeat("a", 64*1024)
	require.NoError(t, model.SaveStoragePolicy(policy))
	requestSnapshotPolicyCache.Lock()
	requestSnapshotPolicyCache.expires = time.Time{}
	requestSnapshotPolicyCache.Unlock()
	_, _, err = resolveRequestSnapshotPolicy()
	require.ErrorContains(t, err, "invalid snapshot object prefix")
}

func TestArchivedSnapshotReaderPreservesInlineFallback(t *testing.T) {
	profile := setupSnapshotDatabase(t)
	payload := []byte(`{"prompt":"legacy request"}`)
	require.NoError(t, model.SaveTaskRequestBody("inline-task", "inline-request", payload))
	snapshots, exists, err := ReadArchivedRequestSnapshots(context.Background(), "inline-task", "inline-request")
	require.NoError(t, err)
	assert.True(t, exists)
	assert.JSONEq(t, string(payload), string(snapshots.Original))
	assert.Empty(t, snapshots.Upstream)
	// The unrelated upstream object deliberately has no usable credentials.
	// Downloading the inline original must remain independent of that object.
	require.NoError(t, model.SaveExternalTaskRequestSnapshot("inline-task", "inline-request", "upstream", "test-node", profile.ID,
		"snapshots/unavailable-upstream.json", int64(len(payload)), fmt.Sprintf("%x", sha256.Sum256(payload)), model.TaskRequestEvidence{Version: 1}))
	body, exists, err := ReadArchivedRequestSnapshot(context.Background(), "inline-task", "inline-request", "original")
	require.NoError(t, err)
	assert.True(t, exists)
	assert.JSONEq(t, string(payload), string(body))
}

func TestSnapshotWorkerShutdownPreservesPendingJobsAndReleasesDirectory(t *testing.T) {
	profile := setupSnapshotDatabase(t)
	dir := t.TempDir()
	spool := snapshotTestSpool(t, dir)
	payload := []byte(`{"prompt":"pending on shutdown"}`)
	require.NoError(t, spool.capture(SnapshotCapture{RequestID: "req-shutdown", Kind: "original"}, bytes.NewReader(payload), int64(len(payload)), model.DefaultRequestSnapshotStoragePolicy(), profile))
	require.NoError(t, unlockSnapshotSpool(spool.lock))
	require.NoError(t, spool.lock.Close())
	t.Setenv("SNAPSHOT_SPOOL_DIR", dir)
	t.Setenv("SNAPSHOT_WORKERS", "2")
	require.NoError(t, StartRequestSnapshotWorkers())
	t.Cleanup(func() { require.NoError(t, StopRequestSnapshotWorkers(context.Background())) })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, StopRequestSnapshotWorkers(ctx))
	recovered := snapshotTestSpool(t, dir)
	assert.Len(t, recovered.jobs, 1, "shutdown must preserve accepted work until publication succeeds")
	assert.EqualValues(t, len(payload), recovered.bytes)
}

func TestSnapshotMigrationKeepsInlineBodyWhenArchiveCannotBeOpened(t *testing.T) {
	profile := setupSnapshotDatabase(t)
	payload := []byte(`{"prompt":"legacy database copy"}`)
	require.NoError(t, model.SaveTaskRequestBody("migration-task", "migration-request", payload))
	_, err := SaveRequestSnapshotPolicy(PolicyInput{StorageProfileID: profile.ID, ObjectPrefix: "snapshots/requests",
		MaxFileBytes: 128 * 1024 * 1024, MaxTotalBytes: 2 * 1024 * 1024 * 1024, Enabled: true})
	require.NoError(t, err)
	cursor, processed, done, err := MigrateDatabaseRequestSnapshots(context.Background(), 0, 20, true)
	require.Error(t, err, "test profile deliberately has no credential")
	assert.Zero(t, cursor)
	assert.Zero(t, processed)
	assert.False(t, done)
	body, exists, err := model.GetArchivedRequestBody("migration-task", "migration-request")
	require.NoError(t, err)
	assert.True(t, exists)
	assert.JSONEq(t, string(payload), string(body))
}

func TestArchiveTableSingleRowMigrationAndCleanupNeverAdvanceToAnotherID(t *testing.T) {
	profile := setupSnapshotDatabase(t)
	_, err := SaveRequestSnapshotPolicy(PolicyInput{StorageProfileID: profile.ID, ObjectPrefix: "snapshots/requests",
		MaxFileBytes: 128 * 1024 * 1024, MaxTotalBytes: 2 * 1024 * 1024 * 1024, Enabled: true})
	require.NoError(t, err)
	require.NoError(t, model.SaveTaskRequestBody("exact-first-task", "exact-first-request", []byte(`{"source":"first"}`)))
	require.NoError(t, model.SaveTaskRequestBody("exact-second-task", "exact-second-request", []byte(`{"source":"second"}`)))
	rows, err := model.FindDatabaseSnapshotBatch(0, 20)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	driver := &snapshotMemoryDriver{objects: make(map[string][]byte)}
	store := snapshotObjectStore{driver: driver}
	archived, err := store.archiveDatabaseRequestSnapshot(context.Background(), rows[0].ID, false, true)
	require.NoError(t, err)
	assert.True(t, archived)
	assert.Equal(t, 1, driver.puts)
	cleared, err := store.archiveDatabaseRequestSnapshot(context.Background(), rows[0].ID, true, false)
	require.NoError(t, err)
	assert.True(t, cleared)
	assert.Equal(t, 1, driver.puts, "verified cleanup must not re-upload")
	for _, upload := range []bool{true, false} {
		processed, err := store.archiveDatabaseRequestSnapshot(context.Background(), rows[0].ID, true, upload)
		require.NoError(t, err)
		assert.False(t, processed, "replaying a cleared ID must not act on the next row")
	}
	remaining, err := model.GetDatabaseSnapshotBody(rows[1].ID)
	require.NoError(t, err)
	assert.JSONEq(t, `{"source":"second"}`, string(remaining.Body))
	assert.Empty(t, remaining.ObjectKey)
	assert.Equal(t, 1, driver.puts)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	processed, err := MigrateDatabaseRequestSnapshot(ctx, rows[1].ID, true)
	require.ErrorIs(t, err, context.Canceled)
	assert.False(t, processed)
}

func TestSnapshotObjectFailureLoadsOnlyAnExactVerifiedInlineFallback(t *testing.T) {
	profile := setupSnapshotDatabase(t) // No credential: object reads fail locally.
	payload := []byte(`{"prompt":"retain the migration copy"}`)
	require.NoError(t, model.SaveTaskRequestBody("fallback-task", "fallback-request", payload))
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))
	require.NoError(t, model.SaveExternalTaskRequestSnapshot("fallback-task", "fallback-request", "original", "test-node",
		profile.ID, "snapshots/unavailable.json", int64(len(payload)), digest, model.TaskRequestEvidence{Version: 1}))
	records, err := model.GetSnapshotRecords("fallback-task", "fallback-request", true)
	require.NoError(t, err)
	require.Len(t, records, 1)
	metadata := records[0]
	assert.Empty(t, metadata.Body)
	body, err := readArchivedSnapshotRecord(context.Background(), metadata)
	require.NoError(t, err)
	assert.Equal(t, payload, body)

	for _, invalid := range [][]byte{append(append([]byte(nil), payload...), ' '), bytes.ReplaceAll(payload, []byte("retain"), []byte("change"))} {
		require.NoError(t, model.DB.Model(&model.TaskRequestBody{}).Where("id = ?", metadata.ID).Update("body", invalid).Error)
		body, err := readArchivedSnapshotRecord(context.Background(), metadata)
		require.Error(t, err, "fallback must match both the published size and SHA")
		assert.Empty(t, body)
	}
	require.NoError(t, model.DB.Model(&model.TaskRequestBody{}).Where("id = ?", metadata.ID).Update("body", nil).Error)
	body, err = readArchivedSnapshotRecord(context.Background(), metadata)
	require.Error(t, err, "a removed migration copy must not hide the unavailable object")
	assert.Empty(t, body)
}

func TestSnapshotCapturePreconditionFailureIsVisibleInNodeStatus(t *testing.T) {
	profile := setupSnapshotDatabase(t)
	requestSnapshotWorkers.Lock()
	savedError := requestSnapshotWorkers.lastError
	requestSnapshotWorkers.Unlock()
	t.Cleanup(func() {
		requestSnapshotWorkers.Lock()
		requestSnapshotWorkers.lastError = savedError
		requestSnapshotWorkers.Unlock()
	})
	t.Setenv("SNAPSHOT_SPOOL_DIR", t.TempDir())
	require.NoError(t, StartRequestSnapshotWorkers())
	t.Cleanup(func() { require.NoError(t, StopRequestSnapshotWorkers(context.Background())) })
	input := PolicyInput{StorageProfileID: profile.ID, ObjectPrefix: "snapshots/requests", MaxFileBytes: 128 * 1024 * 1024,
		MaxTotalBytes: 2 * 1024 * 1024 * 1024, Enabled: true}
	_, err := SaveRequestSnapshotPolicy(input)
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(profile).Update("status", model.StorageProfileStatusDisabled).Error)
	payload := []byte(`{"prompt":"capture continues to fail open"}`)
	err = CaptureRequestSnapshot(SnapshotCapture{RequestID: "req-precondition", Kind: "original"}, bytes.NewReader(payload), int64(len(payload)))
	require.ErrorContains(t, err, "profile is unavailable")
	status := SnapshotWorkerStatus()
	assert.Equal(t, err.Error(), status.LastError)
	assert.Zero(t, status.PendingFiles)
	assert.Zero(t, status.PendingBytes)
	input.Enabled = false
	_, err = SaveRequestSnapshotPolicy(input)
	require.NoError(t, err)
	err = CaptureRequestSnapshot(SnapshotCapture{RequestID: "req-disabled", Kind: "original"}, bytes.NewReader(payload), int64(len(payload)))
	require.ErrorIs(t, err, ErrRequestSnapshotsDisabled)
	assert.Equal(t, status.LastError, SnapshotWorkerStatus().LastError, "a disabled policy must not record a new failure")
}

func TestArchiveTableMigrationPreservesBothBoundSourceReferences(t *testing.T) {
	for _, requestFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("request-first=%t", requestFirst), func(t *testing.T) {
			profile := setupSnapshotDatabase(t)
			_, err := SaveRequestSnapshotPolicy(PolicyInput{StorageProfileID: profile.ID, ObjectPrefix: "snapshots/requests",
				MaxFileBytes: 128 * 1024 * 1024, MaxTotalBytes: 2 * 1024 * 1024 * 1024, Enabled: true})
			require.NoError(t, err)
			taskID, requestID := "archive-dual-task", "archive-dual-request"
			taskOriginal, taskUpstream := []byte(`{"source":"task-original"}`), []byte(`{"source":"task-upstream"}`)
			requestOriginal, requestUpstream := []byte(`{ "source": "request-original" }`), []byte(`{ "source": "request-upstream" }`)
			if requestFirst {
				require.NoError(t, model.SaveTaskRequestSnapshots("", requestID, requestOriginal, requestUpstream))
				require.NoError(t, model.BindTaskRequestSnapshots(taskID, requestID))
			}
			require.NoError(t, model.SaveTaskRequestSnapshots(taskID, requestID, taskOriginal, taskUpstream))
			if !requestFirst {
				require.NoError(t, model.SaveTaskRequestSnapshots("", requestID, requestOriginal, requestUpstream))
				require.NoError(t, model.BindTaskRequestSnapshots(taskID, requestID))
			}
			var originalRows []model.TaskRequestBody
			require.NoError(t, model.DB.Order("id ASC").Find(&originalRows).Error)
			require.Len(t, originalRows, 4)
			driver := &snapshotMemoryDriver{objects: make(map[string][]byte)}
			store := snapshotObjectStore{driver: driver}
			cursor, migrated, done, err := store.migrateDatabaseRequestSnapshots(context.Background(), 0, 20, true)
			require.NoError(t, err)
			assert.Equal(t, 4, migrated)
			assert.True(t, done)
			assert.Equal(t, originalRows[len(originalRows)-1].ID, cursor)
			for _, original := range originalRows {
				var migratedRow model.TaskRequestBody
				require.NoError(t, model.DB.First(&migratedRow, original.ID).Error)
				assert.Equal(t, original.ReferenceID, migratedRow.ReferenceID)
				assert.Equal(t, original.TaskID, migratedRow.TaskID)
				assert.Equal(t, original.RequestID, migratedRow.RequestID)
				assert.Equal(t, original.BodySHA256, migratedRow.BodySHA256)
				assert.Equal(t, []byte(original.Body), driver.objects[migratedRow.ObjectKey])
				assert.Equal(t, model.TaskRequestSnapshotReady, migratedRow.StorageState)
				assert.Empty(t, migratedRow.Body)
			}
			assert.Len(t, driver.objects, 4)
		})
	}
}

func TestLegacySnapshotMigrationPreservesHotBodiesWithoutDatabasePayloadStaging(t *testing.T) {
	profile := setupSnapshotDatabase(t)
	savedLogDB := model.LOG_DB
	model.LOG_DB = model.DB
	t.Cleanup(func() { model.LOG_DB = savedLogDB })
	require.NoError(t, model.LOG_DB.AutoMigrate(&model.Log{}))
	payload := []byte(`{"prompt":"legacy hot body"}`)
	task := &model.Task{TaskID: "hot-task", CreatedAt: 1, SubmitTime: 1, Properties: model.Properties{RequestBody: payload}}
	require.NoError(t, model.DB.Create(task).Error)
	other, err := common.Marshal(map[string]any{"request_body": string(payload), "unrelated": "keep"})
	require.NoError(t, err)
	log := &model.Log{CreatedAt: 1, RequestId: "hot-log-request", Other: string(other)}
	require.NoError(t, model.LOG_DB.Create(log).Error)
	_, err = SaveRequestSnapshotPolicy(PolicyInput{StorageProfileID: profile.ID, ObjectPrefix: "snapshots/requests",
		MaxFileBytes: 128 * 1024 * 1024, MaxTotalBytes: 2 * 1024 * 1024 * 1024, Enabled: true})
	require.NoError(t, err)
	_, processed, _, err := MigrateLegacyTaskRequestSnapshots(context.Background(), 0, task.ID, 20, true)
	require.Error(t, err, "the test profile deliberately has no credentials")
	assert.Zero(t, processed)
	upper, err := model.GetLogRequestBodyArchiveUpperCursor(context.Background())
	require.NoError(t, err)
	_, processed, _, err = MigrateLegacyLogRequestSnapshots(context.Background(), model.LogRequestBodyArchiveCursor{}, upper, 20, true)
	require.Error(t, err)
	assert.Zero(t, processed)
	var storedTask model.Task
	require.NoError(t, model.DB.First(&storedTask, task.ID).Error)
	assert.JSONEq(t, string(payload), string(storedTask.Properties.RequestBody))
	var storedLog model.Log
	require.NoError(t, model.LOG_DB.First(&storedLog, log.Id).Error)
	assert.JSONEq(t, string(other), storedLog.Other)
	var archivedPayloads int64
	require.NoError(t, model.DB.Model(&model.TaskRequestBody{}).Count(&archivedPayloads).Error)
	assert.Zero(t, archivedPayloads, "hot-table migration must not create an intermediate MySQL body row")
	_, processed, _, err = CleanupLegacyTaskRequestSnapshots(context.Background(), 0, task.ID, 20, true)
	require.NoError(t, err)
	assert.Zero(t, processed, "cleanup skips hot bodies that have no verified object")
	_, processed, _, err = CleanupLegacyLogRequestSnapshots(context.Background(), model.LogRequestBodyArchiveCursor{}, upper, 20, true)
	require.NoError(t, err)
	assert.Zero(t, processed)
}

func TestLegacyTaskJSONAndLogTextKeepDifferentOriginalBytesForTheSameTask(t *testing.T) {
	profile := setupSnapshotDatabase(t)
	savedLogDB := model.LOG_DB
	model.LOG_DB = model.DB
	t.Cleanup(func() { model.LOG_DB = savedLogDB })
	require.NoError(t, model.LOG_DB.AutoMigrate(&model.Log{}))
	_, err := SaveRequestSnapshotPolicy(PolicyInput{StorageProfileID: profile.ID, ObjectPrefix: "snapshots/requests",
		MaxFileBytes: 128 * 1024 * 1024, MaxTotalBytes: 2 * 1024 * 1024 * 1024, Enabled: true})
	require.NoError(t, err)
	canonical := []byte(`{"model":"video","prompt":"same request"}`)
	raw := []byte("{\n  \"prompt\": \"same request\",\n  \"model\": \"video\"\n}")
	assert.JSONEq(t, string(canonical), string(raw))
	task := &model.Task{TaskID: "same-task-source", CreatedAt: 1, SubmitTime: 1, Properties: model.Properties{RequestBody: canonical}}
	require.NoError(t, model.DB.Create(task).Error)
	other, err := common.Marshal(map[string]any{"request_body": string(raw), "task_id": task.TaskID, "unrelated": "keep"})
	require.NoError(t, err)
	log := &model.Log{CreatedAt: 1, RequestId: "same-request-source", Other: string(other)}
	require.NoError(t, model.LOG_DB.Create(log).Error)
	driver := &snapshotMemoryDriver{objects: make(map[string][]byte)}
	store := snapshotObjectStore{driver: driver}
	// Log-first recovery binds its request reference before a canonical task
	// reference exists. That fallback must not block the distinct task source.
	logDigest, archived, err := store.archiveLegacyRequestSnapshot(context.Background(), "", log.RequestId, log.CreatedAt, raw, true)
	require.NoError(t, err)
	assert.True(t, archived)
	require.NoError(t, model.BindTaskRequestSnapshots(task.TaskID, log.RequestId))
	taskDigest, archived, err := store.archiveLegacyRequestSnapshot(context.Background(), task.TaskID, "", task.SubmitTime, canonical, true)
	require.NoError(t, err)
	assert.True(t, archived)
	assert.NotEqual(t, taskDigest, logDigest)
	assert.Len(t, driver.objects, 2)
	taskRecords, err := model.GetSnapshotRecords(task.TaskID, "", true)
	require.NoError(t, err)
	require.Len(t, taskRecords, 1)
	assert.Equal(t, taskDigest, taskRecords[0].BodySHA256)
	assert.Equal(t, canonical, driver.objects[taskRecords[0].ObjectKey])
	logRecords, err := model.GetSnapshotRecords(task.TaskID, log.RequestId, true)
	require.NoError(t, err)
	require.Len(t, logRecords, 1)
	assert.Equal(t, logDigest, logRecords[0].BodySHA256)
	assert.Equal(t, raw, driver.objects[logRecords[0].ObjectKey])
	cleared, err := model.ClearLegacyTaskSnapshotBody(context.Background(), task.ID, taskDigest)
	require.NoError(t, err)
	assert.True(t, cleared)
	cleared, err = model.ClearLegacyLogSnapshotBody(context.Background(), model.LogRequestBodyArchiveRow{
		ID: log.Id, CreatedAt: log.CreatedAt, RequestID: log.RequestId, Other: log.Other,
	}, task.TaskID, logDigest)
	require.NoError(t, err)
	assert.True(t, cleared)
	var cleanedTask model.Task
	require.NoError(t, model.DB.First(&cleanedTask, task.ID).Error)
	assert.Empty(t, cleanedTask.Properties.RequestBody)
	var cleanedLog model.Log
	require.NoError(t, model.LOG_DB.First(&cleanedLog, log.Id).Error)
	var cleanedOther map[string]any
	require.NoError(t, common.UnmarshalJsonStr(cleanedLog.Other, &cleanedOther))
	assert.NotContains(t, cleanedOther, "request_body")
	assert.Equal(t, "keep", cleanedOther["unrelated"])
	var bodyRows int64
	require.NoError(t, model.DB.Model(&model.TaskRequestBody{}).Where("body IS NOT NULL").Count(&bodyRows).Error)
	assert.Zero(t, bodyRows, "both hot sources upload directly to objects with only database indexes")
}

func BenchmarkSnapshotCapture(b *testing.B) {
	for _, size := range []int{1 << 20, 8 << 20} {
		b.Run(fmt.Sprintf("%dMiB", size>>20), func(b *testing.B) {
			spool, err := openSnapshotSpool(b.TempDir(), 2)
			require.NoError(b, err)
			defer func() { spool.cancel(); _ = unlockSnapshotSpool(spool.lock); _ = spool.lock.Close() }()
			payload := []byte(`{"prompt":"` + strings.Repeat("x", size) + `"}`)
			policy, profile := model.DefaultRequestSnapshotStoragePolicy(), &model.StorageProfile{ID: 1}
			b.ReportAllocs()
			b.SetBytes(int64(len(payload)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				require.NoError(b, spool.capture(SnapshotCapture{RequestID: fmt.Sprintf("bench-%d", i), Kind: "original"}, bytes.NewReader(payload), int64(len(payload)), policy, profile))
				job := spool.claimDueJob(time.Now().Unix())
				require.NotNil(b, job)
				spool.finishJob(job, nil)
			}
		})
	}
}

var _ ObjectDriver = (*snapshotMemoryDriver)(nil)
