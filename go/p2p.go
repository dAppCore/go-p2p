// SPDX-License-Identifier: EUPL-1.2

// Package p2p is the root composer for the Lethean peer-to-peer stack.
// It exposes a thin canonical Service entry-point per Mantis #1336 that
// re-exports the node-layer Service so consumers can register the p2p
// stack with one import + one core.WithService call.
//
//	c, _ := core.New(
//	    core.WithService(p2p.NewService(p2p.ServiceOptions{
//	        TransportConfig: node.DefaultTransportConfig(),
//	    })),
//	)
//	svc := core.MustServiceFor[*node.Service](c, "node")
//	r := svc.Start()
//
// All wiring lives in node/service.go — this file is the public façade
// keyed off the repository root for IDE discoverability and to match
// the canonical Service shape used across sister go-* repositories.
package p2p

import (
	core "dappco.re/go"

	"dappco.re/go/p2p/node"
)

// ServiceOptions is the typed input shape for the p2p service —
// re-exports node.ServiceOptions so consumers don't need a second
// import for the common case.
type ServiceOptions = node.ServiceOptions

// Service is the runtime handle for the p2p stack — re-exports
// *node.Service so consumers can declare receivers / fields against
// p2p.Service while the implementation lives in the node package.
type Service = node.Service

// NewService returns a factory that constructs the p2p service from
// the supplied options. Delegates to node.NewService.
//
//	core.WithService(p2p.NewService(p2p.ServiceOptions{}))
func NewService(opts ServiceOptions) func(*core.Core) core.Result {
	return node.NewService(opts)
}

// Register wires the p2p service into the Core with empty
// ServiceOptions — the imperative-style alternative to NewService.
//
//	core.New(core.WithService(p2p.Register))
func Register(c *core.Core) core.Result {
	return node.Register(c)
}
