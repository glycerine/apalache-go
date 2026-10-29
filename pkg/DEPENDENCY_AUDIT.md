# Production dependency audit

This inventory covers the modules recorded in the standalone `apalache`
binary's Go build information. `release_integration_test.go` requires this
exact set and versions, so adding or changing a linked module requires an
explicit review here.

| Module | Pinned version | License evidence | Status |
| --- | --- | --- | --- |
| `github.com/glycerine/g3/pkg` | `v0.0.9` (module tag `pkg/v0.0.9`) | Tagged repository-root `LICENSE.txt`, Microsoft Z3 MIT license; reproduced in `third_party_licenses/g3-LICENSE.txt` and included in release archives. | Reviewed |
| `github.com/glycerine/tlago` | `v0.1.10` | `LICENSE`, MIT | Reviewed |
| `github.com/cespare/xxhash/v2` | `v2.3.0` | `LICENSE.txt`, MIT | Reviewed |
| `github.com/glycerine/insdict` | `v0.14.6` | `LICENSE`, MIT | Reviewed |
| `github.com/glycerine/rbtree` | `v0.2.2` | `LICENSE`, MIT | Reviewed |
| `github.com/klauspost/compress` | `v1.18.5` | root `LICENSE` zstd implementation terms, BSD-3-Clause | Reviewed |
| `golang.org/x/net` | `v0.28.0` | `LICENSE`, BSD-3-Clause | Reviewed |
| `golang.org/x/sys` | `v0.24.0` | `LICENSE`, BSD-3-Clause | Reviewed |
| `golang.org/x/text` | `v0.28.0` | `LICENSE`, BSD-3-Clause | Reviewed |
| `google.golang.org/grpc` | `v1.67.1` | `LICENSE`, Apache-2.0 | Reviewed |
| `google.golang.org/protobuf` | `v1.34.2` | `LICENSE`, BSD-3-Clause | Reviewed |
| `google.golang.org/genproto/googleapis/rpc` | `v0.0.0-20240814211410-ddb44dafa142` | `LICENSE`, Apache-2.0 | Reviewed |

The standard library is distributed under Go's BSD-style license. The Go
release build uses `CGO_ENABLED=0`, and the Linux release gate rejects imported
native libraries. It then runs parsing, typechecking, checking, trace, VMT,
diagnostic, and cancellation paths with an empty executable search path.

Some imported dependency packages contain optional external-process helpers
(`tlago` XML schema validation, and g3 diagnostic/LP utilities). The Go port
does not call those helpers. The scrubbed release test guards the exercised
production command surface against accidental solver delegation.

## Whole-program symbol audit

An unstripped `CGO_ENABLED=0` production binary was inspected with `nm` and
`objdump`. Link-time dead-code elimination removes `tlago.ValidateSanyXML` and
`g3/pkg/util/lp.RunDriverGLPK`, so neither the XML validator nor the optional
GLPK comparison driver is present. The g3 solver assertion machinery retains
`diagnostic.(*DebugActionContext).InvokeDebugger` and its Linux
`hostRunCommand` branch in the binary because both action profiles share one
method. Apalache binds g3 v0.0.9's `DebugNoDebugger` process action and disables
process assertions before constructing any solver context. Consequently an
unexpected direct assertion terminates with g3's noninteractive status instead
of reading stdin or reaching the shell branch. `smt_g3_hardening_test.go`
verifies both settings in an isolated subprocess, including input that would
make g3's default interactive action continue.

Reproduce the check from `pkg/` with an unstripped build and inspect these
symbols:

```sh
CGO_ENABLED=0 go build -trimpath -o /tmp/apalache-callgraph ./cmd/apalache
nm -an /tmp/apalache-callgraph | grep -E 'ValidateSanyXML|RunDriverGLPK|hostRunCommand|os/exec\.(Command|LookPath)'
```

The generic shell symbol is therefore presence-only rather than a reachable
Apalache runtime capability. The release remains pinned to tagged g3 code; a
local replacement is intentionally not used.
