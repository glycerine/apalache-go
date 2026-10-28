# Apalache Scala-to-Go port plan

Status: implementation in progress; standalone safety, temporal, and VMT
vertical slices are operational (2026-10-27)

This document is the working plan for a behavior-preserving Go port of
Apalache. The deliverable lives under `pkg/`, builds as a standalone model
checker, and uses the Go TLA+ Toolbox port in `~/tlago` for parsing and TLA+
semantic handling and the pure-Go Z3 port in `~/g3` for SMT solving. The Scala
implementation remains the specification and the primary differential-testing
oracle until the Go implementation reaches parity.

## Goals and definition of done

The port is complete when all of the following are true:

1. `pkg/cmd/apalache` builds a standalone executable without a JVM, Scala,
   SBT, or the Scala Apalache distribution at runtime.
2. The executable accepts TLA+ input, configuration, tuning, and the principal
   checking/simulation/trace-output options of the Scala CLI. Parsing and
   semantic analysis are performed through `github.com/glycerine/tlago`.
3. The Go pipeline implements the Scala pipeline's material semantics:
   importing, typing, preprocessing, transition extraction, arena/SMT
   encoding, bounded search, invariant and deadlock checking, counterexample
   decoding, and output formats. Temporal/liveness and ReTLA/VMT modes are
   included after safety checking, not silently treated as safety properties.
4. The Go unit and integration suite covers every ported subsystem. Golden and
   differential tests compare normalized Go results with the current Scala
   implementation. Existing Scala fixtures are reused where licensing and
   test isolation permit.
5. `go test ./...`, race-sensitive tests, fuzz/property smoke tests, end-to-end
   corpus tests, and representative Scala-vs-Go differential tests are green.
   Solver-dependent tests explicitly report the solver/backend they exercise.
6. The Go executable is tested from a clean environment with only its shipped
   runtime assets/native solver components. No test-only delegation to the
   Scala checker is present in production paths.

## Source survey

The SBT project graph provides the first useful topological ordering:

```text
tlair
  +-- mod-infra
  +-- tla-io
        +-- tla-parser (also mod-infra)
        +-- tla-typechecker (also mod-infra)
        +-- tla-pp (also mod-infra; parser only in tests)
              +-- tla-assignments
                    +-- passes (also parser + typechecker)
                          +-- tla-bmcmt
                                +-- json-rpc --+
                                +-- shai -------+-- mod-tool
                                                  +-- mod-distribution
```

The build currently contains approximately 565 production Scala files in the
main porting scope and 250 Scala test/resource files. Counts observed during
the survey (production / test-tree files) are:

| Scala project | Production | Test tree | Responsibility |
| --- | ---: | ---: | --- |
| `tlair` | 138 | 49 | typed IR, operators, types, builders, transforms, source maps, pure arena/formula types |
| `tla-io` | 79 | 28 | JSON/ITF/TLA output, annotations, TLC config, tool config, traces |
| `tla-parser` | 19 | 6 | SANY-to-Apalache IR translation and annotations |
| `tla-typechecker` | 18 | 9 | constraint generation/solving and type rewriting |
| `tla-pp` | 23 | 20 | desugaring, inlining, normalization, optimization, temporal encoders |
| `tla-assignments` | 6 | 29 | priming/assignment introduction and symbolic-transition extraction |
| `passes` | 30 | 4 | pass contracts and production pipeline composition |
| `tla-bmcmt` | 222 | 95 | arena, SMT contexts, rewriting rules, caches/oracles, transition execution, bounded checker, VMT |
| `mod-infra` | 9 | 2 | errors, exit codes, pass executor, logging |
| `mod-tool` | 12 | 2 | command model and CLI entry point |
| `json-rpc` | 6 | 3 | JSON-RPC server/session state |
| `shai` | 3 | 3 | gRPC transition exploration service |

Important internal clusters, in dependency order, are:

1. Identity, source positions/regions/stores, values, type tags, Type System 1,
   operator metadata, expressions, declarations, and modules.
