package modemctl

import "testing"

func TestDesiredTarget(t *testing.T) {
	tests := []struct {
		name          string
		mode          string
		wifiConnected bool
		want          string
	}{
		{name: "auto with wifi", mode: "auto", wifiConnected: true, want: "standby"},
		{name: "auto without wifi", mode: "auto", wifiConnected: false, want: "on"},
		{name: "standby", mode: "standby", wifiConnected: false, want: "standby"},
		{name: "off", mode: "off", wifiConnected: true, want: "off"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DesiredTarget(tt.mode, tt.wifiConnected); got != tt.want {
				t.Fatalf("DesiredTarget(%q, %t) = %q, want %q", tt.mode, tt.wifiConnected, got, tt.want)
			}
		})
	}
}

func TestWiFiStateConnected(t *testing.T) {
	tests := []struct {
		state string
		want  bool
	}{
		{state: "connected", want: true},
		{state: "connected (externally)", want: true},
		{state: "已连接", want: true},
		{state: "disconnected", want: false},
		{state: "connecting", want: false},
		{state: "unavailable", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			if got := wifiStateConnected(tt.state); got != tt.want {
				t.Fatalf("wifiStateConnected(%q) = %t, want %t", tt.state, got, tt.want)
			}
		})
	}
}

func TestAlternativeNetworkType(t *testing.T) {
	tests := []struct {
		deviceType string
		want       bool
	}{
		{deviceType: "wifi", want: true},
		{deviceType: "ethernet", want: true},
		{deviceType: "tun", want: false},
		{deviceType: "loopback", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.deviceType, func(t *testing.T) {
			if got := alternativeNetworkType(tt.deviceType); got != tt.want {
				t.Fatalf("alternativeNetworkType(%q) = %t, want %t", tt.deviceType, got, tt.want)
			}
		})
	}
}

func TestIsModemNetworkDevice(t *testing.T) {
	state := State{NetworkPort: "eth1"}

	if !isModemNetworkDevice(state, "eth1") {
		t.Fatalf("isModemNetworkDevice() = false, want true")
	}
	if isModemNetworkDevice(state, "wlan0") {
		t.Fatalf("isModemNetworkDevice() = true, want false")
	}
}

func TestLiveSummary(t *testing.T) {
	tests := []struct {
		name  string
		state State
		want  string
	}{
		{
			name: "driverReadyWhenHardwarePresentButNoModemAvailable",
			state: State{
				Installed:          true,
				HardwarePresent:    true,
				ATReady:            true,
				ModemManagerActive: true,
				Available:          false,
			},
			want: "driver ready",
		},
		{
			name: "detachedOnlyWhenModemActuallyAvailable",
			state: State{
				Installed:          true,
				HardwarePresent:    true,
				ATReady:            true,
				ModemManagerActive: true,
				Available:          true,
				ModemState:         "enabled",
			},
			want: "detached",
		},
		{
			name: "presentWhenHardwareHasNetworkPortButMmcliDoesNotExposeModem",
			state: State{
				Installed:       true,
				HardwarePresent: true,
				NetworkPort:     "eth1",
				Available:       false,
				ModemState:      "present",
			},
			want: "present",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LiveSummary(tt.state); got != tt.want {
				t.Fatalf("LiveSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}
