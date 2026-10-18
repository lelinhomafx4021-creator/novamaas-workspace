package model

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestSupplierVideoRunPersistenceAndOwnership(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("SUPPLIER_TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("SUPPLIER_TEST_MYSQL_DSN not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("SUPPLIER_TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("SUPPLIER_TEST_POSTGRES_DSN not configured")
				}
				driver = postgres.Open(dsn)
			}
			db, err := gorm.Open(driver, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "svr_fixture_"}})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })
			require.NoError(t, db.AutoMigrate(&SupplierVideoRun{}))
			t.Cleanup(func() { require.NoError(t, db.Migrator().DropTable(&SupplierVideoRun{})) })
			old := DB
			DB = db
			t.Cleanup(func() { DB = old })
			// Complete provider responses exceed MySQL's plain TEXT boundary.
			report := `{"response":"` + strings.Repeat("阶段证据", 22000) + `","n":9007199254740993}`
			row := &SupplierVideoRun{RunID: "svr_database", OwnerUserID: 7, Report: SupplierVideoDocument(report)}
			require.NoError(t, SaveSupplierVideoRun(context.Background(), row))
			loaded, err := GetSupplierVideoRun(context.Background(), row.RunID, 7)
			require.NoError(t, err)
			assert.Equal(t, report, string(loaded.Report))
			_, err = GetSupplierVideoRun(context.Background(), row.RunID, 8)
			require.ErrorIs(t, err, gorm.ErrRecordNotFound)
			stale := *loaded
			require.NoError(t, ClaimSupplierVideoVerification(context.Background(), loaded, `{"status":"running"}`))
			require.Error(t, ClaimSupplierVideoVerification(context.Background(), &stale, `{"status":"running-again"}`))
		})
	}
}
