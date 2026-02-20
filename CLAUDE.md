# CLAUDE.md

## Project

`go-p2p` is the P2P networking layer for the Lethean network. Module path: `forge.lthn.ai/core/go-p2p`

## Commands

```bash
go test ./...                    # Run all tests
go test -run TestName ./...      # Single test
go test -race ./...              # Race detector (required before any PR)
go test -short ./...             # Skip integration tests
go test -cover ./node            # Coverage for node package
go test -bench . ./...           # Benchmarks
go vet ./...                     # Static analysis
```

## Key Interfaces

```go
// MinerManager — decoupled miner control (worker.go)
type MinerManager interface {
    StartMiner(config map[string]any) error
    StopMiner(id string) error
    GetStats() map[string]any
    GetLogs(id string, lines int) ([]string, error)
}

// ProfileManager — deployment profiles (worker.go)
type ProfileManager interface {
    ApplyProfile(name string, data []byte) error
}
```

## Coding Standards

- UK English (colour, organisation, centre)
- All parameters and return types explicitly annotated
- Tests use `testify` assert/require; table-driven subtests with `t.Run()`
- Licence: EUPL-1.2
- Security-first: do not weaken HMAC, challenge-response, Zip Slip defence, or rate limiting
- Use `logging` package only — no `fmt.Println` or `log.Printf` in library code

## Commit Format

```
type(scope): description

Co-Authored-By: Virgil <virgil@lethean.io>
```

## Documentation

- `docs/architecture.md` — full package and component reference
- `docs/development.md` — build, test, benchmark, standards guide
- `docs/history.md` — completed phases, known limitations, bugs fixed
