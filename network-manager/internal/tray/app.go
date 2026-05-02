package tray

import (
	"context"
	"image/color"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/systray"

	"github.com/vxider/codex-buddy/uconsole/network-manager/internal/configstate"
	"github.com/vxider/codex-buddy/uconsole/network-manager/internal/modemctl"
)

type App struct {
	logger *log.Logger

	ctx    context.Context
	cancel context.CancelFunc

	systemWritesEnabled bool

	mu        sync.Mutex
	refreshMu sync.Mutex

	config configstate.State

	modemState modemctl.State

	modemBusy bool
	lastError string

	lastModemAttemptTarget string
	lastModemAttemptAt     time.Time

	statusItem       *systray.MenuItem
	errorItem        *systray.MenuItem
	modemOnItem      *systray.MenuItem
	modemStandbyItem *systray.MenuItem
	modemOffItem     *systray.MenuItem
	modemAutoItem    *systray.MenuItem
	quitItem         *systray.MenuItem
}

func Run(logger *log.Logger) {
	ctx, cancel := context.WithCancel(context.Background())
	app := &App{
		logger:              logger,
		ctx:                 ctx,
		cancel:              cancel,
		systemWritesEnabled: systemWritesEnabledFromEnv(os.Getenv),
		config:              configstate.Load(),
	}
	systray.Run(app.onReady, app.onExit)
}

func (a *App) onReady() {
	// Some Linux tray hosts do not handle menu-only StatusNotifierItems
	// reliably. Provide activation handlers so icon clicks never return
	// UnknownMethod while the DBusMenu remains available to the host.
	systray.SetOnTapped(a.handleTrayTapped)
	systray.SetOnSecondaryTapped(func() {})

	a.statusItem = systray.AddMenuItem("4G: loading...", "")
	a.statusItem.Disable()
	a.errorItem = systray.AddMenuItem("", "")
	a.errorItem.Disable()
	a.errorItem.Hide()

	systray.AddSeparator()
	a.modemOnItem = systray.AddMenuItemCheckbox("4G On", "", false)
	a.modemStandbyItem = systray.AddMenuItemCheckbox("4G Standby", "", false)
	a.modemOffItem = systray.AddMenuItemCheckbox("4G Off", "", false)
	a.modemAutoItem = systray.AddMenuItemCheckbox("4G Auto", "", false)

	systray.AddSeparator()
	a.quitItem = systray.AddMenuItem("Quit", "Quit the tray app")

	go a.watchQuit()
	go a.watchModeClicks()
	go a.pollLoop()

	// Avoid refreshing from systray.TrayOpenedCh. Some DBusMenu hosts can keep
	// an input grab stuck if the menu layout is mutated while the menu opens.
	a.refresh()
}

func (a *App) handleTrayTapped() {
	go a.refresh()
}