2. Generic transformations, listeners, stable topological sort, language
   predicates, pretty/JSON/ITF writers, annotations, and configuration.
3. Type parsing/unification and typed/unsafe builders.
4. Parsing/import adaptation. `tlago` already exposes checked `Spec`, `Module`,
   declarations, definitions, expressions, source positions, and canonical
   Apalache IR JSON (`ApalacheIRJSON`), so the Scala SANY adapter should not be
   mechanically reimplemented. A thin conversion boundary should consume the
   native `tlago` AST; its Apalache JSON export is also a useful conformance
   oracle, not the in-memory production boundary.
5. ETC type checking, type aliases, row types, and typed rewriting.
6. Preprocessing: declaration sort, substitution, inlining, desugaring,
   normalization, constant/set simplification, keramelization, temporal
   tableau/loop encoding, and language watchdogs.
7. Assignment introduction, priming, and symbolic-transition extraction.
8. Symbolic core: formulas/sorts, arena cells/edges, caches, oracles, SMT
   adapter, expression grading/marking, rewriting rules, symbolic states, and
   decoding.
9. Transition executor, verification-condition generation, bounded search,
   checker listeners, counterexample collection, and traces.
10. Pass orchestration, CLI, server protocols, packaging, and compatibility.

The Scala source has dense cross-links inside IR/type builders and especially
inside `bmcmt`. Reproducing each Scala namespace as a Go package would create
artificial interfaces and likely cycles. The Go layout therefore deliberately
uses one large library package for the model-checker core.

## Go layout and dependency policy

```text
pkg/
  go.mod
  *.go                         package apalache: all checker core layers
  *_test.go                    colocated black/white-box unit tests
  testdata/
    oracle/                    stable normalized Scala outputs
    specs/                     focused and corpus TLA+/CFG fixtures
  internal/oraclectl/          test-only Scala oracle runner/normalizer
  cmd/apalache/                thin standalone CLI (leaf package)
  cmd/apalache-oracle/         optional developer-only golden generator
```

Rules:

- Production checker code stays in package `apalache`, split by cohesive file
  prefixes (`ir_`, `types_`, `transform_`, `smt_`, `arena_`, `rewrite_`,
  `checker_`, and so on). This is intentional, not an invitation to mirror the
  Scala directory tree.
- `cmd/apalache` is an obvious leaf: it may import `apalache`; the core never
  imports a command package.
- `internal/oraclectl` is an obvious test/tool leaf and must never be imported
  by production core code.
- `g3` remains its own external package and is pinned to the stable repository
  tag `v0.0.5` (commit `fe4e2f4f6dae`). Since the repository tags the root
  while its Go module is rooted at `pkg/`, Go records this exact commit as
  `github.com/glycerine/g3/pkg v0.0.0-20261027115743-fe4e2f4f6dae` rather than
  accepting the semver spelling `v0.0.5`. Use a temporary
  `replace github.com/glycerine/g3/pkg => /mnt/oldrog/home/jaten/g3/pkg` only
  when explicitly testing unreleased local `g3` changes; never commit that
  replacement. It is a pure-Go port, so there are no native solver assets to
  extract.
- Solver abstractions and Apalache SMT terms remain in the core package to
  avoid cycles with arena and rewriting code. The production backend adapts
  directly to `g3`'s AST/solver APIs; SMT-LIB recording is a diagnostic/test
  backend, not the production solving path.
- `tlago` remains its own external package and is pinned to the stable tag
  `v0.1.9`. Use a temporary
  `replace github.com/glycerine/tlago => /mnt/oldrog/home/jaten/tlago` directive
  only when explicitly testing unreleased local changes; never commit that
  replacement.
- Keep dependencies small and pure-Go. The final checker must not depend on a
  user-installed solver, native Z3 library, JVM, or Scala toolchain.
- Public types should be introduced only for CLI/library integration. Most of
  the mechanically ported surface remains unexported until consumers require
  it.

## Rough implementation order

Each phase ends with green tests and a commit. Within phases, prefer vertical
slices that produce observable behavior over large untested translations.

