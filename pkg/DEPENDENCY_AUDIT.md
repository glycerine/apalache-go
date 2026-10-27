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

The standard library is distributed under Go's BSD-style license. The Go
release build uses `CGO_ENABLED=0`, and the Linux release gate rejects imported
native libraries. It then runs parsing, typechecking, checking, trace, VMT,
diagnostic, and cancellation paths with an empty executable search path.

Some imported dependency packages contain optional external-process helpers
(`tlago` XML schema validation, and g3 diagnostic/LP utilities). The Go port
does not call those helpers. The scrubbed release test guards the exercised
production command surface against accidental delegation; a recorded
whole-program call-graph artifact is still required for final completion.
