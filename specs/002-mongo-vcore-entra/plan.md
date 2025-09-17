# Implementation Plan: Azure Entra Authentication for MongoDB Cosmos DB

**Branch**: `002-mongo-vcore-entra` | **Date**: September 16, 2025 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/002-mongo-vcore-entra/spec.md`

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
Add Azure Entra (workload identity) authentication support for MongoDB connections to Azure Cosmos DB for MongoDB vCore. This extends existing connection string authentication with secure Azure-managed identity authentication, enabling MONGO-OIDC authentication mechanism for enhanced security and compliance. The generic Entra auth token provider in the auth package will be utilized when generating MongoDB clients.

## Technical Context
**Language/Version**: Go 1.25+  
**Primary Dependencies**: go.mongodb.org/mongo-driver, github.com/Azure/azure-sdk-for-go/sdk/azidentity, existing pkg/auth package  
**Storage**: MongoDB Cosmos DB for MongoDB vCore (existing)  
**Testing**: Go testing with testcontainers for MongoDB integration tests  
**Target Platform**: Kubernetes (existing deployment)
**Project Type**: single (library-first Go project)  
**Performance Goals**: <1s authentication token acquisition, maintain existing 10k events/sec throughput  
**Constraints**: <200ms authentication latency, backward compatibility with connection strings, workload identity support  
**Scale/Scope**: Support both source and target MongoDB connections, minimal configuration changes required

## Constitution Check
*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Simplicity**:
- Projects: 1 (extending existing replicator project)
- Using framework directly? (go.mongodb.org/mongo-driver with OIDC, Azure SDK directly)
- Single data model? (extending existing MongoConfig, no new DTOs)
- Avoiding patterns? (direct authentication integration, no additional wrapper patterns)

**Architecture**:
- EVERY feature as library? (extending pkg/auth library for Entra authentication, extending mongo connection libraries)
- Libraries listed: pkg/auth (Entra token provider), pkg/position (enhanced mongo config), pkg/streams (enhanced mongo stream), pkg/estuary (enhanced mongo endpoint)
- CLI per library: --auth-method entra flag support, --mongo-auth entra option, --help/--version existing
- Library docs: llms.txt format planned for enhanced authentication

**Testing (NON-NEGOTIABLE)**:
- RED-GREEN-Refactor cycle enforced? (contract tests → integration tests → unit tests)
- Git commits show tests before implementation? (test files created first)
- Order: Contract→Integration→E2E→Unit strictly followed? (MongoDB auth contract → testcontainer integration → end-to-end)
- Real dependencies used? (testcontainers with real MongoDB, Azure identity emulation)
- Integration tests for: new auth library, mongo client contract changes, config schema changes

**Observability**:
- Structured logging included? (authentication events, token acquisition, connection failures)
- Frontend logs → backend? (N/A - backend service)
- Error context sufficient? (Azure error codes, MongoDB auth failures, token expiration)

**Versioning**:
- Version number assigned? (1.2.0 - new feature addition)
- BUILD increments on every change? (Yes, following MAJOR.MINOR.BUILD)
- Breaking changes handled? (backward compatible config extension, parallel tests for both auth methods)

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

**Structure Decision**: Option 1 (single project) - extends existing Go replicator with enhanced authentication libraries

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
- Load `/templates/tasks-template.md` as base structure
- Generate tasks from Phase 1 design docs (contracts, data model, quickstart)
- Each contract interface → contract test task [P] (parallel execution safe)
- Each configuration schema → config validation test task [P]
- Each enhanced entity → model enhancement task [P]
- Each user story from spec → integration test task
- Implementation tasks sequenced to make tests pass (TDD order)

**Ordering Strategy**:
- TDD order: Contract tests → Integration tests → Unit tests → Implementation
- Dependency order: Auth provider → Config extensions → Connection handlers → Stream/Position/Estuary enhancements
- Mark [P] for parallel execution (independent file modifications)
- Sequential dependencies: Config parsing → Auth setup → MongoDB connection → Stream operations

**Estimated Task Categories**:
1. **Contract Tests** (5-7 tasks, [P]): MongoDB OIDC callback, Entra token provider interface, config validation contracts
2. **Configuration Enhancement** (3-4 tasks, [P]): MongoConfig extension, schema validation, migration compatibility  
3. **Authentication Integration** (4-5 tasks): Entra provider, OIDC credential handler, token lifecycle management
4. **MongoDB Client Enhancement** (6-8 tasks): Position tracker, stream source, estuary endpoint with Entra auth
5. **Integration Tests** (4-6 tasks): End-to-end authentication flow, error handling, token refresh scenarios
6. **CLI and Observability** (3-4 tasks, [P]): Configuration validation flags, authentication metrics, logging

**Estimated Output**: 25-32 numbered, ordered tasks in tasks.md following constitutional TDD principles

**Task Dependencies**:
- Contract tests (tasks 1-7): Independent, can run in parallel [P]
- Auth provider implementation (tasks 8-12): Depends on contract tests passing
- MongoDB client updates (tasks 13-20): Depends on auth provider completion
- Integration tests (tasks 21-26): Depends on client implementations
- CLI/observability (tasks 27-32): Can run in parallel with integration tests [P]

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
- [ ] Phase 3: Tasks generated (/tasks command)
- [ ] Phase 4: Implementation complete
- [ ] Phase 5: Validation passed

**Gate Status**:
- [x] Initial Constitution Check: PASS
- [x] Post-Design Constitution Check: PASS
- [x] All NEEDS CLARIFICATION resolved
- [x] Complexity deviations documented (none required)

---
*Based on Constitution v2.1.1 - See `/memory/constitution.md`*