### Phase 0: harness and architectural skeleton

- Add `tlago`, the root `apalache` package, a minimal CLI, version/build
  metadata, context cancellation, structured diagnostics, and stable exit-code
  mapping.
- Add `Makefile` or scripts for `test`, `test-race`, `test-oracle`, `build`, and
  `verify`, all capped at `-p 8` / `GOMAXPROCS=8` and runnable on CPUs 1-8 via
  `taskset -c 1-8` on Linux.
- Implement deterministic JSON normalization and the Scala oracle runner. The
  normalizer removes nondeterministic paths, IDs, timing, and ordering only
  where the semantics declare ordering irrelevant.
- Establish CI-friendly test categories: fast unit tests are the default;
  solver, Scala-oracle, corpus, and slow tests use explicit flags/build tags.

Acceptance: a standalone Go binary parses/checks a tiny module through
`tlago`, emits deterministic diagnostics/JSON, and differential harness tests
can invoke both implementations.

### Phase 1: core IR and types

- Port `UID`, values, source positions/regions and source tracking.
- Port Type System 1: primitives, constants/aliases, variables, sets,
  sequences, functions, tuples/sparse tuples, records/rows/variants, operators,
  schemes, monotype and used-variable calculations.
- Port substitutions, unification, variable pools, model values, type parser,
  and type printers.
- Port operator definitions with exact names and arity/argument invariants.
- Port expression/declaration/module nodes with immutable-by-convention
  constructors, explicit cloning, and deterministic equality/hash helpers.
- Port generic builder and typed builder semantics only after the IR invariants
  are locked by tests.

Acceptance: all applicable `tlair` type/value/expression/builder tests have Go
equivalents; property tests cover substitution composition, unification, type
round trips, equality, and constructor rejection.

### Phase 2: transforms, stores, and I/O leaf functionality

- Port transformation/listener contracts, stable topological sort, declaration
  sorting, source maps, disjoint sets, deep copy, replacement, renaming, prime
  propagation, flattening, and language predicates/watchdogs.
- Port Apalache IR JSON read/write with canonical ordering and preserve source
  data from `tlago`.
- Port TLA pretty writing, type-tag writing, annotations, TLC config parsing,
  ITF trace conversion, and counterexample output. Port Quint support after the
  primary TLA/JSON/ITF paths.
- Cross-test Go JSON against Scala `JsonTlaWriter`/readers and against tlago's
  `ApalacheIRJSON` for their shared supported subset.

Acceptance: round-trip and golden tests cover every node/type/operator; source
locations survive transformations; output is byte-stable where the Scala
format is canonical and structurally equivalent otherwise.

### Phase 3: frontend integration and type checking

- Convert checked `tlago.Spec`/`tlago.Module` AST directly into Go Apalache IR,
  including declarations, operator-valued parameters, recursion, LET/IN,
  INSTANCE/substitution, standard modules, EXCEPT/`@`, assumptions/theorems,
  labels, comments/annotations, and accurate source spans.
- Inventory any semantic construct missing from the public `tlago` API and add
  narrowly scoped upstream accessors there rather than duplicating SANY.
- Port ETC expressions, constraints, solver, aliases, listeners, source-aware
  diagnostics, and the module/type rewrite.
- Preserve untyped import as a distinct stage; do not trust provisional type
  strings produced by the parser in place of Apalache type inference.

Acceptance: the existing parser/importer and typechecker fixtures have Go
equivalents, including negative diagnostics. Differential tests compare
normalized untyped and typed IR, not merely success/failure.

### Phase 4: preprocessing and transition extraction

- Port constant/config replacement, operator inlining, parameter and name
  normalization, desugaring, constant folding, set-membership simplification,
  integer-quantifier optimization, LET optimization, keramelization, and the
  temporal tableau/loop encoders.
- Port assignment operator introduction, priming pass, module adapter,
  symbolic-transition generation, and SMT-free extraction.
