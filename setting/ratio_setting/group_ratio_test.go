package ratio_setting

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeGroupRatioTrimsKeysAndAllowsZero(t *testing.T) {
	normalized, err := NormalizeGroupRatio(map[string]float64{
		" gpt-plus ": 0,
		"gpt-pro":    1.25,
	})

	require.NoError(t, err)
	require.Equal(t, map[string]float64{
		"gpt-plus": 0,
		"gpt-pro":  1.25,
	}, normalized)
}

func TestNormalizeGroupRatioReturnsCopy(t *testing.T) {
	input := map[string]float64{"gpt-plus": 0.8}
	normalized, err := NormalizeGroupRatio(input)
	require.NoError(t, err)

	input["gpt-plus"] = 9
	input["gpt-pro"] = 2

	require.Equal(t, map[string]float64{"gpt-plus": 0.8}, normalized)
}

func TestNormalizeGroupRatioRejectsInvalidEntries(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]float64
	}{
		{name: "blank key", input: map[string]float64{" ": 1}},
		{name: "negative", input: map[string]float64{"gpt-plus": -0.1}},
		{name: "nan", input: map[string]float64{"gpt-plus": math.NaN()}},
		{name: "positive infinity", input: map[string]float64{"gpt-plus": math.Inf(1)}},
		{name: "negative infinity", input: map[string]float64{"gpt-plus": math.Inf(-1)}},
		{name: "trim collision", input: map[string]float64{"gpt-plus": 1, " gpt-plus ": 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NormalizeGroupRatio(tt.input)
			require.Error(t, err)
		})
	}
}

func TestParseAndNormalizeGroupRatioJSONReturnsStableJSON(t *testing.T) {
	normalized, normalizedJSON, err := ParseAndNormalizeGroupRatioJSON(`{" z ":0,"a":1.5}`)

	require.NoError(t, err)
	require.Equal(t, map[string]float64{"a": 1.5, "z": 0}, normalized)
	require.Equal(t, `{"a":1.5,"z":0}`, normalizedJSON)
}

func TestParseAndNormalizeGroupRatioJSONRejectsNullEntryAndAllowsZero(t *testing.T) {
	_, _, err := ParseAndNormalizeGroupRatioJSON(`{"paid":null}`)
	require.Error(t, err)
	require.Error(t, CheckGroupRatio(`{"paid":null}`))

	normalized, normalizedJSON, err := ParseAndNormalizeGroupRatioJSON(`{"free":0}`)
	require.NoError(t, err)
	require.Equal(t, map[string]float64{"free": 0}, normalized)
	require.Equal(t, `{"free":0}`, normalizedJSON)
}

func TestNormalizeGroupGroupRatioTrimsNestedKeys(t *testing.T) {
	normalized, err := NormalizeGroupGroupRatio(map[string]map[string]float64{
		" day-card ": {
			" gpt-plus ": 0.8,
			"gpt-pro":    1.2,
		},
	})

	require.NoError(t, err)
	require.Equal(t, map[string]map[string]float64{
		"day-card": {
			"gpt-plus": 0.8,
			"gpt-pro":  1.2,
		},
	}, normalized)
}

func TestNormalizeGroupGroupRatioRejectsInvalidEntries(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]map[string]float64
	}{
		{name: "blank parent", input: map[string]map[string]float64{" ": {"gpt-plus": 1}}},
		{name: "blank child", input: map[string]map[string]float64{"day-card": {" ": 1}}},
		{name: "null child map", input: map[string]map[string]float64{"day-card": nil}},
		{name: "zero", input: map[string]map[string]float64{"day-card": {"gpt-plus": 0}}},
		{name: "negative", input: map[string]map[string]float64{"day-card": {"gpt-plus": -1}}},
		{name: "nan", input: map[string]map[string]float64{"day-card": {"gpt-plus": math.NaN()}}},
		{name: "positive infinity", input: map[string]map[string]float64{"day-card": {"gpt-plus": math.Inf(1)}}},
		{name: "negative infinity", input: map[string]map[string]float64{"day-card": {"gpt-plus": math.Inf(-1)}}},
		{name: "parent trim collision", input: map[string]map[string]float64{
			"day-card":   {"gpt-plus": 1},
			" day-card ": {"gpt-pro": 1},
		}},
		{name: "child trim collision", input: map[string]map[string]float64{
			"day-card": {"gpt-plus": 1, " gpt-plus ": 2},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NormalizeGroupGroupRatio(tt.input)
			require.Error(t, err)
		})
	}
}

