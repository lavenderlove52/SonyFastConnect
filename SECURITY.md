# Security Policy

SonyFastConnect is an early protocol-feasibility project. No camera model is
currently supported.

## Reporting

Please report suspected vulnerabilities privately through the repository's
GitHub Security Advisory feature. Do not publish working exploit details,
credentials or private photo data in a public issue.

Include:

- Affected commit or release
- Operating system and build architecture
- Camera model, firmware and connection mode if relevant
- Reproduction steps with secrets and personal data removed
- Expected and observed behavior

## Sensitive data

Do not commit or attach:

- Wi-Fi passwords or SSH credentials
- Private keys
- Camera serial numbers
- Private photos or complete photo listings
- Unredacted packet captures or diagnostic bundles

The probe stores generated evidence under `artifacts/`, which is excluded from
Git. Review and redact generated files before sharing them outside this project.
