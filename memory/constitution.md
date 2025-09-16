# Replicator Constitution

## Core Principles

### I. Library-First
Every feature starts as a standalone library. Libraries must be self-contained, independently testable, documented. Clear purpose required - no organizational-only libraries.

### II. CLI Interface  
Every library exposes functionality via CLI. Text in/out protocol: stdin/args → stdout, errors → stderr. Support JSON + human-readable formats.

### III. Test-First (NON-NEGOTIABLE)
TDD mandatory: Tests written → User approved → Tests fail → Then implement. Red-Green-Refactor cycle strictly enforced.

**Go Testing Standards**: Test files must live alongside source code with `_test.go` suffix following Go conventions. Test packages mirror source package structure. Integration tests use real dependencies via testcontainers when possible.

### IV. Integration Testing
Focus areas requiring integration tests: New library contract tests, Contract changes, Inter-service communication, Shared schemas.

### V. Observability
Text I/O ensures debuggability. Structured JSON logging required. OpenTelemetry for metrics and tracing.

### VI. Versioning & Breaking Changes
MAJOR.MINOR.BUILD format. BUILD increments on every change. Breaking changes require parallel tests and migration plan.

### VII. Simplicity
Start simple, YAGNI principles. Avoid premature abstraction. Direct implementations preferred over complex patterns.

## Technical Constraints

### Technology Stack
- Go 1.25+ required
- OpenTelemetry for observability
- Azure Entra ID for authentication
- Kubernetes deployment target
- Docker containerization

### Documentation Standards
- All documentation files must reside in the `docs/` folder
- Markdown format required for all documentation
- API documentation auto-generated from code comments
- Architecture diagrams and design documents in `docs/`
- Implementation summaries and guides in `docs/`

### Performance Standards
- 10k events/sec throughput minimum
- <1s replication lag maximum
- <200ms transformation latency
- <512MB container memory limit

## Development Workflow

### Quality Gates
- All tests must pass before merge
- Contract tests → Integration tests → Unit tests (TDD order)
- golangci-lint must pass
- Constitution compliance verified in PR reviews

### Architecture Validation
- Every feature must be library-first
- No circular dependencies allowed
- Configuration centralized in pkg/config

## Governance

Constitution supersedes all other practices. Amendments require documentation, approval, and migration plan.

All PRs/reviews must verify compliance. Complexity must be justified. Use copilot-instructions.md for runtime development guidance.

**Version**: 1.0.0 | **Ratified**: 2025-09-12 | **Last Amended**: 2025-09-12