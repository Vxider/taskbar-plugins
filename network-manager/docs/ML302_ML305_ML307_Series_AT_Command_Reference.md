# ML302 / ML305 / ML307 Series AT Command Reference

## 1. Overview

This document is a normalized English reference for the ML302, ML305, and ML307 modem families. It organizes the command set into functional groups and presents each command with a consistent structure.

Covered model families:

- `ML302A-DCLM`
- `ML302A-DSLM`
- `ML302A-GCLM`
- `ML302A-GSLM`
- `ML302S`
- `ML302S-DNLM`
- `ML305A-DC`
- `ML305A-DS`
- `ML305A-DL`
- `ML305M-DSLM`
- `ML305U`
- `ML305U-DBLN`
- `ML307A-DCLN`
- `ML307A-DSLN`
- `ML307A-GCLN`
- `ML307A-GSLN`
- `ML307A-DL`
- `ML307R-DC`
- `ML307R-DL`
- `ML307S`
- `ML307S-DNLM`

## 2. Command Conventions

AT commands normally start with `AT` and end with `<CR>`.

Common forms:

- Execute: `AT+<CMD>`
- Set: `AT+<CMD>=<value>`
- Read: `AT+<CMD>?`
- Test: `AT+<CMD>=?`
- S-register: `ATS<n>=<value>`

Typical responses:

| Response | Meaning |
| --- | --- |
| `OK` | Command completed successfully |
| `ERROR` | Syntax error or generic failure |
| `+CME ERROR: <err>` | Equipment or ME-related error |
| `+CMS ERROR: <err>` | SMS-related error |
| `+CIS ERROR: <err>` | Vendor-specific or interface-related error |

Practical notes:

- Use `AT+CMEE=2` during development for verbose errors.
- Use `AT+<CMD>=?` to confirm model-specific support before deploying a workflow.
- Voice and supplementary-service commands are only useful on voice-capable variants.

## 3. General Commands

### 3.1 `ATE`

- Purpose: Enable or disable command echo.
- Syntax: `ATE[0|1]`
- Response: `OK` or error.
- Notes: `ATE0` disables local echo and is a common default for serial integration.

### 3.2 `ATS3`

- Purpose: Set the command-line termination character.
- Syntax: `ATS3=<n>`, `ATS3?`
- Response: Returns the configured value on read.
- Notes: The default is typically `13` (`CR`). Support range differs by model.

### 3.3 `ATS4`

- Purpose: Set the response formatting character.
- Syntax: `ATS4=<n>`, `ATS4?`
- Response: Returns the configured value on read.
- Notes: The default is typically `10` (`LF`). Some `-DL` variants do not support this command.

### 3.4 `ATS5`

- Purpose: Set the line-editing character.
- Syntax: `ATS5=<n>`, `ATS5?`
- Response: Returns the configured value on read.
- Notes: The default is typically `8` for backspace.

### 3.5 `+++`

- Purpose: Escape from data mode back to command mode.
- Syntax: `+++`
- Response: `OK` after the guard-time rules are satisfied.
- Notes: No `AT` prefix is used. This command does not drop the underlying data session.

### 3.6 `AT&F`

- Purpose: Restore manufacturer default settings.
- Syntax: `AT&F`
- Response: `OK` or error.
- Notes: Use with care during integration because it resets the active configuration profile.

### 3.7 `ATV`

- Purpose: Select result-code format.
- Syntax: `ATV0`, `ATV1`
- Response: `OK` or error.
- Notes: `ATV0` selects numeric result codes and `ATV1` selects text result codes.

### 3.8 `ATQ`

- Purpose: Control whether result codes are reported.
- Syntax: `ATQ0`, `ATQ1`
- Response: `OK` or no visible result depending on the mode.
- Notes: `ATQ1` suppresses result-code display and is usually avoided during debugging.

### 3.9 `ATZ`

- Purpose: Restore a stored user profile.
- Syntax: `ATZ`
- Response: `OK` or error.
- Notes: Behavior may vary slightly by firmware depending on profile persistence.