func (a *App) onExit() {
	if a.cancel != nil {
		a.cancel()
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

func (a *App) watchModeClicks() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-a.modemOnItem.ClickedCh:
			a.setModemMode(configstate.ModeOn)
		case <-a.modemStandbyItem.ClickedCh:
			a.setModemMode(configstate.ModeStandby)
		case <-a.modemOffItem.ClickedCh:
			a.setModemMode(configstate.ModeOff)
		case <-a.modemAutoItem.ClickedCh:
			a.setModemMode(configstate.ModeAuto)
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
	a.refreshMu.Lock()
	defer a.refreshMu.Unlock()

	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
	defer cancel()

	modemState := modemctl.Load(ctx)

	a.mu.Lock()
	a.modemState = modemState
	config := a.config
	modemBusy := a.modemBusy
	modemError := modemErrorForDisplay(modemState, config)
	if modemError != "" {
		a.lastError = modemError
	}
	if modemError == "" && !modemBusy {
		a.lastError = ""
	}
	a.mu.Unlock()

	a.render(modemState, modemBusy, config)
	a.maybeApplyAuto(modemState, config)
}

func (a *App) setModemMode(mode string) {
	if !a.systemWritesEnabled {
		a.mu.Lock()
		a.lastError = "system writes disabled"
		modemState := a.modemState
		modemBusy := a.modemBusy
		config := a.config
		a.mu.Unlock()
		a.render(modemState, modemBusy, config)
		return
	}

	a.mu.Lock()
	a.config.ModemMode = mode
	config := a.config
	a.mu.Unlock()

	if err := configstate.Save(config); err != nil {
		a.mu.Lock()
		a.lastError = err.Error()
		a.mu.Unlock()
	}

	a.refresh()
	target := modemctl.DesiredTarget(mode, a.modemState.AltNetConnected)
	a.applyModemTarget(target, true)
}

func (a *App) maybeApplyAuto(modemState modemctl.State, config configstate.State) {
	if !a.systemWritesEnabled {
		return
	}

	target := modemctl.DesiredTarget(config.ModemMode, modemState.AltNetConnected)
	if liveTargetSatisfied(modemState, target) {
		a.syncLastAppliedTarget(target, config)
		return
	}

	a.mu.Lock()
	busy := a.modemBusy
	lastApplied := a.config.LastAppliedTarget
	lastTarget := a.lastModemAttemptTarget
	lastAt := a.lastModemAttemptAt
	a.mu.Unlock()

	if !shouldReconcileModemTarget(config.ModemMode, target, lastApplied, lastTarget, lastAt, busy) {
		return
	}
	a.applyModemTarget(target, false)
}

func liveTargetSatisfied(state modemctl.State, target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	if target != configstate.ModeOn {
		return false
	}
	if !state.Available {
		return false
	}
	return !strings.EqualFold(strings.TrimSpace(state.ModemState), "disabled")
}

func (a *App) syncLastAppliedTarget(target string, config configstate.State) {
	target = strings.TrimSpace(target)
	if target == "" || target == config.LastAppliedTarget {
		return
	}

	a.mu.Lock()
	if a.config.LastAppliedTarget == target {
		a.mu.Unlock()
		return
	}
	a.config.LastAppliedTarget = target
	updated := a.config
	a.mu.Unlock()

	if err := configstate.Save(updated); err != nil {
		a.mu.Lock()
		a.lastError = err.Error()
		a.mu.Unlock()
		if a.logger != nil {
			a.logger.Printf("modem helper sync target=%s err=%v", target, err)
		}
	}
}

func shouldReconcileModemTarget(mode, target, lastApplied, lastTarget string, lastAt time.Time, busy bool) bool {
	if busy || target == "" || target == lastApplied {
		return false
	}

	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != configstate.ModeAuto {
		// Manual modes get one automatic reconciliation attempt per app run.
		return target != lastTarget
	}

	// On uConsole, background transitions into modem standby/off have been
	// observed to destabilize the built-in keyboard path. Keep auto-mode
	// restorative only: automatically bring the modem back online, but do not
	// silently push it into lower-power states.
	if target != configstate.ModeOn {
		return false
	}

	if target == lastTarget && time.Since(lastAt) < 45*time.Second {
		return false
	}
	return true
}

func (a *App) applyModemTarget(target string, force bool) {
	if !a.systemWritesEnabled {
		if a.logger != nil {
			a.logger.Printf("modem helper skipped target=%s: system writes disabled", strings.TrimSpace(target))
		}
		return
	}

	target = strings.TrimSpace(target)
	if target == "" {
		return
	}

	a.mu.Lock()
	if a.modemBusy {
		a.mu.Unlock()
		return
	}
	if !force && target == a.config.LastAppliedTarget {
		a.mu.Unlock()
		return
	}
	a.modemBusy = true
	a.lastModemAttemptTarget = target
	a.lastModemAttemptAt = time.Now()
	modemState := a.modemState
	config := a.config
	a.mu.Unlock()

	a.render(modemState, true, config)

	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
		defer cancel()

		if a.logger != nil {
			a.logger.Printf("modem helper start target=%s", target)
		}
		err := runModemHelper(ctx, target)

		a.mu.Lock()
		a.modemBusy = false
		if err != nil {
			a.lastError = err.Error()
		} else {
			a.lastError = ""
			a.config.LastAppliedTarget = target
			config = a.config
		}
		a.mu.Unlock()

		if a.logger != nil {
			if err != nil {
				a.logger.Printf("modem helper failed target=%s err=%v", target, err)
			} else {
				a.logger.Printf("modem helper applied target=%s", target)
			}
		}

		if err == nil {
			if saveErr := configstate.Save(config); saveErr != nil {
				a.mu.Lock()
				a.lastError = saveErr.Error()
				a.mu.Unlock()
				if a.logger != nil {
					a.logger.Printf("modem helper save target=%s err=%v", target, saveErr)
				}
			}
		}

		a.refresh()
	}()
}

