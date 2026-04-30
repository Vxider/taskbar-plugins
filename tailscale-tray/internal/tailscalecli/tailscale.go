package tailscalecli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

type State struct {
	Installed    bool
	Online       bool
	BackendState string
	Error        string
	TailnetName  string
	UserLogin    string
	SelfName     string
	SelfDNSName  string
	SelfIP       string
	ExitNodeName string
	ExitNodeIP   string
	ExitNodeID   string
	Peers        []Peer
	ExitNodes    []ExitNode
}

type Peer struct {
	Name    string
	DNSName string
	IP      string
	Online  bool
}

type ExitNode struct {
	ID      string
	Name    string
	IP      string
	Online  bool
	Current bool
}

func Load(ctx context.Context) State {
	if _, err := exec.LookPath("tailscale"); err != nil {
		return State{}
	}

	state := State{Installed: true}
	statusRaw, err := run(ctx, "status", "--json")
	if err != nil {
		state.Error = err.Error()
		return state
	}

	var statusPayload map[string]any
	if err := json.Unmarshal(statusRaw, &statusPayload); err != nil {
		state.Error = fmt.Sprintf("tailscale status parse failed: %v", err)
		return state
	}

	state.BackendState = strings.TrimSpace(asString(statusPayload["BackendState"]))
	self := asMap(statusPayload["Self"])
	state.Online = asBool(self["Online"])
	state.SelfName = preferredNodeName(self)
	state.SelfDNSName = strings.TrimSuffix(strings.TrimSpace(asString(self["DNSName"])), ".")
	state.SelfIP = firstString(asSlice(self["TailscaleIPs"]))
	state.TailnetName = preferredTailnetName(asMap(statusPayload["CurrentTailnet"]))
	state.UserLogin = lookupUserLogin(statusPayload, self)

	for peerID, rawPeer := range asMap(statusPayload["Peer"]) {
		peer := asMap(rawPeer)
		peerName := preferredNodeName(peer)
		peerIP := firstString(asSlice(peer["TailscaleIPs"]))
		peerOnline := asBool(peer["Online"])

		state.Peers = append(state.Peers, Peer{
			Name:    peerName,
			DNSName: strings.TrimSuffix(strings.TrimSpace(asString(peer["DNSName"])), "."),
			IP:      peerIP,
			Online:  peerOnline,
		})

		if !asBool(peer["ExitNodeOption"]) {
			continue
		}
		state.ExitNodes = append(state.ExitNodes, ExitNode{
			ID:     firstNonEmpty(asString(peer["ID"]), asString(peer["StableID"]), asString(peer["NodeID"]), peerID),
			Name:   peerName,
			IP:     peerIP,
			Online: peerOnline,
		})
	}

	prefsRaw, err := run(ctx, "debug", "prefs")
	if err == nil {
		var prefsPayload map[string]any
		if json.Unmarshal(prefsRaw, &prefsPayload) == nil {
			state.ExitNodeID = strings.TrimSpace(asString(prefsPayload["ExitNodeID"]))
			state.ExitNodeIP = strings.TrimSpace(asString(prefsPayload["ExitNodeIP"]))
		}
	}

	for i := range state.ExitNodes {
		node := &state.ExitNodes[i]
		if node.ID != "" && node.ID == state.ExitNodeID {
			node.Current = true
		}
		if !node.Current && node.IP != "" && node.IP == state.ExitNodeIP {
			node.Current = true
		}
		if node.Current {
			state.ExitNodeName = node.Name
		}
	}

	if state.ExitNodeName == "" {
		state.ExitNodeName = firstNonEmpty(state.ExitNodeIP, state.ExitNodeID)
	}

	sort.SliceStable(state.ExitNodes, func(i, j int) bool {
		if state.ExitNodes[i].Current != state.ExitNodes[j].Current {
			return state.ExitNodes[i].Current
		}
		if state.ExitNodes[i].Online != state.ExitNodes[j].Online {
			return state.ExitNodes[i].Online
		}
		return state.ExitNodes[i].Name < state.ExitNodes[j].Name
	})

	sort.SliceStable(state.Peers, func(i, j int) bool {
		if state.Peers[i].Online != state.Peers[j].Online {
			return state.Peers[i].Online
		}
		return state.Peers[i].Name < state.Peers[j].Name
	})

	return state
}

func SetOnline(ctx context.Context, online bool) error {
	if _, err := exec.LookPath("tailscale"); err != nil {
		return fmt.Errorf("tailscale cli not found")
	}

	command := "down"
	if online {
		command = "up"
	}

	_, err := run(ctx, command)
	return err
}

func SetExitNode(ctx context.Context, target string) error {
	if _, err := exec.LookPath("tailscale"); err != nil {
		return fmt.Errorf("tailscale cli not found")
	}

	arg := "--exit-node="
	if trimmed := strings.TrimSpace(target); trimmed != "" {
		arg = "--exit-node=" + trimmed
	}

	_, err := run(ctx, "set", arg)
	return err
}

func run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "tailscale", args...)
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
		return nil, fmt.Errorf("tailscale %s: %s", strings.Join(args, " "), message)
	}
	return stdout.Bytes(), nil
}

func preferredNodeName(node map[string]any) string {
	dnsName := strings.TrimSuffix(strings.TrimSpace(asString(node["DNSName"])), ".")
	if dnsName != "" {
		parts := strings.Split(dnsName, ".")
		if len(parts) > 0 && parts[0] != "" {
			return parts[0]
		}
		return dnsName
	}
	return firstNonEmpty(
		strings.TrimSpace(asString(node["HostName"])),
		strings.TrimSpace(asString(node["Name"])),
		strings.TrimSpace(asString(node["ComputedName"])),
		"unknown",
	)
}

func preferredTailnetName(tailnet map[string]any) string {
	return firstNonEmpty(
		strings.TrimSpace(asString(tailnet["Name"])),
		strings.TrimSpace(asString(tailnet["MagicDNSSuffix"])),
		strings.TrimSpace(asString(tailnet["DNSName"])),
	)
}

func lookupUserLogin(statusPayload map[string]any, self map[string]any) string {
	userID := strings.TrimSpace(asString(self["UserID"]))
	if userID == "" {
		userID = strings.TrimSpace(asString(self["User"]))
	}

	users := asMap(statusPayload["User"])
	if userID != "" {
		if user := asMap(users[userID]); len(user) > 0 {
			return firstNonEmpty(
				strings.TrimSpace(asString(user["LoginName"])),
				strings.TrimSpace(asString(user["DisplayName"])),
				strings.TrimSpace(asString(user["Name"])),
			)
		}
	}

	for _, rawUser := range users {
		user := asMap(rawUser)
		if userID != "" {
			if strings.TrimSpace(asString(user["ID"])) != userID && strings.TrimSpace(asString(user["UserID"])) != userID {
				continue
			}
		}
		if login := firstNonEmpty(
			strings.TrimSpace(asString(user["LoginName"])),
			strings.TrimSpace(asString(user["DisplayName"])),
			strings.TrimSpace(asString(user["Name"])),
		); login != "" {
			return login
		}
	}

	return ""
}

func asMap(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

func asSlice(value any) []any {
	result, _ := value.([]any)
	return result
}

func asString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func asBool(value any) bool {
	typed, ok := value.(bool)
	return ok && typed
}

func firstString(values []any) string {
	for _, value := range values {
		text := strings.TrimSpace(asString(value))
		if text != "" {
			return text
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
