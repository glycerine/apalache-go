# Scala differential oracle

Build the standalone Go checker and a Scala distribution, then run the
manifest from `pkg/`:

```sh
go build -o /tmp/apalache-go ./cmd/apalache
go run ./cmd/apalache-oracle \
  -manifest testdata/oracle/safety.json \
  -scala ../target/universal/apalache-0.62.3-SNAPSHOT/bin/apalache-mc \
  -go /tmp/apalache-go
```

The oracle command compares normalized semantic outcomes, not logs or timing.
Scala and Go receive separate argument vectors because their flag syntax and
diagnostic formatting differ. Production checker packages never import this
test-only runner.

The same manifest is available as an opt-in Go test gate:

```sh
APALACHE_SCALA=../target/universal/apalache-0.62.3-SNAPSHOT/bin/apalache-mc \
APALACHE_GO=/tmp/apalache-go \
go test -tags=oracle -run TestScalaDifferentialSafetyManifest .
```