### 3.10 `ATX`

- Purpose: Control extended result-code behavior and call progress monitoring.
- Syntax: `ATX<n>`
- Response: `OK` or error.
- Notes: Mainly relevant in dial/call workflows.

### 3.11 `ATI`

- Purpose: Read product identification text.
- Syntax: `ATI`
- Response: Returns module identification strings followed by `OK`.
- Notes: Useful during manufacturing and field diagnostics.

### 3.12 `AT+GMI`

- Purpose: Read manufacturer identification.
- Syntax: `AT+GMI`
- Response: Returns the manufacturer string.
- Notes: Often overlaps with `AT+CGMI`.

### 3.13 `AT+CGMI`

- Purpose: Read manufacturer identification in 3GPP style.
- Syntax: `AT+CGMI`
- Response: Returns the manufacturer string.
- Notes: Use this form when staying close to 3GPP command naming.

### 3.14 `AT+GMM`

- Purpose: Read model identification.
- Syntax: `AT+GMM`
- Response: Returns the model string.
- Notes: Useful for distinguishing families and hardware variants.

### 3.15 `AT+CGMM`

- Purpose: Read model identification in 3GPP style.
- Syntax: `AT+CGMM`
- Response: Returns the model string.
- Notes: Often used together with `AT+CGMR`.

### 3.16 `AT+GMR`

- Purpose: Read firmware revision.
- Syntax: `AT+GMR`
- Response: Returns revision information.
- Notes: Revision string content is firmware-dependent.

### 3.17 `AT+CGMR`

- Purpose: Read firmware revision in 3GPP style.
- Syntax: `AT+CGMR`
- Response: Returns revision information.
- Notes: Use this command in version-detection scripts that already use `CG*` identifiers.

### 3.18 `AT+GSN`

- Purpose: Read the device serial number.
- Syntax: `AT+GSN`
- Response: Returns the serial number or IMEI-related identifier.
- Notes: Output format depends on the module implementation.

### 3.19 `AT+CGSN`

- Purpose: Read the device serial number in 3GPP style.
- Syntax: `AT+CGSN`
- Response: Returns the serial number or IMEI-related identifier.
- Notes: Some firmware revisions may support parameterized forms; validate with `AT+CGSN=?`.

### 3.20 `AT+IPR`

- Purpose: Set or read the fixed DTE baud rate.
- Syntax: `AT+IPR=<rate>`, `AT+IPR?`, `AT+IPR=?`
- Response: Returns the active or supported baud rates.
- Notes: Auto-baud support is not uniform across models.

### 3.21 `AT+CSCS`

- Purpose: Select the TE character set.
- Syntax: `AT+CSCS="<set>"`, `AT+CSCS?`, `AT+CSCS=?`
- Response: Returns the active or supported character sets.
- Notes: Common values include `GSM`, `IRA`, `UCS2`, and `HEX`, depending on model.

## 4. Call Control Commands

Call-control support is model-dependent. `ML307R` variants do not support these commands.

### 4.1 `ATS0`

- Purpose: Configure automatic answering.
- Syntax: `ATS0=<n>`, `ATS0?`
- Response: Returns the configured ring count.
- Notes: `0` disables auto-answer. On some variants this is only meaningful for data-call scenarios.

### 4.2 `ATA`

- Purpose: Answer an incoming call.
- Syntax: `ATA`
- Response: `OK`, call-connect result, or error.
- Notes: Use when a single incoming call is present.

### 4.3 `ATD`

- Purpose: Dial a number or start a data/voice call.
- Syntax: `ATD<number>`, `ATD<number>;`
- Response: `OK`, `CONNECT`, `BUSY`, `NO CARRIER`, or error.
- Notes: A trailing semicolon is typically used for voice-style dialing on supported models.

### 4.4 `ATH`

- Purpose: Hang up an active connection.
- Syntax: `ATH`
- Response: `OK` or error.
- Notes: Commonly used to release active, waiting, or held calls where supported.

