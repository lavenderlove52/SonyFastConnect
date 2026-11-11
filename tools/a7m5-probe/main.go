package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/knallli/alpha2go/ptpip"
)

const (
	defaultPort       = "15740"
	defaultOutputDir  = "artifacts/protocol-probes"
	jpegObjectFormat  = uint16(0x3801)
	sonyCapturedEvent = uint16(0xC206)
	probeVersion      = "0.1.0"
)

type commonFlags struct {
	host            string
	port            string
	sessionID       uint
	connectTimeout  time.Duration
	ioTimeout       time.Duration
	deadline        time.Duration
	jsonOutput      string
	sonyHandshake   bool
	includeRawCodes bool
}

type probeReport struct {
	GeneratedAtUTC             string       `json:"generated_at_utc"`
	Address                    string       `json:"address"`
	ResponderName              string       `json:"responder_name"`
	ConnectionNumber           uint32       `json:"connection_number"`
	Device                     deviceReport `json:"device"`
	SonyHandshake              *sonyReport  `json:"sony_handshake,omitempty"`
	CapabilitySummary          capabilities `json:"capability_summary"`
	Operations                 []string     `json:"operations,omitempty"`
	Events                     []string     `json:"events,omitempty"`
	DeviceProperties           []string     `json:"device_properties,omitempty"`
	CaptureFormats             []string     `json:"capture_formats,omitempty"`
	ImageFormats               []string     `json:"image_formats,omitempty"`
	EvidenceBoundary           string       `json:"evidence_boundary"`
	AutomaticRetryPerformed    bool         `json:"automatic_retry_performed"`
	AutomaticFallbackPerformed bool         `json:"automatic_fallback_performed"`
}

type deviceReport struct {
	Manufacturer            string `json:"manufacturer"`
	Model                   string `json:"model"`
	DeviceVersion           string `json:"device_version"`
	SerialNumberPresent     bool   `json:"serial_number_present"`
	SerialNumberFingerprint string `json:"serial_number_fingerprint,omitempty"`
	StandardVersion         uint16 `json:"standard_version"`
	VendorExtensionID       uint32 `json:"vendor_extension_id"`
	VendorExtensionVersion  uint16 `json:"vendor_extension_version"`
	VendorExtensionDesc     string `json:"vendor_extension_description"`
	FunctionalMode          uint16 `json:"functional_mode"`
}

type sonyReport struct {
	ProtocolVersion uint16   `json:"protocol_version"`
	PropertyCodes   []string `json:"property_codes,omitempty"`
	ControlCodes    []string `json:"control_codes,omitempty"`
}

type capabilities struct {
	IsSony                              bool `json:"is_sony"`
	StandardStorageRead                 bool `json:"standard_storage_read"`
	SonyDeviceDescription               bool `json:"sony_device_description_0x923a"`
	SonyCapturedDateList                bool `json:"sony_captured_date_list_0x923b"`
	SonyRemoteContentsList              bool `json:"sony_remote_contents_list_0x923c"`
	SonyRemoteContentsDownload          bool `json:"sony_remote_contents_download_0x923d"`
	SonyRemoteContentsCompressedPreview bool `json:"sony_remote_contents_preview_0x923e"`
	SonyRemoteTransferComplete          bool `json:"sony_remote_transfer_complete"`
	ShutterControl                      bool `json:"shutter_control_0x9207_advertised"`
	LiveViewURLPropertyAdvertised       bool `json:"live_view_url_property_0xd278_advertised"`
}

type listReport struct {
	GeneratedAtUTC   string     `json:"generated_at_utc"`
	Address          string     `json:"address"`
	Model            string     `json:"model"`
	DeviceVersion    string     `json:"device_version"`
	SlotResults      []slotList `json:"slots"`
	EvidenceBoundary string     `json:"evidence_boundary"`
}

type slotList struct {
	Slot              uint32           `json:"slot"`
	CapturedDateCount int              `json:"captured_date_count"`
	ContentCount      int              `json:"content_count"`
	Contents          []contentSummary `json:"contents,omitempty"`
	Error             string           `json:"error,omitempty"`
}

type contentSummary struct {
	ID             uint32        `json:"id"`
	CreatedUTC     string        `json:"created_utc,omitempty"`
	Representative bool          `json:"representative"`
	Files          []fileSummary `json:"files"`
}

type fileSummary struct {
	ID     uint16 `json:"id"`
	Path   string `json:"path"`
	Format string `json:"format"`
	Size   uint64 `json:"size"`
}

type jpegCandidate struct {
	Slot      uint32
	Content   ptpip.SonyContent
	File      ptpip.SonyContentFile
	CreatedAt time.Time
}

