package common

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRelayInfoGetFinalRequestRelayFormatPrefersExplicitFinal(t *testing.T) {
	info := &RelayInfo{
		RelayFormat:             types.RelayFormatOpenAI,
		RequestConversionChain:  []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude},
		FinalRequestRelayFormat: types.RelayFormatOpenAIResponses,
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatOpenAIResponses), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatFallsBackToConversionChain(t *testing.T) {
	info := &RelayInfo{
		RelayFormat:            types.RelayFormatOpenAI,
		RequestConversionChain: []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude},
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatClaude), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatFallsBackToRelayFormat(t *testing.T) {
	info := &RelayInfo{
		RelayFormat: types.RelayFormatGemini,
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatGemini), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatNilReceiver(t *testing.T) {
	var info *RelayInfo
	require.Equal(t, types.RelayFormat(""), info.GetFinalRequestRelayFormat())
}

func TestGenRelayInfoKeepsUserGroupAndSetsValuePackageBillingGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "vip")
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "gpt-plus")
	common.SetContextKey(ctx, constant.ContextKeyTokenGroup, "gpt-plus")
	common.SetContextKey(ctx, constant.ContextKeyValuePackageSubscriptionId, 123)
	common.SetContextKey(ctx, constant.ContextKeyValuePackagePlanId, 456)
	common.SetContextKey(ctx, constant.ContextKeyValuePackageModelGroup, "month-card")
	common.SetContextKey(ctx, constant.ContextKeyValuePackagePackageType, "month")

	info, err := GenRelayInfo(ctx, types.RelayFormatOpenAI, &dto.GeneralOpenAIRequest{Model: "gpt-5.5"}, nil)

	require.NoError(t, err)
	require.NotNil(t, info)
	require.Equal(t, "vip", info.UserGroup)
	require.Equal(t, "vip", info.RealUserGroup)
	require.Equal(t, "gpt-plus", info.UsingGroup)
	require.Equal(t, "gpt-plus", info.TokenGroup)
	require.Equal(t, "month-card", info.BillingUserGroup)
	require.Equal(t, "month-card", info.ValuePackageBillingGroup)
	require.Equal(t, "month-card", info.ValuePackageModelGroup)
	require.Equal(t, "month", info.ValuePackagePackageType)
	require.Equal(t, "month-card", info.BillingRatioUserGroup())
}