### 4.5 `AT+CHUP`

- Purpose: Release the current call.
- Syntax: `AT+CHUP`, `AT+CHUP=?`
- Response: `OK` or error.
- Notes: Often behaves like `ATH` but is easier to script in 3GPP-oriented workflows.

### 4.6 `AT+CEER`

- Purpose: Read the extended report for the last unsuccessful call event.
- Syntax: `AT+CEER`
- Response: Returns a report string followed by `OK`.
- Notes: Use immediately after a failure to preserve useful diagnostics.

### 4.7 `AT+CRC`

- Purpose: Enable or disable extended incoming-call result codes.
- Syntax: `AT+CRC=<n>`, `AT+CRC?`, `AT+CRC=?`
- Response: Returns the active or supported mode.
- Notes: Relevant only on variants that support incoming call indications.

## 5. Network Service Commands

### 5.1 `AT+CREG`

- Purpose: Query or configure circuit-switched registration reporting.
- Syntax: `AT+CREG=<n>`, `AT+CREG?`, `AT+CREG=?`
- Response: Returns registration state and, depending on mode, location information.
- Notes: Commonly used to monitor network registration readiness.

### 5.2 `AT+COPS`

- Purpose: Query or select the network operator.
- Syntax: `AT+COPS?`, `AT+COPS=?`, `AT+COPS=<mode>[,...]`
- Response: Returns the current operator or supported operator list.
- Notes: Manual operator selection may take time and can fail if the selected PLMN is unavailable.

### 5.3 `AT+CLCK`

- Purpose: Query or control facility locks.
- Syntax: `AT+CLCK="<fac>",<mode>[,...]`
- Response: Returns status or `OK`.
- Notes: Typical uses include SIM PIN and network-related lock functions.

### 5.4 `AT+CHLD`

- Purpose: Control call-hold and multiparty operations.
- Syntax: `AT+CHLD=<n>`, `AT+CHLD=?`
- Response: `OK` or error.
- Notes: Used for releasing, holding, retrieving, or conferencing calls on supported voice variants.

### 5.5 `AT+CLCC`

- Purpose: List current calls.
- Syntax: `AT+CLCC`
- Response: Returns one or more `+CLCC:` lines followed by `OK`.
- Notes: Useful for call-state polling in telephony integrations.

### 5.6 `AT+CPOL`

- Purpose: Read or modify the preferred operator list.
- Syntax: `AT+CPOL=<index>,...`, `AT+CPOL?`, `AT+CPOL=?`
- Response: Returns configured preferred-operator entries.
- Notes: Support and field width differ by firmware and operator database format.

### 5.7 `AT+CPLS`

- Purpose: Select the active preferred PLMN list.
- Syntax: `AT+CPLS=<list>`, `AT+CPLS?`, `AT+CPLS=?`
- Response: Returns the selected list.
- Notes: Useful when multiple preferred lists are available in the module or SIM environment.

### 5.8 `AT+COPN`

- Purpose: Read operator-name mappings.
- Syntax: `AT+COPN`
- Response: Returns one or more operator-name entries.
- Notes: Not all models support full operator-name databases.

## 6. ME Control and Status Commands

### 6.1 `AT+CPAS`

- Purpose: Read the current equipment activity state.
- Syntax: `AT+CPAS`
- Response: Returns a numeric activity state.
- Notes: State values are defined by the command specification and firmware implementation.

### 6.2 `AT+CFUN`

- Purpose: Set or read the functionality level.
- Syntax: `AT+CFUN=<fun>[,<rst>]`, `AT+CFUN?`, `AT+CFUN=?`
- Response: Returns the current or supported functionality modes.
- Notes: Used to switch between full, minimum, offline, or reset-related states.

### 6.3 `AT+CSQ`

- Purpose: Read basic signal quality.
- Syntax: `AT+CSQ`
- Response: Returns RSSI and BER-style values.
- Notes: Use as a lightweight signal check before more advanced diagnostics.

