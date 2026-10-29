# Go port static code review

## Fix follow-up

All six findings below have been fixed in review order using a focused failing regression followed by its fix and a passing rerun. The original findings and their original line references are retained below as review history; code movement means those line numbers are not current fix locations.

| Finding | Observed red result | Regression now passing |
| --- | --- | --- |
| 1: simulation unknown | An injected inconclusive invariant check produced two completed runs. | `TestReviewSimulationPreservesUnknown`: marks the run unknown and stops. |
| 2: view exclusions | Tuple, sequence, record, set, and string values failed ground encoding on the second query. | `TestReviewStructuredViewExclusions`: distinct views and exhausted constant views, across all three SMT encodings. |
| 3: action bound | Bounds 0 and 2 reported action violations at depths 1 and 3. | `TestReviewActionInvariantRespectsBound`: no out-of-bound action checks, while the same violations are found when the bound includes them. |
| 4: cross-run exclusions | Duplicate initial and action-path views consumed all five error slots. | `TestReviewSimulationExcludesViewsAcrossRuns`: persistent exclusions during both transition selection and invariant checking. |
| 5: trace filtering | Both state-only and action-only filters suppressed a violated trace invariant. | `TestReviewTraceInvariantsIgnoreStateActionFilter`: trace checking is independent of those filters. |
| 6: empty Init | Checker, CLI, and remote service reported success for contradictory Init. | `TestReviewInitialStateExhaustion`, `TestReviewCheckUnsatisfiableInit`, and `TestReviewRemoteCheckUnsatisfiableInit`: distinct exhaustion outcomes and no fabricated traces. |

Regressions live in [code_review_regression_test.go](code_review_regression_test.go), [CLI regressions](cmd/apalache/code_review_regression_test.go), and [remote regressions](shai/code_review_regression_test.go).

For finding 6, `SafetyExecutionsTooShort` maps to Scala's `ExecutionsTooShort` when deadlock checking is disabled. With it enabled, an empty initial state space maps to `Deadlock` with no user-state trace. The CLI retains Scala's successful exit classification for `ExecutionsTooShort`, but reports the exhaustion explicitly rather than claiming a completed bounded search. The existing final-state deadlock-probing policy is retained; finding 3 separates that probe from action-invariant checking.

Full-suite validation was deferred until all six focused red/green cycles completed. The first full run found one legacy assertion, `TestCheckBoundedSafetySkipsUnsatisfiablePaths`, that explicitly expected the erroneous empty-Init success from finding 6. That assertion now expects initial deadlock, also checks the deadlock-disabled exhaustion outcome, and passes with all review regressions. The final `taskset -c 1-8 go test -p=8 ./... -count=1` passed in every package (core package: 212.305 seconds). `go vet -p=8 ./...` and `go build -p=8 ./...`, also CPU-capped, passed. No live Scala differential or full race run has been performed in this fixing pass.

## Scope and evidence

This review compares the current Go source with the Scala source in this checkout. It concentrates on bounded checking, simulation, invariant selection, counterexample enumeration, and the transition-executor/SMT boundary. Supporting inspection covered the executor decorators, selected collection/choice rules, trace-evaluation pass, CLI callers, and related test source.

During the original static review, no builds, tests, solvers, application commands, or other validation programs were run. The findings below originally followed from source-level control-flow and data-flow analysis; subsequent executed regressions and fixes are recorded above. This is a focused review, not a claim that every file in `pkg/` or every Scala behavior has been audited.

Severity: P1 means a correctness issue that should be prioritized; P2 means an actionable functional bug or compatibility divergence.

## 1. [P1] Simulation discards an inconclusive invariant-check result

**Go:** [simulator.go](simulator.go), lines 135–153; [bounded_checker.go](bounded_checker.go), lines 733–737; [cmd/apalache/main.go](cmd/apalache/main.go), lines 1107–1111 and 1137–1147.

**Scala:** [SeqModelChecker.scala](../tla-bmcmt/src/main/scala/at/forsyte/apalache/tla/bmcmt/SeqModelChecker.scala), lines 384–388 and 434–438; [SearchState.scala](../tla-bmcmt/src/main/scala/at/forsyte/apalache/tla/bmcmt/search/SearchState.scala), lines 60–67 and 115–118.

`SimulateAndCheckWithOptions` first obtains a simulation run and appends it to `report.Runs`. It then calls `CheckBoundedSafetyWithOptions` to check the selected transition path. It handles `checkErr` and copies `checked.Counterexamples`, but never reads `checked.Outcome`.

The bounded checker represents an SMT `unknown` without a Go error: `stopOnUnknown` sets `SafetyUnknown` and returns normally. If path generation succeeded but an invariant query is inconclusive, the run therefore retains `SimulationCompleted`. With no counterexamples, the CLI prints that simulation completed and returns `exitOK`; its unknown check only examines `report.Runs`.

Scala records invariant-query timeouts in `SearchState`, and its final result is `SmtTimeout` when there are no established violations. Continuing after a timeout does not erase incompleteness.

