package modemctl

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/vxider/codex-buddy/uconsole/network-manager/internal/modemsysfs"
)

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

func TestLoadWWANRadioState(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		wantKnown   bool
		wantEnabled bool
	}{
		{name: "enabled", output: "enabled\n", wantKnown: true, wantEnabled: true},
		{name: "disabled", output: "disabled\n", wantKnown: true, wantEnabled: false},
		{name: "chineseDisabled", output: "已禁用\n", wantKnown: true, wantEnabled: false},
		{name: "unknown", output: "unexpected\n", wantKnown: false, wantEnabled: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origRun := runFunc
			t.Cleanup(func() {
				runFunc = origRun
			})

			runFunc = func(_ context.Context, name string, args ...string) ([]byte, error) {
				if name != "nmcli" || !reflect.DeepEqual(args, []string{"radio", "wwan"}) {
					return nil, errors.New("unexpected command")
				}
				return []byte(tt.output), nil
			}

			var state State
			loadWWANRadioState(context.Background(), &state)
			if state.WWANRadioKnown != tt.wantKnown || state.WWANRadioEnabled != tt.wantEnabled {
				t.Fatalf("WWAN radio = (known=%t enabled=%t), want (known=%t enabled=%t)", state.WWANRadioKnown, state.WWANRadioEnabled, tt.wantKnown, tt.wantEnabled)
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

func TestSignalQualityFromCSQ(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		wantOK bool
	}{
		{
			name:   "normal response",
			input:  "\r\n+CSQ: 20,99\r\n\r\nOK\r\n",
			want:   "64",
			wantOK: true,
		},
		{
			name:   "full scale",
			input:  "+CSQ: 31,99",
			want:   "100",
			wantOK: true,
		},
		{
			name:   "unknown",
			input:  "+CSQ: 99,99",
			wantOK: false,
		},
		{
			name:   "missing",
			input:  "OK",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := signalQualityFromCSQ(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("signalQualityFromCSQ() ok = %t, want %t", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("signalQualityFromCSQ() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadDoesNotProbeATWhenMMSeesModemButSignalIsUnavailable(t *testing.T) {
	origRun := runFunc
	origSend := sendATFunc
	origDeviceNodeExists := deviceNodeExistsFunc
	origLookPath := lookPathFunc
	origFirstDevice := firstDeviceFunc
	t.Cleanup(func() {
		runFunc = origRun
		sendATFunc = origSend
		deviceNodeExistsFunc = origDeviceNodeExists
		lookPathFunc = origLookPath
		firstDeviceFunc = origFirstDevice
	})

	lookPathFunc = func(name string) (string, error) {
		switch name {
		case "mmcli":
			return "/usr/bin/mmcli", nil
		default:
			return "", exec.ErrNotFound
		}
	}
	runFunc = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "mmcli" {
			return nil, errors.New("unexpected command")
		}
		if reflect.DeepEqual(args, []string{"-L"}) {
			return []byte("/org/freedesktop/ModemManager1/Modem/0 [CMCC] ML307C"), nil
		}
		if reflect.DeepEqual(args, []string{"-m", "0", "-K"}) {
			return []byte(strings.Join([]string{
				"modem.generic.primary-port: ttyUSB2",
				"modem.generic.state: registered",
				"modem.generic.signal-quality.value: 0",
				"modem.generic.signal-quality.recent: no",
				"modem.3gpp.registration-state: home",
				"modem.3gpp.packet-service-state: attached",
			}, "\n")), nil
		}
		if reflect.DeepEqual(args, []string{"-m", "0", "--signal-get", "-K"}) {
			return []byte("modem.signal.lte.rsrp: --\nmodem.signal.gsm.rssi: --\n"), nil
		}
		return nil, errors.New("unexpected mmcli args")
	}
	firstDeviceFunc = func() (modemsysfs.Device, bool) {
		return modemsysfs.Device{ATTTYs: []string{"ttyUSB2"}}, true
	}
	deviceNodeExistsFunc = func(path string) bool {
		return path == "/dev/ttyUSB2"
	}
	sendATFunc = func(path, command string, timeout time.Duration) (string, error) {
		if command == "AT" {
			return "\r\nOK\r\n", nil
		}
		t.Fatalf("sendATFunc command = %q, want no signal AT probe", command)
		return "", nil
	}

	state := Load(context.Background())
	if state.SignalQuality != "" {
		t.Fatalf("SignalQuality = %q, want empty", state.SignalQuality)
	}
}

func TestLoadUsesATSignalOnlyWhenMMDoesNotSeeHardwareModem(t *testing.T) {
	origRun := runFunc
	origRunCommand := runCommandFunc
	origSend := sendATFunc
	origDeviceNodeExists := deviceNodeExistsFunc
	origLookPath := lookPathFunc
	origFirstDevice := firstDeviceFunc
	t.Cleanup(func() {
		runFunc = origRun
		runCommandFunc = origRunCommand
		sendATFunc = origSend
		deviceNodeExistsFunc = origDeviceNodeExists
		lookPathFunc = origLookPath
		firstDeviceFunc = origFirstDevice
	})

	lookPathFunc = func(name string) (string, error) {
		switch name {
		case "mmcli":
			return "/usr/bin/mmcli", nil
		default:
			return "", exec.ErrNotFound
		}
	}
	runFunc = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name == "mmcli" && reflect.DeepEqual(args, []string{"-L"}) {
			return []byte("No modems were found"), nil
		}
		return nil, errors.New("unexpected command")
	}
	runCommandFunc = func(_ context.Context, _ string, _ ...string) error {
		return nil
	}
	firstDeviceFunc = func() (modemsysfs.Device, bool) {
		return modemsysfs.Device{ATTTYs: []string{"ttyUSB2"}, NetworkInterface: "eth1"}, true
	}
	deviceNodeExistsFunc = func(path string) bool {
		return path == "/dev/ttyUSB2"
	}
	sendATFunc = func(path, command string, timeout time.Duration) (string, error) {
		switch command {
		case "AT":
			return "\r\nOK\r\n", nil
		case "AT+CSQ":
			return "\r\n+CSQ: 20,99\r\n\r\nOK\r\n", nil
		default:
			t.Fatalf("sendATFunc command = %q, want AT or AT+CSQ", command)
		}
		return "", nil
	}

	state := Load(context.Background())
	if state.SignalQuality != "64" {
		t.Fatalf("SignalQuality = %q, want 64", state.SignalQuality)
	}
}

func TestLoadUsesMMSignalWhenGenericSignalQualityIsStale(t *testing.T) {
	origRun := runFunc
	origRunCommand := runCommandFunc
	origSend := sendATFunc
	origDeviceNodeExists := deviceNodeExistsFunc
	origLookPath := lookPathFunc
	origFirstDevice := firstDeviceFunc
	t.Cleanup(func() {
		runFunc = origRun
		runCommandFunc = origRunCommand
		sendATFunc = origSend
		deviceNodeExistsFunc = origDeviceNodeExists
		lookPathFunc = origLookPath
		firstDeviceFunc = origFirstDevice
	})

	lookPathFunc = func(name string) (string, error) {
		switch name {
		case "mmcli":
			return "/usr/bin/mmcli", nil
		default:
			return "", exec.ErrNotFound
		}
	}
	runFunc = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "mmcli" {
			return nil, errors.New("unexpected command")
		}
		if reflect.DeepEqual(args, []string{"-L"}) {
			return []byte("/org/freedesktop/ModemManager1/Modem/0 [CMCC] ML307C"), nil
		}
		if reflect.DeepEqual(args, []string{"-m", "0", "-K"}) {
			return []byte(strings.Join([]string{
				"modem.generic.primary-port: ttyUSB2",
				"modem.generic.state: registered",
				"modem.generic.signal-quality.value: 0",
				"modem.generic.signal-quality.recent: no",
				"modem.3gpp.registration-state: home",
				"modem.3gpp.packet-service-state: attached",
			}, "\n")), nil
		}
		if reflect.DeepEqual(args, []string{"-m", "0", "--signal-get", "-K"}) {
			return []byte(strings.Join([]string{
				"modem.signal.refresh.rate: 5",
				"modem.signal.gsm.rssi: -79.00",
				"modem.signal.lte.rsrp: -96.00",
			}, "\n")), nil
		}
		return nil, errors.New("unexpected mmcli args")
	}
	runCommandFunc = func(_ context.Context, _ string, _ ...string) error {
		t.Fatal("runCommandFunc should not be called when mmcli signal data is present")
		return nil
	}
	firstDeviceFunc = func() (modemsysfs.Device, bool) {
		return modemsysfs.Device{ATTTYs: []string{"ttyUSB2"}}, true
	}
	deviceNodeExistsFunc = func(path string) bool {
		return path == "/dev/ttyUSB2"
	}
	sendATFunc = func(path, command string, timeout time.Duration) (string, error) {
		if command == "AT" {
			return "\r\nOK\r\n", nil
		}
		t.Fatal("sendATFunc should not be called for signal quality when mmcli signal data is present")
		return "", nil
	}

	state := Load(context.Background())
	if state.SignalQuality != "60" {
		t.Fatalf("SignalQuality = %q, want 60", state.SignalQuality)
	}
}

func TestCandidateATPortsSkipsMissingDeviceNodes(t *testing.T) {
	origDeviceNodeExists := deviceNodeExistsFunc
	origFirstDevice := firstDeviceFunc
	t.Cleanup(func() {
		deviceNodeExistsFunc = origDeviceNodeExists
		firstDeviceFunc = origFirstDevice
	})

	firstDeviceFunc = func() (modemsysfs.Device, bool) {
		return modemsysfs.Device{ATTTYs: []string{"ttyUSB0", "ttyUSB2"}}, true
	}
	deviceNodeExistsFunc = func(path string) bool {
		return path == "/dev/ttyUSB2"
	}

	got := candidateATPorts(State{PrimaryPort: "ttyUSB0"})
	want := []string{"/dev/ttyUSB2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("candidateATPorts() = %#v, want %#v", got, want)
	}
}

func TestHasUsableATDeviceNodeRequiresOpenableDevice(t *testing.T) {
	origDeviceNodeExists := deviceNodeExistsFunc
	t.Cleanup(func() {
		deviceNodeExistsFunc = origDeviceNodeExists
	})

	deviceNodeExistsFunc = func(path string) bool {
		return path == "/dev/ttyUSB2"
	}

	device := modemsysfs.Device{ATTTYs: []string{"ttyUSB0", "ttyUSB2"}}
	if !hasUsableATDeviceNode(device) {
		t.Fatalf("hasUsableATDeviceNode() = false, want true")
	}
}

func TestEnsureATDriverRepairsMissingDeviceNodes(t *testing.T) {
	origRunCommand := runCommandFunc
	origWriteNewID := writeOptionNewIDFunc
	origRunBindHelper := runBindHelperFunc
	origDeviceNodeExists := deviceNodeExistsFunc
	origFirstDevice := firstDeviceFunc
	origSleep := sleepFunc
	t.Cleanup(func() {
		runCommandFunc = origRunCommand
		writeOptionNewIDFunc = origWriteNewID
		runBindHelperFunc = origRunBindHelper
		deviceNodeExistsFunc = origDeviceNodeExists
		firstDeviceFunc = origFirstDevice
		sleepFunc = origSleep
	})

	firstDeviceFunc = func() (modemsysfs.Device, bool) {
		return modemsysfs.Device{ATTTYs: []string{"ttyUSB0"}}, true
	}
	deviceNodeAvailable := false
	deviceNodeExistsFunc = func(path string) bool {
		return deviceNodeAvailable && path == "/dev/ttyUSB0"
	}
	runCommandFunc = func(_ context.Context, _ string, _ ...string) error {
		return nil
	}
	writeOptionNewIDFunc = func() error {
		return nil
	}
	runBindCalls := 0
	runBindHelperFunc = func(_ context.Context) error {
		runBindCalls++
		deviceNodeAvailable = true
		return nil
	}
	sleepFunc = func(time.Duration) {}

	if err := ensureATDriver(nil); err != nil {
		t.Fatalf("ensureATDriver() error = %v", err)
	}
	if runBindCalls != 1 {
		t.Fatalf("runBindHelper calls = %d, want 1", runBindCalls)
	}
}

func TestFindResponsiveATPortSelectsWorkingCandidate(t *testing.T) {
	origSend := sendATFunc
	origDeviceNodeExists := deviceNodeExistsFunc
	origFirstDevice := firstDeviceFunc
	t.Cleanup(func() {
		sendATFunc = origSend
		deviceNodeExistsFunc = origDeviceNodeExists
		firstDeviceFunc = origFirstDevice
	})

	firstDeviceFunc = func() (modemsysfs.Device, bool) {
		return modemsysfs.Device{ATTTYs: []string{"ttyUSB0", "ttyUSB2"}}, true
	}
	deviceNodeExistsFunc = func(path string) bool {
		return path == "/dev/ttyUSB0" || path == "/dev/ttyUSB2"
	}
	sendATFunc = func(path, command string, timeout time.Duration) (string, error) {
		if command != "AT" {
			t.Fatalf("sendAT command = %q, want AT", command)
		}
		switch path {
		case "/dev/ttyUSB0":
			return "", errors.New("wrong serial function")
		case "/dev/ttyUSB2":
			return "OK", nil
		default:
			return "", errors.New("unexpected port")
		}
	}

	port, ok := findResponsiveATPort(State{PrimaryPort: "ttyUSB0"})
	if !ok {
		t.Fatalf("findResponsiveATPort() ok = false, want true")
	}
	if port != "ttyUSB2" {
		t.Fatalf("findResponsiveATPort() port = %q, want ttyUSB2", port)
	}
}
