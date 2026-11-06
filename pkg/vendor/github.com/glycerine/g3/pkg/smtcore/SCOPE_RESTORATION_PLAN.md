# Source-faithful SMT scope restoration

Reference: this repository's `src/smt/smt_context.cpp` (`pop_scope_core`,
`reinit_clauses`) and `src/smt/smt_internalizer.cpp` (`mk_clause`).
Do not modify the sealed `pkg/muz/FIXED_POINT_FINISH.md`.

## Plan

- [x] Establish fast Go-only regressions, first red, for clause identity,
  unchanged surviving watches, literal rebinding, and source relevance after
  backtracking. Expected behavior is stated in Go; C++ is not needed to run it.
- [x] Keep learned clause objects, proof owners, activity, and deletion handlers
  alive across search backtracks. Leave clauses whose atoms survive untouched.
  Delete auxiliary clauses and all clauses crossing a user base as before.
- [x] Reinitialize only affected clauses in place: preserve their expression
  roots, remove obsolete watches, internalize retired atoms (including nongate
  NOT atoms), bind literals, select both watches, and reassert units/conflicts.
  Do not restore relevance merely because the expression was relevant before
  the pop. Preserve retry cursors and close pending owners on abandonment.
- [x] Cover nested/mixed scopes, moved watches, unit/conflict replay, proof and
  deletion ownership, allocation refusal/retry, and cleanup. Existing refusal
  tests must keep testing actual failure boundaries and ownership guarantees.
- [x] Align both benchmark contracts and strengthen their postconditions;
  rerun `make cppbench && make bench`, affected package suites, and one complete
  native suite before finalizing. Record measurements and commit in small
  validated chunks.

## Baseline

`make cppbench && make bench` passed before these changes. C++ Release / Go
untagged: SurvivingAtoms 466.14 / 101614 ns/op (218x); ScopedAtoms 12504.34 /
357211 ns/op (28.6x). Both use 64 three-literal clauses and 128 atoms. The
scoped baseline was not fully equivalent: C++ left zero atoms relevant after
reinitialization, while Go restored all 128. Temporary C++ instrumentation
confirmed this and was removed. The Go benchmark also asserted the extra Go
behavior; correct the implementation and that assertion together.

## Implementation progress

The new `TestSMTScopeRestorationSourceContract` was first run red for both
cases, detecting replacement of the learned clause object. It now passes.
Go-only regressions also cover stable auxiliary-unit replay, scoped unit and
conflict replay, moved watches, nested scopes with auxiliary deletion, nongate
NOT variables, proof identity/reference counts, deletion handlers, and actual
native AST refusal during retirement/rebinding followed by retry or Close.

Learned clauses now acquire their scoped expression roots at construction,
like C++ `clause::mk`; simplification/collection respect their reinitialization
flags. Search pops keep objects and proof owners, detach only affected watches,
and rebind the existing literal storage. At the search base they release the
saved atom roots and clear reinitialization flags. Untouched learned clauses
take no snapshot and keep watch order. Auxiliary lemmas that propagated at a
temporary scope are rechecked without rebuilding surviving bindings.

The preexisting explicit snapshot/restore helpers and their ownership/refusal
tests still work; normal search pops now use the source in-place path. No
failure guard, proof test, or end-to-end expected result was removed.

The source atom owner keeps original clause order even when watch selection
swaps live literals. A Go-only regression checks this order before a two-scope
pop and checks Boolean variable creation order after reinitialization.

## Validation and measurements

- Focused restoration, learned-clause, conflict, proof, and cleanup groups pass
  both natively and with `g3checked,g3debug,g3trace`.
- The first native integration run found only one failed package (`cmd/g3`):
  a fixed work-budget fixture's conservative-phase incumbent changed from
  cost 3 directly to cost 1, skipping its old cost-2 checkpoint. Its original
  strict `unknown [0:2]` assertion is preserved under explicit seeded phases;
  a default-phase subtest additionally requires the tighter `unknown [0:1]`.
  The existing MaxRes-core cancellation test is unchanged. The input has
  optimum 1: at least one variable must be false and making only x1 false
  achieves cost 1. The source-fork `z3` already solves it at limit 220, so
  the C++ and Go rlimit counters do not select matching checkpoints.
- `make cppbench && make bench` passes with matched clause identity and
  relevance postconditions. C++ Release / untagged Go, ns/op:

  | Case | C++ | Go | Go/C++ |
  | --- | ---: | ---: | ---: |
  | SurvivingAtoms | 474.75 | 7340 | 15.46x |
  | ScopedAtoms | 12376.40 | 193571 | 15.64x |

  Surviving allocations fell from 328 to 1 per operation (64 bytes); scoped
  allocations fell from 456 to 136. Setup and cleanup remain outside the
  timers. Go still scans its unified clause records during a pop; C++ stores
  auxiliary clauses and lemmas separately. These are isolated backtrack
  measurements, not a claim of complete solver performance parity.
- Final `cd pkg && go test ./... -count=1` passed all 109 tested packages:
  `api` 45.117 s, `cmd/g3` 65.182 s, `parsers/smt2` 6.507 s, and `smtcore`
  23.545 s. Final Go-only source-contract checks also pass with all three
  diagnostic tags. The isolated benchmarks require no C++ executable for
  their Go correctness assertions.