type downloadReport struct {
	GeneratedAtUTC   string `json:"generated_at_utc"`
	SourceSlot       uint32 `json:"source_slot"`
	ContentID        uint32 `json:"content_id"`
	FileID           uint16 `json:"file_id"`
	CameraPath       string `json:"camera_path"`
	ReportedBytes    uint64 `json:"reported_bytes"`
	DownloadedBytes  int64  `json:"downloaded_bytes"`
	SHA256           string `json:"sha256"`
	JPEGSignatureOK  bool   `json:"jpeg_signature_ok"`
	OutputPath       string `json:"output_path"`
	MetadataPath     string `json:"metadata_path"`
	EvidenceBoundary string `json:"evidence_boundary"`
}

type captureReport struct {
	GeneratedAtUTC                   string   `json:"generated_at_utc"`
	Model                            string   `json:"model"`
	DeviceVersion                    string   `json:"device_version"`
	AutofocusRequested               bool     `json:"autofocus_requested"`
	CommandSequenceReturned          bool     `json:"command_sequence_returned"`
	Status                           string   `json:"status"`
	CaptureConfirmationObserved      bool     `json:"capture_confirmation_observed"`
	ObservedEventCodes               []string `json:"observed_event_codes,omitempty"`
	AutomaticRetryPerformed          bool     `json:"automatic_retry_performed"`
	RequiresManualVisualVerification bool     `json:"requires_manual_visual_verification"`
}

type networkDiagnosis struct {
	GeneratedAtUTC    string             `json:"generated_at_utc"`
	Host              string             `json:"host"`
	ResolvedAddresses []string           `json:"resolved_addresses,omitempty"`
	PortChecks        []portCheck        `json:"port_checks"`
	LocalInterfaces   []interfaceSummary `json:"local_interfaces"`
	EvidenceBoundary  string             `json:"evidence_boundary"`
}

type portCheck struct {
	Port       string  `json:"port"`
	Reachable  bool    `json:"reachable"`
	DurationMS float64 `json:"duration_ms"`
	Error      string  `json:"error,omitempty"`
}

type interfaceSummary struct {
	Name      string   `json:"name"`
	MTU       int      `json:"mtu"`
	Flags     string   `json:"flags"`
	Addresses []string `json:"addresses,omitempty"`
}

type evidenceManifest struct {
	ToolVersion      string             `json:"tool_version"`
	GeneratedAtUTC   string             `json:"generated_at_utc"`
	Model            string             `json:"model"`
	Firmware         string             `json:"firmware"`
	RunLabel         string             `json:"run_label"`
	CameraHost       string             `json:"camera_host,omitempty"`
	OperatingSystem  string             `json:"operating_system"`
	Architecture     string             `json:"architecture"`
	GoVersion        string             `json:"go_version"`
	LocalInterfaces  []interfaceSummary `json:"local_interfaces"`
	EvidenceBoundary string             `json:"evidence_boundary"`
}

type evidenceInitReport struct {
	EvidenceDir  string   `json:"evidence_dir"`
	ManifestPath string   `json:"manifest_path"`
	NotesPath    string   `json:"notes_path"`
	EventsPath   string   `json:"events_path"`
	Directories  []string `json:"directories"`
}

type cameraSession struct {
	client *ptpip.Client
	device *ptpip.DeviceInfo
	sony   *ptpip.SonyExtInfo
	addr   string
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return usageError()
	}
	switch args[0] {
	case "probe":
		return runProbe(args[1:], stdout, stderr)
	case "list":
		return runList(args[1:], stdout, stderr)
	case "download-jpeg":
		return runDownloadJPEG(args[1:], stdout, stderr)
	case "capture-once":
		return runCaptureOnce(args[1:], stdout, stderr)
	case "diagnose":
		return runDiagnose(args[1:], stdout, stderr)
	case "evidence-init":
		return runEvidenceInit(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usageText)
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], usageText)
	}
}

