# Implementation Plan: Database Replication System with Change Streams

**Branch**: `001-replicator-is-a` | **Date**: September 12, 2025 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-replicator-is**Progress Tracking**:
*This checklist is updated during execution flow*

**Phase Status**:
- [x] Phase 0: Research complete (/plan command)
- [x] Phase 1: Design complete (/plan command)
- [x] Phase 2: Task planning complete (/plan command - describe approach only)
- [x] Phase 3: Tasks generated (/tasks command)
- [x] Phase 4: Implementation in progress - Configuration merge COMPLETE ✅, Core Models COMPLETE ✅, Core Libraries COMPLETE ✅ (5/5)
- [ ] Phase 5: Validation passed`

## Execution Flow (/plan command scope)
```
1. Load feature spec from Input path
   → If not found: ERROR "No feature spec at {path}"
2. Fill Technical Context (scan for NEEDS CLARIFICATION)
   → Detect Project Type from context (web=frontend+backend, mobile=app+api)
   → Set Structure Decision based on project type
3. Evaluate Constitution Check section below
   → If violations exist: Document in Complexity Tracking
   → If no justification possible: ERROR "Simplify approach first"
   → Update Progress Tracking: Initial Constitution Check
4. Execute Phase 0 → research.md
   → If NEEDS CLARIFICATION remain: ERROR "Resolve unknowns"
5. Execute Phase 1 → contracts, data-model.md, quickstart.md, agent-specific template file (e.g., `CLAUDE.md` for Claude Code, `.github/copilot-instructions.md` for GitHub Copilot, or `GEMINI.md` for Gemini CLI).
6. Re-evaluate Constitution Check section
   → If new violations: Refactor design, return to Phase 1
   → Update Progress Tracking: Post-Design Constitution Check
