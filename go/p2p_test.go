// SPDX-License-Identifier: EUPL-1.2

package p2p

import (
	"testing"

	core "dappco.re/go"
)

// TestNewService_DelegatesToNode verifies the root p2p.NewService
// delegating factory wires the same Service that node.NewService produces.
func TestNewService_DelegatesToNode(t *testing.T) {
	c := core.New(core.WithService(NewService(ServiceOptions{})))
	r := c.Service("node")
	if !r.OK {
		t.Fatalf("p2p.NewService did not register node Service, got %#v", r.Value)
	}
	if _, ok := r.Value.(*Service); !ok {
		t.Fatalf("expected *p2p.Service (alias of *node.Service), got %T", r.Value)
	}
}

// TestRegister_DelegatesToNode verifies the imperative-style p2p.Register
// shorthand registers via node.Register.
func TestRegister_DelegatesToNode(t *testing.T) {
	c := core.New(core.WithService(Register))
	r := c.Service("node")
	if !r.OK {
		t.Fatalf("p2p.Register did not register node Service, got %#v", r.Value)
	}
}
