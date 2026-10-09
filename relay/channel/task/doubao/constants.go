package doubao

import (
	"strings"

	"github.com/QuantumNous/new-api/pkg/seedancepricing"
)

var ModelList = []string{
	"doubao-seedance-1-0-pro-250528",
	"doubao-seedance-1-0-lite-t2v",
	"doubao-seedance-1-0-lite-i2v",
	"doubao-seedance-1-5-pro-251215",
	"doubao-seedance-2-0",
	"doubao-seedance-2-0-260128",
	"doubao-seedance-2-0-fast-260128",
	"doubao-seedance-2-5",
	"doubao-seedance-2-5-260628",
}

var ChannelName = "doubao-video"

// GetVideoInputRatio 返回指定模型在给定输出分辨率/是否含视频输入下，相对基准价的计费倍率。
// 基准单价为元/百万 token；ModelRatio 需换算为 基准价 / (2 * USD2RMB)。
// 历史重算必须使用 seedancepricing.Lookup 的严格检查，而不是这里的兼容回退。
func GetVideoInputRatio(modelName, resolution string, hasVideo bool) (float64, bool) {
	base, ok := seedancepricing.BasePriceCNY(modelName)
	if !ok || base <= 0 {
		return 0, false
	}
	resolution = strings.ToLower(strings.TrimSpace(resolution))
	if resolution != "1080p" && resolution != "4k" {
		resolution = "720p"
	}
	price, ok := seedancepricing.Lookup(modelName, resolution, hasVideo)
	if !ok {
		// 保持既有模型的上游校验兼容行为；2.5 在请求校验阶段拒绝未知组合。
		return 1.0, true
	}
	return price.Ratio, true
}
