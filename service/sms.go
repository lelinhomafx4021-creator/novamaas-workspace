package service

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dysms "github.com/alibabacloud-go/dysmsapi-20170525/v4/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
)

var ErrSMSUnavailable = errors.New("SMS service is unavailable")

// SendAliyunSMS submits once. SDK automatic retries are disabled because
// SendSms is not idempotent. Never surface SDK errors containing credentials.
func SendAliyunSMS(phone, code string) error {
	settings := system_setting.GetSMSSettings()
	if !settings.IsReady() {
		return ErrSMSUnavailable
	}
	client, err := dysms.NewClient(&openapi.Config{AccessKeyId: tea.String(settings.AccessKeyId),
		AccessKeySecret: tea.String(settings.AccessKeySecret), SecurityToken: tea.String(settings.SecurityToken),
		Endpoint: tea.String("dysmsapi.aliyuncs.com"), ReadTimeout: tea.Int(5000), ConnectTimeout: tea.Int(3000)})
	if err != nil {
		return ErrSMSUnavailable
	}
	params, err := common.Marshal(map[string]string{settings.CodeParameter: code})
	if err != nil {
		return ErrSMSUnavailable
	}
	response, err := client.SendSmsWithOptions(&dysms.SendSmsRequest{PhoneNumbers: tea.String(strings.TrimPrefix(phone, "+86")),
		SignName: tea.String(settings.SignName), TemplateCode: tea.String(settings.TemplateCode), TemplateParam: tea.String(string(params))},
		&util.RuntimeOptions{Autoretry: tea.Bool(false), ReadTimeout: tea.Int(5000), ConnectTimeout: tea.Int(3000)})
	if err != nil || response == nil || response.Body == nil || tea.StringValue(response.Body.Code) != "OK" {
		return ErrSMSUnavailable
	}
	return nil
}

// IssueSMS reserves all budgets before contacting the provider. Failed sends
// still count against abuse limits and never activate their challenge.
func IssueSMS(input model.SMSChallengeInput, sender func(string, string) error) (string, error) {
	if !system_setting.GetSMSSettings().IsReady() {
		return "", ErrSMSUnavailable
	}
	token, code, challenge, err := model.ReserveSMSChallenge(input)
	if err != nil {
		return "", err
	}
	if err := sender(challenge.Phone, code); err != nil {
		return "", ErrSMSUnavailable
	}
	if err := model.MarkSMSDelivered(challenge.Id); err != nil {
		return "", err
	}
	return token, nil
}