- Model the pass chain as ordinary typed Go functions over one pipeline state;
  retain pass inputs/outputs and dump hooks needed for Scala-compatible CLI
  behavior without recreating Guice modules.

Acceptance: every preprocessing/assignment Scala unit case is represented;
goldens compare normalized IR after each pass so a late checker result cannot
hide an early divergence.

### Phase 5: symbolic terms, arena, and solver context

- Port formulas/sorts and establish a solver-neutral `SolverContext` with
  declaration, assertion, push/pop, check, model evaluation, metrics,
  cancellation, and recording.
- First make a deterministic SMT-LIB recording backend for unit tests and
  oracle comparison. Then implement the production adapter over
  `github.com/glycerine/g3/pkg`, using its `ast`, `cmd_context`, `model`,
  `solver`, and `util/lbool` packages as needed. The adapter must verify
  incremental scopes, arrays, integer arithmetic, uninterpreted functions,
  quantifiers, cancellation, and model extraction before symbolic rewriting
  depends on it.
- CVC5 may remain only as an external test oracle for cross-solver comparison;
  it is not a production dependency or fallback. Do not add a CGo Z3 binding
  or an external Z3 process.
- Port pure/mutable arena, cells/pointers/edges, snapshots, bindings,
  stackable contexts, caches, and oracle implementations.

Acceptance: formula and arena suites pass against recording and real solver
backends; Z3/CVC5 cross-tests agree for a generated supported subset; push/pop
and snapshot restoration have randomized state-machine tests.

### Phase 6: symbolic rewriting

- Port analyses/markers and rule dispatch.
- Port base, Boolean, integer, control, assignment, set, sequence, function,
  record/row/variant, tuple, quantifier, CHOOSE/GUESS, folding, and Apalache
  extension rules.
- Implement one encoding completely (the current default array/fun-array
  choice based on compatibility evidence), then the alternative encodings.
  Never map an unsupported encoding flag to a different encoding silently.
- Port stratified rewriting only if still reachable from supported CLI paths;
  otherwise retain an explicit unsupported diagnostic until the phase is
  completed.

Acceptance: each Scala rewriter/rule test has a Go table/property counterpart,
including arena shape, asserted SMT, satisfiability, and decoded value—not only
the returned cell ID. Cross-encoding results agree.

### Phase 7: transition execution and bounded checking

- Port symbolic states, state decoding, VC generation, incremental/offline
  execution contexts and snapshots, filtered/constrained transition executors,
  search state, bounds, listeners, and sequential bounded search.
- Implement initial-state, transition, invariant, deadlock, and action-property
  checks with Scala-compatible trace indexing and error classification.
- Add temporal checking/tableau integration and ReTLA/VMT paths as separately
  tested capabilities.
- Make all loops cancellation-aware and apply configurable time/memory bounds.

Acceptance: unit suites for executor/VC/checker/decoder pass for every solver
encoding; small-model exhaustive and differential tests agree on verdict,
counterexample depth, and normalized trace.

### Phase 8: CLI, services, and standalone distribution

- Implement `parse`, `typecheck`, `check`, `simulate`, `test`, `config`,
  `transpile`, and `tracee` command behavior in descending usage priority.
- Match configuration precedence (CLI, JSON, TLC config, annotations/defaults),
  diagnostics, exit codes, dump files, output directories, and ITF traces.
- Add JSON-RPC and Shai/gRPC only after the library checker API is stable; keep
  them leaf packages if their generated protocol code warrants separate
  packages.
- Embed standard Apalache TLA modules and required runtime data. Build release
  archives and smoke-test them in an environment without Java/Scala/SBT and
  without the source checkout.

Acceptance: end-to-end CLI matrix and protocol tests pass; release archive
smoke tests verify good specs, violations, parse/type errors, cancellation, and
trace output.

### Phase 9: parity closure and hardening

- Run the repository TLA/Quint/tlaplus-suite corpora with explicit allowlists
  for known Scala bugs or consciously unsupported features. Every allowlist
  item needs an issue, reason, expected diagnostic, and expiry/review point.
