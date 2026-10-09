package ratio_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/seedancepricing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedance25DefaultRatioWorksWithSavedLegacyOptions(t *testing.T) {
	saved := ModelRatio2JSONString()
	t.Cleanup(func() { require.NoError(t, UpdateModelRatioByJSONString(saved)) })
	require.NoError(t, UpdateModelRatioByJSONString(`{"custom-model":2}`))
	for _, model := range []string{"doubao-seedance-2-5", "doubao-seedance-2-5-260628"} {
		ratio, ok, name := GetModelRatio(model)
		require.True(t, ok)
		assert.Equal(t, model, name)
		// ModelRatio is quota per token; a million-token base charge must
		// equal the CNY 70 tariff after the project's USD/CNY conversion.
		price, ok := seedancepricing.Lookup(model, "720p", false)
		require.True(t, ok)
		assert.InDelta(t, price.CNYPerMillionTokens, ratio*2*USD2RMB, 1e-12)
	}
	// A configured canonical price uses the operator's conversion rate. The
	// dated alias must inherit that same base until explicitly configured.
	require.NoError(t, UpdateModelRatioByJSONString(`{"doubao-seedance-2-5":5}`))
	ratio, ok, _ := GetModelRatio("doubao-seedance-2-5-260628")
	require.True(t, ok)
	assert.Equal(t, float64(5), ratio)
	assert.Equal(t, float64(5), GetExposedData()["model_ratio"].(map[string]float64)["doubao-seedance-2-5-260628"], "displayed pricing must agree with the active alias price")
	configured, err := common.Marshal(map[string]float64{"doubao-seedance-2-5": 3.25, "doubao-seedance-2-5-260628": 0})
	require.NoError(t, err)
	require.NoError(t, UpdateModelRatioByJSONString(string(configured)))
	ratio, ok, _ = GetModelRatio("doubao-seedance-2-5")
	require.True(t, ok)
	assert.Equal(t, 3.25, ratio)
	ratio, ok, _ = GetModelRatio("doubao-seedance-2-5-260628")
	require.True(t, ok)
	assert.Zero(t, ratio, "an explicit free model must remain free")
}
