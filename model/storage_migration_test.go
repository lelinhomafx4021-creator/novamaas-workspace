package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type legacyStorageObjectWithoutUploadMetadata struct {
	ID       int64  `gorm:"primaryKey"`
	ObjectID string `gorm:"type:varchar(64);uniqueIndex"`
	Status   string `gorm:"type:varchar(32)"`
}

func (legacyStorageObjectWithoutUploadMetadata) TableName() string {
	return "storage_objects"
}

func TestEnsureStorageObjectUploadMetadataColumnsUpgradesLegacyTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&legacyStorageObjectWithoutUploadMetadata{}))
	require.False(t, db.Migrator().HasColumn(&StorageObject{}, "ETag"))
	require.False(t, db.Migrator().HasColumn(&StorageObject{}, "SHA256"))

	require.NoError(t, ensureStorageObjectUploadMetadataColumns(db))
	require.True(t, db.Migrator().HasColumn(&StorageObject{}, "ETag"))
	require.True(t, db.Migrator().HasColumn(&StorageObject{}, "SHA256"))

	// Startup migrations must remain safe when the schema is already current.
	require.NoError(t, ensureStorageObjectUploadMetadataColumns(db))
}