### 6.4 `AT+CESQ`

- Purpose: Read extended signal quality metrics.
- Syntax: `AT+CESQ`
- Response: Returns extended radio-quality fields such as RXLEV, RSRQ, or RSRP, depending on access technology.
- Notes: The meaning of each numeric field depends on the serving RAT.

### 6.5 `AT+CCLK`

- Purpose: Read or set the real-time clock.
- Syntax: `AT+CCLK?`, `AT+CCLK="<time>"`
- Response: Returns the current clock value.
- Notes: Time interpretation can differ by firmware, especially when network time-zone update is enabled.

### 6.6 `AT+CLAC`

- Purpose: List supported AT commands.
- Syntax: `AT+CLAC`
- Response: Returns available commands followed by `OK`.
- Notes: Useful for runtime feature detection.

### 6.7 `AT+CTZU`

- Purpose: Control automatic time-zone updates.
- Syntax: `AT+CTZU=<mode>`, `AT+CTZU?`, `AT+CTZU=?`
- Response: Returns the active or supported mode.
- Notes: Often linked to NITZ-based updates provided by the network.

### 6.8 `AT+CTZR`

- Purpose: Control time-zone reporting.
- Syntax: `AT+CTZR=<mode>`, `AT+CTZR?`, `AT+CTZR=?`
- Response: Returns the active mode and can enable related URCs.
- Notes: Useful when the host must track time-zone changes automatically.

## 7. Packet Domain Commands

### 7.1 `AT+CGDCONT`

- Purpose: Define a PDP context.
- Syntax: `AT+CGDCONT=<cid>,<PDP_type>,<APN>...`, `AT+CGDCONT?`, `AT+CGDCONT=?`
- Response: Returns configured or supported context definitions.
- Notes: Use this command to set APN, PDP type, and related context parameters before data activation.

### 7.2 `AT+CGTFT`

- Purpose: Define a traffic flow template.
- Syntax: `AT+CGTFT=...`, `AT+CGTFT?`, `AT+CGTFT=?`
- Response: Returns configured or supported TFT parameters.
- Notes: Primarily relevant in advanced packet-filtering and QoS scenarios.

### 7.3 `AT+CGATT`

- Purpose: Attach to or detach from packet service.
- Syntax: `AT+CGATT=<state>`, `AT+CGATT?`
- Response: Returns the attach state.
- Notes: `1` attaches and `0` detaches.

### 7.4 `AT+CGACT`

- Purpose: Activate or deactivate PDP contexts.
- Syntax: `AT+CGACT=<state>,<cid>`, `AT+CGACT?`, `AT+CGACT=?`
- Response: Returns active-context state.
- Notes: Commonly used after `AT+CGDCONT` and optional authentication setup.

### 7.5 `AT+CGPADDR`

- Purpose: Read PDP address information.
- Syntax: `AT+CGPADDR[=<cid>]`
- Response: Returns IP-address information for the specified or active contexts.
- Notes: Use after activation when a dynamically assigned address is expected.

### 7.6 `AT+CGCLASS`

- Purpose: Read or set the GPRS mobile-station class.
- Syntax: `AT+CGCLASS=<class>`, `AT+CGCLASS?`, `AT+CGCLASS=?`
- Response: Returns the active or supported class.
- Notes: Availability is model-dependent and less relevant on LTE-centric workflows.

### 7.7 `AT+CGEREP`

- Purpose: Configure packet-domain event reporting.
- Syntax: `AT+CGEREP=<mode>[,<bfr>]`, `AT+CGEREP?`, `AT+CGEREP=?`
- Response: Returns the active reporting configuration.
- Notes: Controls unsolicited packet-service event notifications.

### 7.8 `AT+CGREG`

- Purpose: Query or configure GPRS registration reporting.
- Syntax: `AT+CGREG=<n>`, `AT+CGREG?`, `AT+CGREG=?`
- Response: Returns packet-domain registration state and related data.
- Notes: Useful for 2G/3G packet-service monitoring.

