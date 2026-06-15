package channelconsole

import "testing"

func TestAggregateHealthStatus(t *testing.T) {
	cases := []struct {
		name     string
		statuses []string
		want     string
	}{
		{"empty", nil, HealthUnchecked},
		{"all healthy", []string{HealthHealthy, HealthHealthy}, HealthHealthy},
		{"one failed", []string{HealthHealthy, HealthFailed}, HealthWarning},
		{"all failed", []string{HealthFailed, HealthFailed}, HealthFailed},
		{"disabled wins", []string{HealthDisabled, HealthHealthy}, HealthDisabled},
		{"disabled after warning", []string{HealthWarning, HealthDisabled}, HealthDisabled},
		{"disabled after unchecked", []string{HealthUnchecked, HealthDisabled}, HealthDisabled},
		{"disabled after unknown", []string{"unknown", HealthDisabled}, HealthDisabled},
		{"warning", []string{HealthHealthy, HealthWarning}, HealthWarning},
		{"unchecked", []string{HealthHealthy, HealthUnchecked}, HealthWarning},
		{"unknown status", []string{"unknown"}, HealthWarning},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AggregateHealthStatus(tc.statuses); got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}
