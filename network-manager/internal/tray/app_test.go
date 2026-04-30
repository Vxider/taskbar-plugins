package tray

import (
	"testing"
	"time"

	"github.com/vxider/codex-buddy/uconsole/network-manager/internal/configstate"
	"github.com/vxider/codex-buddy/uconsole/network-manager/internal/modemctl"
)

func TestCompactDetailLabelUsesWiredForAutoStandby(t *testing.T) {
	state := modemctl.State{
		Installed:       true,
		AltNetConnected: true,
		AltNetType:      "ethernet",
	}
	config := configstate.State{ModemMode: configstate.ModeAuto}

	if got := compactDetailLabel(state, config); got != "wired" {
		t.Fatalf("compactDetailLabel() = %q, want %q", got, "wired")
	}
}

func TestModemStateLabelUsesWiredForAuto(t *testing.T) {
	state := modemctl.State{
		Installed:          true,
		ModemManagerActive: true,
		Available:          true,
		AltNetConnected:    true,
		AltNetType:         "ethernet",
	}
	config := configstate.State{ModemMode: configstate.ModeAuto}

	if got := modemStateLabel(state, config); got != "standby by wired (detached)" {
		t.Fatalf("modemStateLabel() = %q, want %q", got, "standby by wired (detached)")
	}
}

func TestTrayTooltipIncludesModemDiagnosticsWhenMmcliUnavailable(t *testing.T) {
	state := modemctl.State{
		Installed:       true,
		HardwarePresent: true,
		NetworkPort:     "eth1",
		PrimaryPort:     "ttyACM0",
	}
	config := configstate.State{ModemMode: configstate.ModeAuto}

	got := trayTooltip(state, config, false)
	want := "4G: auto | present | modem net eth1 | AT ttyACM0"
	if got != want {
		t.Fatalf("trayTooltip() = %q, want %q", got, want)
	}
}

func TestSystemWritesEnabledFromEnv(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "disabled by default", want: false},
		{name: "enabled by one", value: "1", want: true},
		{name: "enabled by true", value: "true", want: true},
		{name: "enabled trims case", value: " YES ", want: true},
		{name: "disabled by zero", value: "0", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := systemWritesEnabledFromEnv(func(key string) string {
				if key != "NETWORK_MANAGER_TRAY_ENABLE_WRITES" {
					t.Fatalf("unexpected env key %q", key)
				}
				return tt.value
			})
			if got != tt.want {
				t.Fatalf("systemWritesEnabledFromEnv() = %t, want %t", got, tt.want)
			}
		})
	}
}

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
			name:        "auto mode does not background-switch modem into standby",
			mode:        configstate.ModeAuto,
			target:      configstate.ModeStandby,
			lastApplied: configstate.ModeOn,
			lastTarget:  "",
			lastAt:      now,
			want:        false,
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
