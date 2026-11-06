# Symbolic overflow equivalence performance, 2026-11-06

## Reproduction

Go runs all 44 cases, including signed 16-bit multiplication, without skips:

```
cd pkg
go test ./api -run '^TestNoOverflowSourceSymbolicEquivalences$' \
  -short -tags g3checked,g3debug,g3trace -v -count=1 \
  -cpuprofile=/tmp/g3-overflow.cpu -o /tmp/g3-overflow.test
go tool pprof -top /tmp/g3-overflow.test /tmp/g3-overflow.cpu
go tool pprof -top -cum /tmp/g3-overflow.test /tmp/g3-overflow.cpu
```

C++ checkout: `/home/jaten/go/src/github.com/Z3Prover/z3`, HEAD
`7976d53fb`, Release build:

```
./build/test-z3 no_overflow_symbolic_equiv /v:5
```

The C++ test runs the same widths, signedness and arithmetic equivalence cases.
Native time was 4.23 seconds with temporary per-case statistics prints (4.25
seconds in the initial measurement). Go's starting time was 75.1 seconds.

## Traced paths and source differences

Both checks push before asserting, so `combined_solver::check_sat` takes its
incremental solver branch. The Go CPU stacks confirm `SMTSolver` ->
`SMTKernel.search` -> Boolean propagation/conflict resolution, rather than the
SAT tactic. Both construct the ripple multiplier and its specialized signed
and unsigned overflow circuits.

Two newer C++ optimizations were absent from the Go port of the older source:

* Current `src/smt/theory_bv.cpp:438`, `add_fixed_eq`, increments a 256-entry
  byte activity table and emits the expensive per-bit equality axioms only
  when the old counter equals 255. Logical fixed-value equality propagation
  still happens on every notification. Go now uses that algorithm, including
  current `util/hash.h`'s combination for this table, byte wraparound, canonical
  pair order and marking emitted word equality relevant. The older hash
  consumers elsewhere in Go are unchanged.
* Current `theory_bv`'s constructor disables AND/OR flattening in its bit
  blaster. Go now applies the same constructor setting. General bit-blaster
  defaults are unchanged.

Temporary C++ prints in `test_equiv` and verbose Go statistics established the
following work counts for signed 16-bit multiplication overflow:

| Counter | C++ | Go before these changes | Go with source settings |
| --- | ---: | ---: | ---: |
| Boolean variables | 2,527 | 5,363 | 2,547 |
| Dynamic equality axiom sets | 1 | 700 | 2 |
| Conflicts | 45,077 | 65,520 | 66,205 |
| Decisions | 69,973 | 105,335 | 101,022 |
| Propagations | 12,856,350 | 20,234,941 | 19,056,962 |

The search counts differ; this is not evidence of identical search traces.
In particular, the Go scope machinery rebuilds learned-clause records more
frequently than native C++ (446,619 creations versus 53,653 in this case).
C++ keeps stable clause objects and reinitializes them when necessary. The
current changes do not eliminate that representation/lifetime difference.

## Profile-driven changes

The initial corrected-query profile spent 63% cumulatively in Boolean
propagation and 47% in relevance assignment events. Repeated manager locks,
AST metadata reads, descriptor decoding and expression-map lookups dominated.

The optimizations preserve source traversal and assignment order:

* A checked little-endian scalar arena read validates its descriptor once
  instead of once for each of four bytes. It exposes no borrowed storage.
* Boolean variables cache immutable operator, NOT-child and AND/OR argument
  metadata. Gate constraints also cache normalized child literals, whose
  variables precede the parent in scope order. Scope retirement discards the
  metadata with its owning variable.
* Assignment events use their known Boolean variable's scalar assignment and
  operator metadata. Standalone propagators keep their checked AST path.
  The context publication guard remains in place, including interrupted
  assignment recovery.
* Propagation validates each watched clause once per visit and traverses its
  contiguous literals directly. Public clause accessors still check lifetime
  and bounds.
* A learned atom can reuse its surviving literal when both expression and
  Boolean creation identity match. Reused slots and retired variables take
  the full internalization path. The arena continuation guard remains before
  cached accesses.

The signed overflow work counts above were unchanged across the low-level
optimizations. Intermediate full equivalence timings were 43.2 and 38.7 seconds
with CPU profiling enabled, compared with the original 75.1 seconds.

## Validation

New regressions cover scalar read byte order, unaligned offsets, pooled/direct
storage, bounds, stale generations and closed arenas; equality activity cadence,
byte wrap, reversed pairs, scope survival and logical equality propagation;
Boolean metadata retirement, reused variable slots, nongate NOT assignments,
signed gate child selection; and learned-literal creation identity.

Existing proof, equality-clause schedule, allocation-refusal, trace and retry
checks still exercise the full axiom-emission branch by setting the activity
counter to its emission frontier. Their assertions were preserved. The new
cadence tests separately cover the default non-emitting branch.

The affected arena, AST, bit-blaster and SMT tests pass with
`g3checked,g3debug,g3trace`.

Final integrated run with all three tags and CPU profiling enabled:

* All 44 equivalence cases: **37.25 seconds**, down from **75.1 seconds**
  (about **2.0x faster**).
* Signed 16-bit multiplication overflow: **18.75 seconds**.
* Signed 16-bit multiplication underflow: **15.19 seconds**.
* Entire API package: **52.808 seconds**, all tests green (previous integrated
  run before this performance work: 90.428 seconds without CPU profiling).
* Affected SMT regression families: **4.090 seconds**, green.
* Arena, AST and bit-blaster integration checks: green, including all three tags.

Native parity has not been reached: 37.25 seconds remains approximately 8.8x
slower than the 4.23-second native comparison. The remaining profile identifies
Boolean propagation/relevance expression-map traffic and Go learned-clause
scope reconstruction as the next costs. These results establish a measured
speedup, not a claim that the performance gap is fully closed.

## Follow-up: native CPU profile and relevance batch correction

A native perf recording revealed that C++ defers relevance assignment callbacks
until BCP and theory case splits complete. Go's immediate callbacks were a
remaining scheduling divergence; they are now deferred, with ordering, conflict,
retry, snapshot and scope regressions. The remeasured complete test takes
**33.59 seconds**. See [the side-by-side CPU comparison](OVERFLOW_PROFILE_COMPARISON.md)
for measured native/Go phase costs, commands and saved profile summaries.

## Fixed-equality regression correction

The modern-C++ BV fixed-equality activity throttle and added word-equality
relevance marking were incompatible with this port. The former made
`TestSMT2BitVectorParetoOptimizationEndToEnd` omit the feasible `(3,0)` point;
the latter changed the string model in
`sequence_word_equations_relations.smt2` from the native result. Both tests
passed before commit `7630b281b`, and isolated removal reproduced the cause
of each failure. Go now follows this repository's `src/smt/theory_bv.cpp`:
emit fixed-equality axioms eagerly, without the added relevance notification.
The timing/profile results above describe the earlier implementation and must
be measured again before using them as the performance baseline. The other
hot-path optimizations and deferred relevance assignment callbacks remain.

Validation after the correction: `cd pkg && go test ./... -count=1` passed all
109 tested packages without build tags; `cmd/g3` took 93.981 s,
`parsers/smt2` 6.208 s, and `smtcore` 23.309 s in that run. Their end-to-end
expected outputs were unchanged. Focused fixed-equality tests also cover eager
emission across scopes, core propagation, and absence of the added relevance
notification.
