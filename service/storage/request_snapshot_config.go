package storage

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

var ErrRequestSnapshotsDisabled = errors.New("request snapshot storage is disabled")

var requestSnapshotPolicyCache struct {
	sync.Mutex
	policy   *model.StoragePolicy
	profile  *model.StorageProfile
	err      error
	expires  time.Time
	database *gorm.DB
}

func GetRequestSnapshotPolicy(contexts ...context.Context) (*model.StoragePolicy, error) {
	policy, err := model.GetStoragePolicyByKey(model.StoragePolicyRequestSnapshot, contexts...)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return model.DefaultRequestSnapshotStoragePolicy(), nil
	}
	return policy, nil
}

func SaveRequestSnapshotPolicy(input PolicyInput) (*model.StoragePolicy, error) {
	policy := &model.StoragePolicy{
		Key: model.StoragePolicyRequestSnapshot, Name: "Request snapshot storage",
		Purpose: model.StorageObjectPurposeRequestSnapshot, StorageProfileID: input.StorageProfileID,
		ObjectPrefix:     strings.Trim(strings.TrimSpace(input.ObjectPrefix), "/"),
		RetentionSeconds: input.RetentionSeconds, MaxFileBytes: input.MaxFileBytes,
		MaxTotalBytes: input.MaxTotalBytes, MaxFiles: 2, AllowedMIMETypes: "application/json", Enabled: input.Enabled,
	}
	if len(policy.ObjectPrefix) == 0 || len(policy.ObjectPrefix) > 255 || strings.Contains(policy.ObjectPrefix, "..") || strings.ContainsAny(policy.ObjectPrefix, "\\\x00") {
		return nil, errors.New("invalid snapshot object prefix")
	}
	if policy.MaxFileBytes <= 0 || policy.MaxFileBytes > 512*1024*1024 {
		return nil, errors.New("snapshot size limit must be between 1 byte and 512 MiB")
	}
	if policy.MaxTotalBytes < policy.MaxFileBytes || policy.MaxTotalBytes > 64*1024*1024*1024 {
		return nil, errors.New("snapshot spool limit must be at least the file limit and no more than 64 GiB")
	}
	if policy.RetentionSeconds < 0 || policy.RetentionSeconds > 3650*24*60*60 {
		return nil, errors.New("snapshot retention must be between 0 and 3650 days")
	}
	if policy.StorageProfileID <= 0 && policy.Enabled {
		return nil, errors.New("an enabled snapshot policy requires a storage profile")
	}
	if policy.StorageProfileID > 0 {
		profile, err := model.GetStorageProfileByID(policy.StorageProfileID)
		if err != nil {
			return nil, err
		}
		if profile == nil || profile.Status == model.StorageProfileStatusArchived {
			return nil, errors.New("snapshot storage profile does not exist")
		}
		if policy.Enabled && profile.Status != model.StorageProfileStatusEnabled {
			return nil, errors.New("snapshot storage profile must be enabled")
		}
	}
	if err := model.SaveStoragePolicy(policy); err != nil {
		return nil, err
	}
	requestSnapshotPolicyCache.Lock()
	requestSnapshotPolicyCache.expires = time.Time{}
	requestSnapshotPolicyCache.Unlock()
	return policy, nil
}

// Other nodes see policy changes within two seconds; only small configuration
// records are read on the request path, never archived request payloads.
func resolveRequestSnapshotPolicy(contexts ...context.Context) (*model.StoragePolicy, *model.StorageProfile, error) {
	ctx := context.Background()
	if len(contexts) > 0 && contexts[0] != nil {
		ctx = contexts[0]
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if ctx.Done() == nil {
		requestSnapshotPolicyCache.Lock()
	} else if !requestSnapshotPolicyCache.TryLock() {
		// Only contended deadline-aware callers need a timer. A canceled
		// maintenance item must not wait behind another caller's policy refresh.
		ticker := time.NewTicker(5 * time.Millisecond)
		acquired := false
		for !acquired {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return nil, nil, ctx.Err()
			case <-ticker.C:
				acquired = requestSnapshotPolicyCache.TryLock()
			}
		}
		ticker.Stop()
	}
	defer requestSnapshotPolicyCache.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if model.DB == requestSnapshotPolicyCache.database && time.Now().Before(requestSnapshotPolicyCache.expires) {
		return requestSnapshotPolicyCache.policy, requestSnapshotPolicyCache.profile, requestSnapshotPolicyCache.err
	}
	policy, err := GetRequestSnapshotPolicy(ctx)
	var profile *model.StorageProfile
	if err == nil && !policy.Enabled {
		err = ErrRequestSnapshotsDisabled
	}
	if err == nil && (len(policy.ObjectPrefix) == 0 || len(policy.ObjectPrefix) > 255 || strings.Contains(policy.ObjectPrefix, "..") || strings.ContainsAny(policy.ObjectPrefix, "\\\x00")) {
		err = errors.New("invalid snapshot object prefix")
	}
	if err == nil {
		profile, err = model.GetStorageProfileByID(policy.StorageProfileID, ctx)
		if err == nil && (profile == nil || profile.Status != model.StorageProfileStatusEnabled) {
			err = errors.New("snapshot storage profile is unavailable")
		}
	}
	if contextErr := ctx.Err(); contextErr != nil {
		return policy, profile, contextErr
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return policy, profile, err
	}
	requestSnapshotPolicyCache.policy, requestSnapshotPolicyCache.profile, requestSnapshotPolicyCache.err = policy, profile, err
	requestSnapshotPolicyCache.expires = time.Now().Add(2 * time.Second)
	requestSnapshotPolicyCache.database = model.DB
	return policy, profile, err
}

func RequestSnapshotPolicyEnabled() bool {
	policy, _, _ := resolveRequestSnapshotPolicy()
	return policy != nil && policy.Enabled
}