const usageText = `A7M5 protocol feasibility probe

Commands:
  probe          Connect, negotiate PTP/IP, and read DeviceInfo.
  list           List contents through Sony 0x923B/0x923C without downloading.
  download-jpeg  Download the newest JPEG original using 0x923D.
  capture-once   Send one S1/S2 shutter sequence; requires --confirm-shutter.
  diagnose       Check camera TCP ports and local network interfaces.
  evidence-init  Create a structured Step 1 evidence directory and manifest.

Common options:
  --host <ip>             Camera address (required).
  --port <port>           PTP/IP port, default 15740.
  --session <id>          PTP session id, default 1.
  --connect-timeout 10s   TCP connect timeout.
  --io-timeout 30s        Maximum silence while waiting for a packet.
  --deadline 90s          Overall command deadline.
  --json-out <path>       Also write the JSON report to a file.
  --sony-handshake        Run the Sony SDIO capability handshake in probe.

Examples:
  a7m5-probe probe --host 192.168.122.1 --json-out artifacts/a7m5-probe.json
  a7m5-probe list --host 192.168.122.1
  a7m5-probe download-jpeg --host 192.168.122.1 --out-dir artifacts/protocol-probes
  a7m5-probe capture-once --host 192.168.122.1 --confirm-shutter
  a7m5-probe diagnose --host 192.168.122.1
  a7m5-probe evidence-init --firmware <firmware> --label run1
`

func usageError() error {
	return errors.New(usageText)
}

func addCommonFlags(fs *flag.FlagSet, stderr io.Writer) *commonFlags {
	cf := &commonFlags{}
	fs.SetOutput(stderr)
	fs.StringVar(&cf.host, "host", "", "camera address")
	fs.StringVar(&cf.port, "port", defaultPort, "PTP/IP port")
	fs.UintVar(&cf.sessionID, "session", 1, "PTP session id")
	fs.DurationVar(&cf.connectTimeout, "connect-timeout", 10*time.Second, "TCP connect timeout")
	fs.DurationVar(&cf.ioTimeout, "io-timeout", 30*time.Second, "PTP/IP I/O timeout")
	fs.DurationVar(&cf.deadline, "deadline", 90*time.Second, "overall command deadline")
	fs.StringVar(&cf.jsonOutput, "json-out", "", "also write JSON report to this path")
	fs.BoolVar(&cf.sonyHandshake, "sony-handshake", false, "run Sony SDIO handshake during probe")
	fs.BoolVar(&cf.includeRawCodes, "include-raw-codes", true, "include advertised operation/property code lists")
	return cf
}

func runProbe(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	cf := addCommonFlags(fs, stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.deadline)
	defer cancel()
	session, err := openCamera(ctx, cf, cf.sonyHandshake)
	if err != nil {
		return err
	}
	defer session.client.Close()

	d := session.device
	report := probeReport{
		GeneratedAtUTC:   time.Now().UTC().Format(time.RFC3339Nano),
		Address:          session.addr,
		ResponderName:    session.client.ResponderName,
		ConnectionNumber: session.client.ConnNumber,
		Device: deviceReport{
			Manufacturer:            d.Manufacturer,
			Model:                   d.Model,
			DeviceVersion:           d.DeviceVersion,
			SerialNumberPresent:     d.SerialNumber != "",
			SerialNumberFingerprint: fingerprint(d.SerialNumber),
			StandardVersion:         d.StandardVersion,
			VendorExtensionID:       d.VendorExtensionID,
			VendorExtensionVersion:  d.VendorExtensionVersion,
			VendorExtensionDesc:     d.VendorExtensionDesc,
			FunctionalMode:          d.FunctionalMode,
		},
		CapabilitySummary: summarizeCapabilities(d),
		EvidenceBoundary:  "PTP/IP advertisement and DeviceInfo only; no A7M5 hardware capability is inferred.",
	}
	if session.sony != nil {
		report.SonyHandshake = &sonyReport{
			ProtocolVersion: session.sony.ProtocolVersion,
			PropertyCodes:   formatUint16(session.sony.Properties),
			ControlCodes:    formatUint16(session.sony.Controls),
		}
	}
	if cf.includeRawCodes {
		report.Operations = formatUint16(d.Operations)
		report.Events = formatUint16(d.Events)
		report.DeviceProperties = formatUint16(d.DeviceProperties)
		report.CaptureFormats = formatUint16(d.CaptureFormats)
		report.ImageFormats = formatUint16(d.ImageFormats)
	}
	return writeReport(stdout, cf.jsonOutput, report)
}

func runList(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	cf := addCommonFlags(fs, stderr)
	slot := fs.Uint("slot", 0, "slot to query; 0 checks slots 1 and 2")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.deadline)
	defer cancel()
	session, err := openCamera(ctx, cf, true)
	if err != nil {
		return err
	}
	defer session.client.Close()
	slots, err := collectSonyContents(ctx, session, uint32(*slot))
	if err != nil && len(slots) == 0 {
		return err
	}
	report := listReport{
		GeneratedAtUTC:   time.Now().UTC().Format(time.RFC3339Nano),
		Address:          session.addr,
		Model:            session.device.Model,
		DeviceVersion:    session.device.DeviceVersion,
		SlotResults:      slots,
		EvidenceBoundary: "Sony remote content operations as advertised by this camera and session. Contents are not yet A7M5-verified unless run on A7M5.",
	}
	return errors.Join(err, writeReport(stdout, cf.jsonOutput, report))
}

