package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"
)

// lockForUpdate must emit FOR UPDATE on databases that support it and skip
// it on SQLite, where the syntax does not exist.
//
// The dummy dialector is used because SQLite drivers strip locking clauses
// from the generated SQL, which would mask what the helper itself does.
func TestLockForUpdateEmitsRowLock(t *testing.T) {
	dummyDB, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{DryRun: true})
	require.NoError(t, err)
	buildSQL := func() string {
		var rows []Redemption
		return lockForUpdate(dummyDB).Where("id = ?", 1).Find(&rows).Statement.SQL.String()
	}

	t.Cleanup(func() {
		common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	})

	common.SetDatabaseTypes(common.DatabaseTypeMySQL, common.DatabaseTypeSQLite)
	assert.Contains(t, buildSQL(), "FOR UPDATE")

	common.SetDatabaseTypes(common.DatabaseTypePostgreSQL, common.DatabaseTypeSQLite)
	assert.Contains(t, buildSQL(), "FOR UPDATE")

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	assert.NotContains(t, buildSQL(), "FOR UPDATE")
}

func TestLockForUpdateUsesIndependentLogDatabaseDialect(t *testing.T) {
	dummyDB, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{DryRun: true})
	require.NoError(t, err)
	mainType, logType := common.MainDatabaseType(), common.LogDatabaseType()
	t.Cleanup(func() { common.SetDatabaseTypes(mainType, logType) })
	for _, tc := range []struct {
		name      string
		main, log common.DatabaseType
		lock      bool
	}{
		{"mysql_main_sqlite_log", common.DatabaseTypeMySQL, common.DatabaseTypeSQLite, false},
		{"sqlite_main_mysql_log", common.DatabaseTypeSQLite, common.DatabaseTypeMySQL, true},
		{"sqlite_main_postgres_log", common.DatabaseTypeSQLite, common.DatabaseTypePostgreSQL, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			common.SetDatabaseTypes(tc.main, tc.log)
			var rows []Log
			sql := lockForUpdate(dummyDB, true).Where("id = ?", 1).Find(&rows).Statement.SQL.String()
			if tc.lock {
				assert.Contains(t, sql, "FOR UPDATE")
			} else {
				assert.NotContains(t, sql, "FOR UPDATE")
			}
		})
	}
}
