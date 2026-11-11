package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
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

func portFromAddr(t *testing.T, addr string) string {
	t.Helper()
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	return port
}
