# Third-party code and licenses

`go-infra` itself is distributed under the MIT license (see [LICENSE](./LICENSE)).
Some packages are derived from third-party open-source code and remain subject
to their original licenses. The original copyright notices are retained in the
source file headers, and full license texts are shipped in [`LICENSES/`](./LICENSES).

| Upstream | License | Original copyright | Derived files in this repository |
|---|---|---|---|
| [panjf2000/gnet](https://github.com/panjf2000/gnet) | Apache-2.0 | Copyright (c) 2021 The Gnet Authors; Copyright (c) 2019-present Andy Pan; Copyright (c) 2017 Joshua J Baker | `buffer/ring/`, `buffer/ringpool/`, `buffer/elastic/`, `buffer/linkedlist/`, `bytepool/`, `bytesconv/`, `mathutil/`, `queue/msqueue.go` |
| [sourcegraph/conc](https://github.com/sourcegraph/conc) | MIT | Copyright (c) 2023 Sourcegraph | `structured/` (`waitgroup.go`, `panics.go`, `try.go`, `pool.go`, `error_pool.go`, `context_pool.go`, `result_*.go`, `iter.go`, `map.go`, `stream.go`) |
| [go-pkgz/pool](https://github.com/go-pkgz/pool) | MIT | Copyright (c) 2025 Umputun | `structured/collector.go`, `structured/retry.go` |

## License texts

- [`LICENSES/Apache-2.0.txt`](./LICENSES/Apache-2.0.txt) — Apache License 2.0 (gnet)
- [`LICENSES/MIT-Sourcegraph.txt`](./LICENSES/MIT-Sourcegraph.txt) — MIT, Sourcegraph
- [`LICENSES/MIT-Umputun.txt`](./LICENSES/MIT-Umputun.txt) — MIT, Umputun

## Modifications

All derived files carry a "Modified by qoderwork" notice as required by
Apache-2.0 §4(b). Typical changes:

- **gnet**: packages renamed, comments rewritten in English, internal imports
  retargeted to `go-infra` packages (`bytepool`, `buffer/ring`, `mathutil`),
  and the lock-free queue rewritten as a generic `MSQueue[T]` using
  `atomic.Pointer` instead of `unsafe.Pointer` plus gnet's `*Task` type.
- **sourcegraph/conc**: the five upstream packages (`conc`, `panics`, `pool`,
  `iter`, `stream`) were merged into one `structured` package; `pool.New()` and
  `stream.New()` were renamed to `NewPool()` / `NewStream()` to avoid a
  collision; imports were retargeted to `github.com/qoderwork/go-infra`.
- **go-pkgz/pool**: `Close()` is idempotent; `Collector.Iter()` now reports a
  canceled context even when the channel is already closed (the upstream
  `select` could pick the closed-channel case and silently end the iteration);
  the `Retry` middleware's exponential-backoff-with-jitter algorithm was
  rewritten as a standalone `structured.Retry` helper with a configurable cap
  and context cancellation.

Apache-2.0 §4(a)/(c) are satisfied by the shipped license text and the
retained copyright headers in every derived file. gnet does not ship a NOTICE
file, so §4(d) does not apply.

## Not copied

`romangurevitch/ConcurrencyWorkshop` (Apache-2.0) was reviewed as reference
material only; no code from it is included in this repository.

`panjf2000/ants` (MIT) and `panjf2000/gnet/v2/pkg/pool/goroutine` (Apache-2.0)
were reviewed as design references for `workerpool/`. That package is an
original implementation and contains no code copied from either project.
