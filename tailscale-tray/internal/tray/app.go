package tray

import (
	"context"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"fyne.io/systray"

	"github.com/vxider/codex-buddy/uconsole/tailscale-tray/internal/tailscalecli"
)

type App struct {
	logger *log.Logger

	ctx    context.Context
	cancel context.CancelFunc

	mu            sync.Mutex
	state         tailscalecli.State
	busy          bool
	lastError     string
	networkItems  []*systray.MenuItem
	exitNodeItems []*systray.MenuItem

	titleItem         *systray.MenuItem
	connectedItem     *systray.MenuItem
	accountItem       *systray.MenuItem
	accountDetailItem *systray.MenuItem
	deviceItem        *systray.MenuItem
	networkMenu       *systray.MenuItem
	errorItem         *systray.MenuItem
	exitNodesMenu     *systray.MenuItem
	quitItem          *systray.MenuItem
}

func Run(logger *log.Logger) {
	ctx, cancel := context.WithCancel(context.Background())
	app := &App{
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
	systray.Run(app.onReady, app.onExit)
}

func (a *App) onReady() {
	a.titleItem = systray.AddMenuItem("Tailscale", "")
	a.titleItem.Disable()
	a.connectedItem = systray.AddMenuItem("Loading...", "Connect or disconnect Tailscale")
	a.accountItem = systray.AddMenuItem("Device: loading...", "Open the Tailscale admin console")
	a.accountDetailItem = systray.AddMenuItem("Account: loading...", "Open the Tailscale admin console")
	a.errorItem = systray.AddMenuItem("", "")
	a.errorItem.Disable()
	a.errorItem.Hide()

	systray.AddSeparator()
	a.deviceItem = systray.AddMenuItem("This Device: loading...", "")
	a.deviceItem.Disable()
	a.networkMenu = systray.AddMenuItem("Network Devices", "Tailnet devices")
	a.exitNodesMenu = systray.AddMenuItem("Exit Nodes", "Available exit nodes")

	systray.AddSeparator()
	a.quitItem = systray.AddMenuItem("Quit", "Quit the tray app")

	go a.watchConnected()
	go a.watchAccount()
	go a.watchAccountDetail()
	go a.watchQuit()
	go a.watchTrayOpen()
	go a.pollLoop()

	a.refresh()
}

func (a *App) onExit() {
	if a.cancel != nil {
		a.cancel()
	}
}

func (a *App) watchConnected() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-a.connectedItem.ClickedCh:
			a.mu.Lock()
			state := a.state
			a.mu.Unlock()
			a.applyOnline(!state.Online)
		}
	}
}

func (a *App) watchAccount() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-a.accountItem.ClickedCh:
			a.openAdmin()
		}
	}
}

func (a *App) watchAccountDetail() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-a.accountDetailItem.ClickedCh:
			a.openAdmin()
		}
	}
}

func (a *App) watchQuit() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-a.quitItem.ClickedCh:
			systray.Quit()
			return
		}
	}
}

func (a *App) watchTrayOpen() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-systray.TrayOpenedCh:
			a.refresh()
		}
	}
}

func (a *App) pollLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.refresh()
		}
	}
}

func (a *App) refresh() {
	ctx, cancel := context.WithTimeout(a.ctx, 4*time.Second)
	defer cancel()

	state := tailscalecli.Load(ctx)

	a.mu.Lock()
	a.state = state
	if state.Error != "" {
		a.lastError = state.Error
	}
	busy := a.busy
	a.mu.Unlock()

	a.render(state, busy)
}

func (a *App) applyOnline(target bool) {
	a.mu.Lock()
	if a.busy {
		a.mu.Unlock()
		return
	}
	a.busy = true
	state := a.state
	a.mu.Unlock()

	a.render(state, true)

	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, 12*time.Second)
		defer cancel()

		err := tailscalecli.SetOnline(ctx, target)
		a.mu.Lock()
		a.busy = false
		if err != nil {
			a.lastError = err.Error()
		}
		a.mu.Unlock()

		a.refresh()
	}()
}