### 7.9 `AT+CEREG`

- Purpose: Query or configure EPS registration reporting.
- Syntax: `AT+CEREG=<n>`, `AT+CEREG?`, `AT+CEREG=?`
- Response: Returns LTE/EPS registration state and, in some modes, location and timer fields.
- Notes: This is one of the main registration checks for LTE deployments.

### 7.10 `AT+CGCONTRDP`

- Purpose: Read dynamic PDP-context parameters.
- Syntax: `AT+CGCONTRDP[=<cid>]`
- Response: Returns APN, address, DNS, and related negotiated values.
- Notes: Useful when the host needs the actual runtime network parameters.

### 7.11 `AT+CGEQOS`

- Purpose: Define EPS QoS parameters.
- Syntax: `AT+CGEQOS=...`, `AT+CGEQOS?`, `AT+CGEQOS=?`
- Response: Returns configured or supported QoS parameters.
- Notes: Support is limited on some models and firmware builds.

### 7.12 `AT+CGEQOSRDP`

- Purpose: Read negotiated or dynamic EPS QoS values.
- Syntax: `AT+CGEQOSRDP[=<cid>]`
- Response: Returns active QoS information.
- Notes: Use together with `AT+CGEQOS` only when the deployment actually depends on QoS tuning.

### 7.13 `AT+CEMODE`

- Purpose: Configure UE operation mode for EPS.
- Syntax: `AT+CEMODE=<mode>`, `AT+CEMODE?`, `AT+CEMODE=?`
- Response: Returns the active or supported mode.
- Notes: Relevant in voice/data service-mode control and similar EPS behavior settings.

### 7.14 `AT+CGDEL`

- Purpose: Delete inactive PDP contexts.
- Syntax: `AT+CGDEL=<cid>`, `AT+CGDEL=?`
- Response: `OK` or error.
- Notes: Removes stored context definitions that are not active.

### 7.15 `AT+CGAUTH`

- Purpose: Configure PDP authentication.
- Syntax: `AT+CGAUTH=<cid>,<auth_type>,<user>,<pwd>`, `AT+CGAUTH?`, `AT+CGAUTH=?`
- Response: Returns configured authentication settings where supported.
- Notes: Use when the APN requires PAP, CHAP, or similar credentials.

## 8. SIM and UICC Commands

### 8.1 `AT+CPIN`

- Purpose: Query SIM state or enter a PIN.
- Syntax: `AT+CPIN?`, `AT+CPIN="<pin>"`
- Response: Returns readiness state or accepts the entered PIN.
- Notes: One of the first commands to run after boot.

### 8.2 `AT+CPWD`

- Purpose: Change a password such as the SIM PIN.
- Syntax: `AT+CPWD="<fac>","<old>","<new>"`
- Response: `OK` or error.
- Notes: Validate the supported facility values with the test form when available.

### 8.3 `AT+CSIM`

- Purpose: Send raw SIM commands.
- Syntax: `AT+CSIM=<length>,"<data>"`
- Response: Returns the SIM response APDU.
- Notes: Intended for low-level SIM/UICC access workflows.

### 8.4 `AT+CRSM`

- Purpose: Perform restricted SIM access.
- Syntax: `AT+CRSM=<cmd>,<fileid>[,...]`
- Response: Returns status words and response data.
- Notes: Commonly used for selected file and record access without opening logical channels.

### 8.5 `AT+CNUM`

- Purpose: Read subscriber numbers stored on the SIM.
- Syntax: `AT+CNUM`
- Response: Returns one or more number entries.
- Notes: Many SIM cards do not populate this information.

### 8.6 `AT+CIMI`

- Purpose: Read the IMSI.
- Syntax: `AT+CIMI`
- Response: Returns the subscriber IMSI.
- Notes: Handle IMSI carefully because it is sensitive identity data.

### 8.7 `AT+CCHO`

