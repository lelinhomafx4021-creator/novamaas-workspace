package model

import (
	"context"
	"errors"
	"net"
	"regexp"
	"sync"

	"github.com/QuantumNous/new-api/pkg/runtimewatch"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var runtimeMySQLDialers sync.Once

// observedMySQLDialector preserves the parsed DSN (including TLS, timeouts and
// escaped credentials) and instruments TCP networks only. Keep the native unix
// network name: the MySQL driver uses it to select secure local authentication.
func observedMySQLDialector(dsn string) (gorm.Dialector, error) {
	if !runtimewatch.Enabled() {
		return mysql.Open(dsn), nil
	}
	config, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	runtimeMySQLDialers.Do(func() {
		for _, network := range []string{"tcp", "tcp4", "tcp6"} {
			mysqldriver.RegisterDialContext("runtimewatch+"+network, func(ctx context.Context, address string) (net.Conn, error) {
				conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
				if err != nil {
					return nil, err
				}
				if tcp, ok := conn.(*net.TCPConn); ok {
					if err := tcp.SetKeepAlive(true); err != nil {
						_ = conn.Close()
						return nil, err
					}
				}
				return runtimewatch.WrapConn("database", conn), nil
			})
		}
	})
	switch config.Net {
	case "tcp", "tcp4", "tcp6":
		config.Net = "runtimewatch+" + config.Net
	}
	return mysql.New(mysql.Config{DSN: config.FormatDSN(), DSNConfig: config}), nil
}

var postgresTimeZonePattern = regexp.MustCompile(`(time_zone|TimeZone)=(.*?)($|&| )`)

func observedPostgresDialector(dsn string) (gorm.Dialector, error) {
	if !runtimewatch.Enabled() {
		return postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), nil
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	// Preserve the GORM postgres driver's legacy time_zone/TimeZone mapping.
	if match := postgresTimeZonePattern.FindStringSubmatch(dsn); len(match) > 2 {
		config.RuntimeParams["timezone"] = match[2]
	}
	dial := config.DialFunc
	config.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dial(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return runtimewatch.WrapConn("database", conn), nil
	}
	return postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true, Conn: stdlib.OpenDB(*config)}), nil
}

type runtimeDatabasePlugin struct{}

func (runtimeDatabasePlugin) Name() string { return "runtimewatch" }

func (runtimeDatabasePlugin) Initialize(db *gorm.DB) error {
	for _, register := range []func(string, func(*gorm.DB)) error{
		db.Callback().Create().Before("*").Register,
		db.Callback().Query().Before("*").Register,
		db.Callback().Update().Before("*").Register,
		db.Callback().Delete().Before("*").Register,
		db.Callback().Row().Before("*").Register,
		db.Callback().Raw().Before("*").Register,
	} {
		if err := register("runtimewatch:begin", beginRuntimeDatabaseOperation); err != nil {
			return err
		}
	}
	for _, register := range []func(string, func(*gorm.DB)) error{
		db.Callback().Create().After("*").Register,
		db.Callback().Query().After("*").Register,
		db.Callback().Update().After("*").Register,
		db.Callback().Delete().After("*").Register,
		db.Callback().Row().After("*").Register,
		db.Callback().Raw().After("*").Register,
	} {
		if err := register("runtimewatch:end", endRuntimeDatabaseOperation); err != nil {
			return err
		}
	}
	return nil
}

func beginRuntimeDatabaseOperation(db *gorm.DB) {
	if !db.DryRun {
		db.Statement.Settings.Store("runtimewatch:completion", runtimewatch.Begin("database", "other"))
	}
}

func endRuntimeDatabaseOperation(db *gorm.DB) {
	if complete, ok := db.Statement.Settings.LoadAndDelete("runtimewatch:completion"); ok {
		status := 200
		if db.Error != nil && !errors.Is(db.Error, gorm.ErrRecordNotFound) {
			status = -1
		}
		complete.(func(int))(status)
	}
}