**Trigger / proposed regression:** A satisfiable simulation path whose invariant query returns `SMTUnknown, nil` (for example, a difficult invariant under a solver resource limit). A deterministic test should inject this result at the checking boundary, assert that no successful completion is reported, and separately verify precedence when a real counterexample has already been found.

**Fix direction:** Propagate the checking outcome into the simulation report and CLI result, retaining incompleteness across runs while preserving established counterexamples.

## 2. [P2] Structured or string views fail when excluding the first counterexample

**Go:** [transition_executor.go](transition_executor.go), lines 734–761 and 783–786; [smt_ground.go](smt_ground.go), lines 297–324, 407–423, and 473–474; [model_decoder.go](model_decoder.go), lines 230–243 and 246–268.

**Scala:** [SeqModelChecker.scala](../tla-bmcmt/src/main/scala/at/forsyte/apalache/tla/bmcmt/SeqModelChecker.scala), lines 485–510; [ConstrainedTransitionExecutor.scala](../tla-bmcmt/src/main/scala/at/forsyte/apalache/tla/bmcmt/trex/ConstrainedTransitionExecutor.scala), lines 64–85.

`checkAndDecodeDistinctView` decodes each view to a TLA expression. On the next search iteration it builds `EQ(viewCell.NameExpr(), prior[index])` and passes that expression directly to `executor.solve` as a temporary ground assertion. The prior value is never passed back through the symbolic rewriter.

This works for integer and Boolean literals, which the ground encoder accepts directly. A tuple or sequence is decoded as a `TUPLE` constructor, a record as `RECORD`, and a set as `SET_ENUM`; the ground encoder does not support these constructors. Strings also fall through its unsupported ground-value branch. Consequently a valid non-scalar view can produce the first witness and then fail with an encoding error when searching for another. The public bounded-checking function returns an error rather than returning the collected result.

Scala constructs a TLA inequality against the decoded view, substitutes the path bindings, and sends it through `trex.assertState`, so ordinary rewriting handles the composite value and its equality before SMT encoding.

**Trigger / proposed regression:** Adapt `TestCheckBoundedSafetyEnumeratesDistinctViewsWithinOneSymbolicPath` in [bounded_checker_test.go](bounded_checker_test.go): keep `Init == x \in {1, 2, 3}`, `Inv == x = 0`, and change `View == x` to `View == <<x, x>>`, requesting two errors. Repeat with record, set, and string views. Failure occurs on the second query even if only one distinct witness exists.

**Fix direction:** Rewrite decoded view values and their equality/disequality in the current arena, using the same structural equality machinery as other TLA expressions, before constructing ground assertions.

## 3. [P2] Deadlock checking causes action-invariant checks beyond the requested bound

**Go:** [bounded_checker.go](bounded_checker.go), lines 548–557, 580–611.

**Scala:** [SeqModelChecker.scala](../tla-bmcmt/src/main/scala/at/forsyte/apalache/tla/bmcmt/SeqModelChecker.scala), lines 61–68 and 236–247; [TransitionExecutorImpl.scala](../tla-bmcmt/src/main/scala/at/forsyte/apalache/tla/bmcmt/trex/TransitionExecutorImpl.scala), lines 322–334.

At `len(path.next) == inputs.maxSteps`, Go returns early only when deadlock checking is disabled. Otherwise it prepares outgoing transitions to test enabledness, but also runs the action-invariant loop on those transitions. The child has depth `maxSteps + 1`, which can be returned as the counterexample depth even though it lies outside the requested bounded search. The child-enqueue guard occurs after the invariant check and cannot prevent this.

Scala applies `Init`, increments the executor step, and applies `Next` only while `stepNo <= stepsBound`. It does not check an extra action after the final bounded state. In particular, bound zero does not execute a Next action.

**Trigger / proposed regression:** `Init == x = 0`, `Next == x' = x + 1`, action invariant `Inv == x' = x`, with `MaxSteps: 0, CheckDeadlocks: true`. Go reaches the violating action and reports depth 1. With deadlock checking disabled it stops at depth 0. Also test a nonzero bound with the first violation exactly one transition beyond it.

**Fix direction:** Separate final-state enabledness probing from invariant checking and bound action checks independently. Review final-state deadlock reporting as a separate compatibility policy: Go also probes deadlock at the last bounded state, whereas Scala stops after generating it.

## 4. [P2] Simulation resets view exclusions between runs and counts duplicate errors

**Go:** [simulator.go](simulator.go), lines 128–150; [bounded_checker.go](bounded_checker.go), lines 231–248 and 673–683; [transition_executor.go](transition_executor.go), lines 745–759.

**Scala:** [SeqModelChecker.scala](../tla-bmcmt/src/main/scala/at/forsyte/apalache/tla/bmcmt/SeqModelChecker.scala), lines 34–35, 74–79, and 485–510; [ConstrainedTransitionExecutor.scala](../tla-bmcmt/src/main/scala/at/forsyte/apalache/tla/bmcmt/trex/ConstrainedTransitionExecutor.scala), lines 25–38 and 53–61.

