package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
)

// These are current permitted group rates, not proof of a historical price.
// Freeze them in the snapshot so retries cannot follow later setting changes.
func captureBillingRateReferences(ctx context.Context, userID int) (map[string][]BillingGroupRate, error) {
	user, err := model.GetUserById(userID, false)
	if err != nil {
		return nil, err
	}
	userGroup := user.Group
	groups := GetUserUsableGroups(userGroup)
	allowed := make([]string, 0, len(groups))
	for group := range groups {
		if IsUserSelectableGroup(userGroup, group) {
			allowed = append(allowed, group)
		}
	}
	sort.Strings(allowed)
	abilities, err := model.GetBillingRateAbilities(ctx, allowed)
	if err != nil {
		return nil, err
	}
	byModel := make(map[string]map[string]string)
	for _, ability := range abilities {
		ratio := GetUserGroupRatio(userGroup, ability.Group)
		if ratio < 0 || math.IsNaN(ratio) || math.IsInf(ratio, 0) {
			continue
		}
		if byModel[ability.Model] == nil {
			byModel[ability.Model] = make(map[string]string)
		}
		byModel[ability.Model][ability.Group] = decimal.NewFromFloat(ratio).String()
	}
	result := make(map[string][]BillingGroupRate, len(byModel))
	for name, rates := range byModel {
		for _, group := range allowed {
			if ratio, ok := rates[group]; ok {
				result[name] = append(result[name], BillingGroupRate{Group: group, Ratio: ratio})
			}
		}
	}
	return result, nil
}

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

func RegenerateBillingStatementContext(ctx context.Context, source *model.BillingStatement, actorID int) (*model.BillingStatement, error) {
	if err := source.VerifySnapshot(); err != nil {
		return nil, err
	}
	var snapshot BillingSnapshot
	if err := common.UnmarshalJsonStr(source.Snapshot, &snapshot); err != nil {
		return nil, err
	}
	branding, err := captureBillingDocumentBranding(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBillingDocumentBranding, err)
	}
	rates, err := captureBillingRateReferences(ctx, source.UserID)
	if err != nil {
		return nil, err
	}
	snapshot.PDFTemplateVersion = 9
	snapshot.Issuer, snapshot.PDFLogoPNG, snapshot.PDFFooter = branding.Issuer, branding.LogoPNG, branding.Footer
	snapshot.OperatingName, snapshot.PDFOperatingLogoPNG = branding.OperatingName, branding.OperatingLogoPNG
	snapshot.RateReferenceAt = common.GetTimestamp()
	return model.RegenerateBillingStatement(source, actorID, func(_ *model.BillingStatement, totals []model.BillingModelTotal) (string, string, error) {
		if err := attachBillingModelRows(&snapshot, totals); err != nil {
			return "", "", err
		}
		for i := range snapshot.Models {
			snapshot.Models[i].CurrentRates = rates[snapshot.Models[i].ModelName]
		}
		body, err := common.Marshal(snapshot)
		if err != nil {
			return "", "", err
		}
		if len(body) > 60*1024 {
			return "", "", errors.New("billing document snapshot exceeds the safe database TEXT limit")
		}
		digest := sha256.Sum256(body)
		return string(body), hex.EncodeToString(digest[:]), nil
	})
}
