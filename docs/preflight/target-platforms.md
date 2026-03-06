# Target Platforms

Status date: 2026-03-06

## MVP Targets (CLI)

- OS: Linux, macOS, Windows
- CPU: x86_64 (amd64), arm64
- Runtime: native binaries, no container requirement

## Non-Goals for MVP

- Browser/WASM runtime is optional and deferred.
- GPU acceleration is out of scope for early phases.

## Platform Constraints

- Prefer pure Go dependencies for portability.
- cgo usage requires explicit justification and a fallback strategy.
- Deterministic outputs must hold across supported platforms within defined numeric tolerances.

## Validation Matrix (minimum)

- Linux amd64: required in CI
- Linux arm64: required in CI (or emulator fallback if needed)
- macOS arm64: required pre-release check
- Windows amd64: required pre-release check