func runDownloadJPEG(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("download-jpeg", flag.ContinueOnError)
	cf := addCommonFlags(fs, stderr)
	slot := fs.Uint("slot", 0, "slot to query; 0 checks slots 1 and 2")
	outDir := fs.String("out-dir", defaultOutputDir, "destination directory")
	overwrite := fs.Bool("overwrite", false, "replace an existing final file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.deadline)
	defer cancel()
	session, err := openCamera(ctx, cf, true)
	if err != nil {
		return err
	}
	defer session.client.Close()
	slots, listErr := collectSonyContents(ctx, session, uint32(*slot))
	candidate, ok := newestJPEG(slots)
	if !ok {
		if listErr != nil {
			return fmt.Errorf("no JPEG candidate; listing also failed: %w", listErr)
		}
		return errors.New("no JPEG original was listed")
	}
	report, err := downloadCandidate(ctx, session, candidate, *outDir, *overwrite)
	if err != nil {
		return err
	}
	return writeReport(stdout, cf.jsonOutput, report)
}

func runCaptureOnce(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("capture-once", flag.ContinueOnError)
	cf := addCommonFlags(fs, stderr)
	confirm := fs.Bool("confirm-shutter", false, "required acknowledgement that this physically releases the shutter")
	autofocus := fs.Bool("autofocus", true, "send S1 half-press before S2")
	eventWait := fs.Duration("event-wait", 3*time.Second, "time to observe capture events after sending")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*confirm {
		return errors.New("capture-once requires --confirm-shutter; no command was sent")
	}
	ctx, cancel := context.WithTimeout(context.Background(), cf.deadline)
	defer cancel()
	session, err := openCamera(ctx, cf, true)
	if err != nil {
		return err
	}
	defer session.client.Close()

	report, captureErr := executeCaptureOnce(ctx, session, *autofocus, *eventWait)
	reportErr := writeReport(stdout, cf.jsonOutput, report)
	if captureErr != nil {
		return errors.Join(fmt.Errorf("shutter command result is unknown; do not retry automatically: %w", captureErr), reportErr)
	}
	return reportErr
}

func validateCaptureCapabilities(session *cameraSession) error {
	if !session.device.Supports(ptpip.OpSonyControlDevice) {
		return errors.New("camera does not advertise Sony control operation 0x9207; no shutter command was sent")
	}
	if session.sony == nil || !containsUint16(session.sony.Controls, ptpip.SonyCtlShutterHalf) || !containsUint16(session.sony.Controls, ptpip.SonyCtlShutterFull) {
		return errors.New("camera does not advertise both shutter controls 0xD2C1 and 0xD2C2; no shutter command was sent")
	}
	return nil
}

func executeCaptureOnce(ctx context.Context, session *cameraSession, autofocus bool, eventWait time.Duration) (captureReport, error) {
	report := captureReport{
		GeneratedAtUTC:                   time.Now().UTC().Format(time.RFC3339Nano),
		Model:                            session.device.Model,
		DeviceVersion:                    session.device.DeviceVersion,
		AutofocusRequested:               autofocus,
		Status:                           "not_sent",
		RequiresManualVisualVerification: true,
	}
	if err := validateCaptureCapabilities(session); err != nil {
		report.Status = "blocked_missing_capability"
		return report, err
	}

	codes := make(chan uint16, 32)
	go func() {
		for event := range session.client.Events() {
			select {
			case codes <- event.Code:
			default:
			}
		}
	}()

	if err := session.client.SonyCapture(ctx, autofocus); err != nil {
		report.Status = "send_failed_result_unknown"
		return report, err
	}
	report.CommandSequenceReturned = true
	report.Status = "sequence_accepted_result_unconfirmed"

	timer := time.NewTimer(eventWait)
	defer timer.Stop()
collect:
	for {
		select {
		case code, ok := <-codes:
			if !ok {
				break collect
			}
			if code == sonyCapturedEvent {
				report.CaptureConfirmationObserved = true
			}
			report.ObservedEventCodes = appendUniqueCode(report.ObservedEventCodes, code)
			if report.CaptureConfirmationObserved {
				break collect
			}
		case <-timer.C:
			break collect
		case <-ctx.Done():
			break collect
		}
	}
	if report.CaptureConfirmationObserved {
		report.Status = "capture_confirmation_observed"
	}
	return report, nil
}

func runDiagnose(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	fs.SetOutput(stderr)
	host := fs.String("host", "", "camera address")
	portSpec := fs.String("ports", "15740,64321,60152", "comma-separated TCP ports to check")
	timeout := fs.Duration("timeout", 2*time.Second, "timeout per TCP connection")
	jsonOutput := fs.String("json-out", "", "also write JSON report to this path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*host) == "" {
		return errors.New("--host is required")
	}
	ports, err := parsePortSpec(*portSpec)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	report := diagnoseNetwork(ctx, *host, ports, *timeout)
	return writeReport(stdout, *jsonOutput, report)
}

func diagnoseNetwork(ctx context.Context, host string, ports []string, timeout time.Duration) networkDiagnosis {
	report := networkDiagnosis{
		GeneratedAtUTC:    time.Now().UTC().Format(time.RFC3339Nano),
		Host:              host,
		ResolvedAddresses: resolveHost(ctx, host),
		LocalInterfaces:   collectInterfaceSummaries(),
		EvidenceBoundary:  "TCP reachability and local interface state only; an open port does not prove PTP/IP, DLNA or live-view compatibility.",
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	for _, port := range ports {
		started := time.Now()
		checkCtx, cancel := context.WithTimeout(ctx, timeout)
		conn, err := (&net.Dialer{Timeout: timeout}).DialContext(checkCtx, "tcp", net.JoinHostPort(host, port))
		cancel()
		duration := time.Since(started)
		result := portCheck{
			Port:       port,
			Reachable:  err == nil,
			DurationMS: float64(duration.Microseconds()) / 1000,
		}
		if err != nil {
			result.Error = err.Error()
		} else {
			_ = conn.Close()
		}
		report.PortChecks = append(report.PortChecks, result)
	}
	return report
}

func resolveHost(ctx context.Context, host string) []string {
	if ip := net.ParseIP(host); ip != nil {
		return []string{ip.String()}
	}
	addrs, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil {
		return nil
	}
	sort.Strings(addrs)
	return addrs
}

func collectInterfaceSummaries() []interfaceSummary {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	out := make([]interfaceSummary, 0, len(interfaces))
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		summary := interfaceSummary{Name: iface.Name, MTU: iface.MTU, Flags: iface.Flags.String()}
		addrs, err := iface.Addrs()
		if err == nil {
			for _, addr := range addrs {
				summary.Addresses = append(summary.Addresses, addr.String())
			}
			sort.Strings(summary.Addresses)
		}
		out = append(out, summary)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func parsePortSpec(spec string) ([]string, error) {
	parts := strings.Split(spec, ",")
	seen := make(map[int]bool)
	ports := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		value, err := strconv.Atoi(part)
		if err != nil || value < 1 || value > 65535 {
			return nil, fmt.Errorf("invalid TCP port %q", part)
		}
		if seen[value] {
			continue
		}
		seen[value] = true
		ports = append(ports, strconv.Itoa(value))
	}
	if len(ports) == 0 {
		return nil, errors.New("at least one TCP port is required")
	}
	return ports, nil
}

func runEvidenceInit(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("evidence-init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", defaultOutputDir, "root directory for protocol evidence")
	model := fs.String("model", "A7M5", "camera model label")
	firmware := fs.String("firmware", "firmware-unknown", "recorded camera firmware")
	label := fs.String("label", "run1", "short run label")
	cameraHost := fs.String("camera-host", "", "optional camera address")
	jsonOutput := fs.String("json-out", "", "also write creation report to this path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	report, err := initializeEvidenceRun(*root, *model, *firmware, *label, *cameraHost)
	if err != nil {
		return err
	}
	return writeReport(stdout, *jsonOutput, report)
}

func initializeEvidenceRun(root, model, firmware, label, cameraHost string) (evidenceInitReport, error) {
	now := time.Now()
	modelSafe := sanitizeToken(model, "camera-unknown")
	firmwareSafe := sanitizeToken(firmware, "firmware-unknown")
	labelSafe := sanitizeToken(label, "run1")
	dirName := fmt.Sprintf("%s-%s-%s-%s", modelSafe, firmwareSafe, now.Format("20060102"), labelSafe)
	evidenceDir := filepath.Join(root, dirName)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return evidenceInitReport{}, fmt.Errorf("create evidence root: %w", err)
	}
	if err := os.Mkdir(evidenceDir, 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			return evidenceInitReport{}, fmt.Errorf("evidence run already exists: %s", evidenceDir)
		}
		return evidenceInitReport{}, fmt.Errorf("create evidence directory: %w", err)
	}
	screenshotsDir := filepath.Join(evidenceDir, "screenshots")
	downloadsDir := filepath.Join(evidenceDir, "downloaded")
	for _, dir := range []string{screenshotsDir, downloadsDir} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			return evidenceInitReport{}, fmt.Errorf("create evidence subdirectory: %w", err)
		}
	}

	manifest := evidenceManifest{
		ToolVersion:      probeVersion,
		GeneratedAtUTC:   now.UTC().Format(time.RFC3339Nano),
		Model:            model,
		Firmware:         firmware,
		RunLabel:         label,
		CameraHost:       cameraHost,
		OperatingSystem:  runtime.GOOS,
		Architecture:     runtime.GOARCH,
		GoVersion:        runtime.Version(),
		LocalInterfaces:  collectInterfaceSummaries(),
		EvidenceBoundary: "Environment manifest only. Record camera menu mode, user confirmations and test outcomes separately. Never store Wi-Fi passwords here.",
	}
	manifestPath := filepath.Join(evidenceDir, "environment.json")
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return evidenceInitReport{}, err
	}
	if err := os.WriteFile(manifestPath, append(manifestJSON, '\n'), 0o644); err != nil {
		return evidenceInitReport{}, fmt.Errorf("write environment manifest: %w", err)
	}

	notesPath := filepath.Join(evidenceDir, "notes.md")
	notes := `# A7M5 Step 1 Test Notes

- Camera firmware:
- Camera connection mode:
- Camera menu settings:
- Protocol host OS/build:
- SSID rule and host IP:
- Camera UI state after each command:
- Unexpected observations:
- Decision: PASS / FAIL / BLOCKED
`
	if err := os.WriteFile(notesPath, []byte(notes), 0o644); err != nil {
		return evidenceInitReport{}, fmt.Errorf("write notes template: %w", err)
	}
	eventsPath := filepath.Join(evidenceDir, "events.log")
	if err := os.WriteFile(eventsPath, nil, 0o644); err != nil {
		return evidenceInitReport{}, fmt.Errorf("write events log: %w", err)
	}
	return evidenceInitReport{
		EvidenceDir:  evidenceDir,
		ManifestPath: manifestPath,
		NotesPath:    notesPath,
		EventsPath:   eventsPath,
		Directories:  []string{screenshotsDir, downloadsDir},
	}, nil
}

