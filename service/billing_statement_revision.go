package service

import "github.com/shopspring/decimal"

func billingRateLabel(row BillingModelRow) string {
	if len(row.CurrentRates) == 0 {
		return "未配置"
	}
	var low, high decimal.Decimal
	for i, rate := range row.CurrentRates {
		value, err := decimal.NewFromString(rate.Ratio)
		if err != nil || value.IsNegative() {
			return "未配置"
		}
		if i == 0 || value.LessThan(low) {
			low = value
		}
		if i == 0 || value.GreaterThan(high) {
			high = value
		}
	}
	if low.Equal(high) {
		return low.Mul(decimal.NewFromInt(100)).StringFixed(2) + "%"
	}
	return low.Mul(decimal.NewFromInt(100)).StringFixed(2) + "–" + high.Mul(decimal.NewFromInt(100)).StringFixed(2) + "%"
}

func billingHistoricalRateLabel(row BillingModelRow) string {
	ratio, err := decimal.NewFromString(row.BillingRate)
	if err != nil || ratio.IsNegative() {
		return "未记录"
	}
	return ratio.Mul(decimal.NewFromInt(100)).StringFixed(2) + "%"
}

func billingModelCount(rows []BillingModelRow) int {
	names := make(map[string]struct{})
	for _, row := range rows {
		names[row.ModelName] = struct{}{}
	}
	return len(names)
}

func billingModelDisplayName(row BillingModelRow, version int) string {
	name := row.ModelName
	if name == "" {
		name = "未标注模型 / OTHER"
	}
	if version == 10 {
		group := row.BillingGroup
		if group == "" {
			group = "未记录"
		}
		name += " · 计费组：" + group
	}
	return name
}
