package common

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"testing"

	"github.com/QuantumNous/new-api/pkg/runtimewatch"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runtimeCacheTraffic(t *testing.T) runtimewatch.TrafficRow {
	t.Helper()
	for _, row := range runtimewatch.Snapshot(1).Traffic {
		if row.Scope == "cache" && row.Category == "other" {
			return row
		}
	}
	return runtimewatch.TrafficRow{Scope: "cache", Category: "other"}
}

func TestDisabledRedisMonitoringPreservesConfiguredDialer(t *testing.T) {
	previousEnabled := runtimewatch.Enabled()
	runtimewatch.SetEnabled(false)
	t.Cleanup(func() { runtimewatch.SetEnabled(previousEnabled) })
	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
	options := &redis.Options{Dialer: func(context.Context, string, string) (net.Conn, error) { return client, nil }}
	before := runtimewatch.Snapshot(1).Traffic
	observeRedisOptions(options)
	connection, err := options.Dialer(context.Background(), "tcp", "redis.invalid:6379")
	require.NoError(t, err)
	assert.Same(t, client, connection)
	require.NoError(t, connection.Close())
	assert.Equal(t, before, runtimewatch.Snapshot(1).Traffic)
	defaults := &redis.Options{TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	observeRedisOptions(defaults)
	assert.Nil(t, defaults.Dialer, "the Redis client must retain responsibility for configuring its native TLS dialer")
}

func TestRedisDialerCountsActualSocketBytesAndLifetime(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
	options := &redis.Options{Dialer: func(context.Context, string, string) (net.Conn, error) { return client, nil }}
	observeRedisOptions(options)
	before := runtimeCacheTraffic(t)
	conn, err := options.Dialer(context.Background(), "tcp", "redis.invalid:6379")
	require.NoError(t, err)
	assert.Equal(t, before.Connections+1, runtimeCacheTraffic(t).Connections)
	writeResult := make(chan error, 1)
	go func() { _, err := conn.Write([]byte("PING\r\n")); writeResult <- err }()
	request := make([]byte, 6)
	_, err = io.ReadFull(server, request)
	require.NoError(t, err)
	require.NoError(t, <-writeResult)
	go func() { _, err := server.Write([]byte("+PONG\r\n")); writeResult <- err }()
	response := make([]byte, 7)
	_, err = io.ReadFull(conn, response)
	require.NoError(t, err)
	require.NoError(t, <-writeResult)
	require.NoError(t, conn.Close())
	require.NoError(t, conn.Close())
	after := runtimeCacheTraffic(t)
	assert.Equal(t, before.SentBytes+6, after.SentBytes)
	assert.Equal(t, before.ReceivedBytes+7, after.ReceivedBytes)
	assert.Equal(t, before.Connections, after.Connections)
}

func TestRedisHookCountsMissingKeysAndFailedPipelines(t *testing.T) {
	before := runtimeCacheTraffic(t)
	hook := runtimeRedisHook{}
	missing := redis.NewStringCmd(context.Background(), "get", "missing")
	ctx, err := hook.BeforeProcess(context.Background(), missing)
	require.NoError(t, err)
	missing.SetErr(redis.Nil)
	require.NoError(t, hook.AfterProcess(ctx, missing))
	failed := redis.NewStatusCmd(context.Background(), "set", "key", "value")
	commands := []redis.Cmder{failed}
	ctx, err = hook.BeforeProcessPipeline(context.Background(), commands)
	require.NoError(t, err)
	failed.SetErr(io.ErrUnexpectedEOF)
	require.NoError(t, hook.AfterProcessPipeline(ctx, commands))
	after := runtimeCacheTraffic(t)
	assert.Equal(t, before.Requests+2, after.Requests)
	assert.Equal(t, before.Completed+2, after.Completed)
	assert.Equal(t, before.Errors+1, after.Errors)
	assert.Equal(t, before.Active, after.Active)
}
