# Tailscale Tray

Standalone taskbar plugin for:

- Tailscale connect / disconnect
- Tailnet device visibility
- Exit node selection

## Behavior

The tray app shells out to the local `tailscale` CLI and refreshes state periodically.

- `Connected` toggles between `tailscale up` and `tailscale down`
- `Network Devices` shows known tailnet peers
- `Exit Nodes` lets you enable, switch, or clear the current exit node with `tailscale set --exit-node=...`
- Clicking the account entries opens the Tailscale admin machines page

The app also keeps a single-instance lock under `XDG_RUNTIME_DIR` and supports `--replace-existing` so desktop launchers can restart it cleanly.

## Requirements

- Go 1.22+
- `tailscale` installed and available in `PATH`
- An active Linux tray host with StatusNotifier / AppIndicator support

The app reads state from `tailscale status --json` and `tailscale debug prefs`, so it is most useful after the local node has already joined a tailnet.

## Install

Dry run:

```bash
./scripts/build-install.sh --dry-run
```

Install:

```bash
./scripts/build-install.sh
```

Install without autostart:

```bash
./scripts/build-install.sh --no-autostart
```

By default the installer:

- builds the module locally
- installs the binary to `~/.local/bin/tailscale-tray`
- writes `~/.local/share/applications/tailscale-tray.desktop`
- installs an autostart entry at `~/.config/autostart/tailscale-tray.desktop`

## Notes

- Runtime logs are appended to `~/.cache/tailscale-tray/tailscale-tray.log` when the cache directory is available.
- If `tailscale` is missing or returns an error, the tray remains visible and shows the latest compact error message.
- Exit nodes that are offline are shown but cannot be selected unless they are already active.
