package router

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsBackendRouteAllowsAssetLibrarySPARefresh(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{path: "/assets", expected: false},
		{path: "/assets/", expected: false},
		{path: "/api/asset-library/assets", expected: true},
		{path: "/v1/chat/completions", expected: true},
		{path: "/dashboard/overview", expected: false},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			assert.Equal(t, test.expected, isBackendRoute(test.path))
		})
	}
}
