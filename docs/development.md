# Development Guide — go-p2p

## Prerequisites

- Go 1.25 or later (the module declares `go 1.25.5`)
- Network access to `forge.lthn.ai` for private dependencies (Borg, Poindexter, Enchantrix)
- SSH key configured for `git@forge.lthn.ai:2223` (HTTPS auth is not supported on Forge)

Private modules are hosted at `forge.lthn.ai`. Ensure your `GONOSUMCHECK` or `GONOSUMDB` environment variable includes `forge.lthn.ai` if sum database verification fails for those paths, and that `GOPRIVATE=forge.lthn.ai` is set so the Go toolchain does not proxy them through `proxy.golang.org`.

## Build and Test

```bash
# Run all tests
go test ./...

# Run a single test by name
go test -run TestName ./...

# Run tests with race detector (required before any PR)
go test -race ./...

# Skip integration tests (they bind real TCP ports)
go test -short ./...

# Run benchmarks
go test -bench . ./...
go test -bench BenchmarkName ./...

# Coverage per package
go test -cover ./node
go test -cover ./ueps
go test -cover ./logging

# Coverage report (HTML)
go test -coverprofile=cover.out ./... && go tool cover -html=cover.out

# Static analysis
go vet ./...
```

## Test Patterns

### Table-Driven Subtests

All tests use table-driven subtests with `t.Run()`. A test that does not follow this pattern should be refactored before merging.

```go
func TestFoo(t *testing.T) {
    cases := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {name: "valid input", input: "abc", want: "ABC"},
        {name: "empty input", input: "", wantErr: true},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got, err := Foo(tc.input)
            if tc.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tc.want, got)
        })
    }
}
```

### Test Naming Suffixes

Inherited from the wider go-p2p test tradition:

| Suffix | Meaning |
|--------|---------|
| `_Good` | Happy path |
| `_Bad` | Expected error conditions |
| `_Ugly` | Panic or edge-case conditions |

### Assertions

Use `github.com/stretchr/testify`. Import both `assert` (non-fatal) and `require` (fatal on failure):

```go
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)
```

Use `require` for setup steps and preconditions. Use `assert` for verification steps where partial results are still informative.

### Transport Test Helper

The `node` package provides a reusable helper for tests that need two live transport endpoints:

```go
tp := setupTestTransportPair(t)
// tp.Server, tp.Client     — *Transport
// tp.ServerNode, tp.ClientNode — *NodeManager
// tp.ServerReg, tp.ClientReg  — *PeerRegistry

pc := tp.connectClient(t) // performs handshake, returns *PeerConnection
```

`setupTestTransportPairWithConfig` accepts custom `TransportConfig` for each side, useful for testing keepalive and rate limiting behaviours.

The helper registers a `t.Cleanup` function that calls `Stop()` on both transports, so tests do not need to manage teardown.

### Integration Tests

Integration tests are gated with `testing.Short()`:

```go
if testing.Short() {
    t.Skip("skipping integration test in short mode")
}
```

Run them explicitly with `go test ./...` (without `-short`). They bind real localhost TCP ports and are safe to run in parallel with distinct transports because each test uses an ephemeral listen address (`:0`-style via `net/http/httptest` internally).

### Benchmark Structure

Benchmarks live in `bench_test.go` files within each package. They follow the standard Go benchmark pattern:

```go
func BenchmarkFoo(b *testing.B) {
    // setup outside loop
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        Foo()
    }
}
```

Run with `-benchmem` to track allocations:

```bash
go test -bench . -benchmem ./...
```

Reference timings (Apple M-series, 2025):

| Benchmark | Time | Allocs |
|-----------|------|--------|
| Identity keygen | 217 µs | — |
| Shared secret derivation | 53 µs | — |
| Message serialise | 4 µs | — |
| SMSG encrypt+decrypt | 4.7 µs | — |
| Challenge sign+verify | 505 ns | — |
| KD-tree peer select | 349 ns | — |
| KD-tree rebuild | 2.5 µs | — |
| UEPS marshal | 621 ns | — |
| UEPS read+verify | 1 µs | — |
| bufpool get/put | 8 ns | 0 |
| Challenge generation | 211 ns | — |

## Coding Standards

### UK English

All identifiers, comments, log messages, and documentation must use UK English spellings:

- colour (not color)
- organisation (not organization)
- centre (not center)
- behaviour (not behavior)
- recognise (not recognize)

### Strict Types

All parameters and return types must carry explicit type annotations. Avoid `interface{}` except where a generic pool or JSON-raw interface genuinely requires it; prefer `any` (the Go 1.18 alias) if you must. Do not use blank identifiers to discard typed return values without good reason.

### Error Handling

- Never discard errors silently.
- Wrap errors with context using `fmt.Errorf("context: %w", err)`.
- Return typed sentinel errors for conditions callers need to inspect programmatically.

### Licence Header

Every new file must carry the EUPL-1.2 licence identifier. The module's `LICENSE` file governs the package. Do not include the full licence text in each file; a short SPDX identifier comment at the top is sufficient for new files:

```go
// SPDX-License-Identifier: EUPL-1.2
```

### Security-First

- HMAC verification is required on all wire traffic (UEPS frames, not negotiable).
- Challenge-response authentication must not be weakened or bypassed in tests; use the `setupTestTransportPair` helper, which performs a real handshake.
- Any code that extracts archives must use `extractTarball` (or equivalent defensive logic) with Zip Slip defence, symlink rejection, and a size limit.
- Rate limiting and deduplication are not optional features; they are core to the security posture.

### Logging

Use the `logging` package throughout. Do not use `fmt.Println` or `log.Printf` in library code.

```go
logging.Debug("connected to peer", logging.Fields{"peer_id": pc.Peer.ID})
logging.Warn("peer rate limited", logging.Fields{"peer_id": pc.Peer.ID})
```

For hot paths (read loop), use the debug log sampling pattern already established in `transport.go` to avoid flooding logs:

```go
if debugLogCounter.Add(1)%debugLogInterval == 0 {
    logging.Debug("received message", logging.Fields{...})
}
```

## Conventional Commits

All commits follow the Conventional Commits specification:

```
type(scope): short description

Body (optional): longer explanation of the why, not the what.

Co-Authored-By: Virgil <virgil@lethean.io>
```

**Types**: `feat`, `fix`, `test`, `refactor`, `docs`, `chore`, `perf`, `ci`

**Scopes**: `node`, `ueps`, `logging`, `transport`, `peer`, `dispatcher`, `identity`, `bundle`, `controller`

Examples:

```
feat(dispatcher): implement UEPS threat circuit breaker

test(transport): add keepalive timeout and MaxConns enforcement tests

fix(peer): prevent data race in GracefulClose (P2P-RACE-1)
```

## Forge Remote

The canonical remote is:

```
ssh://git@forge.lthn.ai:2223/core/go-p2p.git
```

Push to `forge` remote only. GitHub remotes are disabled for push.

## Dependency Management

After adding or removing a dependency:

```bash
go mod tidy
go work sync   # if working within the go-p2p workspace
```

Do not vendor dependencies. The module uses the standard module proxy for public packages and Forge for private ones.