func TestParseAndNormalizeGroupGroupRatioJSONDropsEmptyParents(t *testing.T) {
	normalized, normalizedJSON, err := ParseAndNormalizeGroupGroupRatioJSON(`{" empty ":{}," month-card ":{" gpt-pro ":1.3}}`)

	require.NoError(t, err)
	require.Equal(t, map[string]map[string]float64{
		"month-card": {"gpt-pro": 1.3},
	}, normalized)
	require.Equal(t, `{"month-card":{"gpt-pro":1.3}}`, normalizedJSON)
}

func TestParseAndCheckGroupGroupRatioRejectNestedNull(t *testing.T) {
	_, _, err := ParseAndNormalizeGroupGroupRatioJSON(`{"day-card":null}`)
	require.Error(t, err)
	require.Error(t, CheckGroupGroupRatio(`{"day-card":null}`))
}

func TestNormalizeGroupGroupRatioReturnsDeepCopy(t *testing.T) {
	input := map[string]map[string]float64{
		"day-card": {"gpt-plus": 0.8},
	}
	normalized, err := NormalizeGroupGroupRatio(input)
	require.NoError(t, err)

	input["day-card"]["gpt-plus"] = 9
	input["day-card"]["gpt-pro"] = 2

	require.Equal(t, map[string]map[string]float64{
		"day-card": {"gpt-plus": 0.8},
	}, normalized)
}

func TestUpdateGroupRatioByJSONStringPreservesRuntimeOnInvalidInput(t *testing.T) {
	original := GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupRatioByJSONString(original))
	})

	require.NoError(t, UpdateGroupRatioByJSONString(`{" stable ":0}`))
	require.Equal(t, `{"stable":0}`, GroupRatio2JSONString())

	require.Error(t, UpdateGroupRatioByJSONString(`{"broken":`))
	require.Equal(t, `{"stable":0}`, GroupRatio2JSONString())

	require.Error(t, UpdateGroupRatioByJSONString(`{"negative":-1}`))
	require.Equal(t, `{"stable":0}`, GroupRatio2JSONString())

	require.Error(t, UpdateGroupRatioByJSONString(`null`))
	require.Equal(t, `{"stable":0}`, GroupRatio2JSONString())
}

func TestUpdateGroupGroupRatioByJSONStringPreservesRuntimeOnInvalidInput(t *testing.T) {
	original := GroupGroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupGroupRatioByJSONString(original))
	})

	require.NoError(t, UpdateGroupGroupRatioByJSONString(`{" day-card ":{" gpt-plus ":0.7}}`))
	require.Equal(t, `{"day-card":{"gpt-plus":0.7}}`, GroupGroupRatio2JSONString())

	require.Error(t, UpdateGroupGroupRatioByJSONString(`{"broken":`))
	require.Equal(t, `{"day-card":{"gpt-plus":0.7}}`, GroupGroupRatio2JSONString())

	require.Error(t, UpdateGroupGroupRatioByJSONString(`{"day-card":{"gpt-plus":0}}`))
	require.Equal(t, `{"day-card":{"gpt-plus":0.7}}`, GroupGroupRatio2JSONString())

	require.Error(t, UpdateGroupGroupRatioByJSONString(`null`))
	require.Equal(t, `{"day-card":{"gpt-plus":0.7}}`, GroupGroupRatio2JSONString())

	require.Error(t, UpdateGroupGroupRatioByJSONString(`{"day-card":null}`))
	require.Equal(t, `{"day-card":{"gpt-plus":0.7}}`, GroupGroupRatio2JSONString())
}