func (a *App) render(modemState modemctl.State, modemBusy bool, config configstate.State) {
	busy := modemBusy
	iconMode, iconBars := traySignalIcon(modemState, config, a.systemWritesEnabled)
	icon := trayIcon(
		trayColor(modemState, config, busy),
		iconMode,
		iconBars,
	)
	systray.SetTemplateIcon(icon, icon)
	systray.SetTitle(trayTitle(modemState, config, busy))
	systray.SetTooltip(trayTooltip(modemState, config, busy))

	a.statusItem.SetTitle(modemMenuLabel(modemState, config, modemBusy))

	a.modemOnItem.Uncheck()
	a.modemStandbyItem.Uncheck()
	a.modemOffItem.Uncheck()
	a.modemAutoItem.Uncheck()
	switch config.ModemMode {
	case configstate.ModeStandby:
		a.modemStandbyItem.Check()
	case configstate.ModeOff:
		a.modemOffItem.Check()
	case configstate.ModeAuto:
		a.modemAutoItem.Check()
	default:
		a.modemOnItem.Check()
	}
	if modemBusy || !a.systemWritesEnabled {
		a.modemOnItem.Disable()
		a.modemStandbyItem.Disable()
		a.modemOffItem.Disable()
		a.modemAutoItem.Disable()
	} else {
		a.modemOnItem.Enable()
		a.modemStandbyItem.Enable()
		a.modemOffItem.Enable()
		a.modemAutoItem.Enable()
	}

	if modemError := modemErrorForDisplay(modemState, config); modemError != "" {
		a.errorItem.SetTitle(compactError(modemError))
		a.errorItem.Show()
	} else if a.lastError != "" {
		a.errorItem.SetTitle(compactError(a.lastError))
		a.errorItem.Show()
	} else {
		a.errorItem.Hide()
	}
}