func sanitizeToken(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	result := strings.Trim(b.String(), "-.")
	if result == "" {
		return fallback
	}
	return result
}

func openCamera(ctx context.Context, cf *commonFlags, withSonyHandshake bool) (*cameraSession, error) {
	if strings.TrimSpace(cf.host) == "" {
		return nil, errors.New("--host is required")
	}
	if cf.sessionID == 0 || cf.sessionID > uint(^uint32(0)) {
		return nil, errors.New("--session must be a non-zero uint32")
	}
	addr := net.JoinHostPort(cf.host, cf.port)
	client, err := ptpip.Dial(ctx, addr, ptpip.Options{
		Name:        "SonyFastConnect a7m5-probe",
		DialTimeout: cf.connectTimeout,
		IOTimeout:   cf.ioTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("PTP/IP handshake %s: %w", addr, err)
	}
	session := &cameraSession{client: client, addr: addr}
	fail := func(err error) (*cameraSession, error) {
		client.Close()
		return nil, err
	}
	device, err := client.GetDeviceInfo(ctx)
	if err != nil {
		return fail(fmt.Errorf("GetDeviceInfo: %w", err))
	}
	session.device = device
	if err := client.OpenSession(ctx, uint32(cf.sessionID)); err != nil {
		return fail(fmt.Errorf("OpenSession(%d): %w", cf.sessionID, err))
	}
	if withSonyHandshake {
		if !device.IsSony() {
			return fail(fmt.Errorf("camera does not advertise Sony vendor extensions (vendor=0x%X, manufacturer=%q)", device.VendorExtensionID, device.Manufacturer))
		}
		if !device.Supports(ptpip.OpSonySDIOConnect) || !device.Supports(ptpip.OpSonyGetExtDevInfo) {
			return fail(errors.New("camera does not advertise both Sony SDIO_Connect 0x9201 and SDIO_GetExtDeviceInfo 0x9202"))
		}
		ext, err := client.SonyHandshake(ctx)
		if err != nil {
			return fail(fmt.Errorf("Sony SDIO handshake: %w", err))
		}
		session.sony = ext
	}
	return session, nil
}

func summarizeCapabilities(d *ptpip.DeviceInfo) capabilities {
	return capabilities{
		IsSony:                              d.IsSony(),
		StandardStorageRead:                 d.Supports(ptpip.OpGetStorageIDs) && d.Supports(ptpip.OpGetObjectHandles),
		SonyDeviceDescription:               d.Supports(ptpip.OpSonyGetDeviceDescription),
		SonyCapturedDateList:                d.Supports(ptpip.OpSonyGetCapturedDateList),
		SonyRemoteContentsList:              d.Supports(ptpip.OpSonyGetContentsInfoList),
		SonyRemoteContentsDownload:          d.Supports(ptpip.OpSonyGetContentsData),
		SonyRemoteContentsCompressedPreview: d.Supports(ptpip.OpSonyGetContentsCompressedData),
		SonyRemoteTransferComplete:          d.SupportsSonyRemoteTransfer(),
		ShutterControl:                      d.Supports(ptpip.OpSonyControlDevice),
		LiveViewURLPropertyAdvertised:       containsUint16(d.DeviceProperties, 0xD278),
	}
}

func collectSonyContents(ctx context.Context, session *cameraSession, requestedSlot uint32) ([]slotList, error) {
	if !session.device.SupportsSonyRemoteTransfer() {
		return nil, fmt.Errorf("camera does not advertise Sony remote transfer set 0x923B/0x923C/0x923D")
	}
	slots := []uint32{requestedSlot}
	if requestedSlot == 0 {
		slots = []uint32{1, 2}
	}
	out := make([]slotList, 0, len(slots))
	var joined error
	for _, slot := range slots {
		dates, err := session.client.SonyGetCapturedDateList(ctx, slot)
		if err != nil {
			out = append(out, slotList{Slot: slot, Error: err.Error()})
			if !errors.Is(err, ptpip.ErrSlotEmpty) {
				joined = errors.Join(joined, fmt.Errorf("slot %d date list: %w", slot, err))
			}
			continue
		}
		contents, err := session.client.SonyListAllContents(ctx, slot)
		if err != nil {
			out = append(out, slotList{Slot: slot, CapturedDateCount: len(dates), Error: err.Error()})
			joined = errors.Join(joined, fmt.Errorf("slot %d contents: %w", slot, err))
			continue
		}
		summaries := make([]contentSummary, 0, len(contents))
		for _, content := range contents {
			files := make([]fileSummary, 0, len(content.Files))
			for _, f := range content.Files {
				files = append(files, fileSummary{
					ID: f.ID, Path: f.Path, Format: fmt.Sprintf("0x%04X", f.Format), Size: f.Size,
				})
			}
			created := ""
			if !content.CreatedUTC.IsZero() {
				created = content.CreatedUTC.Format(time.RFC3339Nano)
			}
			summaries = append(summaries, contentSummary{
				ID: content.ID, CreatedUTC: created, Representative: content.Representative, Files: files,
			})
		}
		sort.SliceStable(summaries, func(i, j int) bool { return summaries[i].CreatedUTC > summaries[j].CreatedUTC })
		out = append(out, slotList{Slot: slot, CapturedDateCount: len(dates), ContentCount: len(contents), Contents: summaries})
	}
	return out, joined
}

func newestJPEG(slots []slotList) (jpegCandidate, bool) {
	var candidates []jpegCandidate
	for _, slot := range slots {
		for _, content := range slot.Contents {
			created, _ := time.Parse(time.RFC3339Nano, content.CreatedUTC)
			for _, f := range content.Files {
				ext := strings.ToLower(filepath.Ext(strings.ReplaceAll(f.Path, "\\", "/")))
				if ext != ".jpg" && ext != ".jpeg" {
					continue
				}
				format := parseHex16(f.Format)
				candidates = append(candidates, jpegCandidate{
					Slot:    slot.Slot,
					Content: ptpip.SonyContent{ID: content.ID, CreatedUTC: created, Representative: content.Representative},
					File: ptpip.SonyContentFile{
						ID: f.ID, Path: f.Path, Format: uint32(format), Size: f.Size,
					},
					CreatedAt: created,
				})
			}
		}
	}
	if len(candidates) == 0 {
		return jpegCandidate{}, false
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
			return candidates[i].File.Path > candidates[j].File.Path
		}
		return candidates[i].CreatedAt.After(candidates[j].CreatedAt)
	})
	return candidates[0], true
}

