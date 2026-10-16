package model

import (
	"testing"

	"github.com/QuantumNous/new-api/pkg/runtimewatch"
	"github.com/glebarez/sqlite"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestObservedMySQLDialectorPreservesAuthenticationAndConnectionSettings(t *testing.T) {
	for _, test := range []struct {
		name    string
		dsn     string
		network string
	}{
		{
			name:    "unix retains secure socket authentication",
			dsn:     "monitor:p@ss:word@unix(/tmp/runtimewatch-mysql.sock)/test%2Fname?parseTime=true&loc=Asia%2FShanghai&timeout=2s&readTimeout=3s&writeTimeout=4s&charset=utf8mb4",
			network: "unix",
		},
		{
			name:    "unix retains explicit TLS and credentials",
			dsn:     "monitor:p@ss:word@unix(/tmp/runtimewatch-mysql.sock)/test%2Fname?tls=skip-verify&timeout=2s",
			network: "unix",
		},
		{
			name:    "TCP retains verified TLS and connection options",
			dsn:     "monitor:p@ss:word@tcp(db.example:3306)/test%2Fname?tls=true&parseTime=true&loc=Asia%2FShanghai&timeout=2s&readTimeout=3s&writeTimeout=4s&charset=utf8mb4",
			network: "runtimewatch+tcp",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			expected, err := mysqldriver.ParseDSN(test.dsn)
			require.NoError(t, err)
			expected.Net = test.network
			dialector, err := observedMySQLDialector(test.dsn)
			require.NoError(t, err)
			observed, ok := dialector.(*mysql.Dialector)
			require.True(t, ok)
			// The driver selects the caching_sha2_password secure full-auth path
			// from Net=="unix"; changing this name would require RSA exchange.
			assert.Equal(t, test.network, observed.DSNConfig.Net)
			assert.Equal(t, expected, observed.DSNConfig)
			reparsed, err := mysqldriver.ParseDSN(observed.DSN)
			require.NoError(t, err)
			assert.Equal(t, expected, reparsed)
		})
	}
}

func TestDisabledRuntimeDatabasePreservesNativeConnectionSettings(t *testing.T) {
	previousEnabled := runtimewatch.Enabled()
	runtimewatch.SetEnabled(false)
	t.Cleanup(func() { runtimewatch.SetEnabled(previousEnabled) })
	for _, dsn := range []string{
		"monitor:p@ss:word@tcp(db.example:3306)/test%2Fname?tls=true&parseTime=true&loc=Asia%2FShanghai&timeout=2s&readTimeout=3s&writeTimeout=4s",
		"monitor:p@ss:word@unix(/tmp/runtimewatch-mysql.sock)/test%2Fname?parseTime=true&timeout=2s",
	} {
		expected, err := mysqldriver.ParseDSN(dsn)
		require.NoError(t, err)
		dialector, err := observedMySQLDialector(dsn)
		require.NoError(t, err)
		native, ok := dialector.(*mysql.Dialector)
		require.True(t, ok)
		assert.Equal(t, dsn, native.DSN)
		assert.Equal(t, expected, native.DSNConfig)
	}
	dsn := "postgres://monitor:password@db.example:5432/runtimewatch?sslmode=verify-full&connect_timeout=2&TimeZone=Asia%2FShanghai"
	dialector, err := observedPostgresDialector(dsn)
	require.NoError(t, err)
	native, ok := dialector.(*postgres.Dialector)
	require.True(t, ok)
	assert.Equal(t, dsn, native.DSN)
	assert.True(t, native.PreferSimpleProtocol)
	assert.Nil(t, native.Conn, "disabled monitoring must retain the PostgreSQL driver's native connection creation")
}

func TestDisabledRuntimeDatabaseExecutesWithoutRecording(t *testing.T) {
	previousEnabled := runtimewatch.Enabled()
	runtimewatch.SetEnabled(false)
	t.Cleanup(func() { runtimewatch.SetEnabled(previousEnabled) })
	before := runtimewatch.Snapshot(1).Traffic
	db, err := gorm.Open(sqlite.Open(":memory:"), newGormConfig(false))
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	var result int
	require.NoError(t, db.Raw("SELECT 42").Scan(&result).Error)
	assert.Equal(t, 42, result)
	require.Error(t, db.Exec("SELECT missing_runtimewatch_column").Error)
	assert.Equal(t, before, runtimewatch.Snapshot(1).Traffic)
}

func runtimeDatabaseTraffic(t *testing.T) runtimewatch.TrafficRow {
	t.Helper()
	for _, row := range runtimewatch.Snapshot(1).Traffic {
		if row.Scope == "database" && row.Category == "other" {
			return row
		}
	}
	return runtimewatch.TrafficRow{Scope: "database", Category: "other"}
}

func TestRuntimeDatabaseTracksSQLiteOperationsWithoutNetworkTraffic(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), newGormConfig(false))
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	type runtimeTrafficRecord struct {
		ID    int `gorm:"primaryKey"`
		Value string
	}
	require.NoError(t, db.AutoMigrate(&runtimeTrafficRecord{}))
	before := runtimeDatabaseTraffic(t)
	require.NoError(t, db.Create(&runtimeTrafficRecord{ID: 1, Value: "stored"}).Error)
	var found runtimeTrafficRecord
	require.NoError(t, db.First(&found, 1).Error)
	assert.Equal(t, "stored", found.Value)
	require.ErrorIs(t, db.First(&runtimeTrafficRecord{}, 2).Error, gorm.ErrRecordNotFound)
	require.Error(t, db.Exec("SELECT missing_runtimewatch_column FROM runtime_traffic_records").Error)
	after := runtimeDatabaseTraffic(t)
	assert.Equal(t, before.Requests+4, after.Requests)
	assert.Equal(t, before.Completed+4, after.Completed)
	assert.Equal(t, before.Errors+1, after.Errors)
	assert.Equal(t, before.Active, after.Active)
	assert.Equal(t, before.ReceivedBytes, after.ReceivedBytes)
	assert.Equal(t, before.SentBytes, after.SentBytes)
}
