package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRedemptionListFilterTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Redemption{}))
	cleanup := func() {
		DB.Exec("DELETE FROM redemptions")
	}
	cleanup()
	t.Cleanup(cleanup)
}

func insertRedemptionListFilterFixtures(t *testing.T, now int64) {
	t.Helper()
	insertRedemptionCode(t, &Redemption{Key: "filter-usable-no-expiry", Name: "Filter usable no expiry", Quota: 1})
	insertRedemptionCode(t, &Redemption{Key: "filter-usable-future", Name: "Filter usable future", Quota: 1, ExpiredTime: now + 3600})
	insertRedemptionCode(t, &Redemption{Key: "filter-expired", Name: "Filter expired", Quota: 1, ExpiredTime: now - 3600})
	insertRedemptionCode(t, &Redemption{Key: "filter-disabled", Name: "Filter disabled", Quota: 1, Status: common.RedemptionCodeStatusDisabled})
	insertRedemptionCode(t, &Redemption{Key: "filter-used", Name: "Filter used", Quota: 1, Status: common.RedemptionCodeStatusUsed})
}

func redemptionKeysForListFilterTest(redemptions []*Redemption) []string {
	keys := make([]string, 0, len(redemptions))
	for _, redemption := range redemptions {
		keys = append(keys, redemption.Key)
	}
	return keys
}

func TestGetAllRedemptionsStatusFilter(t *testing.T) {
	setupRedemptionListFilterTest(t)
	insertRedemptionListFilterFixtures(t, common.GetTimestamp())

	testCases := []struct {
		name         string
		statusFilter string
		wantKeys     []string
	}{
		{
			name:         "no filter returns everything",
			statusFilter: "",
			wantKeys: []string{
				"filter-usable-no-expiry",
				"filter-usable-future",
				"filter-expired",
				"filter-disabled",
				"filter-used",
			},
		},
		{
			name:         "usable excludes expired codes",
			statusFilter: "1",
			wantKeys:     []string{"filter-usable-no-expiry", "filter-usable-future"},
		},
		{
			name:         "disabled",
			statusFilter: "2",
			wantKeys:     []string{"filter-disabled"},
		},
		{
			name:         "used",
			statusFilter: "3",
			wantKeys:     []string{"filter-used"},
		},
		{
			name:         "expired matches enabled codes past expiry",
			statusFilter: "expired",
			wantKeys:     []string{"filter-expired"},
		},
		{
			name:         "comma separated values are OR-ed",
			statusFilter: "2,3",
			wantKeys:     []string{"filter-disabled", "filter-used"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			redemptions, total, err := GetAllRedemptions(0, 10, tc.statusFilter)
			require.NoError(t, err)
			assert.EqualValues(t, len(tc.wantKeys), total)
			assert.ElementsMatch(t, tc.wantKeys, redemptionKeysForListFilterTest(redemptions))
		})
	}
}

func TestGetAllRedemptionsStatusFilterKeepsTotalAcrossPages(t *testing.T) {
	setupRedemptionListFilterTest(t)
	insertRedemptionListFilterFixtures(t, common.GetTimestamp())

	firstPage, total, err := GetAllRedemptions(0, 1, "1")
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, firstPage, 1)

	secondPage, total, err := GetAllRedemptions(1, 1, "1")
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, secondPage, 1)
	assert.NotEqual(t, firstPage[0].Key, secondPage[0].Key)
}

func TestSearchRedemptionsStatusFilter(t *testing.T) {
	setupRedemptionListFilterTest(t)
	insertRedemptionListFilterFixtures(t, common.GetTimestamp())
	// Matches the status filter but not the keyword; must stay excluded.
	insertRedemptionCode(t, &Redemption{Key: "other-used", Name: "Other used", Quota: 1, Status: common.RedemptionCodeStatusUsed})

	testCases := []struct {
		name         string
		keyword      string
		statusFilter string
		wantKeys     []string
	}{
		{
			name:         "keyword only",
			keyword:      "Other",
			statusFilter: "",
			wantKeys:     []string{"other-used"},
		},
		{
			name:         "keyword and usable",
			keyword:      "Filter",
			statusFilter: "1",
			wantKeys:     []string{"filter-usable-no-expiry", "filter-usable-future"},
		},
		{
			name:         "keyword and disabled",
			keyword:      "Filter",
			statusFilter: "2",
			wantKeys:     []string{"filter-disabled"},
		},
		{
			name:         "keyword and used excludes non-matching keyword",
			keyword:      "Filter",
			statusFilter: "3",
			wantKeys:     []string{"filter-used"},
		},
		{
			name:         "keyword and expired",
			keyword:      "Filter",
			statusFilter: "expired",
			wantKeys:     []string{"filter-expired"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			redemptions, total, err := SearchRedemptions(tc.keyword, 0, 10, tc.statusFilter)
			require.NoError(t, err)
			assert.EqualValues(t, len(tc.wantKeys), total)
			assert.ElementsMatch(t, tc.wantKeys, redemptionKeysForListFilterTest(redemptions))
		})
	}
}
