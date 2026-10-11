# Security Policy

## Supported Versions

We actively maintain and provide security updates for the latest minor release line.

| Version | Supported          |
| ------- | ------------------ |
| 1.2.x   | :white_check_mark: |
| 1.1.x   | :white_check_mark: |
| < 1.1   | :x:                |

## Reporting a Vulnerability

The Octo CLI project takes security vulnerabilities seriously. We appreciate your efforts to responsibly disclose findings to give us time to patch before public exposure.

### Reporting Guidelines

- **Do NOT report suspected security vulnerabilities through public GitHub issues, pull requests, or public discussions.**
- Please report vulnerabilities privately using [GitHub Private Vulnerability Reporting](https://github.com/Octo-sh/Octo-CLI/security/advisories/new).
- Alternatively, you can email security reports directly to `vishuseervi825@gmail.com` with the subject line `[SECURITY] Octo CLI Vulnerability Report`.

### What to Include in Your Report

To help us triage and validate the report as quickly as possible, please include:
1. **Type of issue**: (e.g., command injection, privilege escalation, path traversal, sensitive credential exposure).
2. **Component affected**: The CLI command, package, or Model Context Protocol (MCP) tool involved.
3. **Step-by-step reproduction instructions**: Clear instructions, scripts, or sample repositories demonstrating the issue.
4. **Impact**: An objective assessment of what an attacker could achieve.
5. **Mitigation / Suggested Fix**: If you have identified an effective workaround or patch.

## Response Process & SLA

1. **Acknowledgment**: We aim to acknowledge receipt of your vulnerability report within **48 hours**.
2. **Assessment**: We will investigate, reproduce, and confirm the issue within **5 business days**.
3. **Remediation**: Once confirmed, we will develop a patch and coordinate a release timeline.
4. **Disclosure**: A public security advisory and patched release will be published simultaneously to protect all users.

## Scope & Exclusions

The following scenarios are generally excluded from vulnerability consideration:
- Non-exploitable Denial of Service (DoS) attacks on local machines.
- Running untrusted code explicitly commanded by the user via interactive execution flags.
- Reports from automated scanners without verifiable proof of exploitability.