- Purpose: Open a logical UICC channel.
- Syntax: `AT+CCHO="<AID>"`
- Response: Returns the allocated channel number.
- Notes: Use before application-level APDU exchange when a dedicated channel is required.

### 8.8 `AT+CCHC`

- Purpose: Close a logical UICC channel.
- Syntax: `AT+CCHC=<channel>`
- Response: `OK` or error.
- Notes: Close channels after use to avoid resource leaks.

### 8.9 `AT+CGLA`

- Purpose: Send APDU data over a logical channel.
- Syntax: `AT+CGLA=<channel>,<length>,"<data>"`
- Response: Returns the response APDU and status words.
- Notes: Used together with `AT+CCHO` and `AT+CCHC`.

## 9. SMS Commands and URCs

### 9.1 `AT+CSMS`

- Purpose: Select the SMS service profile.
- Syntax: `AT+CSMS=<service>`, `AT+CSMS?`, `AT+CSMS=?`
- Response: Returns selected and supported SMS service capabilities.
- Notes: Usually configured once during SMS stack initialization.

### 9.2 `AT+CMGF`

- Purpose: Select SMS message format.
- Syntax: `AT+CMGF=<mode>`, `AT+CMGF?`, `AT+CMGF=?`
- Response: Returns text mode or PDU mode state.
- Notes: `0` is PDU mode and `1` is text mode.

### 9.3 `AT+CSMP`

- Purpose: Set text-mode SMS parameters.
- Syntax: `AT+CSMP=<fo>,<vp>,<pid>,<dcs>`, `AT+CSMP?`
- Response: Returns the configured text-mode parameters.
- Notes: Mostly relevant when sending text-mode SMS with custom PID, DCS, or validity period.

### 9.4 `AT+CSCA`

- Purpose: Read or set the SMS service center address.
- Syntax: `AT+CSCA?`, `AT+CSCA="<sca>"[,...]`
- Response: Returns or stores the SMSC address.
- Notes: Usually left to the SIM or operator defaults unless troubleshooting SMS delivery.

### 9.5 `AT+CSDH`

- Purpose: Control display of detailed text-mode SMS fields.
- Syntax: `AT+CSDH=<show>`, `AT+CSDH?`, `AT+CSDH=?`
- Response: Returns the active display mode.
- Notes: Affects the formatting of `AT+CMGR` and `AT+CMGL` in text mode.

### 9.6 `AT+CNMI`

- Purpose: Configure new-message indication behavior.
- Syntax: `AT+CNMI=<mode>,<mt>,<bm>,<ds>,<bfr>`
- Response: Returns the active or supported indication configuration.
- Notes: This is the main command for controlling whether new SMS are stored, forwarded, or reported by URC.

### 9.7 `AT+CMGR`

- Purpose: Read a stored message by index.
- Syntax: `AT+CMGR=<index>[,...]`
- Response: Returns message metadata and content.
- Notes: Output differs between text and PDU mode.

### 9.8 `AT+CMGC`

- Purpose: Send an SMS command message.
- Syntax: `AT+CMGC=<length>` followed by payload entry in the required format.
- Response: Returns command-send result or error.
- Notes: Primarily used in advanced PDU-based SMS control flows.

### 9.9 `AT+CMGL`

- Purpose: List stored messages.
- Syntax: `AT+CMGL[=<status>]`
- Response: Returns zero or more stored-message entries.
- Notes: The format depends on the current SMS mode.

### 9.10 `AT+CMGD`

- Purpose: Delete stored messages.
- Syntax: `AT+CMGD=<index>[,<delflag>]`
- Response: `OK` or error.
- Notes: Use the delete flag carefully because some values remove multiple messages.

### 9.11 `AT+CMGW`

- Purpose: Write an SMS to storage without sending it.
- Syntax: `AT+CMGW[=<length>]` followed by message content entry.
- Response: Returns the storage index for the saved message.
- Notes: Useful for later sending with `AT+CMSS`.

