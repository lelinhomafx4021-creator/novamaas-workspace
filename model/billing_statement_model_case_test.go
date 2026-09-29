package model

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestBillingStatementKeepsCaseDistinctModelNames(t *testing.T) {
	id := seedBillingCustomer(t)
	start, _, err := BillingMonthBounds("2020-02")
	require.NoError(t, err)
	require.NoError(t, DB.Model(&BillingAccount{}).Where("user_id = ?", id).Update("accounting_start_at", start).Error)
	for _, entry := range []BillingEntry{
		{EventKey: "case-upper", UserID: id, Sequence: 1, PostedAt: start, Kind: "usage", ModelName: "Foo", Quota: 20},
		{EventKey: "case-lower", UserID: id, Sequence: 2, PostedAt: start + 1, Kind: "usage", ModelName: "foo", Quota: 30},
		{EventKey: "case-upper-refund", UserID: id, Sequence: 3, PostedAt: start + 86400, Kind: "refund", ModelName: "Foo", Quota: -5},
	} {
		require.NoError(t, DB.Create(&entry).Error)
	}
	require.NoError(t, DB.Model(&BillingAccount{}).Where("user_id = ?", id).Update("sequence", 3).Error)
	for _, hour := range []BillingHour{
		{UserID: id, Hour: start, Charge: 50, Count: 2, FirstSequence: 1, LastSequence: 2},
		{UserID: id, Hour: start + 86400, Refund: 5, Count: 1, FirstSequence: 3, LastSequence: 3},
	} {
		require.NoError(t, DB.Create(&hour).Error)
	}

	statement := &BillingStatement{ID: "case-distinct-models", UserID: id, Month: "2020-02"}
	var models []BillingModelTotal
	require.NoError(t, CreateBillingStatement(statement, func(_ *BillingAccount, _ []BillingHour, totals []BillingModelTotal) (string, string, error) {
		models = append(models, totals...)
		return "{}", "", nil
	}))
	require.Len(t, models, 2)
	assert.Equal(t, BillingModelTotal{ModelName: "Foo", Charge: 20, Refund: 5, Count: 2, ChargeCount: 1, RefundCount: 1, ActiveDays: 2, FirstPosted: start, LastPosted: start + 86400}, models[0])
	assert.Equal(t, BillingModelTotal{ModelName: "foo", Charge: 30, Count: 1, ChargeCount: 1, ActiveDays: 1, FirstPosted: start + 1, LastPosted: start + 1}, models[1])
}

func TestBillingStatementKeepsCaseDistinctModelsUnderMySQLCICollation(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN is not configured")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	tableName := fmt.Sprintf("test_billing_model_ci_%d", time.Now().UnixNano())
	require.NoError(t, db.Table(tableName).AutoMigrate(&BillingEntry{}))
	t.Cleanup(func() { _ = db.Migrator().DropTable(tableName) })
	require.NoError(t, db.Exec("ALTER TABLE `"+tableName+"` CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci").Error)
	for _, entry := range []BillingEntry{
		{EventKey: "case-upper", UserID: 1, Sequence: 1, PostedAt: 10, Kind: "usage", ModelName: "Foo", Quota: 20},
		{EventKey: "case-lower", UserID: 1, Sequence: 2, PostedAt: 11, Kind: "usage", ModelName: "foo", Quota: 30},
	} {
		require.NoError(t, db.Table(tableName).Create(&entry).Error)
	}
	var distinct int64
	require.NoError(t, db.Table(tableName).Distinct("model_name").Count(&distinct).Error)
	require.Equal(t, int64(1), distinct, "the fixture must exercise a case-insensitive database collation")

	models, err := summarizeBillingModels(db.Table(tableName), 1, 1, 2, 0, 100)
	require.NoError(t, err)
	require.Len(t, models, 2)
	assert.Equal(t, BillingModelTotal{ModelName: "Foo", Charge: 20, Count: 1, ChargeCount: 1, ActiveDays: 1, FirstPosted: 10, LastPosted: 10}, models[0])
	assert.Equal(t, BillingModelTotal{ModelName: "foo", Charge: 30, Count: 1, ChargeCount: 1, ActiveDays: 1, FirstPosted: 11, LastPosted: 11}, models[1])
}
