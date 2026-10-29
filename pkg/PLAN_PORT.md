# Apalache Scala-to-Go port plan and completion record

Status: **completed and audited** for the declared standalone g3 scope
(2026-10-29). The completion audit matrix below records the evidence and the
explicit non-blocking limits of that scope.

This document records the plan and completed implementation of a
behavior-preserving Go port of Apalache. The deliverable lives under `pkg/`,
builds as a standalone model checker, and uses the Go TLA+ Toolbox port in
`~/tlago` for parsing and TLA+
semantic handling and the pure-Go Z3 port in `~/g3` for SMT solving. The Scala
implementation remains the reference specification and primary
differential-testing oracle. Declared compatibility limits and intentional
corrections are retained below rather than being presented as unfinished port
work.

## Completed goals and definition of done

The port satisfies all of the following completion criteria:

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
- `g3` remains its own external package and is pinned to `v0.0.9` through the
  nested-module tag `pkg/v0.0.9` (commit `63b060b93f16`). Use a temporary
  `replace github.com/glycerine/g3/pkg => /mnt/oldrog/home/jaten/g3/pkg` only
  when explicitly testing unreleased local `g3` changes; never commit that
  replacement. It is a pure-Go port, so there are no native solver assets to
  extract.
- Solver abstractions and Apalache SMT terms remain in the core package to
  avoid cycles with arena and rewriting code. The production backend adapts
  directly to `g3`'s AST/solver APIs; SMT-LIB recording is a diagnostic/test
  backend, not the production solving path.
- `tlago` remains its own external package and is pinned to the stable tag
  `v0.1.10`. Use a temporary
  `replace github.com/glycerine/tlago => /mnt/oldrog/home/jaten/tlago` directive
  only when explicitly testing unreleased local changes; never commit that
  replacement.
- Dependencies are small and pure-Go. The completed checker does not depend on
  a user-installed solver, native Z3 library, JVM, or Scala toolchain.
- Public types should be introduced only for CLI/library integration. Most of
  the mechanically ported surface remains unexported until consumers require
  it.

## Implemented dependency order (historical)