func (a *App) applyExitNode(target string) {
	a.mu.Lock()
	if a.busy {
		a.mu.Unlock()
		return
	}
	a.busy = true
	state := a.state
	a.mu.Unlock()

	a.render(state, true)

	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, 8*time.Second)
		defer cancel()

		err := tailscalecli.SetExitNode(ctx, target)
		a.mu.Lock()
		a.busy = false
		if err != nil {
			a.lastError = err.Error()
		}
		a.mu.Unlock()

		a.refresh()
	}()
}

func (a *App) render(state tailscalecli.State, busy bool) {
	icon := trayIcon()
	systray.SetIcon(icon)
	systray.SetTitle(trayTitle(state, busy))
	systray.SetTooltip(trayTooltip(state, busy))

	a.connectedItem.SetTitle(connectedMenuLabel(state))
	if busy || !state.Installed || state.Error != "" {
		a.connectedItem.Disable()
	} else {
		a.connectedItem.Enable()
	}

	a.accountItem.SetTitle(accountMenuLabel(state))
	a.accountDetailItem.SetTitle(accountDetailMenuLabel(state))
	if accountDetailMenuLabel(state) == "" {
		a.accountDetailItem.Hide()
	} else {
		a.accountDetailItem.Show()
	}
	if !state.Installed || state.Error != "" {
		a.accountItem.Disable()
		a.accountDetailItem.Disable()
	} else {
		a.accountItem.Enable()
		a.accountDetailItem.Enable()
	}
	a.deviceItem.SetTitle(deviceMenuLabel(state))
	a.deviceItem.SetTooltip(deviceMenuTooltip(state))

	if state.Error != "" {
		a.errorItem.SetTitle(compactError(state.Error))
		a.errorItem.Show()
	} else if a.lastError != "" {
		a.errorItem.SetTitle(compactError(a.lastError))
		a.errorItem.Show()
	} else {
		a.errorItem.Hide()
	}

	a.renderNetworkDevices(state)
	a.renderExitNodes(state, busy)
}

func (a *App) renderNetworkDevices(state tailscalecli.State) {
	a.mu.Lock()
	oldItems := append([]*systray.MenuItem(nil), a.networkItems...)
	a.networkItems = nil
	a.mu.Unlock()

	for _, item := range oldItems {
		item.Remove()
	}

	var newItems []*systray.MenuItem
	switch {
	case !state.Installed:
		item := a.networkMenu.AddSubMenuItem("tailscale not installed", "")
		item.Disable()
		newItems = append(newItems, item)
	case state.Error != "" && len(state.Peers) == 0:
		item := a.networkMenu.AddSubMenuItem("unavailable", "")
		item.Disable()
		newItems = append(newItems, item)
	case len(state.Peers) == 0:
		item := a.networkMenu.AddSubMenuItem("no devices", "")
		item.Disable()
		newItems = append(newItems, item)
	default:
		for _, peer := range state.Peers {
			peer := peer
			item := a.networkMenu.AddSubMenuItem(networkDeviceLabel(peer), networkDeviceTooltip(peer))
			go func(item *systray.MenuItem, peer tailscalecli.Peer) {
				for range item.ClickedCh {
					a.copyPeerURL(peer)
					return
				}
			}(item, peer)
			newItems = append(newItems, item)
		}
	}

	a.mu.Lock()
	a.networkItems = newItems
	a.mu.Unlock()
}