Each simulated run invokes a fresh `CheckBoundedSafetyWithOptions`. It receives only the remaining error budget, not the previously reported views. That invocation initializes an empty `BoundedSafetyResult`, so its exclusion list contains only witnesses found during that invocation. Appending its counterexamples to the outer report does not deduplicate them.

Scala keeps one `ConstrainedTransitionExecutor` across runs. Its path-constraint map survives recovery of the underlying executor to the constants snapshot, and the constraints are asserted again when transitions are selected. Previously excluded path views therefore remain excluded across runs.

**Trigger / proposed regression:** `Init == x = 0`, `Next == x' = x`, `Inv == x = 1`, `View == x`, with `MaxSteps: 0, MaxRuns: 2, MaxErrors: 2`. The first Go checking call finds the only distinct view and exhausts it. The second call starts with no exclusions, reports the same view again, and consumes the second error slot. No random branching is needed. The existing simulation budget test finds all requested errors in its first run and does not cover this case.

**Fix direction:** Retain view exclusions for the whole simulation search and apply them while selecting and checking subsequent runs. Deduplicating only the output would still let repeated witnesses waste runs or prematurely consume the error budget.

## 5. [P2] A state-invariant filter silently suppresses trace invariants

**Go:** [bounded_checker.go](bounded_checker.go), lines 501–508; [filtered_transition_executor.go](filtered_transition_executor.go), lines 67–82.

**Scala:** [SeqModelChecker.scala](../tla-bmcmt/src/main/scala/at/forsyte/apalache/tla/bmcmt/SeqModelChecker.scala), lines 177–178 and 398–419; [FilteredTransitionExecutor.scala](../tla-bmcmt/src/main/scala/at/forsyte/apalache/tla/bmcmt/trex/FilteredTransitionExecutor.scala), `mayChangeAssertion`.

Go runs trace invariants through `allowsInvariant`, assigning them synthetic state-invariant numbers starting at `len(inputs.stateInvariants)`. Scala's trace-invariant loop directly iterates over every trace invariant; it does not call the executor's filtered `mayChangeAssertion` method. That filter applies to state/action invariants only.

Consequently the same tuning string can silently remove a trace property from the Go search even though Scala still checks it. The standalone Go decorator resembles Scala, but the production bounded checker adds this different filtering behavior outside the decorator.

**Trigger / proposed regression:** Configure one ordinary state invariant `x >= 0` and a trace invariant `HistInv(h) == Len(h) = 0`, with a correctly annotated history parameter. Use `Init == x = 0`, stuttering Next, and `search.invariantFilter = ".*->state0"`. Scala still detects the trace violation on the singleton initial history. Go labels it `state1` and skips it. A filter accepting only action invariants likewise suppresses all Go trace invariants.

**Fix direction:** Match Scala by checking trace invariants independently of the state/action filter, or explicitly introduce and document a separate trace-filter contract rather than silently reusing state indices.

## 6. [P2] No satisfiable initial state is reported as successful bounded checking

**Go:** [bounded_checker.go](bounded_checker.go), lines 231, 250–269, and 453–470.

**Scala:** [SeqModelChecker.scala](../tla-bmcmt/src/main/scala/at/forsyte/apalache/tla/bmcmt/SeqModelChecker.scala), lines 61–63, 224–280, and 295–312.

The Go result starts as `SafetyNoCounterexample`. An unsatisfiable replayed initial transition simply returns `false, nil`; a disabled one is similarly skipped. If all initial transitions are unsatisfiable, the queue empties and the unchanged success outcome is returned, even when `CheckDeadlocks` is true. No branch records that the search never reached an initial state.

Scala uses `makeStep` for Init as well as Next. With disabled-transition checking enabled, an empty set of prepared initial transitions takes the explicit `Deadlock` branch when deadlock checking is enabled, or `ExecutionsTooShort` otherwise. It does not return an ordinary successful bounded run in this situation.

**Trigger / proposed regression:** Use a normalized initial transition equivalent to `Init == (x = 0) /\ (x > 0)`, with a complete assignment and a contradictory guard, plus `Next == x' = x`. Enable deadlock checking. Verify the zero-initial-state outcome separately from normal termination after an actual reachable state, and from a transition filter intentionally removing all initial transitions.

**Fix direction:** Track whether any initial transition is satisfiable and map initial-state exhaustion to the Scala-compatible outcome. If vacuous success is an intentional port policy, document it as a compatibility difference and expose it distinctly to callers.

## Coverage notes for follow-up

The existing multi-error tests inspected use integer views; the simulation error-budget test stops within one run. Those cases do not exercise findings 2 and 4. Ordinary bounded-counter tests predominantly use state invariants and do not establish the action-invariant boundary in finding 3. A future fixing pass should add the targeted regressions above and run them against both implementations where practical.

The selected finite CHOOSE/Guess and default OOPSLA19 SetAsFun paths inspected did not establish an additional independent bug. In particular, nondeterministic CHOOSE is explicitly shared with Scala and is not reported here as a port divergence. Parsing, full type inference, all rewriting rules, temporal checking, and server transports have not been exhaustively reviewed by this pass.
