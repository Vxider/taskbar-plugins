package configstate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const (
	ModeOn      = "on"
	ModeStandby = "standby"
	ModeOff     = "off"
	ModeAuto    = "auto"
)

type State struct {
	ModemMode         string `json:"modem_mode"`
	LastAppliedTarget string `json:"last_applied_target"`
}

func Load() State {
	path, err := filePath()
	if err != nil {
		return defaultState()
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return defaultState()
	}

	var state State
	if err := json.Unmarshal(raw, &state); err != nil {
		return defaultState()
	}

	state.ModemMode = normalizeMode(state.ModemMode)
	state.LastAppliedTarget = normalizeTarget(state.LastAppliedTarget)
	return state
}

func Save(state State) error {
	path, err := filePath()
	if err != nil {
		return err
	}

	state.ModemMode = normalizeMode(state.ModemMode)
	state.LastAppliedTarget = normalizeTarget(state.LastAppliedTarget)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func defaultState() State {
	return State{
		ModemMode:         ModeAuto,
		LastAppliedTarget: ModeOn,
	}
}

func normalizeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case ModeStandby:
		return ModeStandby
	case ModeOff:
		return ModeOff
	case ModeAuto:
		return ModeAuto
	default:
		return ModeOn
	}
}

func normalizeTarget(target string) string {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case ModeStandby:
		return ModeStandby
	case ModeOff:
		return ModeOff
	default:
		return ModeOn
	}
}

func filePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "network-manager-tray", "state.json"), nil
}
