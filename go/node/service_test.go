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

// TestService_StartStop_Good brings a fully-wired service up on an ephemeral
// port and shuts it back down, covering the Start/Stop success branches.
func TestService_StartStop_Good(t *testing.T) {
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
	svc := c.Service("node").Value.(*Service)

	if r := svc.Start(); !r.OK {
		t.Fatalf("Start: %v", asError(r))
	}
	t.Cleanup(func() {
		if r := svc.Stop(); !r.OK {
			t.Fatalf("Stop: %v", asError(r))
		}
	})
}

// TestService_asError_Good returns the underlying error of a failed Result.
func TestService_asError_Good(t *testing.T) {
	want := core.NewError("boom")
	got := asError(core.Fail(want))
	if !core.Is(got, want) {
		t.Fatalf("asError: got %v, want %v", got, want)
	}
}

// TestService_asError_Bad returns nil when the Result value is not an error.
func TestService_asError_Bad(t *testing.T) {
	if got := asError(core.Ok("not-an-error")); got != nil {
		t.Fatalf("asError on non-error value: got %v, want nil", got)
	}
}

// TestService_asError_Ugly returns nil for a nil-valued Result.
func TestService_asError_Ugly(t *testing.T) {
	if got := asError(core.Ok(nil)); got != nil {
		t.Fatalf("asError on nil value: got %v, want nil", got)
	}
}