func (a *App) renderExitNodes(state tailscalecli.State, busy bool) {
	a.mu.Lock()
	oldItems := append([]*systray.MenuItem(nil), a.exitNodeItems...)
	a.exitNodeItems = nil
	a.mu.Unlock()

	for _, item := range oldItems {
		item.Remove()
	}

	var newItems []*systray.MenuItem
	switch {
	case !state.Installed:
		item := a.exitNodesMenu.AddSubMenuItem("tailscale not installed", "")
		item.Disable()
		newItems = append(newItems, item)
	case state.Error != "" && len(state.ExitNodes) == 0:
		item := a.exitNodesMenu.AddSubMenuItem("unavailable", "")
		item.Disable()
		newItems = append(newItems, item)
	default:
		offSelected := strings.TrimSpace(state.ExitNodeName) == "" && strings.TrimSpace(state.ExitNodeIP) == ""
		offItem := a.exitNodesMenu.AddSubMenuItemCheckbox("Off", "", offSelected)
		if busy {
			offItem.Disable()
		}
		go func(item *systray.MenuItem) {
			for range item.ClickedCh {
				a.applyExitNode("")
				return
			}
		}(offItem)
		newItems = append(newItems, offItem)

		if len(state.ExitNodes) == 0 {
			item := a.exitNodesMenu.AddSubMenuItem("no exit nodes", "")
			item.Disable()
			newItems = append(newItems, item)
			break
		}

		for _, node := range state.ExitNodes {
			node := node
			item := a.exitNodesMenu.AddSubMenuItemCheckbox(exitNodeLabel(node), exitNodeTooltip(node), node.Current)
			if busy || (!node.Online && !node.Current) {
				item.Disable()
			}
			go func(item *systray.MenuItem, node tailscalecli.ExitNode) {
				for range item.ClickedCh {
					if node.Current {
						a.applyExitNode("")
						return
					}
					target := strings.TrimSpace(node.IP)
					if target == "" {
						target = strings.TrimSpace(node.Name)
					}
					a.applyExitNode(target)
					return
				}
			}(item, node)
			newItems = append(newItems, item)
		}
	}

	a.mu.Lock()
	a.exitNodeItems = newItems
	a.mu.Unlock()
}

func (a *App) openURL(target string) {
	if err := openURL(target); err != nil {
		a.mu.Lock()
		a.lastError = err.Error()
		state := a.state
		busy := a.busy
		a.mu.Unlock()
		a.render(state, busy)
	}
}

func (a *App) openAdmin() {
	a.openURL("https://login.tailscale.com/admin/machines")
}

func (a *App) copyPeerURL(peer tailscalecli.Peer) {
	url := networkDeviceURL(peer)
	if url == "" {
		a.mu.Lock()
		a.lastError = "device has no Tailscale address"
		state := a.state
		busy := a.busy
		a.mu.Unlock()
		a.render(state, busy)
		return
	}

	if err := copyText(url); err != nil {
		a.mu.Lock()
		a.lastError = "copy failed: " + err.Error()
		state := a.state
		busy := a.busy
		a.mu.Unlock()
		a.render(state, busy)
		return
	}

	if err := showCopiedNotification(networkDeviceLabel(peer), url); err != nil {
		if a.logger != nil {
			a.logger.Printf("copy notification failed: %v", err)
		}
	}
}

func trayTitle(state tailscalecli.State, busy bool) string {
	if state.Online {
		return "TS on"
	}
	return "TS off"
}

func trayTooltip(state tailscalecli.State, busy bool) string {
	switch {
	case busy:
		return "Applying Tailscale change..."
	case !state.Installed:
		return "tailscale CLI not found"
	case state.Error != "":
		return compactError(state.Error)
	case state.ExitNodeName != "":
		return "Online via exit node: " + state.ExitNodeName
	case state.Online:
		return "Tailscale online"
	default:
		return "Tailscale offline"
	}
}

func connectedMenuLabel(state tailscalecli.State) string {
	switch {
	case !state.Installed:
		return "Unavailable"
	case state.Error != "":
		return "Unavailable"
	case state.Online:
		return "Connected"
	default:
		return "Disconnected"
	}
}

func accountMenuLabel(state tailscalecli.State) string {
	name := strings.TrimSpace(state.SelfDNSName)
	if name == "" {
		name = strings.TrimSpace(state.SelfName)
	}
	switch {
	case name != "":
		return name
	case strings.TrimSpace(state.TailnetName) != "":
		return state.TailnetName
	default:
		return "Account"
	}
}

