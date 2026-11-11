**语言 / Language:** [English](README.md) | [简体中文](README.zh-CN.md)

[![A7M5 probe CI](https://github.com/lavenderlove52/SonyFastConnect/actions/workflows/a7m5-probe.yml/badge.svg)](https://github.com/lavenderlove52/SonyFastConnect/actions/workflows/a7m5-probe.yml)

[![Repository quality](https://github.com/lavenderlove52/SonyFastConnect/actions/workflows/quality.yml/badge.svg)](https://github.com/lavenderlove52/SonyFastConnect/actions/workflows/quality.yml)

# SonyFastConnect

SonyFastConnect is an early feasibility project for local-first Sony camera
photo transfer and remote control, starting with A7M5 (ILCE-7M5) on iOS.

## Current status

- Step 0 product scope is confirmed.
- Step 1 A7M5 protocol feasibility is in progress.
- No Sony camera model is currently marked as supported.
- JPEG original download, one-shot still capture and live view still require
  A7M5 hardware evidence.

The target validation environment is:

- iPhone 14 Pro Max, iOS 18.7.7
- Sony A7M5 / ILCE-7M5, firmware to be recorded during hardware testing

## Documentation

- [Product requirements and system design](REQUIREMENTS_AND_DESIGN.md)
- [A7M5 protocol probe](tools/a7m5-probe/README.md)
- [A7M5 Step 1 hardware verification runbook](docs/A7M5_STEP1_RUNBOOK.md)
- [A7M5 protocol evidence matrix](docs/A7M5_PROTOCOL_EVIDENCE_MATRIX.md)
- [Security policy](SECURITY.md)

## Protocol probe

The first development artifact is a command-line experiment under
`tools/a7m5-probe`. It can run on Windows or macOS and covers:

- Camera TCP reachability and local network diagnostics
- Structured evidence directories and environment manifests
- PTP/IP connection and device information
- Sony SDIO capability negotiation
- Content listing
- JPEG original download with byte-count, JPEG-signature and SHA-256 checks
- One explicitly confirmed shutter sequence without automatic retry

The probe is a laboratory tool only. It is not part of the future iOS app and
does not decide whether the production core will use Swift or Rust.

## License

Apache License 2.0. See [LICENSE](LICENSE) and
[third-party notices](tools/a7m5-probe/THIRD_PARTY_NOTICES.md).
