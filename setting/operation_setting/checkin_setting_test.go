package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestDefaultCheckinQuotaRangeAwardsZeroToOneDollar(t *testing.T) {
	oldDisplayType := GetGeneralSetting().QuotaDisplayType
	GetGeneralSetting().QuotaDisplayType = QuotaDisplayTypeUSD
	t.Cleanup(func() {
		GetGeneralSetting().QuotaDisplayType = oldDisplayType
	})

	min, max := GetCheckinQuotaRange()

	require.Equal(t, 0, min)
	require.Equal(t, int(common.QuotaPerUnit), max)
}

func TestNormalizeCheckinQuotaRangeTreatsSmallLegacyValuesAsDollars(t *testing.T) {
	oldDisplayType := GetGeneralSetting().QuotaDisplayType
	GetGeneralSetting().QuotaDisplayType = QuotaDisplayTypeUSD
	t.Cleanup(func() {
		GetGeneralSetting().QuotaDisplayType = oldDisplayType
	})

	min, max := NormalizeCheckinQuotaRange(1, 5)

	require.Equal(t, int(common.QuotaPerUnit), min)
	require.Equal(t, int(5*common.QuotaPerUnit), max)
}