7. Plan Phase 2 → Describe task generation approach (DO NOT create tasks.md)
8. STOP - Ready for /tasks command
```

**IMPORTANT**: The /plan command STOPS at step 7. Phases 2-4 are executed by other commands:
- Phase 2: /tasks command creates tasks.md
- Phase 3-4: Implementation execution (manual or via tools)

## Summary
Database replication system utilizing change streams to track and replicate database changes in real-time. Configuration-driven autonomous backend service supporting multiple database types, optional data transformation, failure recovery with position tracking, and comprehensive metrics. Technical approach: Go 1.25, containerized for Kubernetes deployment, OpenTelemetry metrics, Azure Entra authentication, Cosmos MongoDB API support, structured JSON logging.

## Technical Context
**Language/Version**: Go 1.25  
**Primary Dependencies**: OpenTelemetry SDK, Azure SDK for Go, MongoDB driver, Kazaam transformation, Docker, Kubernetes client  
**Storage**: Position tracking (persistent storage), configuration files (YAML/JSON)  
**Testing**: Go testing framework, testcontainers for integration tests  
**Target Platform**: Kubernetes clusters (Linux containers)
**Project Type**: single (backend service)  
**Performance Goals**: 10k events/sec throughput, <1s replication lag  
**Constraints**: Zero data loss, <200ms transformation latency, container memory <512MB  
**Scale/Scope**: Support 100+ concurrent streams, multi-database types, enterprise scale

## Constitution Check
*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Simplicity**:
- Projects: 1 (replicator service)
- Using framework directly? Yes (Go standard library, minimal dependencies)
- Single data model? Yes (unified event/position model)
- Avoiding patterns? Yes (direct implementations, no unnecessary abstractions)

**Architecture**:
- EVERY feature as library? Yes (streams, transform, metrics, auth libraries)
- Libraries listed: 
  - streams: change stream abstraction
  - transform: data transformation engine  
  - metrics: OpenTelemetry integration
  - auth: Azure Entra authentication
  - estuary: destination database abstraction
- CLI per library: Yes (--help/--version/--format for each)
- Library docs: llms.txt format planned

**Testing (NON-NEGOTIABLE)**:
- RED-GREEN-Refactor cycle enforced? Yes (contracts → integration → unit)
- Git commits show tests before implementation? Required
- Order: Contract→Integration→E2E→Unit strictly followed? Yes
- Real dependencies used? Yes (actual DBs via testcontainers)
- Integration tests for: new libraries, contract changes, shared schemas? Yes
- FORBIDDEN: Implementation before test, skipping RED phase

**Observability**:
- Structured logging included? Yes (JSON format, contextual fields)
- Frontend logs → backend? N/A (backend service only)
- Error context sufficient? Yes (tracing, correlation IDs)

**Versioning**:
- Version number assigned? 1.0.0 (initial implementation)
- BUILD increments on every change? Yes
- Breaking changes handled? Yes (parallel tests, migration plan)

## Project Structure

### Documentation (this feature)
```
specs/[###-feature]/
├── plan.md              # This file (/plan command output)
├── research.md          # Phase 0 output (/plan command)
├── data-model.md        # Phase 1 output (/plan command)
├── quickstart.md        # Phase 1 output (/plan command)
├── contracts/           # Phase 1 output (/plan command)
└── tasks.md             # Phase 2 output (/tasks command - NOT created by /plan)
```

### Source Code (repository root)
```
# Option 1: Single project (DEFAULT)
src/
├── models/
├── services/
├── cli/
└── lib/

tests/
├── contract/
├── integration/
└── unit/

# Option 2: Web application (when "frontend" + "backend" detected)
backend/
├── src/
│   ├── models/
│   ├── services/
│   └── api/
└── tests/

frontend/
├── src/
│   ├── components/
│   ├── pages/
│   └── services/
└── tests/

# Option 3: Mobile + API (when "iOS/Android" detected)
api/
└── [same as backend above]

ios/ or android/
└── [platform-specific structure]
```

**Structure Decision**: Option 1 (Single project) - Backend service with library-based architecture

## Phase 0: Outline & Research
1. **Extract unknowns from Technical Context** above:
   - For each NEEDS CLARIFICATION → research task
   - For each dependency → best practices task
   - For each integration → patterns task

2. **Generate and dispatch research agents**:
   ```
   For each unknown in Technical Context:
     Task: "Research {unknown} for {feature context}"
   For each technology choice:
     Task: "Find best practices for {tech} in {domain}"
   ```

3. **Consolidate findings** in `research.md` using format:
   - Decision: [what was chosen]
   - Rationale: [why chosen]
   - Alternatives considered: [what else evaluated]

**Output**: research.md with all NEEDS CLARIFICATION resolved

## Phase 1: Design & Contracts
*Prerequisites: research.md complete*

1. **Extract entities from feature spec** → `data-model.md`:
   - Entity name, fields, relationships
   - Validation rules from requirements
   - State transitions if applicable

2. **Generate API contracts** from functional requirements:
   - For each user action → endpoint
   - Use standard REST/GraphQL patterns
   - Output OpenAPI/GraphQL schema to `/contracts/`

3. **Generate contract tests** from contracts:
   - One test file per endpoint
   - Assert request/response schemas
   - Tests must fail (no implementation yet)

4. **Extract test scenarios** from user stories:
   - Each story → integration test scenario
   - Quickstart test = story validation steps

5. **Update agent file incrementally** (O(1) operation):
   - Run `/scripts/update-agent-context.sh [claude|gemini|copilot]` for your AI assistant
   - If exists: Add only NEW tech from current plan
   - Preserve manual additions between markers
   - Update recent changes (keep last 3)
   - Keep under 150 lines for token efficiency
   - Output to repository root

**Output**: data-model.md, /contracts/*, failing tests, quickstart.md, agent-specific file

## Phase 2: Task Planning Approach
*This section describes what the /tasks command will do - DO NOT execute during /plan*

**Task Generation Strategy**:
- Load `/templates/tasks-template.md` as base
- Generate tasks from Phase 1 design docs (contracts, data model, quickstart)
- Library creation tasks for each component (streams, transform, metrics, auth, estuary)
- Contract test tasks for Management API endpoints
- Integration test tasks for end-to-end scenarios
- Configuration validation implementation tasks
- Docker containerization and Kubernetes deployment tasks

**Ordering Strategy**:
- TDD order: Contract tests → Integration tests → Implementation
- Dependency order: Core libraries → Service assembly → API layer → Deployment
- Mark [P] for parallel execution where dependencies allow
- Priority on position tracking and recovery mechanisms (critical path)

**Estimated Output**: 35-40 numbered, ordered tasks in tasks.md

**Key Task Categories**:
1. **Library Development** (Go 1.25 upgrade, core abstractions)
2. **Authentication Integration** (Azure Entra ID, credential management)
3. **Database Connectors** (Cosmos DB MongoDB API, existing database support)
4. **Observability** (OpenTelemetry integration, structured JSON logging)
5. **Containerization** (Docker, Kubernetes manifests)
6. **Testing** (Contract, integration, end-to-end scenarios)

**IMPORTANT**: This phase is executed by the /tasks command, NOT by /plan

## Phase 3+: Future Implementation
*These phases are beyond the scope of the /plan command*

**Phase 3**: Task execution (/tasks command creates tasks.md)  
**Phase 4**: Implementation (execute tasks.md following constitutional principles)  
**Phase 5**: Validation (run tests, execute quickstart.md, performance validation)

## Complexity Tracking
*Fill ONLY if Constitution Check has violations that must be justified*

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |


## Progress Tracking
*This checklist is updated during execution flow*

**Phase Status**:
- [x] Phase 0: Research complete (/plan command)
- [x] Phase 1: Design complete (/plan command)
- [x] Phase 2: Task planning complete (/plan command - describe approach only)
- [x] Phase 3: Tasks generated (/tasks command)
- [ ] Phase 4: Implementation complete
- [ ] Phase 5: Validation passed

**Gate Status**:
- [x] Initial Constitution Check: PASS
- [x] Post-Design Constitution Check: PASS
- [x] All NEEDS CLARIFICATION resolved
- [x] Complexity deviations documented

---
*Based on Constitution v2.1.1 - See `/memory/constitution.md`*