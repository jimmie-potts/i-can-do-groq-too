# 2026-08-02 ICGT-005 FastGate lifecycle

- **Story:** [ICGT-005](../icgt-005-bootstrap-fastgate-service.md)
- **Decision:** [ADR 0004](../../docs/adr/0004-go-toolchain-and-module-strategy.md)
- **Lesson:** [Go service lifecycle](../../docs/lessons/icgt-005-go-service-lifecycle.md)
- **Visual:** [FastGate service lifecycle](../../docs/lessons/assets/icgt-005-go-service-lifecycle.pptx)

## Observation

ICGT-005 began with no Go toolchain in WSL. The official `go1.26.5.linux-amd64.tar.gz` archive was
verified against SHA-256
`5c2c3b16caefa1d968a94c1daca04a7ca301a496d9b086e17ad77bb81393f053`, installed at
`/usr/local/go`, and added to the login-shell path. The verified environment is Go 1.26.5 on
Linux/amd64 with cgo enabled and GCC 15.2.

The shared `/tmp` tmpfs was full. Go and GCC therefore failed while writing race-build temporary
files even though the root filesystem had ample free space. Local validation used task-owned
temporary and build-cache directories under `.git`; no unrelated `/tmp` data was removed. The
first sandboxed loopback test also failed before binding with `socket: operation not permitted`.
The same injected-listener tests passed with loopback permission and made no external request.

## Locked implementation choices

- The only module is the repository-root
  `github.com/jimmie-potts/i-can-do-groq-too` module with `go 1.26.5`.
- The executable is `gateway/cmd/fastgate`; the reviewable lifecycle package is
  `gateway/internal/service`.
- `go run ./gateway/cmd/fastgate` listens on `127.0.0.1:8080`; `-listen` permits another explicit
  address.
- `GET /healthz` returns `200`, `text/plain; charset=utf-8`, `Cache-Control: no-store`, and `ok\n`.
  Normal ServeMux behavior rejects other methods and paths.
- Defaults are a five-second read-header timeout, ten-second read/write timeouts, sixty-second idle
  timeout, five-second shutdown grace, and 16 KiB maximum header configuration. Configuration
  rejects nonpositive timeouts, surrounding address whitespace, addresses over 255 bytes, and a
  maximum-header setting over one MiB.
- Cancellation or an unexpected serving failure calls deadline-bounded graceful shutdown. Any
  shutdown error forces server closure; the original serving error is preserved, and every path
  receives the `Serve` result before returning.
- The service remains standard-library-only. No `go.sum`, workspace, provider, inference schema,
  retry, metric, authentication, TLS, Docker, or Kubernetes behavior was added.

## Test and gate design

Handler tests use `httptest` for health success, method rejection, and path rejection. One lifecycle
test injects an ephemeral loopback listener, disables ambient proxy use, performs a real bounded
health request, cancels the owner context, and observes clean return. One controlled-server test
proves an early serving failure still invokes bounded cleanup and preserves the original cause. The
shutdown-failure test uses channels and a controlled server: it verifies a deadline is present, returns
`context.DeadlineExceeded`, waits for forced close to release serving, and proves the final join
without a timing sleep.

The Python repository checker now rejects manifest drift, nested modules, workspaces, and checksum
metadata, and it verifies the CI provisioning shape. The shell gate uses the complete ADR 0004
offline environment, verifies matching `go` and `gofmt`, checks the resolved root module, discovers
all nonignored Go source through NUL-delimited Git output, and runs format, vet, ordinary-test,
race-preflight, and race-test stages unconditionally.

The shell gate re-executes with a small allowlist instead of trying to recognize every possible
credential name. Exported secrets are omitted, `HOME` and Go configuration move into fresh temporary
directories, telemetry is set to `off` and verified there, and the temporary state is deleted on exit.
This does not claim an operating-system filesystem or network sandbox. The CI checker now requires
exactly one unconditional `setup-go` step, associates `go-version-file` and `cache` with that step,
and rejects mutable references or setup-shaped text hidden inside a shell block. It also resolves the
static policy, `GOVERSION` verification, and canonical gate as unconditional structural run steps, so
comments and echoed command strings cannot satisfy their ordering contract. All four stages must
belong to the same `steps` block so separate concurrently scheduled jobs cannot fake sequential
toolchain ownership.

The first sanitizer revision exported its internal re-execution marker. The policy test's nested gate
invocation inherited that marker and failed because its sentinel secret remained visible. The marker
is now unset immediately after re-execution, and the same regression test passes both alone and from
inside the canonical gate.

CI statically checks the repository before provisioning Go, pins `actions/setup-go` v6.5.0 by
commit SHA, reads the version from `go.mod`, disables dependency caching, verifies the provisioned
`GOVERSION`, and then calls the canonical gate.

## Validation evidence

- `python3 -m unittest discover -s tests -p 'test_*.py'` passed all 17 repository-policy tests.
- Focused FastGate tests passed with the injected loopback listener.
- A live loopback request returned `200`, `Cache-Control: no-store`,
  `Content-Type: text/plain; charset=utf-8`, and `ok\n`. Because terminal Ctrl+C status can describe
  the PTY or `go run` wrapper rather than the child service, the final shutdown smoke sent SIGTERM
  directly to the compiled FastGate process and observed `health=ok process_exit=0`.
- `./scripts/check` passed environment isolation, static policy, links, 17 policy tests, toolchain
  and telemetry identity, module identity, Go formatting, vet, ordinary tests, race prerequisites,
  and race tests.
- Separate final reviews of lifecycle correctness, gate isolation/CI structure, and learning/status
  artifacts reported no actionable findings after their feedback was resolved. Documented residuals
  are the single-use service contract, controlled rather than real blocked-handler cleanup test, and
  the intentionally constrained block-style workflow parser.
- `git diff --check` passed.
- Artifact Tool rendered six slide PNGs, and each was inspected individually at full size. The
  bundled `render_slides.py` then round-tripped the final PPTX through the independent document
  renderer; all six resulting PNGs were inspected again. `slides_test.py` reported `Test passed. No
  overflow detected.`
- The temporary presentation workspace used the conversation's persistent Windows-side build area
  because Windows Node cannot create the required Artifact Tool junction inside a WSL UNC path. The
  only presentation artifact retained in the repository is the verified PPTX.

## Follow-up

ICGT-006 is next. It owns the versioned non-streaming model-turn schema, mapping, fixtures, and
offline validation selected by ADR 0003. It must not turn `/healthz` into an inference endpoint or
add a provider contract before its schema evidence is reviewed.