func accountDetailMenuLabel(state tailscalecli.State) string {
	login := strings.TrimSpace(state.UserLogin)
	if login != "" {
		return login
	}
	if tailnet := strings.TrimSpace(state.TailnetName); tailnet != "" {
		return tailnet
	}
	return ""
}

func deviceMenuLabel(state tailscalecli.State) string {
	ip := strings.TrimSpace(state.SelfIP)
	if ip != "" {
		return "This Device: " + ip
	}
	name := strings.TrimSpace(state.SelfName)
	if name == "" {
		name = "unknown"
	}
	return "This Device: " + name
}

func deviceMenuTooltip(state tailscalecli.State) string {
	ip := strings.TrimSpace(state.SelfIP)
	if ip == "" {
		return ""
	}
	return ip
}

func networkDeviceLabel(peer tailscalecli.Peer) string {
	label := strings.TrimSpace(peer.Name)
	if label == "" {
		label = strings.TrimSpace(peer.IP)
	}
	if label == "" {
		label = "unknown"
	}
	return label
}

func networkDeviceTooltip(peer tailscalecli.Peer) string {
	parts := make([]string, 0, 3)
	if url := networkDeviceURL(peer); url != "" {
		parts = append(parts, "copy "+url)
	}
	if ip := strings.TrimSpace(peer.IP); ip != "" {
		parts = append(parts, ip)
	}
	if !peer.Online {
		parts = append(parts, "offline")
	}
	return strings.Join(parts, " ")
}

func networkDeviceURL(peer tailscalecli.Peer) string {
	if dnsName := strings.Trim(strings.TrimSpace(peer.DNSName), "."); dnsName != "" {
		return "https://" + dnsName
	}
	if ip := strings.TrimSpace(peer.IP); ip != "" {
		return "http://" + ip
	}
	return ""
}

func exitNodeLabel(node tailscalecli.ExitNode) string {
	label := strings.TrimSpace(node.Name)
	if label == "" {
		label = strings.TrimSpace(node.IP)
	}
	if label == "" {
		return "unknown"
	}
	return label
}

func exitNodeTooltip(node tailscalecli.ExitNode) string {
	parts := make([]string, 0, 2)
	if ip := strings.TrimSpace(node.IP); ip != "" {
		parts = append(parts, ip)
	}
	if !node.Online {
		parts = append(parts, "offline")
	}
	return strings.Join(parts, " ")
}

func compactError(message string) string {
	message = strings.Join(strings.Fields(strings.TrimSpace(message)), " ")
	if len(message) > 72 {
		return message[:69] + "..."
	}
	if message == "" {
		return "unknown error"
	}
	return message
}

func openURL(target string) error {
	openers := [][]string{
		{"xdg-open", target},
		{"gio", "open", target},
	}

	for _, args := range openers {
		if _, err := exec.LookPath(args[0]); err != nil {
			continue
		}
		cmd := exec.Command(args[0], args[1:]...)
		if err := cmd.Start(); err != nil {
			return err
		}
		go func() {
			_ = cmd.Wait()
		}()
		return nil
	}

	return exec.ErrNotFound
}

func copyText(text string) error {
	copiers := [][]string{
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
	}

	for _, args := range copiers {
		if _, err := exec.LookPath(args[0]); err != nil {
			continue
		}
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			return err
		}
		return nil
	}

	return exec.ErrNotFound
}

func showCopiedNotification(device, url string) error {
	title := "Tailscale URL copied"
	message := strings.TrimSpace(device)
	if message == "" || message == "unknown" {
		message = url
	} else {
		message += ": " + url
	}

	if _, err := exec.LookPath("notify-send"); err == nil {
		return exec.Command("notify-send", title, message).Run()
	}
	if _, err := exec.LookPath("gdbus"); err == nil {
		return exec.Command(
			"gdbus", "call", "--session",
			"--dest", "org.freedesktop.Notifications",
			"--object-path", "/org/freedesktop/Notifications",
			"--method", "org.freedesktop.Notifications.Notify",
			"tailscale-tray", "0", "", title, message, "[]", "{}", "3000",
		).Run()
	}

	return exec.ErrNotFound
}
