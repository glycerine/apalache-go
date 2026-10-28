# Production dependency audit

This inventory covers the modules recorded in the standalone `apalache`
binary's Go build information. `release_integration_test.go` requires this
exact set and versions, so adding or changing a linked module requires an
explicit review here.

| Module | Pinned version | License evidence | Status |
| --- | --- | --- | --- |
| `github.com/glycerine/g3/pkg` | `v0.0.0-20261027115743-fe4e2f4f6dae` (repository tag `v0.0.5`) | The repository README says MIT and links `LICENSE.txt`, but that file is absent from both the repository and tagged module. | **Blocking: upstream license file required.** |
| `github.com/glycerine/tlago` | `v0.1.9` | `LICENSE`, MIT | Reviewed |
| `github.com/cespare/xxhash/v2` | `v2.3.0` | `LICENSE.txt`, MIT | Reviewed |
| `github.com/glycerine/insdict` | `v0.14.6` | `LICENSE`, MIT | Reviewed |
| `github.com/glycerine/rbtree` | `v0.2.2` | `LICENSE`, MIT | Reviewed |
| `golang.org/x/text` | `v0.28.0` | `LICENSE`, BSD-3-Clause | Reviewed |

The module also contains a staged Shai gRPC implementation that is not yet
imported by the production CLI and therefore does not change the linked module
set above. Building and testing `./shai/...` additionally uses these pinned
modules:

| Module | Pinned version | License evidence | Status |
| --- | --- | --- | --- |
| `google.golang.org/grpc` | `v1.67.1` | `LICENSE`, Apache-2.0 | Reviewed |
| `google.golang.org/protobuf` | `v1.34.2` | `LICENSE`, BSD-3-Clause | Reviewed |
| `google.golang.org/genproto/googleapis/rpc` | `v0.0.0-20240814211410-ddb44dafa142` | `LICENSE`, Apache-2.0 | Reviewed |
| `golang.org/x/net` | `v0.28.0` | `LICENSE`, BSD-3-Clause | Reviewed |
| `golang.org/x/sys` | `v0.24.0` | `LICENSE`, BSD-3-Clause | Reviewed |

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
GLPK comparison driver is present. The g3 solver assertion machinery does,
however, retain `diagnostic.(*DebugActionContext).InvokeDebugger` and its Linux
`hostRunCommand` branch, which executes `/bin/sh` if an interactive assertion
failure receives the `G` debugger command. This is not a solver delegation
path, but it is an external-process capability in the production call graph.

Reproduce the check from `pkg/` with an unstripped build and inspect these
symbols:

```sh
CGO_ENABLED=0 go build -trimpath -o /tmp/apalache-callgraph ./cmd/apalache
nm -an /tmp/apalache-callgraph | grep -E 'ValidateSanyXML|RunDriverGLPK|hostRunCommand|os/exec\.(Command|LookPath)'
```

Final hardening requires a tagged g3 API/profile that disables the interactive
debugger path, followed by updating the pinned dependency and rerunning this
audit. A local replacement is intentionally not used because release builds
must stay pinned to tagged g3 code.