### 9.12 `AT+CMGS`

- Purpose: Send an SMS.
- Syntax: `AT+CMGS=...` followed by message content entry.
- Response: Returns a message reference or error.
- Notes: In text mode the modem prompts for content entry; in PDU mode the specified length is required.

### 9.13 `AT+CMSS`

- Purpose: Send a message from storage.
- Syntax: `AT+CMSS=<index>[,...]`
- Response: Returns a message reference or error.
- Notes: Useful when the message body has already been prepared with `AT+CMGW`.

### 9.14 `+CMT` / `+CMTI`

- Purpose: Report new incoming SMS.
- Syntax: Unsolicited result codes.
- Response: `+CMT` usually carries message content directly and `+CMTI` reports the storage location.
- Notes: Controlled by `AT+CNMI`.

### 9.15 `+CDS` / `+CDSI`

- Purpose: Report SMS delivery status information.
- Syntax: Unsolicited result codes.
- Response: `+CDS` usually carries status-report content and `+CDSI` points to stored reports.
- Notes: Also controlled by `AT+CNMI`.

### 9.16 `AT+CPMS`

- Purpose: Select preferred message storage.
- Syntax: `AT+CPMS=<mem1>[,<mem2>,<mem3>]`, `AT+CPMS?`, `AT+CPMS=?`
- Response: Returns selected storage and usage counters.
- Notes: Common storage names include SIM and ME-related stores.

### 9.17 `AT+CMMS`

- Purpose: Control multiple-message or concatenation-related behavior.
- Syntax: `AT+CMMS=<mode>`, `AT+CMMS?`, `AT+CMMS=?`
- Response: Returns the active or supported mode.
- Notes: Support and exact behavior depend on firmware.

## 10. Error Reporting

### 10.1 `AT+CMEE`

- Purpose: Configure the error-reporting format.
- Syntax: `AT+CMEE=<mode>`, `AT+CMEE?`, `AT+CMEE=?`
- Response: Returns the current or supported error-reporting mode.
- Notes: `AT+CMEE=2` is recommended during development.

### 10.2 `+CME ERROR`

- Purpose: Report equipment or ME-related failures.
- Syntax: Unsolicited or command response format `+CME ERROR: <err>`
- Response: Numeric or verbose error detail, depending on `AT+CMEE`.
- Notes: Capture the exact value when troubleshooting network, SIM, configuration, or radio problems.

### 10.3 `+CMS ERROR`

- Purpose: Report SMS-related failures.
- Syntax: Unsolicited or command response format `+CMS ERROR: <err>`
- Response: Numeric or verbose SMS error detail.
- Notes: Use together with the current SMS mode and SMSC configuration when debugging message flows.

## 11. Common Bring-Up Sequence

Use a short, repeatable initialization path before deeper testing:

1. `ATE0`
2. `AT+CMEE=2`
3. `ATI`
4. `AT+CGMM`
5. `AT+CGMR`
6. `AT+CPIN?`
7. `AT+CSQ`
8. `AT+CREG?`
9. `AT+CEREG?`

For packet data:

1. `AT+CGDCONT=...`
2. `AT+CGAUTH=...` if required
3. `AT+CGATT=1`
4. `AT+CGACT=1,<cid>`
5. `AT+CGPADDR=<cid>`

For SMS:

1. `AT+CMGF=1` or `AT+CMGF=0`
2. `AT+CNMI=...`
3. `AT+CSCA?`
4. `AT+CMGS=...`

## 12. Model-Specific Cautions

- Always confirm feature support with `AT+<CMD>=?` on the exact target firmware.
- `ML305U` often accepts wider S-register value ranges than other family members.
- Some `-DL` variants do not support `ATS3`, `ATS4`, or `ATS5`.
- Identity strings and revision formats are not fully uniform across families.
- Packet-domain optional fields are not consistently supported across all models.
- Voice, hold, multiparty, and incoming-call indication commands only apply to voice-capable variants.
