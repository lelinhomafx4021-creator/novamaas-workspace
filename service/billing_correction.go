package service

import (
	"errors"
	"math"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/shopspring/decimal"
)

type BillingCorrectionGroup struct {
	Group string `json:"group"`
	Rate  string `json:"rate"`
}

func BillingCorrectionGroups(userID int) ([]BillingCorrectionGroup, error) {
	user, err := model.GetUserById(userID, false)
	if err != nil {
		return nil, err
	}
	return billingCorrectionGroups(user.Group), nil
}

func billingCorrectionGroups(userGroup string) []BillingCorrectionGroup {
	groups := make([]BillingCorrectionGroup, 0)
	for name, base := range ratio_setting.GetGroupRatioCopy() {
		if override, ok := ratio_setting.GetGroupGroupRatio(userGroup, name); ok {
			base = override
		}
		if math.IsNaN(base) || math.IsInf(base, 0) || base < 0 || base > 10 {
			continue
		}
		groups = append(groups, BillingCorrectionGroup{Group: name, Rate: decimal.NewFromFloat(base).String()})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Group < groups[j].Group })
	return groups
}

func billingCorrectionTarget(userID int, group string) (string, string, error) {
	user, err := model.GetUserById(userID, false)
	if err != nil {
		return "", "", err
	}
	groups := billingCorrectionGroups(user.Group)
	for _, item := range groups {
		if item.Group == group {
			return item.Rate, user.Group, nil
		}
	}
	return "", "", errors.New("unknown or invalid target billing group")
}

func PreviewBillingCorrection(input model.BillingCorrectionInput, actorID int) (*model.BillingCorrection, error) {
	rate, group, err := billingCorrectionTarget(input.UserID, input.TargetGroup)
	if err != nil {
		return nil, err
	}
	return model.PreviewBillingCorrection(input, actorID, rate, group)
}

func ApplyBillingCorrection(id, digest string, actorID int, reverse bool, reason string) (*model.BillingCorrection, error) {
	batch, err := model.GetBillingCorrection(id)
	if err != nil {
		return nil, err
	}
	rate, group := "", ""
	if !reverse {
		rate, group, err = billingCorrectionTarget(batch.UserID, batch.TargetGroup)
		if err != nil {
			return nil, err
		}
	}
	result, err := model.ApplyBillingCorrection(id, digest, rate, group, actorID, reverse, reason)
	if err != nil {
		return nil, err
	}
	if err := model.PublishBillingCorrectionAudit(id); err != nil {
		common.SysError("billing correction audit pending: " + err.Error())
	}
	return model.GetBillingCorrection(result.ID)
}
