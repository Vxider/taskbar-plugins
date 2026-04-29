package tray

import (
	"testing"
	"time"

	"github.com/vxider/codex-buddy/uconsole/network-manager/internal/configstate"
	"github.com/vxider/codex-buddy/uconsole/network-manager/internal/modemctl"
)

func TestShouldReconcileModemTarget(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		mode        string
		target      string
		lastApplied string
		lastTarget  string
		lastAt      time.Time
		busy        bool
		want        bool
	}{
		{
			name:        "manual mode attempts initial reconciliation once",
			mode:        configstate.ModeOn,
			target:      configstate.ModeOn,
			lastApplied: configstate.ModeStandby,
			lastTarget:  "",
			lastAt:      now,
			want:        true,
		},
		{
			name:        "manual mode does not keep retrying same failed target",
			mode:        configstate.ModeOn,
			target:      configstate.ModeOn,
			lastApplied: configstate.ModeStandby,
			lastTarget:  configstate.ModeOn,
			lastAt:      now.Add(-10 * time.Minute),
			want:        false,
		},
		{
			name:        "auto mode retries after cooldown",
			mode:        configstate.ModeAuto,
			target:      configstate.ModeOn,
			lastApplied: configstate.ModeStandby,
			lastTarget:  configstate.ModeOn,
			lastAt:      now.Add(-46 * time.Second),
			want:        true,
		},
		{
			name:        "auto mode respects retry cooldown",
			mode:        configstate.ModeAuto,
			target:      configstate.ModeOn,
			lastApplied: configstate.ModeStandby,
			lastTarget:  configstate.ModeOn,
			lastAt:      now.Add(-30 * time.Second),
			want:        false,
		},
		{
			name:        "matching applied target skips reconciliation",
			mode:        configstate.ModeAuto,
			target:      configstate.ModeOn,
			lastApplied: configstate.ModeOn,
			lastTarget:  "",
			lastAt:      now,
			want:        false,
		},
		{
			name:        "busy modem skips reconciliation",
			mode:        configstate.ModeAuto,
			target:      configstate.ModeOn,
			lastApplied: configstate.ModeStandby,
			lastTarget:  "",
			lastAt:      now,
			busy:        true,
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldReconcileModemTarget(tt.mode, tt.target, tt.lastApplied, tt.lastTarget, tt.lastAt, tt.busy)
			if got != tt.want {
				t.Fatalf("shouldReconcileModemTarget() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestLiveTargetSatisfied(t *testing.T) {
	tests := []struct {
		name   string
		state  modemctl.State
		target string
		want   bool
	}{
		{
			name: "on target satisfied when modem available and enabled",
			state: modemctl.State{
				Available:  true,
				ModemState: "enabled",
			},
			target: configstate.ModeOn,
			want:   true,
		},
		{
			name: "on target not satisfied when modem disabled",
			state: modemctl.State{
				Available:  true,
				ModemState: "disabled",
			},
			target: configstate.ModeOn,
			want:   false,
		},
		{
			name: "on target not satisfied when modem unavailable",
			state: modemctl.State{
				Available: false,
			},
			target: configstate.ModeOn,
			want:   false,
		},
		{
			name: "standby target is not inferred from live modem state",
			state: modemctl.State{
				Available:  true,
				ModemState: "enabled",
			},
			target: configstate.ModeStandby,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := liveTargetSatisfied(tt.state, tt.target); got != tt.want {
				t.Fatalf("liveTargetSatisfied() = %t, want %t", got, tt.want)
			}
		})
	}
}
