package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSSRFDNSValidationRespectsRequestCancellation(t *testing.T) {
	protection, err := NewSSRFProtectionFromFetchSetting(false, false, true, nil, nil, []string{"443"}, true)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = protection.ValidateURLWithContext(ctx, "https://cancelled-webhook.invalid/events")
	require.ErrorIs(t, err, context.Canceled)
}