- Differential fuzz TLA fragments, IR, types, pass pipelines, and bounded
  models. Minimize discrepancies into committed regression fixtures.
- Run `go test -race`, fuzz smoke, static analysis, leak/resource tests,
  cancellation tests, and repeated deterministic-output tests.
- Benchmark representative models against Scala for correctness first, then
  address material performance regressions with profiles.
- Audit all Scala production files and reachable commands against a port
  manifest. Delete oracle/runtime scaffolding that accidentally couples the Go
  executable to Scala.

Acceptance: the completion criteria and audit matrix below are fully evidenced.

## Comprehensive verification strategy

### Unit tests

Every ported Scala class/object with semantic behavior gets a focused Go test.
Table-driven tests should name the corresponding Scala suite/case in comments
or fixture metadata. Tests must include success, boundary, malformed input,
wrong arity/type, empty collection, recursive definition, shadowing, source
location, and deterministic-order cases as applicable.

Use Go fuzz/property tests for algebraic laws and broad spaces:

- parse/print and JSON/IR round trips;
- type substitution identity/composition/idempotence and unifier soundness;
- transformation identity, idempotence where promised, and UID/source mapping;
- stable topological order and cycle reporting;
- arena snapshot/restore and cache consistency;
- oracle selection bounds and encoding equivalence;
- solver push/pop scope and model evaluation;
- trace encode/decode and name restoration.

### Scala correctness oracle

The oracle has two modes:

1. Committed goldens for default fast tests. A versioned manifest records the
   Scala git commit, command, input digest, normalization schema, solver/version,
   and expected exit status.
2. Live differential tests for development/CI environments with Java/SBT. The
   harness invokes the current Scala checker and Go checker on the same isolated
   fixture, then compares normalized stage output or final results.

Comparison strength, from early to late stages:

- exact type/value/operator/source structures;
- exact or alpha-normalized IR after each pass;
- normalized SMT declarations/assertions plus solver result;
- arena graph and decoded values modulo deterministic renaming;
- verdict, violated property, trace length, and each state value;
- CLI exit code, structured diagnostics, and output files.

An oracle discrepancy is a failure unless captured as a reviewed known
difference. The harness must retain both raw outputs and the minimized/normalized
diff to avoid normalization masking defects.

### Integration, corpus, and release tests

- Reuse existing test resources from all Scala modules and repository-level
  `test/tla`, `test/quint`, and `test/tlaplus-suite` inputs.
- Maintain tiny purpose-built specifications for one feature each, plus mixed
  regression models and known real-world models.
- Cross-check satisfiable/unsatisfiable formulas with two solvers when possible.
- Test deterministic results at worker counts 1 and up to 8. Keep this port's
  routine jobs on logical CPUs 1-8 (`taskset -c 1-8`) and Go build parallelism
  at eight or fewer.
- Build the actual release artifact, copy it and fixtures to a temporary clean
  directory, hide Java/Scala/source-tree paths, and execute it there.

### Required gates

Fast checkpoint gate:

```sh
cd pkg
taskset -c 1-8 go test -p 8 ./...
taskset -c 1-8 go vet ./...
```

Regular full gate (as applicable by phase):

```sh
cd pkg
taskset -c 1-8 go test -p 8 -race ./...
taskset -c 1-8 go test -p 8 -tags=oracle ./...
taskset -c 1-8 go test -p 8 -tags=solver,corpus ./...
taskset -c 1-8 go build -p 8 ./cmd/apalache
```

The eventual scripts should hide platform-specific `taskset` use while
preserving the CPU/parallelism cap.

## Commit and work discipline

- Work on the existing branch; do not rebase. Background pushes are treated as
  backups and must not be rewritten.
- Aim for a green, coherent commit about every ten minutes while actively
  implementing. Do not create knowingly failing checkpoint commits merely to
  meet the clock.
- Before each commit: inspect `git diff`, preserve unrelated user changes, run
  the phase-appropriate fast gate on CPUs 1-8, and record the exact command in
  the commit/report.
