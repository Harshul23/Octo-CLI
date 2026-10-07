## Description
<!-- Provide a brief summary of the change, why it was made, and the problem it solves. -->

## Changes
- 

## Ecosystem & Architecture Verification
Please check all that apply:
- [ ] **Evidence-Backed**: No speculative guessing or fabricated commands. Every candidate or decision is backed by observable repository evidence.
- [ ] **Hermetic Testing**: All new logic includes unit tests that run in milliseconds without network access, third-party daemons, or internet dependencies (`go test -count=1 ./...`).
- [ ] **Explainability**: Preserved explainability tracing (`BuildDecisionTrace` or `.octo.lock`).
- [ ] **Registry Isolation**: Language or ecosystem additions are registered through typed registries (`ProjectDetectorRegistry`, `CandidateProviderRegistry`, `VerificationRegistry`, or `RuntimeAdapterRegistry`) without mutating core graph traversal.

## How to Test
<!-- Provide reproducible commands or steps to verify this change. -->
```bash
go test -v ./...
```
