# Taskbar Plugins

Standalone taskbar plugins for this workspace.

## Projects

### `tailscale-tray`

A lightweight Tailscale tray app.

- Toggle Tailscale connection state
- Show current device and account info
- List network devices and exit nodes
- Installable with a local desktop entry and autostart integration

Build/install:

```bash
./tailscale-tray/scripts/build-install.sh
```

Dry run:

```bash
./tailscale-tray/scripts/build-install.sh --dry-run
```

### `network-manager`

A tray app focused on 4G modem power and mode control.

- Modes: `On`, `Standby`, `Off`, `Auto`
- Uses the modem AT control port
- Can stop `ModemManager` in lower-power states for stability
- Optional polkit rule for smoother privileged actions

Build/install:

```bash
./network-manager/scripts/build-install.sh
```

Dry run:

```bash
./network-manager/scripts/build-install.sh --dry-run
```

Install with the optional polkit rule:

```bash
./network-manager/scripts/build-install.sh --install-polkit-rule
```

## Repository Layout

```text
.
├── network-manager/
└── tailscale-tray/
```

## Notes

- Build outputs are intentionally ignored by git.
- Each project is a separate Go module with its own `go.mod`.
