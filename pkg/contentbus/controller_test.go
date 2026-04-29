// SPDX-License-Identifier: EUPL-1.2

package contentbus

import (
	core "dappco.re/go"
	"testing"
	"time"

	p2pnode "dappco.re/go/p2p/node"
)

func newTestController(t *testing.T) Controller {
	t.Helper()

	dir := t.TempDir()
	nm, err := p2pnode.NewNodeManagerWithPaths(
		core.PathJoin(dir, "private.key"),
		core.PathJoin(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("contentbus-test", p2pnode.RoleController); err != nil {
		t.Fatalf("generate identity: %v", err)
	}

	registry, err := p2pnode.NewPeerRegistryWithPath(core.PathJoin(dir, "peers.json"))
	if err != nil {
		t.Fatalf("create peer registry: %v", err)
	}
	t.Cleanup(func() {
		if err := registry.Close(); err != nil {
			t.Fatalf("close peer registry: %v", err)
		}
	})

	controller, err := NewController(
		WithNodeManager(nm),
		WithPeerRegistry(registry),
	)
	if err != nil {
		t.Fatalf("create controller: %v", err)
	}
	t.Cleanup(func() {
		if err := controller.Close(); err != nil {
			t.Fatalf("close controller: %v", err)
		}
	})

	return controller
}

func newContentbusNodeAndRegistry(t *testing.T) (*p2pnode.NodeManager, *p2pnode.PeerRegistry) {
	t.Helper()
	dir := t.TempDir()
	nm, err := p2pnode.NewNodeManagerWithPaths(
		core.PathJoin(dir, "private.key"),
		core.PathJoin(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("contentbus-triplet", p2pnode.RoleController); err != nil {
		t.Fatalf("generate identity: %v", err)
	}
	registry, err := p2pnode.NewPeerRegistryWithPath(core.PathJoin(dir, "peers.json"))
	if err != nil {
		t.Fatalf("create peer registry: %v", err)
	}
	t.Cleanup(func() { registry.Close() })
	return nm, registry
}

func TestController_NewController_Good(t *testing.T) {
	node, registry := newContentbusNodeAndRegistry(t)
	controller, err := NewController(WithNodeManager(node), WithPeerRegistry(registry))
	if err != nil {
		t.Fatalf("NewController: %v", err)
	}
	if controller == nil {
		t.Fatal("expected controller")
	}
}

func TestController_NewController_Bad(t *testing.T) {
	_, registry := newContentbusNodeAndRegistry(t)
	transport := p2pnode.NewTransport(nil, registry, p2pnode.DefaultTransportConfig())
	controller, err := NewController(WithTransport(transport))
	if err == nil {
		t.Fatal("expected missing node manager error")
	}
	if controller != nil {
		t.Fatalf("controller: got %#v, want nil", controller)
	}
}

func TestController_NewController_Ugly(t *testing.T) {
	node, registry := newContentbusNodeAndRegistry(t)
	controller, err := NewController(nil, WithNodeManager(node), WithPeerRegistry(registry), WithChannelBuffer(1))
	if err != nil {
		t.Fatalf("NewController with nil option: %v", err)
	}
	if controller == nil {
		t.Fatal("expected controller")
	}
}

func TestController_WithNodeManager_Good(t *testing.T) {
	node, _ := newContentbusNodeAndRegistry(t)
	var opts options
	err := WithNodeManager(node)(&opts)
	if err != nil {
		t.Fatalf("WithNodeManager: %v", err)
	}
	if opts.node != node {
		t.Fatal("node manager not set")
	}
}

func TestController_WithNodeManager_Bad(t *testing.T) {
	var opts options
	err := WithNodeManager(nil)(&opts)
	if err == nil {
		t.Fatal("expected nil node manager error")
	}
	if opts.node != nil {
		t.Fatal("node manager should remain nil")
	}
}

func TestController_WithNodeManager_Ugly(t *testing.T) {
	node, _ := newContentbusNodeAndRegistry(t)
	opts := options{node: nil}
	err := WithNodeManager(node)(&opts)
	if err != nil {
		t.Fatalf("WithNodeManager: %v", err)
	}
	if opts.node.GetIdentity().Name != "contentbus-triplet" {
		t.Fatal("unexpected node identity")
	}
}

func TestController_WithPeerRegistry_Good(t *testing.T) {
	_, registry := newContentbusNodeAndRegistry(t)
	var opts options
	err := WithPeerRegistry(registry)(&opts)
	if err != nil {
		t.Fatalf("WithPeerRegistry: %v", err)
	}
	if opts.registry != registry {
		t.Fatal("registry not set")
	}
}

func TestController_WithPeerRegistry_Bad(t *testing.T) {
	var opts options
	err := WithPeerRegistry(nil)(&opts)
	if err == nil {
		t.Fatal("expected nil registry error")
	}
	if opts.registry != nil {
		t.Fatal("registry should remain nil")
	}
}

func TestController_WithPeerRegistry_Ugly(t *testing.T) {
	_, registry := newContentbusNodeAndRegistry(t)
	opts := options{registry: nil}
	err := WithPeerRegistry(registry)(&opts)
	if err != nil {
		t.Fatalf("WithPeerRegistry: %v", err)
	}
	if opts.registry.Count() != 0 {
		t.Fatal("new registry should be empty")
	}
}

func TestController_WithTransport_Good(t *testing.T) {
	node, registry := newContentbusNodeAndRegistry(t)
	transport := p2pnode.NewTransport(node, registry, p2pnode.DefaultTransportConfig())
	var opts options
	err := WithTransport(transport)(&opts)
	if err != nil {
		t.Fatalf("WithTransport: %v", err)
	}
	if opts.transport != transport {
		t.Fatal("transport not set")
	}
}

func TestController_WithTransport_Bad(t *testing.T) {
	var opts options
	err := WithTransport(nil)(&opts)
	if err == nil {
		t.Fatal("expected nil transport error")
	}
	if opts.transport != nil {
		t.Fatal("transport should remain nil")
	}
}

func TestController_WithTransport_Ugly(t *testing.T) {
	node, registry := newContentbusNodeAndRegistry(t)
	transport := p2pnode.NewTransport(node, registry, p2pnode.DefaultTransportConfig())
	opts := options{transport: nil}
	err := WithTransport(transport)(&opts)
	if err != nil {
		t.Fatalf("WithTransport: %v", err)
	}
	if opts.transport.ConnectedPeers() != 0 {
		t.Fatal("new transport should have no peers")
	}
}

func TestController_WithTransportConfig_Good(t *testing.T) {
	cfg := p2pnode.DefaultTransportConfig()
	cfg.ListenAddr = "127.0.0.1:0"
	var opts options
	err := WithTransportConfig(cfg)(&opts)
	if err != nil {
		t.Fatalf("WithTransportConfig: %v", err)
	}
	if opts.transportConfig.ListenAddr != "127.0.0.1:0" {
		t.Fatalf("listen addr: got %q", opts.transportConfig.ListenAddr)
	}
}

func TestController_WithTransportConfig_Bad(t *testing.T) {
	var opts options
	err := WithTransportConfig(p2pnode.TransportConfig{})(&opts)
	if err != nil {
		t.Fatalf("WithTransportConfig: %v", err)
	}
	if opts.transportConfig.ListenAddr != "" {
		t.Fatalf("listen addr: got %q", opts.transportConfig.ListenAddr)
	}
}

func TestController_WithTransportConfig_Ugly(t *testing.T) {
	cfg := p2pnode.DefaultTransportConfig()
	cfg.MaxConns = 1
	var opts options
	err := WithTransportConfig(cfg)(&opts)
	if err != nil {
		t.Fatalf("WithTransportConfig: %v", err)
	}
	if opts.transportConfig.MaxConns != 1 {
		t.Fatalf("max conns: got %d", opts.transportConfig.MaxConns)
	}
}

func TestController_WithChannelBuffer_Good(t *testing.T) {
	var opts options
	err := WithChannelBuffer(8)(&opts)
	if err != nil {
		t.Fatalf("WithChannelBuffer: %v", err)
	}
	if opts.channelBuffer != 8 {
		t.Fatalf("buffer: got %d", opts.channelBuffer)
	}
}

func TestController_WithChannelBuffer_Bad(t *testing.T) {
	var opts options
	err := WithChannelBuffer(0)(&opts)
	if err == nil {
		t.Fatal("expected invalid buffer error")
	}
	if opts.channelBuffer != 0 {
		t.Fatalf("buffer: got %d", opts.channelBuffer)
	}
}

func TestController_WithChannelBuffer_Ugly(t *testing.T) {
	var opts options
	err := WithChannelBuffer(1)(&opts)
	if err != nil {
		t.Fatalf("WithChannelBuffer: %v", err)
	}
	if opts.channelBuffer != 1 {
		t.Fatalf("buffer: got %d", opts.channelBuffer)
	}
}

func TestController_WithStartTransport_Good(t *testing.T) {
	var opts options
	err := WithStartTransport(true)(&opts)
	if err != nil {
		t.Fatalf("WithStartTransport: %v", err)
	}
	if !opts.startTransport {
		t.Fatal("startTransport not set")
	}
}

func TestController_WithStartTransport_Bad(t *testing.T) {
	opts := options{startTransport: true}
	err := WithStartTransport(false)(&opts)
	if err != nil {
		t.Fatalf("WithStartTransport: %v", err)
	}
	if opts.startTransport {
		t.Fatal("startTransport should be false")
	}
}

func TestController_WithStartTransport_Ugly(t *testing.T) {
	var opts options
	_ = WithStartTransport(true)(&opts)
	err := WithStartTransport(false)(&opts)
	if err != nil {
		t.Fatalf("WithStartTransport: %v", err)
	}
	if opts.startTransport {
		t.Fatal("last option should win")
	}
}

func readEvent(t *testing.T, ch <-chan Event) Event {
	t.Helper()

	select {
	case event, ok := <-ch:
		if !ok {
			t.Fatal("subscription channel closed")
		}
		return event
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	return Event{}
}

func TestControllerSubscribePublishSameTopic(t *testing.T) {
	controller := newTestController(t)

	ch, err := controller.Subscribe("content.created")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	payload := []byte("hello")
	if err := controller.Publish("content.created", payload); err != nil {
		t.Fatalf("publish: %v", err)
	}
	payload[0] = 'H'

	event := readEvent(t, ch)
	if event.Topic != "content.created" {
		t.Fatalf("topic: got %q, want %q", event.Topic, "content.created")
	}
	if !core.DeepEqual(event.Payload, []byte("hello")) {
		t.Fatalf("payload: got %q, want %q", event.Payload, []byte("hello"))
	}
	if event.PeerID == "" {
		t.Fatal("peer id should be set")
	}
	if event.Timestamp.IsZero() {
		t.Fatal("timestamp should be set")
	}
}

func TestControllerSubscribeTopicAPublishTopicB(t *testing.T) {
	controller := newTestController(t)

	ch, err := controller.Subscribe("topic.a")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	if err := controller.Publish("topic.b", []byte("payload")); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case event := <-ch:
		t.Fatalf("unexpected event: %+v", event)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestControllerCloseWhileSubscribed(t *testing.T) {
	controller := newTestController(t)

	ch, err := controller.Subscribe("topic.close")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	if err := controller.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("subscription channel should be closed")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for subscription channel to close")
	}
}

func TestControllerTwoConcurrentSubscribersSameTopic(t *testing.T) {
	controller := newTestController(t)

	ch1, err := controller.Subscribe("topic.shared")
	if err != nil {
		t.Fatalf("subscribe first: %v", err)
	}
	ch2, err := controller.Subscribe("topic.shared")
	if err != nil {
		t.Fatalf("subscribe second: %v", err)
	}

	if err := controller.Publish("topic.shared", []byte("payload")); err != nil {
		t.Fatalf("publish: %v", err)
	}

	event1 := readEvent(t, ch1)
	event2 := readEvent(t, ch2)

	if !core.DeepEqual(event1.Payload, []byte("payload")) {
		t.Fatalf("first payload: got %q, want %q", event1.Payload, []byte("payload"))
	}
	if !core.DeepEqual(event2.Payload, []byte("payload")) {
		t.Fatalf("second payload: got %q, want %q", event2.Payload, []byte("payload"))
	}
}
