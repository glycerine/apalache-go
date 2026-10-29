# Differential audit reproducers

These drivers reproduce the observations in [CODE_REVIEW.md](../../CODE_REVIEW.md).
They are deliberately outside the default Go test suite. They record disagreements
instead of asserting that buggy behavior is correct. The recorded results preserve
the original audit baseline; fixes and regression tests are listed in the report.

From the repository root, build current Scala:

```sh
taskset -c 1-8 sbt -batch 'Universal/stage'
```

From `pkg/`, build Go and the probe clients:

```sh
taskset -c 1-8 go build -mod=readonly -p=8 -o build/apalache ./cmd/apalache
taskset -c 1-8 go build -mod=readonly -p=8 -o build/audit_shai ./testdata/code_review/shai_client.go
taskset -c 1-8 go build -mod=readonly -p=8 -o build/audit_compression ./testdata/code_review/compression_client.go
```

From the repository root, run the drivers:

```sh
python3 pkg/testdata/code_review/cli.py
python3 pkg/testdata/code_review/jsonrpc.py > pkg/build/audit_rpc_current.json
python3 pkg/testdata/code_review/shai.py > pkg/build/audit_shai_current.json
```

The CLI driver writes full inputs/output to
`pkg/build/code_review_cli/results.json`. The server drivers print JSON to stdout
and put server logs under `pkg/build/`. They start local servers on unused ports
and terminate them in finally blocks. Python 3, Java, Go, and Linux taskset are
required. No Python packages are needed. Server probes require loopback sockets.

All drivers accept `APALACHE_SCALA` and `APALACHE_GO` environment variables.
Defaults are the freshly staged Scala executable and `pkg/build/apalache`.
All subprocesses are capped to CPUs 1–8, matching the audit environment.

To reproduce finding 5, from the repository root:

```sh
target/universal/stage/bin/apalache-mc typecheck pkg/testdata/code_review/BadTypedIR.json
pkg/build/apalache typecheck pkg/testdata/code_review/BadTypedIR.json
```

Both should now reject the intentionally inconsistent `1 + TRUE`. The original
audit recorded Go incorrectly accepting it.

The existing live oracle gate can be rerun from `pkg/`:

```sh
APALACHE_SCALA="$PWD/../target/universal/stage/bin/apalache-mc" \
APALACHE_GO="$PWD/build/apalache" \
taskset -c 1-8 go test -mod=readonly -p=8 -tags=oracle \
  -run '^(TestScalaDifferentialSafetyManifest|TestFrontendType.*MatchScalaExactly|TestReTLAVMTScalaOracle.*|TestUndefinedVMTGeneratedFunctions|Test.*DocumentsScalaUnderapproximation)$' \
  -count=1 -timeout=30m -v .
```

[recorded_results.json](recorded_results.json) preserves the audit revision,
binary hashes, CLI sources/exit statuses/output excerpts, and complete decoded
server responses. CLI entries include exploratory controls as well as confirmed
findings; not every difference is a new bug (see the report's coverage limits).
Regenerate observations after fixing issues instead of treating those recorded
failures as golden expected behavior.

[remediation_server_results.json](remediation_server_results.json) preserves the
post-fix live server comparison. The original `recorded_results.json` remains
unchanged as the pre-fix baseline. Parser messages and trace timestamp metadata
may differ even when the protocol shape and semantic result agree.
