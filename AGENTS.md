# go-p2p Agent Guide

This repository contains the P2P transport, identity, and control surface used
by Lethean services.  The module is `dappco.re/go/p2p`; local development is
expected to run with `GOWORK=off` so the module replacements in `go.mod` point
at the sibling core repositories consistently.

The main package layout is deliberately small.  `node/` owns identities,
peer registries, encrypted WebSocket transport, controller and worker message
handling, deployment bundles, and the Levin portable-storage helpers under
`node/levin/`.  `ueps/` owns the signed TLV packet format used by the intent
dispatcher.  `logging/` is the package-local logger used by the transport and
node components.  `pkg/contentbus/` exposes a publish/subscribe API on top of
the node transport, and `pkg/api/` adapts the P2P controller into the
`dappco.re/go/api` provider shape.

Use `dappco.re/go` core wrappers for formatting, JSON, paths, filesystem, and
error helpers in both production code and tests.  Direct imports of the banned
stdlib packages are intentionally avoided so this repo stays aligned with the
core/go v0.9 compliance shape.  Tests belong beside their source file:
`<source>_test.go` for behavior and `<source>_example_test.go` for runnable
usage examples.  Keep triplet tests named `Test<File>_<Symbol>_{Good,Bad,Ugly}`
and make each test body exercise the symbol it claims to cover.

Before handing work back, run the compliance audit and the normal Go gates from
the brief.  The audit script is the contract for repository shape; passing unit
tests alone is not sufficient.