- Prefer small dependency-ordered commits: test/harness, implementation, then
  integration. Never mix generated solver binaries or large corpus updates
  into unrelated semantic changes.

## Risks and explicit decisions still requiring evidence

- `~/tlago` is presently a symlink to `/mnt/b/github.com/tlaplus/tlago`, module
  `github.com/glycerine/tlago`, and already includes a native checked AST plus
  Apalache IR JSON support. The reproducible dependency is pinned at `v0.1.9`;
  local changes require an explicit temporary replacement.
- `~/g3` is presently a symlink to
  `/mnt/oldrog/home/jaten/go/src/github.com/glycerine/g3`, with its Go module at
  `pkg/` (`github.com/glycerine/g3/pkg`). It is the required production Z3
  implementation, pinned at `v0.0.5`; the exact stable adapter API must be
  validated before symbolic rewriting depends on it. Solver semantic coverage
  remains an early technical risk even though native packaging is eliminated.
- Scala mutation/implicit type tags do not map safely to casual Go pointers.
  Constructors must enforce invariants and mutation must be localized to the
  few pipeline stages that require it.
- UID, map iteration, solver model order, and generated names can introduce
  flaky diffs. All externally compared structures need explicit stable order
  and a narrowly justified alpha-normalizer.
- Temporal, ReTLA/VMT, JSON-RPC, Shai, and Quint support are real scope, but are
  sequenced after the standalone safety-checking vertical slice. Unsupported
  modes must fail clearly until implemented.
- Existing Scala behavior may contain bugs. Differential parity is the default;
  intentional corrections require a regression demonstrating the Scala issue
  and a documented reviewed difference, rather than silently changing results.
- The current Scala `ConfigurationPass` parses TLC `CONSTRAINT` and
  `ACTION_CONSTRAINT` entries but does not propagate them to the checker. The
  Go CLI deliberately matches that behavior, covered by live differential
  cases; the Go library's explicit `SpecificationConfiguration` constraint
  fields remain independently unit-tested for a future coordinated change.

## Completion audit matrix

Before declaring the port complete, fill every evidence cell with a file,
command log, or CI artifact; absence of a discrepancy is not evidence.

