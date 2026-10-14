package controller

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

func Playground(c *gin.Context) {
	var newAPIError *types.NewAPIError

	defer func() {
		if newAPIError != nil {
			c.JSON(newAPIError.StatusCode, gin.H{
				"error": newAPIError.ToOpenAIError(),
			})
		}
	}()

	useAccessToken := c.GetBool("use_access_token")
	if useAccessToken {
		newAPIError = types.NewError(errors.New("暂不支持使用 access token"), types.ErrorCodeAccessDenied, types.ErrOptionWithSkipRetry())
		return
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAI, nil, nil)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		return
	}

	if !c.GetBool("playground_key_selected") {
		if err := setupPlaygroundToken(c, relayInfo.UsingGroup, nil); err != nil {
			newAPIError = types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
			return
		}
	}

	Relay(c, types.RelayFormatOpenAI)
}

func setupPlaygroundToken(c *gin.Context, group string, selected *model.Token) error {
	userId := c.GetInt("id")
	userCache, err := model.GetUserCache(userId)
	if err != nil {
		return err
	}
	userCache.WriteContext(c)
	if selected != nil {
		return middleware.SetupContextForToken(c, selected)
	}
	tempToken := &model.Token{
		UserId: userId,
		Name:   fmt.Sprintf("playground-%s", group),
		Group:  group,
	}
	return middleware.SetupContextForToken(c, tempToken)
}
