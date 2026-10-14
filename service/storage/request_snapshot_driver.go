package storage

import (
	"context"
	"crypto/sha256"
	"errors"
	"sync"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

type snapshotDriverIdentity struct {
	ProfileID         int
	Provider          string
	Endpoint          string
	Region            string
	Bucket            string
	ProfileConfig     [32]byte
	CredentialID      int64
	CredentialVersion int
	AuthType          string
	KeyVersion        string
	CredentialPayload [32]byte
	EncryptionKey     [32]byte
}

type snapshotDriverEntry struct {
	identity snapshotDriverIdentity
	driver   ObjectDriver
	lastUsed uint64
}

var requestSnapshotDriverCache struct {
	sync.Mutex
	database *gorm.DB
	entries  map[int]snapshotDriverEntry
	sequence uint64
}

// Metadata is deliberately read on every open. Reusing a connection pool must
// not delay credential rotation or permit reads through an archived profile.
func openSnapshotDriver(ctx context.Context, profileID int) (*model.StorageProfile, ObjectDriver, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	profile, err := model.GetStorageProfileByID(profileID, ctx)
	if err != nil {
		return nil, nil, err
	}
	if profile == nil || profile.Status == model.StorageProfileStatusArchived {
		return nil, nil, errors.New("snapshot storage profile is unavailable")
	}
	credential, err := model.GetActiveStorageCredential(profileID, ctx)
	if err != nil {
		return nil, nil, err
	}
	if credential == nil {
		return nil, nil, errors.New("storage credential is required")
	}
	identity := snapshotDriverIdentity{
		ProfileID: profile.ID, Provider: profile.ProviderType, Endpoint: profile.Endpoint,
		Region: profile.Region, Bucket: profile.Bucket, ProfileConfig: sha256.Sum256([]byte(profile.Config)),
		CredentialID: credential.ID, CredentialVersion: credential.Version, AuthType: credential.AuthType,
		KeyVersion: credential.KeyVersion, CredentialPayload: sha256.Sum256([]byte(credential.EncryptedPayload)),
	}
	if credential.AuthType == model.StorageCredentialAuthStatic {
		key, err := storageCredentialEncryptionKey()
		if err != nil {
			return nil, nil, err
		}
		identity.EncryptionKey = sha256.Sum256(key)
	}
	cache := &requestSnapshotDriverCache
	cache.Lock()
	defer cache.Unlock()
	if cache.database != model.DB {
		for _, entry := range cache.entries {
			if closer, ok := entry.driver.(interface{ CloseIdleConnections() }); ok {
				closer.CloseIdleConnections()
			}
		}
		cache.database, cache.entries = model.DB, make(map[int]snapshotDriverEntry)
	}
	cache.sequence++
	entry, exists := cache.entries[profileID]
	if exists && entry.identity == identity {
		entry.lastUsed = cache.sequence
		cache.entries[profileID] = entry
		return profile, entry.driver, nil
	}
	driver, err := driverForProfile(profile, credential)
	if err != nil {
		return profile, nil, err
	}
	if exists {
		if closer, ok := entry.driver.(interface{ CloseIdleConnections() }); ok {
			closer.CloseIdleConnections()
		}
	} else if len(cache.entries) >= 64 {
		var oldestID int
		var oldest snapshotDriverEntry
		for id, candidate := range cache.entries {
			if oldestID == 0 || candidate.lastUsed < oldest.lastUsed {
				oldestID, oldest = id, candidate
			}
		}
		if closer, ok := oldest.driver.(interface{ CloseIdleConnections() }); ok {
			closer.CloseIdleConnections()
		}
		delete(cache.entries, oldestID)
	}
	cache.entries[profileID] = snapshotDriverEntry{identity: identity, driver: driver, lastUsed: cache.sequence}
	return profile, driver, nil
}
