# A7M5 Protocol Probe

Step 1 experimental tool for SonyFastConnect. It verifies the wireless PTP/IP
chain without building the iOS UI or assuming that A7M5 support is already
proven.

This command-line tool is a laboratory harness, not a product architecture
decision. The production app will not embed the Go tool or call into it.

This tool uses [`alpha2go`](https://github.com/knallli/alpha2go) v0.3.0
(Apache-2.0) as a protocol library. The dependency is pinned in `go.mod`; no
Sony SDK code or documentation is included.

## Safety boundary

- Connect only to a camera you own or are authorized to test.
- Use a disposable or restorable SD card.
- The probe command is read-only apart from opening a PTP/IP session.
- `list` sends Sony read/list operations.
- `download-jpeg` writes a local `.part` file and commits it only after exact
  size and JPEG-signature checks pass.
- `capture-once` physically releases the shutter and requires
  `--confirm-shutter`. It never retries after an error or timeout.
- No operation or capability is promoted to the support matrix without A7M5
  hardware evidence.

## Build

From this directory:

```powershell
go test ./...
go build -o a7m5-probe.exe .
```

## Running on another computer

The protocol probe can run on either Windows or macOS. It only needs network
access to the camera's Wi-Fi network; it does not require Xcode or an iPhone.
The real iOS implementation still requires macOS, Xcode and an iPhone for later
validation.

- Another Windows AMD64/ARM64 computer with the same tools: copy the
  `a7m5-probe` directory and run the existing binary. Go is not required unless
  you want to modify, test or rebuild the source.
- Windows computer without a compatible binary: double-click `install-go.cmd`.
  It installs Go 1.27.4 per user, verifies the official SHA-256, runs the tests
  and rebuilds `a7m5-probe.exe`.
- macOS: run `chmod +x install-go.sh && ./install-go.sh`. It installs Go 1.27.4
  per user after checksum verification, updates the shell profile, runs the
  tests and builds a native probe binary. `install-go.command` can later be
  launched from Finder after making it executable.

The installers pin `https://go.dev/dl` archives for Go 1.27.4. They do not use
administrator privileges and do not overwrite a different versioned install
directory.

## Step 1 sequence

Join the camera Wi-Fi manually, then determine the camera IP from the camera or
the network configuration. Run the stages separately and save each report.

Create a structured evidence directory first:

```powershell
.\a7m5-probe.exe evidence-init --firmware <camera-firmware> --label run1
```

Check local interfaces and expected TCP ports:

```powershell
.\a7m5-probe.exe diagnose --host 192.168.122.1
```

An open TCP port only proves reachability. It does not prove PTP/IP, DLNA or
live-view compatibility.

1. Read-only PTP/IP and DeviceInfo probe:

```powershell
.\a7m5-probe.exe probe --host 192.168.122.1 `
  --json-out ..\..\artifacts\protocol-probes\a7m5-device-info.json
```

2. Run the optional Sony SDIO capability handshake:

```powershell
.\a7m5-probe.exe probe --host 192.168.122.1 --sony-handshake `
  --json-out ..\..\artifacts\protocol-probes\a7m5-sony-handshake.json
```

3. List contents without downloading:

```powershell
.\a7m5-probe.exe list --host 192.168.122.1 `
  --json-out ..\..\artifacts\protocol-probes\a7m5-list.json
```

4. Download the newest JPEG original and preserve its hash report:

```powershell
.\a7m5-probe.exe download-jpeg --host 192.168.122.1 `
  --out-dir ..\..\artifacts\protocol-probes\downloads `
  --json-out ..\..\artifacts\protocol-probes\a7m5-jpeg-download.json
```

5. Only after reviewing the prior stages, test one shutter sequence:

```powershell
.\a7m5-probe.exe capture-once --host 192.168.122.1 --confirm-shutter `
  --json-out ..\..\artifacts\protocol-probes\a7m5-capture-once.json
```

Do not rerun `capture-once` merely because the command timed out. Check the
camera and card first.

## Evidence to record

For every run, record:

- A7M5 firmware version
- Camera connection/remote mode and menu selections
- Camera IP and connection topology
- Command exit status and JSON report
- Original file name, reported size, downloaded size, JPEG signature and
  SHA-256
- Whether camera UI stayed usable
- Any disconnect, timeout or mode transition

The tool deliberately does not automate BLE pairing, Wi-Fi joining, SSH access
authentication or live-view verification. Those require separate A7M5 tests.

Use the full [A7M5 Step 1 hardware verification runbook](../../docs/A7M5_STEP1_RUNBOOK.md)
for test order, evidence layout and Go/No-Go criteria.

Cross-check every observed capability against the
[A7M5 protocol evidence matrix](../../docs/A7M5_PROTOCOL_EVIDENCE_MATRIX.md)
instead of carrying over A7R V or A6700 conclusions.
