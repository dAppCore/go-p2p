// SPDX-License-Identifier: EUPL-1.2

// Service registration for the node package — exposes the canonical
// `NewService(opts)` + `Register(c)` shape per Mantis #1336, wrapping
// the existing NodeManager / PeerRegistry / Transport / Controller
// constructors in a Core-registerable factory.
//
//	c, _ := core.New(
//	    core.WithService(node.NewService(node.ServiceOptions{
//	        TransportConfig: node.DefaultTransportConfig(),
//	    })),
//	)
//	svc := core.MustServiceFor[*node.Service](c, "node")
//	r := svc.Start()
//	defer svc.Stop()
//
// Service composes the four stateful types this package owns:
// NodeManager (identity), PeerRegistry (known peers), Transport
// (WebSocket I/O), Controller (request/response over Transport).
// They form a single runtime — there is no use case for one without
// the others — so a unified Service captures the natural lifecycle.

package node

import (
	core "dappco.re/go"
	coreerr "dappco.re/go/log"
)

// ServiceOptions configures the node service. Empty paths fall back
// to NewNodeManager / NewPeerRegistry XDG defaults; an empty
// TransportConfig.ListenAddr falls back to DefaultTransportConfig().
//
//	node.ServiceOptions{
//	    KeyPath:         "/tmp/test-key",
//	    ConfigPath:      "/tmp/test-config.json",
//	    PeersPath:       "/tmp/test-peers.json",
//	    TransportConfig: node.DefaultTransportConfig(),
//	}
type ServiceOptions struct {
	// KeyPath is the on-disk path for the node's private X25519 key.
	// Empty → resolves via xdg.DataFile("lethean-desktop/node/private.key").
	KeyPath string
	// ConfigPath is the on-disk path for the node's identity JSON.
	// Empty → resolves via xdg.ConfigFile("lethean-desktop/node.json").
	ConfigPath string
	// PeersPath is the on-disk path for the peer registry JSON.
	// Empty → uses the PeerRegistry default location.
	PeersPath string
	// TransportConfig configures the WebSocket transport. Zero-value
	// ListenAddr → DefaultTransportConfig() applied.
	TransportConfig TransportConfig
}

// Service is the registerable handle for the node package — embeds
// *core.ServiceRuntime[ServiceOptions] for typed options access and
// holds the four wired components ready for Start / Stop.
//
// Usage example: `svc := core.MustServiceFor[*node.Service](c, "node"); r := svc.Start()`
type Service struct {
	*core.ServiceRuntime[ServiceOptions]
	// NodeManager owns the local identity (keys + name + role).
	NodeManager *NodeManager
	// Peers is the registry of known remote peers.
	Peers *PeerRegistry
	// Transport is the WebSocket I/O layer over which peers exchange
	// encrypted messages.
	Transport *Transport
	// Controller wraps Transport with request/response semantics for
	// remote-miner orchestration calls.
	Controller *Controller
}

// NewService returns a factory that constructs the four node
// components from the supplied options and wraps them as a
// Core-registerable *Service.
//
//	core.WithService(node.NewService(node.ServiceOptions{
//	    TransportConfig: node.DefaultTransportConfig(),
//	}))
//
// Returns an error result if NodeManager or PeerRegistry construction
// fails (typically a path / permissions error). Transport + Controller
// construction is infallible at this layer.
func NewService(opts ServiceOptions) func(*core.Core) core.Result {
	return func(c *core.Core) core.Result {
		nodeResult := nodeManagerForOptions(opts)
		if !nodeResult.OK {
			return core.Fail(coreerr.E("node.NewService", "node manager construction failed", asError(nodeResult)))
		}
		nm, _ := nodeResult.Value.(*NodeManager)

		peersResult := peerRegistryForOptions(opts)
		if !peersResult.OK {
			return core.Fail(coreerr.E("node.NewService", "peer registry construction failed", asError(peersResult)))
		}
		peers, _ := peersResult.Value.(*PeerRegistry)

		transportConfig := opts.TransportConfig
		if transportConfig.ListenAddr == "" {
			transportConfig = DefaultTransportConfig()
		}
		transport := NewTransport(nm, peers, transportConfig)
		controller := NewController(nm, peers, transport)

		return core.Ok(&Service{
			ServiceRuntime: core.NewServiceRuntime(c, opts),
			NodeManager:    nm,
			Peers:          peers,
			Transport:      transport,
			Controller:     controller,
		})
	}
}

// Register wires the node service into the Core with empty
// ServiceOptions — the imperative-style alternative to NewService.
// The resulting *Service uses XDG defaults for identity/peers paths
// and DefaultTransportConfig() for the WebSocket transport.
//
//	core.New(core.WithService(node.Register))
func Register(c *core.Core) core.Result {
	return NewService(ServiceOptions{})(c)
}

// Start brings up the WebSocket transport (the only component with a
// runtime lifecycle). NodeManager / PeerRegistry / Controller are
// passive after construction.
//
//	r := svc.Start()
func (s *Service) Start() core.Result {
	if s == nil || s.Transport == nil {
		return core.Fail(coreerr.E("node.Service.Start", "transport not configured", nil))
	}
	return s.Transport.Start()
}

// Stop shuts down the WebSocket transport and gracefully closes all
// peer connections.
//
//	r := svc.Stop()
func (s *Service) Stop() core.Result {
	if s == nil || s.Transport == nil {
		return core.Fail(coreerr.E("node.Service.Stop", "transport not configured", nil))
	}
	return s.Transport.Stop()
}

func nodeManagerForOptions(opts ServiceOptions) core.Result {
	if opts.KeyPath == "" && opts.ConfigPath == "" {
		return NewNodeManager()
	}
	return NewNodeManagerWithPaths(opts.KeyPath, opts.ConfigPath)
}

func peerRegistryForOptions(opts ServiceOptions) core.Result {
	if opts.PeersPath == "" {
		return NewPeerRegistry()
	}
	return NewPeerRegistryWithPath(opts.PeersPath)
}

func asError(r core.Result) error {
	if err, ok := r.Value.(error); ok {
		return err
	}
	return nil
}
