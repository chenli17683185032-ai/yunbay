package relay

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
)

func TestResponsesCompactSubscriptionRepriceRestoresOneXBeforeSettlement(t *testing.T) {
	info := &relaycommon.RelayInfo{
		BillingSource:           service.BillingSourceSubscription,
		FinalPreConsumedQuota:   1000,
		SubscriptionPreConsumed: 1000,
		PriceData: types.PriceData{
			QuotaBeforeGroup:  1000,
			Quota:             0,
			QuotaToPreConsume: 300,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio:        0.3,
				GroupSpecialRatio: -1,
			},
		},
	}

	ensureResponsesCompactSubscriptionBillingRatio(info)

	assert.Equal(t, 1.0, info.PriceData.GroupRatioInfo.GroupRatio)
	assert.Equal(t, 1000, info.PriceData.QuotaToPreConsume)
	assert.Equal(t, 0, info.PriceData.Quota)
	assert.True(t, info.PriceData.SubscriptionRatioApplied)
	assert.True(t, info.PriceData.HasOriginalGroupRatioInfo)
	assert.Equal(t, 0.3, info.PriceData.OriginalGroupRatioInfo.GroupRatio)
}