| Requirement | Required evidence | Current evidence and remaining work |
| --- | --- | --- |
| All reachable Scala semantics surveyed | source-to-Go manifest covering every production Scala file/command | **Inventoried, refinement ongoing.** `SCALA_PORT_MANIFEST.tsv` contains all 564 current `src/main/scala` files with an honest `partial`, `dependency`, or `open` disposition, Go evidence, and remaining work. `TestScalaPortManifestCoversEveryProductionSource` rejects missing, duplicate, or stale rows as the Scala tree changes. Per-file evidence is initially grouped by subsystem and must become more specific as the remaining Scala suites are mapped. |
| Parsing uses `tlago` | pinned dependency plus frontend integration tests for full syntax corpus | **Partial, strong corpus coverage.** `go.mod` pins `tlago v0.1.9`; `frontend_tlago_*.go` and frontend unit tests exercise the native checked AST. The tagged corpus requires all 267 top-level `test/tla` fixtures to parse except four Scala-confirmed negative fixtures with mapped diagnostics, and typechecks more than 220 annotated fixtures including nine mapped negatives. The Scala instance suite is fully mapped, including nested exports, composed explicit and implicit substitutions, qualified lookup, higher-order arguments, standard modules, and named and unqualified `LOCAL` scoping reconstructed as source-mapped, annotation-preserving `LET-IN` definitions. Remaining parser-module resource cases and broader exact diagnostic normalization remain open. |
| Core model checker is Go | dependency/license audit and production-call-graph review showing no Scala/JVM/native-Z3 delegation | **Partial, audited with upstream blockers.** `DEPENDENCY_AUDIT.md` inventories the exact linked module set, which the release gate enforces, and records a whole-program symbol audit: native Z3, Scala/JVM, tlago XML validation, and g3's GLPK driver are absent from the production path. Builds default to `CGO_ENABLED=0`. The missing upstream g3 license file and g3's retained interactive assertion-debugger shell path must be resolved in a tagged g3 release. |
| Z3 uses `g3` | pinned `github.com/glycerine/g3/pkg` dependency plus incremental/array/integer/UF/quantifier/model adapter tests | **Implemented for the current solver boundary.** `g3` tag `v0.0.5` is pinned by exact pseudo-version; `smt_g3*_test.go` covers the active ground encoding, integer and uninterpreted sorts, models, incremental scopes, cancellation, array store/select, and quantified integer formulas under an explicit quantified logic. |
| Standalone runtime | clean release-archive tests with Java/Scala/SBT, native Z3, external solver executables, and source checkout unavailable | **Implemented for the current command surface.** The Makefile defaults release builds to `CGO_ENABLED=0`; `release_integration_test.go` rejects native ELF dependencies and exercises the archive with an empty executable search path, including typed JSON input, violations, ITF, VMT, diagnostics, and cancellation. Repeat after every packaging change. |
| Safety and deadlock correctness | unit, solver-cross, small exhaustive, and Scala differential suites | **Partial, strong vertical slice.** Bounded checker/executor tests and 48 live Scala differential cases compare verdict and available depths, including a depth-4 deadlock, TLC constraint compatibility, core function constructors and function sets, sequence-extension operators, variants, and rejection of symbolic-range choice. A deterministic generated suite independently explores 312 transition systems: 216 span nine scalar, tuple, record, set, sequence, function, variant, and nested state domains; 24 operator-rich integer machines exercise arithmetic, modulo, conditionals, guarded transitions, nondeterminism, and deadlocks; 24 set machines exercise union, difference, intersection, complement, membership guards, and nondeterministic updates; 24 sequence machines exercise `Head`, `Tail`, `Append`, indexing, `EXCEPT`, `@`, reversal, guards, and nondeterministic shifts; and 24 finite-quantifier machines exercise existential, universal, and nested binders plus nondeterministic quantified target assignments. It compares both minimum invariant-violation depth and minimum deadlock depth through bounds 0–3 against the full parser/typechecker/normalizer/g3 pipeline. `Next` extraction expands finite existential assignments to user state variables over explicit enumerations so each target is searched independently, while preserving symbolic initial-state and temporal-tableau choices; minimum-deadlock projection for quantified symbolic domains remains open. |
| Temporal/ReTLA/VMT correctness | dedicated semantic and end-to-end differential suites | **Partial.** Temporal loop/tableau unit tests and nine live lasso differential cases are green. VMT has semantic and release smoke tests plus an explicit `solver`-tagged oracle that sends generated initial-state, transition, and invariant sat/unsat queries to both native Z3 and CVC5. Broader ReTLA/VMT differential coverage remains open. |
| Counterexample correctness | state-by-state normalized trace comparisons and ITF round trips | **Partial, strong vertical slice.** Decoder and ITF round-trip tests are present. The live oracle compares verdict/property/depth for all 48 cases and exact normalized user-state values for nine deterministic action, temporal, function, sequence, set, variant, row-record, and deadlock counterexamples. For three nondeterministic trace-invariant, temporal-lasso, and record/set deadlock cases, it independently constrains every Scala and Go ITF state against normalized Init/Next alternatives and proves that both witnesses are valid behaviors. Broader nondeterministic data-shape coverage remains open. |
| CLI/config compatibility | command/precedence/exit-code/output-file matrix | **Partial.** Principal commands, TLC configuration, strict JSON configuration for `parse`, `typecheck`, `check`, `simulate`, `test`, `tracee`, `transpile`, and both `explorer` and `checker` server transports with CLI/`PORT` environment/JSON precedence are tested. The JSON-RPC explorer implements the complete Scala method list, atomic batching, negotiated gzip/zstd request and response compression above the Scala 512-byte threshold, and the typed Quint JSON IR path exercised by Scala's arbitrary-precision oracle fixture. Quint's complete Scala-supported type, expression, declaration, lambda, binder, variant, nondeterministic-let, collection, action, and temporal conversion surface is implemented; Scala's intentionally unsupported `repeated` and invalid standalone `oneOf` remain explicit errors. The `tictactoe.json` oracle imports as 40 typed declarations, loads through safety preprocessing, produces Init/Next/invariant metadata, initializes the Go SMT executor, and proves its first Init transition enabled; `clockSync3.json` additionally covers the full type table. The Shai v1 gRPC server registers both services with the Scala 1 GiB inbound limit: transition exploration covers connection validation and root/auxiliary model loading, while command execution covers parse, typecheck, TLA formatting, bounded checking, view-distinct multi-counterexample collection, remote filesystem rejection, both `discard-disabled` strategies, Scala-compatible transition/invariant search filters, and options exposed by the pinned g3 tag's typed parameter catalog (including integer, Boolean, floating-point, and symbolic values). Simulation restores constants and randomly shuffles alternatives for up to `--max-run`/`search.simulation.maxRun` independent runs, using Scala's default of 100; selected paths reuse the state/action/trace checker and stop at the configured view-distinct error budget. `--output-traces`/`search.outputTraces` emits deterministic TLA, TLC-console, legacy Apalache-IR JSON, and ITF siblings for safe examples and simulation/check/test violations, with numbered files for multiple errors; explicit `--itf` remains a single-file request. Every local source command (`parse`, `typecheck`, `check`, `simulate`, `test`, `tracee`, and `transpile`) accepts `--out-dir`/`--run-dir` through CLI, strict JSON, and `OUT_DIR`/`RUN_DIR`, creates Scala-shaped source namespaces and unique run directories, and writes reproducible `run.txt` invocations; artifact-producing commands additionally mirror implicit trace or VMT output and route evaluated traces without changing stdout defaults. Remote Shai requests reject filesystem-writing options. Scala-only solver parameters not exposed by g3, exact Shai error-shape parity, wall-clock banner parity, and debug configuration/intermediate files remain open. |
| Comprehensive tests | mapped Scala-test manifest, Go coverage report by subsystem, property/fuzz inventory | **Inventoried, mapping ongoing.** `SCALA_TEST_MANIFEST.tsv` accounts for all 213 current `src/test/scala` files with Go test evidence and explicit remaining work, while `TestScalaTestManifestCoversEveryScalaTestSource` rejects omissions and stale rows. `make coverage` regenerates the statement-weighted subsystem report in `COVERAGE.md`; the current fast-suite snapshot ranges from 67.7% for CLI/configuration to 84.6% for frontend/annotations, excluding generated protobuf code. Unit coverage is broad, the annotated corpus and live oracle manifests exist, and type parsing has a fuzz smoke gate. Per-case Scala assertion mapping and a formal property/fuzz inventory remain open. |
| Robustness | race, cancellation, resource/leak, deterministic-repeat, malformed-input results | **Partial.** Race, malformed-input, reproducible archive, parser fuzz, and standalone deterministic-repeat checks are green. The repeat gate compares diagnostics and byte-identical ITF counterexamples across four runs each at `GOMAXPROCS=1` and `8`; cancellation tests repeatedly reset one live g3 context, and JSON-RPC request contexts inherit server cancellation with a bounded shutdown fallback. An explicit `resource`-tagged gate creates, solves with, and closes 128 g3 contexts and verifies file descriptors return exactly to baseline and goroutines remain within the runtime allowance. Broader whole-server load/leak checks remain open. |
| CPU constraint | build/test scripts and CI settings capped at CPUs/parallelism 1-8 | **Implemented locally.** `Makefile` caps `-p=8` and uses `taskset -c 1-8` when that affinity is available; CI invocation still needs final review. |
| Green repository state | final `go test`, race, oracle, corpus, vet/static, build, and package logs | **Not a final claim.** Fast tests, vet, build, fuzz smoke, release/race checks, and the full 48-case live oracle have been green during implementation; all gates must be rerun together for completion. |

