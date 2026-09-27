package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperatingEntityOptionValidation(t *testing.T) {
	for _, value := range []string{"", "https://cdn.example/operator.png", "http://cdn.example/operator.webp"} {
		require.NoError(t, validateOptionValue("OperatingEntityLogo", value))
	}
	for _, value := range []string{"file:///etc/passwd", "https://user:secret@cdn.example/logo.png", "https://cdn.example/logo.png#fragment", "https://"} {
		assert.Error(t, validateOptionValue("OperatingEntityLogo", value))
	}
	require.NoError(t, validateOptionValue("OperatingEntityName", strings.Repeat("企", 120)))
	assert.Error(t, validateOptionValue("OperatingEntityName", strings.Repeat("企", 121)))
}
