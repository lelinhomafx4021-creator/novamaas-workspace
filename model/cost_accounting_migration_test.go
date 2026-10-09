package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCostAccountingAdjustmentMigrationPreservesLegacyRowsAndEventUniqueness(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE cost_accounting_adjustments ("id" integer PRIMARY KEY, "snapshot_id" integer, "delta_cost_quota" bigint, "new_cost_quota" bigint, "cost_discount" varchar(16), "reason" varchar(500), "actor_id" integer, "batch_id" varchar(64), "created_at" bigint)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO cost_accounting_adjustments (id, snapshot_id, delta_cost_quota, new_cost_quota, reason) VALUES (1, 42, -5, 10, 'legacy correction'), (2, 43, -2, 8, 'second correction')`).Error)
	require.NoError(t, ensureCostAccountingAdjustmentEventKeyColumn(db))
	require.NoError(t, db.AutoMigrate(&CostAccountingAdjustment{}))
	var old CostAccountingAdjustment
	require.NoError(t, db.First(&old, 1).Error)
	assert.Equal(t, int64(42), old.SnapshotID)
	assert.Equal(t, int64(-5), old.DeltaCostQuota)
	assert.Equal(t, int64(10), old.NewCostQuota)
	assert.Equal(t, "legacy correction", old.Reason)
	assert.Nil(t, old.EventKey)
	eventKey := "correction-1"
	require.NoError(t, db.Create(&CostAccountingAdjustment{EventKey: &eventKey}).Error)
	assert.Error(t, db.Create(&CostAccountingAdjustment{EventKey: &eventKey}).Error)
	require.NoError(t, db.Create(&CostAccountingAdjustment{}).Error)
	require.NoError(t, ensureCostAccountingAdjustmentEventKeyColumn(db))
	require.NoError(t, db.AutoMigrate(&CostAccountingAdjustment{}))
	var count int64
	require.NoError(t, db.Model(&CostAccountingAdjustment{}).Count(&count).Error)
	assert.Equal(t, int64(4), count)
}
