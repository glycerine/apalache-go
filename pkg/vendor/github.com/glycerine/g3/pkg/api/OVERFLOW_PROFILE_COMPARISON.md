# Go and C++ overflow CPU profiles, 2026-11-06

The native measurements below used a separate modern Z3 checkout, not this
project's C++ fork. They are historical reference measurements, not a matching
source baseline for this port. Project C++ benchmarks and positive controls
belong under this repository's `src/` tree. The matching scope-restoration
benchmark is now `src/test/smt_scope_restoration.cpp`.

These measurements predate the fixed-equality regression correction described
in [OVERFLOW_PERFORMANCE.md](OVERFLOW_PERFORMANCE.md#fixed-equality-regression-correction).
The BV heuristic used here has been removed; new timings are required before
treating this report as the current Go performance baseline.

Both runs execute the 44 symbolic equivalence cases, without skips. Native C++
uses the Release `test-z3 no_overflow_symbolic_equiv` test; Go uses
`TestNoOverflowSourceSymbolicEquivalences` with
`-short -tags g3checked,g3debug,g3trace -count=1`.

## Measurements after correcting relevance callback scheduling

| Measurement | C++ | Go |
| --- | ---: | ---: |
| Test wall time | 4.29 s | 33.59 s |
| Sampled CPU time | 4.269 s | 38.66 s |
| Sampling tool | perf, cpu-clock:u, 499 Hz | go tool pprof, 100 Hz |
| Samples | 2,130 | 3,866 |

Go CPU time exceeds wall time because background GC uses additional threads.
The wall times above are the test's own timing, excluding compilation and
perf's recording/serialization overhead. The external perf wrapper elapsed
4.92 seconds. These are individual profiled runs, not statistical benchmarks.

The native binary is the existing unstripped `-O3 -DNDEBUG` build at
`/home/jaten/go/src/github.com/Z3Prover/z3/build/test-z3`.

## Side-by-side CPU costs

All entries below are **inclusive**: they include callees, overlap with other
rows, and must not be added. Native times are estimates from sampled periods.

| Solver activity | C++ CPU seconds (% of CPU) | Go CPU seconds (% of CPU) |
| --- | ---: | ---: |
| Propagation, including relevance/theory phases | 2.76 (64.6%) | 16.88 (43.7%) |
| Boolean propagation | 2.22 (52.0%) | 12.48 (32.3%) |
| Relevance assignment callbacks | 0.36 (8.4%) | 3.85 (10.0%) |
| Conflict resolution, including minimization | 0.92 (21.6%) | 4.28 (11.1%) |
| Backtracking / scope restoration | 0.24 (5.6%) | 7.35 (19.0%) |
| Background garbage collection | No equivalent GC phase | 4.01 (10.4%) |

Function mappings:

| Activity | C++ | Go |
| --- | --- | --- |
| Propagation | `context::propagate` | `SMTInternalizerState.PropagateCore` |
| Boolean propagation | `context::bcp` | `SMTInternalizerState.Propagate` |
| Relevance callbacks | `relevancy_propagator_imp::assign_eh` | `SMTRelevancyPropagator.assignmentEvent` |
| Conflict resolution | `conflict_resolution::resolve` | `SMTConflictResolver.resolve` |
| Scope restoration | `context::pop_scope_core` | `SMTInternalizerState.PopScopes` |

These are similar phases, not identical instruction sequences or search
traces. The existing work-count comparison in [OVERFLOW_PERFORMANCE.md](OVERFLOW_PERFORMANCE.md)
records the remaining differences in variable, conflict and clause counts.

## Structural divergence discovered and corrected

Current C++ `src/smt/smt_context.cpp:1840`, `context::propagate`, runs:

1. Boolean constraint propagation (`bcp`).
2. Theory case splits.
3. Relevance assignment callbacks for the completed batch, followed by
   propagation of relevance marks.
4. Assigned-atom, equality, theory and quantifier propagation.

`context::assign_core` publishes assignments without delivering relevance
callbacks. Previously Go delivered those callbacks synchronously from
`assignLiteralCore` and from restored learned-assignment completion. A callback
could therefore see only part of the Boolean batch, or run for a batch that
subsequently hit a Boolean conflict.

Go now schedules those events after successful BCP and theory case splits.
The delivery bound is captured once, matching C++: assignments made by callbacks
wait for a later BCP batch. The Go batch cursor also preserves completed work
across retryable failures and is restored with its scope.

Three new fast regressions first failed on the old implementation: early
assignment delivery, wrong OR-child selection from a partial batch, and
callbacks delivered despite a Boolean conflict. Additional tests cover scope
restoration, interrupted delivery after the BCP queue has advanced, learned
assignment continuation, callback-generated assignments, and discarding a
pending batch on pop. Existing test assertions were preserved.

Full checked/debug/trace SMT package: **52.784 seconds**, green.
Full checked/debug/trace API package: **48.943 seconds**, green.

## Before/after the scheduling correction

| Measurement | Immediate callbacks | Post-BCP callbacks |
| --- | ---: | ---: |
| Complete test wall time | 37.33 s | 33.59 s |
| Sampled CPU time | 42.14 s | 38.66 s |
| Callback CPU time | 8.96 s | 3.85 s |
| Callback share of CPU | 21.3% | 10.0% |

The overflow query's recorded search counts did not change in this comparison:
66,205 conflicts, 101,022 decisions, 19,056,962 propagations. The OR regression
shows that the scheduling distinction does affect behavior on other formulas.

## Largest remaining costs

* Scope restoration is about 31 times the native sampled cost. Go rebuilds
  learned-clause records and repairs their literal bindings across scope pops;
  C++ normally keeps surviving clause objects. Go's profile includes 2.37 CPU
  seconds in `restoreLearnedClauses`, as well as capture, release and scope
  bookkeeping.
* Relevance callbacks are about 11 times the native sampled cost even after
  batching. Go still performs expression-map lookups and checked handle access
  where C++ uses direct expression/variable access.
* Go background GC consumes about 4 CPU seconds. Native allocation/free costs
  are included in their calling phases; the absence of a GC phase does not
  mean C++ memory management is free.

The native and Go flat profiles cannot be compared by leaf name alone: native
optimization and Go inline attribution place helper costs in different symbols.
Inclusive phase comparisons above are more informative.

## Saved summaries and reproduction

* [Native flat profile](profiles/overflow_cpp_flat.txt)
* [Native inclusive profile](profiles/overflow_cpp_inclusive.txt)
* [Go flat profile](profiles/overflow_go_flat.txt)
* [Go inclusive profile](profiles/overflow_go_inclusive.txt)

```
# /usr/bin/perf is a wrapper with no tool matching this machine's kernel.
# The installed 6.17 perf executable works with the running kernel's ABI.
/usr/lib/linux-tools/6.17.0-35-generic/perf record \
  -e cpu-clock:u -F 499 --call-graph dwarf,16384 \
  -o /tmp/g3-overflow-cpp.perf -- \
  /home/jaten/go/src/github.com/Z3Prover/z3/build/test-z3 \
  no_overflow_symbolic_equiv
/usr/lib/linux-tools/6.17.0-35-generic/perf report \
  -i /tmp/g3-overflow-cpp.perf --stdio --children --call-graph none

cd pkg
go test ./api -run '^TestNoOverflowSourceSymbolicEquivalences$' \
  -short -tags g3checked,g3debug,g3trace -count=1 \
  -cpuprofile=/tmp/g3-overflow-batched.cpu \
  -o /tmp/g3-overflow-batched.test
go tool pprof -top -cum \
  /tmp/g3-overflow-batched.test /tmp/g3-overflow-batched.cpu
```

Binary/raw profiles are in `/tmp`; the small text summaries are stored here.
