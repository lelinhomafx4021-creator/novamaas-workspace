package model

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// SupplierVideoRun contains the immutable input and the latest completed stage
// evidence. Unabridged evidence belongs in private object storage.
type SupplierVideoRun struct {
	ID               int64                 `json:"-" gorm:"primaryKey"`
	RunID            string                `json:"id" gorm:"type:varchar(64);uniqueIndex"`
	OwnerUserID      int                   `json:"-" gorm:"index:idx_supplier_video_owner_created,priority:1"`
	Report           SupplierVideoDocument `json:"-"`
	EvidenceObjectID string                `json:"-" gorm:"type:varchar(64)"`
	CreatedAt        int64                 `json:"created_at" gorm:"index:idx_supplier_video_owner_created,priority:2"`
	UpdatedAt        int64                 `json:"updated_at"`
}

type SupplierVerificationCredential struct {
	OwnerUserID     int    `json:"-" gorm:"primaryKey;autoIncrement:false"`
	AccessKeyID     string `json:"-" gorm:"type:varchar(256)"`
	EncryptedSecret string `json:"-" gorm:"type:text"`
	UpdatedAt       int64  `json:"updated_at"`
}

func SaveSupplierVideoRun(ctx context.Context, row *SupplierVideoRun) error {
	row.UpdatedAt = time.Now().Unix()
	if row.ID == 0 {
		row.CreatedAt = row.UpdatedAt
		return DB.WithContext(ctx).Create(row).Error
	}
	return DB.WithContext(ctx).Model(&SupplierVideoRun{}).Where("id = ? AND owner_user_id = ?", row.ID, row.OwnerUserID).Updates(map[string]any{"report": row.Report, "evidence_object_id": row.EvidenceObjectID, "updated_at": row.UpdatedAt}).Error
}

func GetSupplierVideoRun(ctx context.Context, runID string, ownerID int) (*SupplierVideoRun, error) {
	var row SupplierVideoRun
	err := DB.WithContext(ctx).Where("run_id = ? AND owner_user_id = ?", runID, ownerID).First(&row).Error
	return &row, err
}

// ClaimSupplierVideoVerification serializes supplement attempts without holding
// a database transaction open during an upstream call.
func ClaimSupplierVideoVerification(ctx context.Context, row *SupplierVideoRun, next string) error {
	result := DB.WithContext(ctx).Model(&SupplierVideoRun{}).Where("id = ? AND owner_user_id = ? AND report = ?", row.ID, row.OwnerUserID, row.Report).Update("report", next)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrInvalidTransaction
	}
	row.Report = SupplierVideoDocument(next)
	return nil
}

// Reports can contain complete provider JSON beyond MySQL TEXT's 64 KiB limit.
// SQLite and PostgreSQL use their native unbounded TEXT representation.
type SupplierVideoDocument string

func (SupplierVideoDocument) GormDataType() string { return "text" }
func (SupplierVideoDocument) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	if db.Dialector.Name() == "mysql" {
		return "LONGTEXT"
	}
	return "TEXT"
}
