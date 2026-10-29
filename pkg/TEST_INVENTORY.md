# Verification inventory

`TEST_INVENTORY.tsv` is the authoritative inventory of fuzz, generated,
property, exhaustive, differential, cross-solver, corpus, release,
determinism, cancellation, and resource-lifecycle checks. Each row records the
Make gate, exact Go test functions, the property established, and its known
limit. `TestVerificationInventoryNamesExistingTests` rejects duplicate or stale
evidence names and requires every Go fuzz function and every test behind the
`oracle`, `solver`, `corpus`, `release`, `deterministic`, or `resource` build
tags to appear in the inventory.

The standard commands are:

```sh
make test
make test-race
make test-fuzz-smoke
make test-oracle
make test-solver
make test-corpus
make test-release
make test-deterministic
make test-resource
```

Every target caps Go package parallelism at eight and uses logical CPUs 1–8
when `taskset` is available. Tagged gates are kept separate because some need
external Scala/solver installations or intentionally expensive lifecycle and
packaging work. The race gate uses a configurable 60-minute timeout because
instrumenting the exhaustive checker families exceeds Go's 10-minute default
and has measured beyond 30 minutes on the capped eight-CPU configuration.
Passing a bounded property test proves only the guarantee and range stated in
its inventory row; it is not evidence for unbounded input spaces.
