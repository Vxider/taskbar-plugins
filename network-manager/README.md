# Network Manager Tray

Standalone taskbar plugin for:

- CMCC ML307-series 4G modem mode control: `On`, `Standby`, `Off`, `Auto`

## Hardware

The current implementation is written for CMCC ML307-series hardware.

- README-level target: `ML307`
- Sysfs fallback match in code: USB vendor/product `2ecc:3012`
- Fallback model label in code when `mmcli` cannot identify the modem: `ML307C/ML307B`

## 4G behavior

The plugin uses the ML307 modem's AT control port and maps modes like this:

- `On` -> `AT+CFUN=1`
- `Standby` -> `AT+CFUN=4`
- `Off` -> `AT+CFUN=0`
- `Auto` -> `Standby` when Wi-Fi is connected, `On` when Wi-Fi is not connected

To make low-power states stable, the helper keeps `ModemManager` stopped while the modem is in `Standby` or `Off`.
When switching back to `On`, the helper restores `AT+CFUN=1` and starts `ModemManager` again.

## Permissions

4G write actions require privilege. The tray app uses:

```text
pkexec <installed-binary> --helper modem <on|standby|off>
```

That means:

- Manual modem mode clicks can work with a pkexec prompt.
- `Auto` mode is much smoother if you install the optional polkit rule.

## Install

Dry run:

```bash
./scripts/build-install.sh --dry-run
```

Install:

```bash
./scripts/build-install.sh
```

Install with passwordless helper authorization for the current user:

```bash
./scripts/build-install.sh --install-polkit-rule
```

## Notes

- The plugin persists the selected 4G mode under the user's config directory.
- After restoring the modem from `Standby` or `Off` to `On`, the modem network interface comes back immediately; `NetworkManager` visibility may lag slightly.