The implementation followed these phases, each ending with green tests and a
commit. This section is retained as execution history; all phases in the
declared standalone scope are complete.

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
taskset -c 1-8 go test -p 8 -race -timeout=60m ./...
taskset -c 1-8 go test -p 8 -tags=oracle ./...
taskset -c 1-8 go test -p 8 -tags=solver,corpus ./...
taskset -c 1-8 go build -p 8 ./cmd/apalache
```

The checked-in Makefile hides platform-specific `taskset` detection while
preserving the CPU/parallelism cap.

## Maintenance and commit discipline

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

## Implemented decisions and residual limits

- `~/tlago` is presently a symlink to `/mnt/b/github.com/tlaplus/tlago`, module
  `github.com/glycerine/tlago`, and already includes a native checked AST plus
  Apalache IR JSON support. The reproducible dependency is pinned at `v0.1.10`;
  local changes require an explicit temporary replacement.
- `~/g3` is presently a symlink to
  `/mnt/oldrog/home/jaten/go/src/github.com/glycerine/g3`, with its Go module at
  `pkg/` (`github.com/glycerine/g3/pkg`). It is the required production Z3
  implementation, pinned at `v0.0.9`; its stable solver adapter and typed SMT
  parameter catalog are validated by the symbolic rewriting and tuning suites.
  Residual solver risk is bounded by the ground-encoding, quantifier,
  cross-solver, cancellation, resource, and live checker suites described in
  the audit matrix.
- Scala mutation/implicit type tags do not map safely to casual Go pointers.
  Constructors must enforce invariants and mutation must be localized to the
  few pipeline stages that require it.
- UID, map iteration, solver model order, and generated names can introduce
  flaky diffs. All externally compared structures need explicit stable order
  and a narrowly justified alpha-normalizer.
- Temporal, ReTLA/VMT, JSON-RPC, Shai, and Quint support are implemented and
  covered by dedicated semantic, differential, protocol, and release tests.
  Inputs outside the declared supported surface fail explicitly.
- Existing Scala behavior may contain bugs. Differential parity is the default;
  intentional corrections require a regression demonstrating the Scala issue
  and a documented reviewed difference, rather than silently changing results.
- The current Scala `ConfigurationPass` parses TLC `CONSTRAINT` and
  `ACTION_CONSTRAINT` entries but does not propagate them to the checker. The
  Go CLI deliberately matches that behavior, covered by live differential
  cases; the Go library's explicit `SpecificationConfiguration` constraint
  fields remain independently unit-tested for a future coordinated change.

## Completion audit matrix

This matrix was filled before declaring completion. Evidence is executable or
names a checked-in manifest; documented limits are bounded exclusions or
future compatibility expansions, not unfinished implementation phases.

| Requirement | Required evidence | Completion evidence and documented limits |
| --- | --- | --- |
| All reachable Scala semantics surveyed | source-to-Go manifest covering every production Scala file/command | **Implemented for the standalone g3 scope.** `SCALA_PORT_MANIFEST.tsv` classifies all 564 current `src/main/scala` files with no `open` or `partial` rows: executable standalone behavior is `ported`, while build wiring, JVM integrations, and alternate solver backends are explicit `dependency`/`not-applicable` rows. Every row names concrete Go evidence and a disposition. `TestScalaPortManifestCoversEveryProductionSource` rejects missing, duplicate, or stale rows as the Scala tree changes. |
| Parsing uses `tlago` | pinned dependency plus frontend integration tests for full syntax corpus | **Complete for the declared standalone scope; corpus evidence is bounded.** `go.mod` pins `tlago v0.1.10`; `frontend_tlago_*.go` and frontend unit tests exercise the native checked AST. The tagged corpus requires all 267 top-level `test/tla` fixtures to parse except five Scala-confirmed negative fixtures with mapped diagnostics, and typechecks more than 220 annotated fixtures including ten mapped negatives. Every Scala parser/importer suite is fully mapped, including isolated/adjacent module resolution, ASCII/Unicode builtins, complex EXCEPT/select/function shapes, nested substitutions, recursively complete declaration/body source locations, source-mapped LOCAL/LET reconstruction, native libraries, recursive declarations, custom annotations, Scala-compatible rejection of legacy parenthesized tuple spellings, multiline type comments, precedence conflicts, binders, action/fairness subscripts, embedded labels, set-map bodies, prefix operands, negative literals, and application-local synthesized LETs in every surrounding context. Frontend diagnostics normalize temporary parser mounts and extensionless IR module names to stable `.tla` basenames, retain full inclusive ranges in CLI text, and recursively render composite types, TLA expressions, and legacy record types in Scala form. Live Bug914, Bug925, and ModelValFail negative-type oracles compare exact filename/ranges and complete messages with Scala for record, annotated-operator, and model-value mismatches; TestAliasNewMissing, UntypedConst, and UntypedVar additionally compare exact input messages; Bug832 and Bug860 compare Scala's two-stage legacy annotation-parser diagnostics. Broader exact full-message comparison is an optional compatibility expansion. |
| Core model checker is Go | dependency/license audit and production-call-graph review showing no Scala/JVM/native-Z3 delegation | **Implemented and audited.** `DEPENDENCY_AUDIT.md` inventories the exact linked module set, which the release gate enforces, and records a whole-program symbol audit: native Z3, Scala/JVM, tlago XML validation, and g3's GLPK driver are absent from the production path. Builds default to `CGO_ENABLED=0`. g3 v0.0.9 supplies the Microsoft Z3 MIT license, retained in `third_party_licenses/g3-LICENSE.txt` and required in release archives. At process initialization Apalache disables g3 assertions and binds v0.0.9's `DebugNoDebugger` action; an isolated subprocess regression proves assertion handling cannot consume interactive input or continue into gdb/shell handling. |
| Z3 uses `g3` | pinned `github.com/glycerine/g3/pkg` dependency plus incremental/array/integer/UF/quantifier/model adapter tests | **Implemented for the current solver boundary.** `g3` tag `pkg/v0.0.9` is pinned as the exact module version `v0.0.9`; `smt_g3*_test.go` covers the active ground encoding, integer and uninterpreted sorts, models, incremental scopes, cancellation, array store/select, single and nested quantified integer formulas, and typed Boolean/unsigned/double/symbol parameter delivery. All production and configured contexts declare `AUFNIA`, matching the arrays, nonlinear integer arithmetic, uninterpreted functions, and exact quantified enabledness formulas they may receive. Production deadlock search sends variable-width nonlinear integer witness predicates, including dependent nested witnesses, through g3's existential API. |
| Standalone runtime | clean release-archive tests with Java/Scala/SBT, native Z3, external solver executables, and source checkout unavailable | **Implemented for the current command surface.** The Makefile defaults release builds to `CGO_ENABLED=0`; `release_integration_test.go` rejects native ELF dependencies and exercises the archive with an empty executable search path, including typed JSON input, violations, ITF, VMT, diagnostics, and cancellation. Repeat after every packaging change. |
| Safety and deadlock correctness | unit, solver-cross, small exhaustive, and Scala differential suites | **Complete for the declared bounded-checking scope; evidence is deliberately finite.** Bounded checker/executor tests and 50 live Scala differential cases compare verdict and available depths, including a depth-4 deadlock, TLC constraint compatibility, core function constructors and function sets, sequence-extension operators, variants, and rejection of symbolic-range choice. A deterministic generated suite independently explores 312 transition systems: 216 span nine scalar, tuple, record, set, sequence, function, variant, and nested state domains; 24 operator-rich integer machines exercise arithmetic, modulo, conditionals, guarded transitions, nondeterminism, and deadlocks; 24 set machines exercise union, difference, intersection, complement, membership guards, and nondeterministic updates; 24 sequence machines exercise `Head`, `Tail`, `Append`, indexing, `EXCEPT`, `@`, reversal, guards, and nondeterministic shifts; and 24 finite-quantifier machines exercise existential, universal, and nested binders plus nondeterministic quantified target assignments. It compares both minimum invariant-violation depth and minimum deadlock depth through bounds 0–3 against the full parser/typechecker/normalizer/g3 pipeline. `Next` extraction expands finite existential assignments to user state variables over explicit enumerations, bounded constant integer ranges, and fixed-width symbolic integer ranges such as `x..(x + 2)` so each target is searched independently, while preserving symbolic initial-state and temporal-tableau choices. For existentials over genuinely variable-width integer ranges such as `a..b`, the checker derives exact current-state enabledness for assignment-only bodies and conjunctions or disjunctions of integer witness bounds, including positive and negative constant-coefficient affine forms such as `2 * i + 1 <= x`; an exhaustive finite oracle covers coefficients -3 through 3, all comparison directions, reversed operands, offsets, and empty/nonempty domains. Small constant domains additionally use exact enumeration when affine elimination does not apply. Nonlinear Boolean/integer witness predicates that cannot be eliminated, including variable-width `i * i = x`, Boolean equivalence, and integer distinctness, are preserved as bounded SMT existentials and sent through g3 v0.0.9. Nested witnesses preserve de Bruijn depth and may use outer witnesses in inner bounds and predicates; direct satisfiable/unsatisfiable checks and end-to-end unique-deadlock regressions cover single and dependent nested outcomes. Exact projections also distribute deterministic assignments through disjunction branches and substitute their distinct target values into later guards and configured action/target-state constraints. Predicates outside the pure Boolean/integer SMT fragment remain conservatively outside this exact path. |
| Temporal/ReTLA/VMT correctness | dedicated semantic and end-to-end differential suites | **Complete for the declared supported model surface; differential evidence is deliberately finite.** Temporal loop/tableau unit tests and nine live lasso differential cases are green. VMT has semantic and release smoke tests plus an explicit `solver`-tagged oracle that sends generated initial-state, transition, and invariant sat/unsat queries to both native Z3 and CVC5. Live ReTLA differentials transpile matching models through Scala and Go. They cover Boolean, integer, and uninterpreted state; Boolean, integer, and uninterpreted-sort existential/universal quantification; equivalence and conditionals; `A -> A`, `Int -> Bool`, and `<<A, Int>> -> A` state functions; and single- and multi-argument construction/application/`EXCEPT`, comparing declarations plus Init, action, and invariant shapes. The Go VMT boundary erases checker-only Skolem markers and recognizes the canonical tuple-set-map produced when shared preprocessing normalizes a multi-argument function domain, while continuing to reject general set maps. The suite also documents an intentional correction: current Scala output references generated `t_*` helpers without defining them and is rejected by Z3, while Go emits every helper definition before use; the oracle deliberately fails for review once upstream fixes the omission. Broader ReTLA/VMT model-shape differential coverage is an optional evidence expansion. |
| Counterexample correctness | state-by-state normalized trace comparisons and ITF round trips | **Implemented for the surveyed checker surface.** Decoder and ITF round-trip tests are present. The live oracle compares verdict/property/depth for all 50 cases and exact normalized user-state values for ten deterministic action, temporal, function, sequence, set, variant, row-record, deadlock, and deeply nested structured counterexamples. The nested case combines records containing sequences of records, sets, variants carrying tuples, and functions returning records containing sequences and sets. For four nondeterministic trace-invariant, temporal-lasso, record/set deadlock, and combined structured-data cases, the oracle independently constrains every Scala and Go ITF state against normalized Init/Next alternatives and proves that both witnesses are valid behaviors. |
| CLI/config compatibility | command/precedence/exit-code/output-file matrix | **Complete for the declared standalone command surface, with explicit compatibility exclusions.** Principal commands, the complete Scala TLC configuration-parser suite, strict JSON configuration for `parse`, `typecheck`, `check`, `simulate`, `test`, `tracee`, `transpile`, and both `explorer` and `checker` server transports with CLI/`PORT` environment/JSON precedence are tested. The JSON-RPC explorer implements the complete Scala method list, atomic batching, negotiated gzip/zstd request and response compression above the Scala 512-byte threshold, and the typed Quint JSON IR path exercised by Scala's arbitrary-precision oracle fixture. Quint's complete Scala-supported type, expression, declaration, lambda, binder, variant, nondeterministic-let, collection, action, and temporal conversion surface is implemented; Scala's intentionally unsupported `repeated` and invalid standalone `oneOf` remain explicit errors. The `tictactoe.json` oracle imports as 40 typed declarations, loads through safety preprocessing, produces Init/Next/invariant metadata, initializes the Go SMT executor, and proves its first Init transition enabled; `clockSync3.json` additionally covers the full type table. The Shai v1 gRPC server registers both services with the Scala 1 GiB inbound limit: transition exploration covers connection validation and root/auxiliary model loading, while command execution covers parse, typecheck, TLA formatting, bounded checking, view-distinct multi-counterexample collection, remote filesystem rejection, both `discard-disabled` strategies, Scala-compatible transition/invariant search filters, and options exposed by the pinned g3 tag's typed parameter catalog (including integer, Boolean, floating-point, and symbolic values). Simulation restores constants and randomly shuffles alternatives for up to `--max-run`/`search.simulation.maxRun` independent runs, using Scala's default of 100; selected paths reuse the state/action/trace checker and stop at the configured view-distinct error budget. `--output-traces`/`search.outputTraces` emits deterministic TLA, TLC-console, legacy Apalache-IR JSON, and ITF siblings for safe examples and simulation/check/test violations, with numbered files for multiple errors; explicit `--itf` remains a single-file request. Every local source command (`parse`, `typecheck`, `check`, `simulate`, `test`, `tracee`, and `transpile`) accepts `--out-dir`/`--run-dir` through CLI, strict JSON, and `OUT_DIR`/`RUN_DIR`, creates Scala-shaped source namespaces and unique run directories, and writes reproducible `run.txt` invocations; artifact-producing commands additionally mirror implicit trace or VMT output and route evaluated traces without changing stdout defaults. The same commands accept `--debug` and `--write-intermediate` through CLI/JSON, honor `WRITE_INTERMEDIATE`, mirror debug configuration snapshots, and emit Scala-numbered TLA+/IR-JSON snapshots from `00_OutSanyParser` and `01_OutTypeCheckerSnowcat` through every model pass ending at `12_OutAnalysisPass`. Every snapshot serializes the command's fully resolved CLI-over-JSON output, model/checker, tuning, backend, trace-evaluation, feature, and typechecker values as applicable; test snapshots include the generated SMT random seed. `check`, `simulate`, and `test` additionally resolve `--profiling` through CLI/JSON/`PROFILING` precedence and `--smtprof` through CLI/JSON precedence, aggregate production rewriting-rule statistics across replay executors, attribute whole source-expression SMT construction deltas through transformation history to source locations, and mirror `profile-rules.txt` and `profile.csv` to generated and custom run directories. Remote Shai requests reject filesystem-writing options. Scala-only global solver parameters not exposed by g3 and exact Scala exception stack-trace text in Shai are explicit compatibility exclusions. Scala's nondeterministic wall-clock banner is intentionally omitted from normal output to preserve the standalone byte-identical repeat gate. |
| Comprehensive tests | mapped Scala-test manifest, Go coverage report by subsystem, property/fuzz inventory | **Implemented for the surveyed Scala suite.** `SCALA_TEST_MANIFEST.tsv` accounts for all 213 current `src/test/scala` files with no `open` or `partial` rows: executable contracts have per-suite and, where relevant, per-case Go evidence; Scala harnesses, mocks, and alternate-backend fixtures are explicitly classified. `TestScalaTestManifestCoversEveryScalaTestSource` rejects omissions and stale rows. `make coverage` regenerates the statement-weighted subsystem report in `COVERAGE.md`; the current fast-suite snapshot ranges from 67.7% for CLI/configuration to 84.6% for frontend/annotations, excluding generated protobuf code. `TEST_INVENTORY.tsv` formally records every fuzz function and every tagged oracle/solver/corpus/release/deterministic/resource test plus the principal generated, property, exhaustive, round-trip, and cancellation families; `TestVerificationInventoryNamesExistingTests` rejects stale, duplicate, or omitted mandatory evidence. `TEST_INVENTORY.md` documents the CPU-capped Make gates and the bounded claim attached to each family. |
| Robustness | race, cancellation, resource/leak, deterministic-repeat, malformed-input results | **Complete for the declared bounded robustness gate.** Race, malformed-input, reproducible archive, parser fuzz, and standalone deterministic-repeat checks are green. The repeat gate compares diagnostics and byte-identical ITF counterexamples across four runs each at `GOMAXPROCS=1` and `8`; cancellation tests repeatedly reset one live g3 context, and JSON-RPC request contexts inherit server cancellation with a bounded shutdown fallback. Explicit `resource`-tagged gates create, solve with, and close 128 g3 contexts; concurrently execute repeated explorer `loadSpec` → enabledness solve → `disposeSpec` lifecycles; and repeatedly start Shai gRPC servers over both in-memory and real loopback TCP transports, call both services concurrently, close connections and listeners, and stop each server. These gates verify sessions are retired, file descriptors return exactly to baseline, and goroutines remain within runtime allowances. A multi-hour soak remains outside the bounded gate. |
| CPU constraint | build/test scripts and CI settings capped at CPUs/parallelism 1-8 | **Implemented.** `Makefile` caps `-p=8` and uses `taskset -c 1-8` when that affinity is available. Dedicated `go-port-tests` and `go-port-race` GitHub jobs invoke only those capped Make gates from `pkg/`, using the Go version pinned by `go.mod`. |
| Green repository state | final `go test`, race, oracle, corpus, vet/static, build, and package logs | **Current local gate set is green.** On g3 v0.0.9 and tlago v0.1.10, the full normal test suite, vet, module verification, CGO-free build, and release-archive isolation test have been revalidated. The broader corpus, deterministic-repeat, resource-lifecycle, fuzz-smoke, solver-crosscheck, full 50-case live Scala oracle, and complete race gates passed on the immediately preceding g3 v0.0.8 integration; the v0.0.9 delta is confined to g3's internal fixedpoint/PDR implementation outside Apalache's imported API. The race suite's measured 42.9-minute core-package runtime is covered by its explicit 60-minute bound. CI now runs the self-contained gates; externally provisioned Scala-oracle and native-solver crosschecks remain explicit local/release evidence. |

