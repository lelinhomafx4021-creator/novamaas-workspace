package seedancepricing

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedanceResolutionAndVideoInputPrices(t *testing.T) {
	tests := []struct {
		model      string
		resolution string
		hasVideo   bool
		base       float64
		price      float64
	}{
		{"doubao-seedance-2-5", "480p", false, 70, 70},
		{"doubao-seedance-2-5", "480p", true, 70, 42},
		{"doubao-seedance-2-5", "720p", false, 70, 70},
		{"doubao-seedance-2-5", "720p", true, 70, 42},
		{"doubao-seedance-2-5", "1080p", false, 70, 77},
		{"doubao-seedance-2-5", "1080p", true, 70, 46},
		{"doubao-seedance-2-5-260628", "480p", false, 70, 70},
		{"doubao-seedance-2-5-260628", "480p", true, 70, 42},
		{"doubao-seedance-2-5-260628", "720p", false, 70, 70},
		{"doubao-seedance-2-5-260628", "720p", true, 70, 42},
		{"doubao-seedance-2-5-260628", "1080p", false, 70, 77},
		{"doubao-seedance-2-5-260628", "1080p", true, 70, 46},
		{"doubao-seedance-2-0", "480p", false, 46, 46},
		{"doubao-seedance-2-0", "720p", true, 46, 28},
		{"doubao-seedance-2-0", "1080p", false, 46, 51},
		{"doubao-seedance-2-0", "1080p", true, 46, 31},
		{"doubao-seedance-2-0", "4k", false, 46, 26},
		{"doubao-seedance-2-0", "4k", true, 46, 16},
		{"doubao-seedance-2-0-260128", "1080p", true, 46, 31},
		{"doubao-seedance-2-0-fast-260128", "720p", false, 37, 37},
		{"doubao-seedance-2-0-fast-260128", "720p", true, 37, 22},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%s/video=%t", tt.model, tt.resolution, tt.hasVideo), func(t *testing.T) {
			price, ok := Lookup(tt.model, tt.resolution, tt.hasVideo)
			require.True(t, ok)
			assert.Equal(t, tt.price, price.CNYPerMillionTokens)
			assert.Equal(t, tt.base, price.BaseCNYPerMillionTokens)
			assert.InDelta(t, tt.price/tt.base, price.Ratio, 1e-12)
		})
	}
}

func TestHistoricalPricingDoesNotGuessMissingOrUnsupportedResolution(t *testing.T) {
	for _, resolution := range []string{"", "4k", "2160p", "automatic", "1440p"} {
		for _, model := range []string{"doubao-seedance-2-5", "doubao-seedance-2-5-260628"} {
			_, ok := Lookup(model, resolution, false)
			assert.False(t, ok, "%s resolution %q must block correction", model, resolution)
		}
	}
	_, ok := Lookup("custom-seedance", "720p", false)
	assert.False(t, ok)
	_, ok = Lookup("doubao-seedance-2-0-fast-260128", "1080p", false)
	assert.False(t, ok)
	price, ok := Lookup("doubao-seedance-2-5", " 1080P ", true)
	require.True(t, ok)
	assert.Equal(t, float64(46), price.CNYPerMillionTokens)
}