func downloadCandidate(ctx context.Context, session *cameraSession, candidate jpegCandidate, outDir string, overwrite bool) (downloadReport, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return downloadReport{}, fmt.Errorf("create output directory: %w", err)
	}
	name := filepath.Base(strings.ReplaceAll(candidate.File.Path, "\\", "/"))
	if name == "." || name == "" || name == string(filepath.Separator) {
		return downloadReport{}, fmt.Errorf("unsafe camera filename %q", candidate.File.Path)
	}
	finalPath := filepath.Join(outDir, name)
	partPath := finalPath + ".part"
	if !overwrite {
		if _, err := os.Stat(finalPath); err == nil {
			return downloadReport{}, fmt.Errorf("output already exists: %s (use --overwrite after verifying it)", finalPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return downloadReport{}, err
		}
	}
	if _, err := os.Stat(partPath); err == nil {
		return downloadReport{}, fmt.Errorf("partial file already exists; automatic resume is intentionally not enabled: %s", partPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return downloadReport{}, err
	}
	part, err := os.OpenFile(partPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return downloadReport{}, fmt.Errorf("create part file: %w", err)
	}
	n, downloadErr := session.client.SonyDownloadContentFile(
		ctx, candidate.Slot, candidate.Content.ID, candidate.File.ID, candidate.File.Size, part, nil,
	)
	closeErr := part.Close()
	if downloadErr != nil {
		return downloadReport{}, fmt.Errorf("download stopped at %d/%d bytes; part retained at %s: %w", n, candidate.File.Size, partPath, downloadErr)
	}
	if closeErr != nil {
		return downloadReport{}, fmt.Errorf("close part file: %w", closeErr)
	}
	if n != int64(candidate.File.Size) {
		return downloadReport{}, fmt.Errorf("length mismatch: camera reported %d bytes, downloaded %d; part retained at %s", candidate.File.Size, n, partPath)
	}
	hash, signatureOK, err := inspectFile(partPath)
	if err != nil {
		return downloadReport{}, err
	}
	if !signatureOK {
		return downloadReport{}, fmt.Errorf("downloaded file is not a JPEG (bad signature); part retained at %s", partPath)
	}
	if overwrite {
		if err := os.Remove(finalPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return downloadReport{}, fmt.Errorf("replace existing output: %w", err)
		}
	}
	if err := os.Rename(partPath, finalPath); err != nil {
		return downloadReport{}, fmt.Errorf("commit downloaded file: %w", err)
	}
	metadataPath := finalPath + ".probe.json"
	report := downloadReport{
		GeneratedAtUTC:   time.Now().UTC().Format(time.RFC3339Nano),
		SourceSlot:       candidate.Slot,
		ContentID:        candidate.Content.ID,
		FileID:           candidate.File.ID,
		CameraPath:       candidate.File.Path,
		ReportedBytes:    candidate.File.Size,
		DownloadedBytes:  n,
		SHA256:           hash,
		JPEGSignatureOK:  signatureOK,
		OutputPath:       finalPath,
		MetadataPath:     metadataPath,
		EvidenceBoundary: "File size and local SHA-256 only; no trusted remote digest was available.",
	}
	meta, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return downloadReport{}, err
	}
	if err := os.WriteFile(metadataPath, append(meta, '\n'), 0o644); err != nil {
		return downloadReport{}, fmt.Errorf("write download metadata: %w", err)
	}
	return report, nil
}

