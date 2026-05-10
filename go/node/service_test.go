// SPDX-License-Identifier: EUPL-1.2

package node

import (
	"path/filepath"
	"testing"

	core "dappco.re/go"
)

// TestNewService_CustomPaths_ConstructsAllComponents verifies the four
// composed components (NodeManager, PeerRegistry, Transport, Controller)
// are wired when explicit paths + a TransportConfig are supplied.
func TestNewService_CustomPaths_ConstructsAllComponents(t *testing.T) {
	dir := t.TempDir()
	opts := ServiceOptions{
		KeyPath:    filepath.Join(dir, "key"),
		ConfigPath: filepath.Join(dir, "node.json"),
		PeersPath:  filepath.Join(dir, "peers.json"),
		TransportConfig: TransportConfig{
			ListenAddr: "127.0.0.1:0",
			WSPath:     "/ws",
			MaxConns:   10,
		},
	}
	c := core.New(core.WithService(NewService(opts)))
	r := c.Service("node")
	if !r.OK {
		t.Fatal("node service not registered via NewService")
	}
	svc := r.Value.(*Service)
	if svc.NodeManager == nil || svc.Peers == nil || svc.Transport == nil || svc.Controller == nil {
		t.Fatalf("expected all components wired: nm=%v peers=%v transport=%v controller=%v",
			svc.NodeManager != nil, svc.Peers != nil, svc.Transport != nil, svc.Controller != nil)
	}
}

// TestNewService_EmptyTransportConfig_FallsBackToDefault verifies the
// zero-value ListenAddr branch swaps in DefaultTransportConfig.
func TestNewService_EmptyTransportConfig_FallsBackToDefault(t *testing.T) {
	dir := t.TempDir()
	opts := ServiceOptions{
		KeyPath:    filepath.Join(dir, "key"),
		ConfigPath: filepath.Join(dir, "node.json"),
		PeersPath:  filepath.Join(dir, "peers.json"),
	}
	c := core.New(core.WithService(NewService(opts)))
	r := c.Service("node")
	if !r.OK {
		t.Fatal("node service not registered")
	}
	svc := r.Value.(*Service)
	if svc.Transport == nil {
		t.Fatal("expected non-nil Transport with default config")
	}
}

// TestRegister_DefaultsRegistersAllComponents verifies the imperative-style
// Register(c) shorthand registers the Service with XDG default paths +
// DefaultTransportConfig.
func TestRegister_DefaultsRegistersAllComponents(t *testing.T) {
	c := core.New(core.WithService(Register))
	r := c.Service("node")
	if !r.OK {
		t.Fatalf("node service not registered via Register, got %#v", r.Value)
	}
	svc := r.Value.(*Service)
	if svc.NodeManager == nil || svc.Peers == nil || svc.Transport == nil || svc.Controller == nil {
		t.Fatal("expected all components wired via Register")
	}
}

// TestService_NilReceiver_GuardsStartStop verifies the nil-receiver guards
// in Start / Stop don't panic.
func TestService_NilReceiver_GuardsStartStop(t *testing.T) {
	var svc *Service
	if r := svc.Start(); r.OK {
		t.Fatal("expected nil-receiver Start to fail")
	}
	if r := svc.Stop(); r.OK {
		t.Fatal("expected nil-receiver Stop to fail")
	}
}

// TestService_NilTransport_StartStopFail verifies that a Service with a
// nil Transport (post-construction mutation) returns a configuration error
// from Start/Stop rather than panicking.
func TestService_NilTransport_StartStopFail(t *testing.T) {
	svc := &Service{}
	if r := svc.Start(); r.OK {
		t.Fatal("expected nil-Transport Start to fail")
	}
	if r := svc.Stop(); r.OK {
		t.Fatal("expected nil-Transport Stop to fail")
	}
}
