package common

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"time"

	"github.com/QuantumNous/new-api/pkg/runtimewatch"
	"github.com/go-redis/redis/v8"
)

// observeRedisOptions wraps the existing dial policy before client creation.
// Socket measurements include RESP framing and TLS records when TLS is enabled.
func observeRedisOptions(options *redis.Options) {
	if !runtimewatch.Enabled() {
		return
	}
	dial := options.Dialer
	options.Dialer = func(ctx context.Context, network, address string) (net.Conn, error) {
		if dial != nil {
			conn, err := dial(ctx, network, address)
			if err != nil {
				return nil, err
			}
			return runtimewatch.WrapConn("cache", conn), nil
		}
		timeout := options.DialTimeout
		if timeout == 0 {
			timeout = 5 * time.Second
		}
		conn, err := (&net.Dialer{Timeout: timeout, KeepAlive: 5 * time.Minute}).DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		conn = runtimewatch.WrapConn("cache", conn)
		if options.TLSConfig == nil {
			return conn, nil
		}
		tlsConn := tls.Client(conn, options.TLSConfig)
		handshakeCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		if err := tlsConn.HandshakeContext(handshakeCtx); err != nil {
			_ = conn.Close()
			return nil, err
		}
		return tlsConn, nil
	}
}

type runtimeRedisHook struct{}
type runtimeRedisCompletionKey struct{}

func (runtimeRedisHook) BeforeProcess(ctx context.Context, _ redis.Cmder) (context.Context, error) {
	return context.WithValue(ctx, runtimeRedisCompletionKey{}, runtimewatch.Begin("cache", "other")), nil
}

func (runtimeRedisHook) AfterProcess(ctx context.Context, command redis.Cmder) error {
	if complete, ok := ctx.Value(runtimeRedisCompletionKey{}).(func(int)); ok {
		status := 200
		if command.Err() != nil && !errors.Is(command.Err(), redis.Nil) {
			status = -1
		}
		complete(status)
	}
	return nil
}

func (runtimeRedisHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return context.WithValue(ctx, runtimeRedisCompletionKey{}, runtimewatch.Begin("cache", "other")), nil
}

func (runtimeRedisHook) AfterProcessPipeline(ctx context.Context, commands []redis.Cmder) error {
	if complete, ok := ctx.Value(runtimeRedisCompletionKey{}).(func(int)); ok {
		status := 200
		for _, command := range commands {
			if command.Err() != nil && !errors.Is(command.Err(), redis.Nil) {
				status = -1
				break
			}
		}
		complete(status)
	}
	return nil
}