func systemWritesEnabledFromEnv(getenv func(string) string) bool {
	switch strings.ToLower(strings.TrimSpace(getenv("NETWORK_MANAGER_TRAY_ENABLE_WRITES"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func runModemHelper(ctx context.Context, target string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	mode := target
	switch target {
	case configstate.ModeOn, configstate.ModeStandby, configstate.ModeOff:
	default:
		mode = configstate.ModeOff
	}

	cmd := exec.CommandContext(ctx, "pkexec", exe, "--helper", "modem", mode)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := compactError(string(output))
		if message == "" {
			message = err.Error()
		}
		return execError("pkexec helper: " + message)
	}
	return nil
}

type execError string

func (e execError) Error() string { return string(e) }

func trayColor(modemState modemctl.State, config configstate.State, busy bool) color.NRGBA {
	modemError := modemErrorForDisplay(modemState, config)
	switch {
	case busy:
		return color.NRGBA{R: 0x2F, G: 0x78, B: 0xD6, A: 0xFF}
	case modemError != "":
		return color.NRGBA{R: 0xC7, G: 0x83, B: 0x19, A: 0xFF}
	case strings.EqualFold(modemctl.LiveSummary(modemState), "online"):
		return color.NRGBA{R: 0x2D, G: 0x9A, B: 0x5F, A: 0xFF}
	case modemState.AltNetConnected || strings.EqualFold(modemctl.LiveSummary(modemState), "registered"):
		return color.NRGBA{R: 0x2B, G: 0x84, B: 0xC6, A: 0xFF}
	default:
		return color.NRGBA{R: 0x6B, G: 0x72, B: 0x79, A: 0xFF}
	}
}

func trayTitle(modemState modemctl.State, config configstate.State, busy bool) string {
	modemError := modemErrorForDisplay(modemState, config)
	switch {
	case busy:
		return "4G ..."
	case modemError != "":
		return "4G !"
	case config.ModemMode == configstate.ModeAuto:
		return "4G A"
	case strings.EqualFold(modemctl.LiveSummary(modemState), "online"), strings.EqualFold(modemctl.LiveSummary(modemState), "registered"):
		return "4G on"
	default:
		return "4G off"
	}
}

func trayTooltip(modemState modemctl.State, config configstate.State, busy bool) string {
	label := modemMenuLabel(modemState, config, busy)
	diag := modemDiagnosticLabel(modemState)
	if diag == "" {
		return label
	}
	return label + " | " + diag
}

func modemMenuLabel(modemState modemctl.State, config configstate.State, busy bool) string {
	if busy {
		return "4G: applying"
	}

	modeText := compactModeLabel(config.ModemMode)
	detailText := compactDetailLabel(modemState, config)
	if detailText == "" {
		return "4G: " + modeText
	}
	return "4G: " + modeText + " | " + detailText
}

func modemDiagnosticLabel(state modemctl.State) string {
	var parts []string

	if strings.TrimSpace(state.NetworkPort) != "" && !state.Available {
		parts = append(parts, "modem net "+strings.TrimSpace(state.NetworkPort))
	}
	if strings.TrimSpace(state.PrimaryPort) != "" && (!state.Available || !state.ModemManagerActive) {
		parts = append(parts, "AT "+strings.TrimSpace(state.PrimaryPort))
	}

	return strings.Join(parts, " | ")
}

func modemStateLabel(state modemctl.State, config configstate.State) string {
	if !state.Installed {
		return "not installed"
	}
	if !state.ModemManagerActive {
		switch config.LastAppliedTarget {
		case configstate.ModeStandby:
			if config.ModemMode == configstate.ModeAuto {
				return "standby by " + alternateNetworkLabel(state) + " (helper-managed)"
			}
			return "standby (helper-managed)"
		case configstate.ModeOff:
			return "off (helper-managed)"
		}
	}
	if modemErrorForDisplay(state, config) != "" {
		return "unavailable"
	}

	summary := modemctl.LiveSummary(state)
	switch config.ModemMode {
	case configstate.ModeStandby:
		return "standby (" + summary + ")"
	case configstate.ModeOff:
		return "off (" + summary + ")"
	case configstate.ModeAuto:
		if state.AltNetConnected {
			return "standby by " + alternateNetworkLabel(state) + " (" + summary + ")"
		}
		return "on by auto (" + summary + ")"
	default:
		return "on (" + summary + ")"
	}
}

func targetLabel(target string) string {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case configstate.ModeStandby:
		return "standby"
	case configstate.ModeOff:
		return "off"
	default:
		return "on"
	}
}

func compactModeLabel(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case configstate.ModeAuto:
		return "auto"
	case configstate.ModeStandby:
		return "stdby"
	case configstate.ModeOff:
		return "off"
	default:
		return "on"
	}
}

func compactDetailLabel(state modemctl.State, config configstate.State) string {
	if !state.Installed {
		return "no mm"
	}

	target := modemctl.DesiredTarget(config.ModemMode, state.AltNetConnected)
	if modemErrorForDisplay(state, config) != "" {
		return "n/a"
	}
	if target == configstate.ModeOff {
		return ""
	}
	if target == configstate.ModeStandby {
		if config.ModemMode == configstate.ModeAuto && state.AltNetConnected {
			return alternateNetworkCompactLabel(state)
		}
		return "stdby"
	}
	if signalText := signalQualityCompact(state, config); signalText != "" {
		return signalText
	}

	switch modemctl.LiveSummary(state) {
	case "online":
		return "online"
	case "registered":
		return "reg"
	case "disabled":
		return "off"
	case "detached":
		return "idle"
	case "not installed":
		return "no mm"
	case "not found":
		return "no dev"
	case "driver ready":
		return "ready"
	case "present":
		return "present"
	default:
		return ""
	}
}

func alternateNetworkLabel(state modemctl.State) string {
	switch strings.ToLower(strings.TrimSpace(state.AltNetType)) {
	case "ethernet":
		return "wired"
	case "wifi":
		return "wifi"
	default:
		return "network"
	}
}

func alternateNetworkCompactLabel(state modemctl.State) string {
	switch strings.ToLower(strings.TrimSpace(state.AltNetType)) {
	case "ethernet":
		return "wired"
	case "wifi":
		return "wifi"
	default:
		return "net"
	}
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

func traySignalIcon(state modemctl.State, config configstate.State, systemWritesEnabled bool) (signalIconMode, int) {
	if !systemWritesEnabled {
		if bars, ok := signalBarsFromQuality(state.SignalQuality); ok {
			return signalIconBars, bars
		}
	}

	target := modemctl.DesiredTarget(config.ModemMode, state.AltNetConnected)
	switch target {
	case configstate.ModeOff:
		return signalIconOff, 0
	case configstate.ModeStandby:
		return signalIconStandby, 0
	}
	if modemErrorForDisplay(state, config) != "" {
		return signalIconUnavailable, 0
	}

	bars, ok := signalBarsFromQuality(state.SignalQuality)
	if !ok {
		return signalIconUnavailable, 0
	}
	return signalIconBars, bars
}

func signalQualityCompact(state modemctl.State, config configstate.State) string {
	target := modemctl.DesiredTarget(config.ModemMode, state.AltNetConnected)
	if target != configstate.ModeOn {
		return ""
	}
	quality := strings.TrimSpace(strings.TrimSuffix(state.SignalQuality, "%"))
	if quality == "" {
		return ""
	}

	if _, ok := signalBarsFromQuality(quality); !ok {
		return ""
	}
	return quality + "%"
}

func signalBarsFromQuality(raw string) (int, bool) {
	raw = strings.TrimSpace(strings.TrimSuffix(raw, "%"))
	if raw == "" {
		return 0, false
	}

	quality, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	if quality < 0 {
		quality = 0
	}
	if quality > 100 {
		quality = 100
	}
	if quality == 0 {
		return 0, true
	}
	if quality >= 75 {
		return 4, true
	}
	if quality >= 50 {
		return 3, true
	}
	if quality >= 25 {
		return 2, true
	}
	return 1, true
}

func modemErrorForDisplay(state modemctl.State, config configstate.State) string {
	message := strings.TrimSpace(state.Error)
	if message == "" {
		return ""
	}

	target := modemctl.DesiredTarget(config.ModemMode, state.AltNetConnected)
	if target != configstate.ModeOn {
		lower := strings.ToLower(message)
		if strings.Contains(lower, "couldn't find the modemmanager process in the bus") {
			return ""
		}
	}

	return message
}
