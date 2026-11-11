package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/knallli/alpha2go/ptpip"
	"github.com/knallli/alpha2go/ptpip/ptpiptest"
)

func TestProbeListAndDownloadJPEG(t *testing.T) {
	root := t.TempDir()
	slot1 := filepath.Join(root, "slot1")
	if err := os.MkdirAll(slot1, 0o755); err != nil {
		t.Fatal(err)
	}
	jpeg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 300)...)
	if err := os.WriteFile(filepath.Join(slot1, "DSC00001.JPG"), jpeg, 0o644); err != nil {
		t.Fatal(err)
	}
	server, err := ptpiptest.Start(ptpiptest.Options{
		Dir: root, Sony: true, A6700: true, Model: "ILCE-7M5",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cf := &commonFlags{
		host: "127.0.0.1", port: portFromAddr(t, server.Addr()), sessionID: 1,
		connectTimeout: 3 * time.Second, ioTimeout: 5 * time.Second, deadline: 20 * time.Second,
	}
	session, err := openCamera(ctx, cf, true)
	if err != nil {
		t.Fatal(err)
	}
	defer session.client.Close()
	if session.device.Model != "ILCE-7M5" {
		t.Fatalf("model=%q", session.device.Model)
	}
	if !session.device.SupportsSonyRemoteTransfer() {
		t.Fatal("fake camera should advertise Sony remote transfer")
	}

	slots, err := collectSonyContents(ctx, session, 1)
	if err != nil {
		t.Fatal(err)
	}
	candidate, ok := newestJPEG(slots)
	if !ok {
		t.Fatal("no JPEG candidate")
	}
	outDir := filepath.Join(t.TempDir(), "downloads")
	report, err := downloadCandidate(ctx, session, candidate, outDir, false)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(report.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(jpeg) {
		t.Fatalf("downloaded bytes differ: got %d want %d", len(got), len(jpeg))
	}
	if !report.JPEGSignatureOK || report.DownloadedBytes != int64(len(jpeg)) {
		t.Fatalf("unexpected report: %+v", report)
	}
	wantHash := sha256.Sum256(jpeg)
	if report.SHA256 != hex.EncodeToString(wantHash[:]) {
		t.Fatalf("sha256=%q, want %q", report.SHA256, hex.EncodeToString(wantHash[:]))
	}
	if _, err := os.Stat(report.OutputPath + ".part"); !os.IsNotExist(err) {
		t.Fatalf("part file should have been committed: %v", err)
	}
	if _, err := os.Stat(report.MetadataPath); err != nil {
		t.Fatalf("metadata missing: %v", err)
	}
}

func TestNewestJPEGChoosesLatestContent(t *testing.T) {
	slots := []slotList{
		{Slot: 1, Contents: []contentSummary{
			{ID: 1, CreatedUTC: "2026-11-10T10:00:00Z", Files: []fileSummary{{ID: 1, Path: "A:/old.JPG", Format: "0x3801", Size: 10}}},
			{ID: 2, CreatedUTC: "2026-11-11T10:00:00Z", Files: []fileSummary{
				{ID: 1, Path: "A:/new.ARW", Format: "0xB101", Size: 20},
				{ID: 2, Path: "A:/new.JPG", Format: "0x3801", Size: 11},
			}},
		}},
	}
	got, ok := newestJPEG(slots)
	if !ok {
		t.Fatal("no candidate")
	}
	if got.Content.ID != 2 || got.File.ID != 2 || got.File.Path != "A:/new.JPG" {
		t.Fatalf("unexpected candidate: %+v", got)
	}
}

func TestSummarizeCapabilities(t *testing.T) {
	d := &ptpip.DeviceInfo{
		VendorExtensionID: ptpip.VendorSony,
		Manufacturer:      "Sony",
		Operations: []uint16{
			ptpip.OpGetStorageIDs, ptpip.OpGetObjectHandles,
			ptpip.OpSonyGetDeviceDescription, ptpip.OpSonyGetCapturedDateList,
			ptpip.OpSonyGetContentsInfoList, ptpip.OpSonyGetContentsData,
			ptpip.OpSonyGetContentsCompressedData, ptpip.OpSonyControlDevice,
		},
		DeviceProperties: []uint16{0xD278},
	}
	got := summarizeCapabilities(d)
	if !got.IsSony || !got.StandardStorageRead || !got.SonyRemoteTransferComplete || !got.ShutterControl || !got.LiveViewURLPropertyAdvertised {
		t.Fatalf("unexpected summary: %+v", got)
	}
}

func TestCaptureOnceRequiresConfirmationAndAdvertisedControls(t *testing.T) {
	server, err := ptpiptest.Start(ptpiptest.Options{
		Dir: t.TempDir(), Sony: true, A6700: true, Model: "ILCE-7M5",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	var stdout, stderr bytes.Buffer
	err = runCaptureOnce([]string{"--host", "127.0.0.1", "--port", portFromAddr(t, server.Addr())}, &stdout, &stderr)
	if err == nil || server.Ops.Load() != 0 {
		t.Fatalf("capture without confirmation should stop before connecting: err=%v ops=%d", err, server.Ops.Load())
	}
	stdout.Reset()
	stderr.Reset()
	err = runCaptureOnce([]string{
		"--host", "127.0.0.1", "--port", portFromAddr(t, server.Addr()),
		"--confirm-shutter", "--event-wait", "10ms",
	}, &stdout, &stderr)
	if err == nil {
		t.Fatal("fake camera advertises only 0xD2C1; capture must not be sent")
	}
	controls := server.Controls()
	if len(controls) != 0 {
		t.Fatalf("controls=%+v, want no shutter command before capability confirmation", controls)
	}
}

func TestCaptureSequenceIsSentOnceWithoutConfirmationEvent(t *testing.T) {
	server, err := ptpiptest.Start(ptpiptest.Options{
		Dir: t.TempDir(), Sony: true, A6700: true, Model: "ILCE-7M5",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := openCaptureTestSession(t, ctx, server)
	defer session.client.Close()
	report, err := executeCaptureOnce(ctx, session, true, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	controls := server.Controls()
	if len(controls) != 4 {
		t.Fatalf("controls=%+v, want exactly one S1/S2 sequence", controls)
	}
	if controls[0].Code != ptpip.SonyCtlShutterHalf || controls[1].Code != ptpip.SonyCtlShutterFull ||
		controls[2].Code != ptpip.SonyCtlShutterFull || controls[3].Code != ptpip.SonyCtlShutterHalf {
		t.Fatalf("unexpected control order: %+v", controls)
	}
	if !report.CommandSequenceReturned || report.CaptureConfirmationObserved || report.Status != "sequence_accepted_result_unconfirmed" {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestCaptureConfirmationEventIsRecorded(t *testing.T) {
	server, err := ptpiptest.Start(ptpiptest.Options{
		Dir: t.TempDir(), Sony: true, A6700: true, Model: "ILCE-7M5",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := openCaptureTestSession(t, ctx, server)
	defer session.client.Close()
	go func() {
		time.Sleep(50 * time.Millisecond)
		server.Emit(sonyCapturedEvent, 1)
	}()
	report, err := executeCaptureOnce(ctx, session, false, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !report.CaptureConfirmationObserved || report.Status != "capture_confirmation_observed" {
		t.Fatalf("capture event was not recorded: %+v", report)
	}
}

func TestCaptureFailureIsNotRetried(t *testing.T) {
	server, err := ptpiptest.Start(ptpiptest.Options{
		Dir: t.TempDir(), Sony: true, A6700: true, Model: "ILCE-7M5",
		FailControl: ptpip.SonyCtlShutterFull,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session := openCaptureTestSession(t, ctx, server)
	defer session.client.Close()
	report, err := executeCaptureOnce(ctx, session, false, 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected shutter command failure")
	}
	if report.Status != "send_failed_result_unknown" || report.AutomaticRetryPerformed {
		t.Fatalf("unexpected report: %+v", report)
	}
	controls := server.Controls()
	if len(controls) != 2 {
		t.Fatalf("controls=%+v, failure must not repeat the S2 sequence", controls)
	}
}

func TestProbeReportRedactsSerialNumber(t *testing.T) {
	const serial = "A7M5-SECRET-123"
	server, err := ptpiptest.Start(ptpiptest.Options{
		Dir: t.TempDir(), Sony: true, Model: "ILCE-7M5", SerialNumber: serial,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	var stdout, stderr bytes.Buffer
	err = runProbe([]string{
		"--host", "127.0.0.1", "--port", portFromAddr(t, server.Addr()),
		"--include-raw-codes=false",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("probe failed: %v; stderr=%s", err, stderr.String())
	}
	if strings.Contains(stdout.String(), serial) {
		t.Fatal("raw serial number leaked into the probe report")
	}
	var report probeReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON report: %v", err)
	}
	if !report.Device.SerialNumberPresent || report.Device.SerialNumberFingerprint != fingerprint(serial) {
		t.Fatalf("serial metadata is missing: %+v", report.Device)
	}
}

func TestOpenCameraRejectsInitFailure(t *testing.T) {
	server, err := ptpiptest.Start(ptpiptest.Options{Dir: t.TempDir(), RefuseInit: true})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cf := testCommonFlags(t, server)
	_, err = openCamera(ctx, cf, false)
	if err == nil || !strings.Contains(err.Error(), "camera refused connection") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListCardBusyIsAnError(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "slot1"), 0o755); err != nil {
		t.Fatal(err)
	}
	server, err := ptpiptest.Start(ptpiptest.Options{
		Dir: root, Sony: true, A6700: true, CardBusy: true, Model: "ILCE-7M5",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := openCamera(ctx, testCommonFlags(t, server), true)
	if err != nil {
		t.Fatal(err)
	}
	defer session.client.Close()
	slots, err := collectSonyContents(ctx, session, 1)
	if !errors.Is(err, ptpip.ErrCardUnavailable) {
		t.Fatalf("error=%v, want ErrCardUnavailable", err)
	}
	if len(slots) != 1 || slots[0].Error == "" {
		t.Fatalf("card error was not preserved in report: %+v", slots)
	}
}

func TestListRejectsCameraWithoutRemoteTransfer(t *testing.T) {
	server, err := ptpiptest.Start(ptpiptest.Options{
		Dir: t.TempDir(), Sony: true, A6700: true, NoRemote: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := openCamera(ctx, testCommonFlags(t, server), true)
	if err != nil {
		t.Fatal(err)
	}
	defer session.client.Close()
	_, err = collectSonyContents(ctx, session, 1)
	if err == nil || !strings.Contains(err.Error(), "0x923B/0x923C/0x923D") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDownloadDropKeepsPartAndNoFinal(t *testing.T) {
	root := t.TempDir()
	slot1 := filepath.Join(root, "slot1")
	if err := os.MkdirAll(slot1, 0o755); err != nil {
		t.Fatal(err)
	}
	jpeg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 4096)...)
	if err := os.WriteFile(filepath.Join(slot1, "DSC01000.JPG"), jpeg, 0o644); err != nil {
		t.Fatal(err)
	}
	server, err := ptpiptest.Start(ptpiptest.Options{
		Dir: root, Sony: true, A6700: true, DropAfter: 100, Model: "ILCE-7M5",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session, err := openCamera(ctx, testCommonFlags(t, server), true)
	if err != nil {
		t.Fatal(err)
	}
	defer session.client.Close()
	slots, err := collectSonyContents(ctx, session, 1)
	if err != nil {
		t.Fatal(err)
	}
	candidate, ok := newestJPEG(slots)
	if !ok {
		t.Fatal("no JPEG candidate")
	}
	outDir := t.TempDir()
	_, err = downloadCandidate(ctx, session, candidate, outDir, false)
	if err == nil {
		t.Fatal("expected a download error after connection drop")
	}
	finalPath := filepath.Join(outDir, "DSC01000.JPG")
	if _, statErr := os.Stat(finalPath); !os.IsNotExist(statErr) {
		t.Fatalf("incomplete download was committed as final: %v", statErr)
	}
	if _, statErr := os.Stat(finalPath + ".part"); statErr != nil {
		t.Fatalf("partial evidence should be retained: %v", statErr)
	}
}

func TestDownloadRefusesExistingFinalWithoutOverwrite(t *testing.T) {
	root := t.TempDir()
	slot1 := filepath.Join(root, "slot1")
	if err := os.MkdirAll(slot1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(slot1, "DSC02000.JPG"), []byte{0xFF, 0xD8, 0xFF, 1}, 0o644); err != nil {
		t.Fatal(err)
	}
	server, err := ptpiptest.Start(ptpiptest.Options{
		Dir: root, Sony: true, A6700: true, Model: "ILCE-7M5",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := openCamera(ctx, testCommonFlags(t, server), true)
	if err != nil {
		t.Fatal(err)
	}
	defer session.client.Close()
	slots, err := collectSonyContents(ctx, session, 1)
	if err != nil {
		t.Fatal(err)
	}
	candidate, ok := newestJPEG(slots)
	if !ok {
		t.Fatal("no JPEG candidate")
	}
	outDir := t.TempDir()
	finalPath := filepath.Join(outDir, "DSC02000.JPG")
	if err := os.WriteFile(finalPath, []byte("keep-me"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := len(server.ContentRequests())
	_, err = downloadCandidate(ctx, session, candidate, outDir, false)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(server.ContentRequests()) != before {
		t.Fatal("download request was sent despite refusing to overwrite")
	}
	got, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep-me" {
		t.Fatalf("existing file was modified: %q", got)
	}
}

func TestDownloadRejectsNonJPEGSignature(t *testing.T) {
	root := t.TempDir()
	slot1 := filepath.Join(root, "slot1")
	if err := os.MkdirAll(slot1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(slot1, "DSC03000.JPG"), []byte("not-a-jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}
	server, err := ptpiptest.Start(ptpiptest.Options{
		Dir: root, Sony: true, A6700: true, Model: "ILCE-7M5",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := openCamera(ctx, testCommonFlags(t, server), true)
	if err != nil {
		t.Fatal(err)
	}
	defer session.client.Close()
	slots, err := collectSonyContents(ctx, session, 1)
	if err != nil {
		t.Fatal(err)
	}
	candidate, ok := newestJPEG(slots)
	if !ok {
		t.Fatal("no JPEG candidate")
	}
	outDir := t.TempDir()
	_, err = downloadCandidate(ctx, session, candidate, outDir, false)
	if err == nil || !strings.Contains(err.Error(), "not a JPEG") {
		t.Fatalf("unexpected error: %v", err)
	}
	finalPath := filepath.Join(outDir, "DSC03000.JPG")
	if _, statErr := os.Stat(finalPath); !os.IsNotExist(statErr) {
		t.Fatalf("invalid JPEG was committed: %v", statErr)
	}
	if _, statErr := os.Stat(finalPath + ".part"); statErr != nil {
		t.Fatalf("part file should remain for diagnostics: %v", statErr)
	}
}

func TestDiagnoseNetworkChecksReachablePort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	report := diagnoseNetwork(context.Background(), "127.0.0.1", []string{port}, time.Second)
	if len(report.PortChecks) != 1 || !report.PortChecks[0].Reachable {
		t.Fatalf("unexpected diagnosis: %+v", report)
	}
	if len(report.ResolvedAddresses) != 1 || report.ResolvedAddresses[0] != "127.0.0.1" {
		t.Fatalf("unexpected resolved address: %+v", report.ResolvedAddresses)
	}
}

func TestParsePortSpecRejectsInvalidInput(t *testing.T) {
	got, err := parsePortSpec("15740, 64321,15740")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "15740" || got[1] != "64321" {
		t.Fatalf("unexpected ports: %+v", got)
	}
	for _, invalid := range []string{"", "0", "65536", "abc", "15740,,bad"} {
		if _, err := parsePortSpec(invalid); err == nil {
			t.Fatalf("expected %q to fail", invalid)
		}
	}
}

func TestInitializeEvidenceRunCreatesManifestAndRefusesOverwrite(t *testing.T) {
	root := t.TempDir()
	report, err := initializeEvidenceRun(root, "A7M5", "v4.00 (test)", "run 1", "192.168.122.1")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{report.EvidenceDir, report.ManifestPath, report.NotesPath, report.EventsPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing evidence artifact %s: %v", path, err)
		}
	}
	for _, dir := range report.Directories {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			t.Fatalf("missing evidence directory %s: %v", dir, err)
		}
	}
	data, err := os.ReadFile(report.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest evidenceManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Model != "A7M5" || manifest.Firmware != "v4.00 (test)" || manifest.RunLabel != "run 1" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if _, err := initializeEvidenceRun(root, "A7M5", "v4.00 (test)", "run 1", "192.168.122.1"); err == nil {
		t.Fatal("evidence-init must not overwrite an existing run")
	}
}

func TestSanitizeToken(t *testing.T) {
	if got := sanitizeToken(" v4.00 / A7M5 ", "fallback"); got != "v4.00---A7M5" {
		t.Fatalf("sanitizeToken=%q", got)
	}
	if got := sanitizeToken("///", "fallback"); got != "fallback" {
		t.Fatalf("fallback=%q", got)
	}
}

func testCommonFlags(t *testing.T, server *ptpiptest.Server) *commonFlags {
	t.Helper()
	return &commonFlags{
		host: "127.0.0.1", port: portFromAddr(t, server.Addr()), sessionID: 1,
		connectTimeout: 3 * time.Second, ioTimeout: 5 * time.Second, deadline: 20 * time.Second,
	}
}

func openCaptureTestSession(t *testing.T, ctx context.Context, server *ptpiptest.Server) *cameraSession {
	t.Helper()
	session, err := openCamera(ctx, testCommonFlags(t, server), true)
	if err != nil {
		t.Fatal(err)
	}
	if !containsUint16(session.sony.Controls, ptpip.SonyCtlShutterFull) {
		session.sony.Controls = append(session.sony.Controls, ptpip.SonyCtlShutterFull)
	}
	return session
}

func portFromAddr(t *testing.T, addr string) string {
	t.Helper()
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	return port
}
