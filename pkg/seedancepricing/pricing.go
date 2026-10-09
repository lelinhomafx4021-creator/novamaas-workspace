// Package seedancepricing shares the provider's resolution and video-input
// pricing between new task billing and historical billing corrections.
package seedancepricing

import "strings"

type priceKey struct {
	resolution string
	hasVideo   bool
}

var seedance20Prices = map[priceKey]float64{
	{resolution: "720p"}:                  46,
	{resolution: "720p", hasVideo: true}:  28,
	{resolution: "1080p"}:                 51,
	{resolution: "1080p", hasVideo: true}: 31,
	{resolution: "4k"}:                    26,
	{resolution: "4k", hasVideo: true}:    16,
}

var seedance25Prices = map[priceKey]float64{
	{resolution: "720p"}:                  70,
	{resolution: "720p", hasVideo: true}:  42,
	{resolution: "1080p"}:                 77,
	{resolution: "1080p", hasVideo: true}: 46,
}

var modelPrices = map[string]map[priceKey]float64{
	"doubao-seedance-2-0":        seedance20Prices,
	"doubao-seedance-2-0-260128": seedance20Prices,
	"doubao-seedance-2-0-fast-260128": {
		{resolution: "720p"}:                 37,
		{resolution: "720p", hasVideo: true}: 22,
	},
	"doubao-seedance-2-5":        seedance25Prices,
	"doubao-seedance-2-5-260628": seedance25Prices,
}

// Price contains the published CNY-per-million-token prices and the multiplier
// applied to the configured base model ratio. Custom base pricing stays intact.
type Price struct {
	CNYPerMillionTokens     float64
	BaseCNYPerMillionTokens float64
	Ratio                   float64
}

func IsSeedance25(modelName string) bool {
	return modelName == "doubao-seedance-2-5" || modelName == "doubao-seedance-2-5-260628"
}

func BasePriceCNY(modelName string) (float64, bool) {
	prices, ok := modelPrices[modelName]
	if !ok {
		return 0, false
	}
	return prices[priceKey{resolution: "720p"}], true
}

// Lookup requires an explicit, supported output resolution. In particular,
// historical corrections must not guess a price from missing request evidence
// or silently treat an unsupported Seedance 2.5 4k request as the base tier.
func Lookup(modelName, resolution string, hasVideo bool) (Price, bool) {
	prices, ok := modelPrices[modelName]
	if !ok {
		return Price{}, false
	}
	resolution = strings.ToLower(strings.TrimSpace(resolution))
	if resolution == "480p" {
		resolution = "720p"
	}
	price, ok := prices[priceKey{resolution: resolution, hasVideo: hasVideo}]
	if !ok {
		return Price{}, false
	}
	base := prices[priceKey{resolution: "720p"}]
	return Price{CNYPerMillionTokens: price, BaseCNYPerMillionTokens: base, Ratio: price / base}, true
}
