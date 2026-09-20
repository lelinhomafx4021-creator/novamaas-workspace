package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type legacyChannelWithoutCostDiscount struct {
	ID int `gorm:"primaryKey"`
}

func (legacyChannelWithoutCostDiscount) TableName() string {
	return "channels"
}

func TestEnsureChannelCostDiscountColumnUpgradesLegacyTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&legacyChannelWithoutCostDiscount{}))
	require.False(t, db.Migrator().HasColumn(&Channel{}, "CostDiscount"))

	require.NoError(t, ensureChannelCostDiscountColumn(db))
	require.True(t, db.Migrator().HasColumn(&Channel{}, "CostDiscount"))

	// Startup migrations must remain safe when the schema is already current.
	require.NoError(t, ensureChannelCostDiscountColumn(db))
}