func TestGetGroupGroupRatioRejectsInvalidHistoricalRuntimeValue(t *testing.T) {
	original := GroupGroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupGroupRatioByJSONString(original))
	})

	for _, ratio := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		groupGroupRatioMap.Set("legacy", map[string]float64{"gpt-plus": ratio})
		got, ok := GetGroupGroupRatio("legacy", "gpt-plus")
		require.False(t, ok)
		require.Equal(t, float64(-1), got)
	}
}

func TestUpdateGroupRatioPairByJSONStringValidatesBothBeforeMutation(t *testing.T) {
	originalGroupRatio, originalGroupGroupRatio := GroupRatioPair2JSONStrings()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupRatioPairByJSONString(originalGroupRatio, originalGroupGroupRatio))
	})
	require.NoError(t, UpdateGroupRatioPairByJSONString(`{"stable":1}`, `{"user":{"stable":2}}`))

	err := UpdateGroupRatioPairByJSONString(`{"changed":3}`, `{"user":{"changed":0}}`)

	require.Error(t, err)
	groupRatio, groupGroupRatio := GroupRatioPair2JSONStrings()
	require.Equal(t, `{"stable":1}`, groupRatio)
	require.Equal(t, `{"user":{"stable":2}}`, groupGroupRatio)
}

func TestGroupRatioPairReadersNeverObserveMixedGeneration(t *testing.T) {
	originalGroupRatio, originalGroupGroupRatio := GroupRatioPair2JSONStrings()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupRatioPairByJSONString(originalGroupRatio, originalGroupGroupRatio))
	})
	const (
		baseA    = `{"target":1}`
		nestedA  = `{}`
		baseB    = `{"target":3}`
		nestedB  = `{"user":{"target":4}}`
		attempts = 10000
	)
	require.NoError(t, UpdateGroupRatioPairByJSONString(baseA, nestedA))

	attemptStarted := make(chan struct{})
	writerDone := make(chan error, 1)
	go func() {
		close(attemptStarted)
		for i := 0; i < attempts; i++ {
			if err := UpdateGroupRatioPairByJSONString(baseB, nestedB); err != nil {
				writerDone <- err
				return
			}
			if err := UpdateGroupRatioPairByJSONString(baseA, nestedA); err != nil {
				writerDone <- err
				return
			}
		}
		writerDone <- nil
	}()
	<-attemptStarted

	for {
		select {
		case err := <-writerDone:
			require.NoError(t, err)
			return
		default:
			info := GetGroupRatioInfo("user", "target")
			validA := info.GroupRatio == 1 && info.GroupSpecialRatio == -1 && !info.HasSpecialRatio
			validB := info.GroupRatio == 4 && info.GroupSpecialRatio == 4 && info.HasSpecialRatio
			if !validA && !validB {
				require.NoError(t, <-writerDone)
				t.Fatalf("observed mixed group ratio generation: %+v", info)
			}
		}
	}
}

func TestGetUserGroupRatioSnapshotUsesOnePairGeneration(t *testing.T) {
	originalGroupRatio, originalGroupGroupRatio := GroupRatioPair2JSONStrings()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupRatioPairByJSONString(originalGroupRatio, originalGroupGroupRatio))
	})
	require.NoError(t, UpdateGroupRatioPairByJSONString(
		`{"gpt-plus":0.3,"gpt-pro":0.4}`,
		`{"vip":{"gpt-plus":1.2}}`,
	))

	snapshot := GetUserGroupRatioSnapshot("vip")

	require.Equal(t, map[string]float64{"gpt-plus": 1.2, "gpt-pro": 0.4}, snapshot)
}
