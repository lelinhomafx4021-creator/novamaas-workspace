package storage

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSnapshotPolicyContextTest(t *testing.T) *model.StorageProfile {
	t.Helper()
	profile := setupSnapshotDatabase(t)
	_, err := SaveRequestSnapshotPolicy(PolicyInput{StorageProfileID: profile.ID, ObjectPrefix: "snapshots/requests",
		MaxFileBytes: 128 * 1024 * 1024, MaxTotalBytes: 2 * 1024 * 1024 * 1024, Enabled: true})
	require.NoError(t, err)
	return profile
}

func TestSnapshotCanceledPolicyRefreshDoesNotPoisonOtherRequests(t *testing.T) {
	for _, canceledQuery := range []string{"StoragePolicy", "StorageProfile"} {
		t.Run(canceledQuery, func(t *testing.T) {
			profile := setupSnapshotPolicyContextTest(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			require.NoError(t, model.DB.Callback().Query().Before("gorm:query").Register("snapshot:cancel_policy_refresh", func(tx *gorm.DB) {
				if tx.Statement.Schema != nil && tx.Statement.Schema.Name == canceledQuery && tx.Statement.Context == ctx {
					cancel()
				}
			}))
			t.Cleanup(func() { require.NoError(t, model.DB.Callback().Query().Remove("snapshot:cancel_policy_refresh")) })
			_, _, err := resolveRequestSnapshotPolicy(ctx)
			require.ErrorIs(t, err, context.Canceled)
			policy, current, err := resolveRequestSnapshotPolicy()
			require.NoError(t, err, "one canceled migration item must not disable snapshots for other requests")
			require.NotNil(t, policy)
			require.NotNil(t, current)
			assert.True(t, policy.Enabled)
			assert.Equal(t, profile.ID, current.ID)
		})
	}
}

// Exposes that a resolver has started observing cancellation after its initial
// context check, without sleeping or releasing the contested cache lock.
type snapshotObservedPolicyContext struct {
	context.Context
	observed chan struct{}
	once     sync.Once
}

func (ctx *snapshotObservedPolicyContext) Done() <-chan struct{} {
	ctx.once.Do(func() { close(ctx.observed) })
	return ctx.Context.Done()
}

func TestSnapshotPolicyCacheWaitCanBeCanceledWithoutReleasingOwner(t *testing.T) {
	profile := setupSnapshotPolicyContextTest(t)
	_, _, err := resolveRequestSnapshotPolicy()
	require.NoError(t, err)
	requestSnapshotPolicyCache.Lock()
	lockHeld := true
	defer func() {
		if lockHeld {
			requestSnapshotPolicyCache.Unlock()
		}
	}()
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &snapshotObservedPolicyContext{Context: base, observed: make(chan struct{})}
	result := make(chan error, 1)
	go func() {
		_, _, err := resolveRequestSnapshotPolicy(ctx)
		result <- err
	}()
	select {
	case <-ctx.observed:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "policy resolver did not observe its cancellation context")
	}
	cancel()
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "canceled policy resolver waited for the cache owner")
	}
	requestSnapshotPolicyCache.Unlock()
	lockHeld = false
	policy, current, err := resolveRequestSnapshotPolicy()
	require.NoError(t, err)
	assert.True(t, policy.Enabled)
	assert.Equal(t, profile.ID, current.ID)
}

func TestSnapshotExpiredContextCannotUseAValidPolicyCache(t *testing.T) {
	setupSnapshotPolicyContextTest(t)
	_, _, err := resolveRequestSnapshotPolicy()
	require.NoError(t, err)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	_, _, err = resolveRequestSnapshotPolicy(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	_, err = GetRequestSnapshotPolicy(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded, "the public policy getter must propagate the query deadline too")
	policy, _, err := resolveRequestSnapshotPolicy()
	require.NoError(t, err)
	assert.True(t, policy.Enabled)
}