func inspectFile(path string) (string, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer f.Close()
	hash := sha256.New()
	header := make([]byte, 3)
	if _, err := io.ReadFull(f, header); err != nil {
		return "", false, fmt.Errorf("read JPEG header: %w", err)
	}
	if _, err := hash.Write(header); err != nil {
		return "", false, fmt.Errorf("hash JPEG header: %w", err)
	}
	if _, err := io.Copy(hash, f); err != nil {
		return "", false, fmt.Errorf("hash downloaded file: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), header[0] == 0xFF && header[1] == 0xD8 && header[2] == 0xFF, nil
}

func writeReport(stdout io.Writer, jsonPath string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if _, err := stdout.Write(data); err != nil {
		return err
	}
	if jsonPath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(jsonPath), 0o755); err != nil && filepath.Dir(jsonPath) != "." {
		return fmt.Errorf("create report directory: %w", err)
	}
	if err := os.WriteFile(jsonPath, data, 0o644); err != nil {
		return fmt.Errorf("write JSON report: %w", err)
	}
	return nil
}

func fingerprint(value string) string {
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:6])
}

func formatUint16(values []uint16) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = fmt.Sprintf("0x%04X", value)
	}
	return out
}

func appendUniqueCode(values []string, code uint16) []string {
	value := fmt.Sprintf("0x%04X", code)
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func containsUint16(values []uint16, target uint16) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func parseHex16(value string) uint16 {
	var parsed uint16
	_, _ = fmt.Sscanf(value, "0x%04X", &parsed)
	return parsed
}
