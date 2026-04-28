package modemctl

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/vxider/codex-buddy/uconsole/network-manager/internal/modemsysfs"
)

type State struct {
	Installed          bool
	Available          bool
	HardwarePresent    bool
	ATReady            bool
	ModemManagerActive bool
	Error              string
	Manufacturer       string
	Model              string
	Revision           string
	PrimaryPort        string
	NetworkPort        string
	ModemState         string
	PowerState         string
	RegistrationState  string
	PacketServiceState string
	SignalQuality      string
	WiFiConnected      bool
	WiFiDevice         string
	WiFiConnection     string
	WiFiState          string
}

func Load(ctx context.Context) State {
	state := State{}

	loadModemManagerState(ctx, &state)
	loadSysfsFallback(&state)

	if _, err := exec.LookPath("nmcli"); err == nil {
		loadNMCLIState(ctx, &state)
	}

	if _, err := exec.LookPath("mmcli"); err != nil {
		return state
	}
	state.Installed = true

	listRaw, err := run(ctx, "mmcli", "-L")
	if err != nil {
		state.Error = err.Error()
		return state
	}

	modemID := parseFirstModemID(string(listRaw))
	if modemID == "" {
		if strings.Contains(string(listRaw), "No modems were found") {
			if state.HardwarePresent {
				if state.Manufacturer == "" {
					state.Manufacturer = "CMCC"
				}
				if state.Model == "" {
					state.Model = "ML307C/ML307B"
				}
				if state.ModemState == "" {
					state.ModemState = "present"
				}
				return state
			}
			state.Error = "mmcli reports no modems"
			return state
		}
		state.Error = "mmcli -L returned no modem id: " + strings.Join(strings.Fields(string(listRaw)), " ")
		return state
	}

	raw, err := run(ctx, "mmcli", "-m", modemID, "-K")
	if err != nil {
		state.Error = err.Error()
		return state
	}

	state.Available = true
	values := parseKeyValue(raw)
	state.Manufacturer = values["modem.generic.manufacturer"]
	state.Model = values["modem.generic.model"]
	state.Revision = values["modem.generic.revision"]
	state.PrimaryPort = values["modem.generic.primary-port"]
	state.ModemState = values["modem.generic.state"]
	state.PowerState = values["modem.generic.power-state"]
	state.RegistrationState = values["modem.3gpp.registration-state"]
	state.PacketServiceState = values["modem.3gpp.packet-service-state"]
	state.SignalQuality = values["modem.generic.signal-quality.value"]

	for key, value := range values {
		if !strings.HasPrefix(key, "modem.generic.ports.value[") {
			continue
		}
		if strings.Contains(value, "(net)") && state.NetworkPort == "" {
			state.NetworkPort = strings.TrimSpace(strings.TrimSuffix(value, "(net)"))
		}
	}

	return state
}

func LiveSummary(state State) string {
	if state.Error != "" {
		return "unavailable"
	}
	switch {
	case !state.Installed:
		return "not installed"
	case state.HardwarePresent && state.ATReady && !state.ModemManagerActive:
		return "present"
	case state.HardwarePresent && state.ATReady && state.ModemManagerActive && !state.Available:
		return "driver ready"
	case !state.Available:
		return "not found"
	case strings.EqualFold(state.PacketServiceState, "attached"):
		return "online"
	case strings.EqualFold(state.RegistrationState, "home"), strings.EqualFold(state.RegistrationState, "roaming"):
		return "registered"
	case strings.EqualFold(state.ModemState, "disabled"):
		return "disabled"
	default:
		return "detached"
	}
}

func DesiredTarget(mode string, wifiConnected bool) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "standby":
		return "standby"
	case "off":
		return "off"
	case "auto":
		if wifiConnected {
			return "standby"
		}
		return "on"
	default:
		return "on"
	}
}

func loadNMCLIState(ctx context.Context, state *State) {
	raw, err := run(ctx, "nmcli", "-t", "-f", "DEVICE,TYPE,STATE,CONNECTION", "device", "status")
	if err != nil {
		return
	}

	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 4 {
			continue
		}
		device := parts[0]
		deviceType := parts[1]
		deviceState := parts[2]
		connection := strings.Join(parts[3:], ":")
		if deviceType != "wifi" {
			continue
		}
		state.WiFiDevice = device
		state.WiFiConnection = connection
		state.WiFiState = deviceState
		if wifiStateConnected(deviceState) {
			state.WiFiConnected = true
			return
		}
	}
}

func wifiStateConnected(deviceState string) bool {
	state := strings.ToLower(strings.TrimSpace(deviceState))
	if state == "connected" || strings.HasPrefix(state, "connected ") || strings.HasPrefix(state, "connected(") {
		return true
	}

	trimmed := strings.TrimSpace(deviceState)
	return trimmed == "已连接" || strings.HasPrefix(trimmed, "已连接 ")
}

func loadModemManagerState(ctx context.Context, state *State) {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return
	}
	cmd := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", "ModemManager")
	if err := cmd.Run(); err == nil {
		state.ModemManagerActive = true
	}
}

func loadSysfsFallback(state *State) {
	device, ok := modemsysfs.FirstDevice()
	if !ok {
		return
	}

	state.HardwarePresent = true
	if device.ATTTY != "" {
		state.ATReady = true
		if state.PrimaryPort == "" {
			state.PrimaryPort = device.ATTTY
		}
	}
	if state.NetworkPort == "" {
		state.NetworkPort = device.NetworkInterface
	}
}

func parseKeyValue(raw []byte) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		values[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return values
}

var modemPathPattern = regexp.MustCompile(`/Modem/([0-9]+)`)

func parseFirstModemID(text string) string {
	match := modemPathPattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), message)
	}
	return stdout.Bytes(), nil
}
