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
