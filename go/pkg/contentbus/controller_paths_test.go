// SPDX-License-Identifier: EUPL-1.2

package contentbus

import (
	"testing"
	"time"

	core "dappco.re/go"
	p2pnode "dappco.re/go/p2p/node"
)

// newRawContentbusController returns the concrete *controller for direct
// access to unexported handlers such as handleMessage.
func newRawContentbusController(t *testing.T) *controller {
	t.Helper()
	c, _ := newTestController(t).(*controller)
	if c == nil {
		t.Fatal("expected *controller")
	}
	return c
}

// TestController_HandleMessage_Good delivers a well-formed content_event from a
// remote peer to a local subscriber.
func TestController_HandleMessage_Good(t *testing.T) {
	c := newRawContentbusController(t)

	ch, err := contentbusResultValue[<-chan Event](c.Subscribe("remote.topic"))
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	msg, err := contentbusResultValue[*p2pnode.Message](p2pnode.NewMessage(
		contentEventMessageType, "remote-peer", "", wireEvent{
			Topic:   "remote.topic",
			Payload: []byte("from-peer"),
		}))
	if err != nil {
		t.Fatalf("new message: %v", err)
	}

	c.handleMessage(nil, msg)

	event := readEvent(t, ch)
	if event.Topic != "remote.topic" {
		t.Fatalf("topic: got %q, want %q", event.Topic, "remote.topic")
	}
	if !core.DeepEqual(event.Payload, []byte("from-peer")) {
		t.Fatalf("payload: got %q, want %q", event.Payload, []byte("from-peer"))
	}
	if event.PeerID != "remote-peer" {
		t.Fatalf("peer id: got %q, want %q", event.PeerID, "remote-peer")
	}
	if event.Timestamp.IsZero() {
		t.Fatal("timestamp should be set from the message")
	}
}

// TestController_HandleMessage_Bad ignores messages that are nil, of the wrong
// type, carry an unparsable payload, or name an empty topic — none should
// reach a subscriber.
func TestController_HandleMessage_Bad(t *testing.T) {
	c := newRawContentbusController(t)

	ch, err := contentbusResultValue[<-chan Event](c.Subscribe("guard.topic"))
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// nil message
	c.handleMessage(nil, nil)

	// wrong message type
	wrongType, err := contentbusResultValue[*p2pnode.Message](p2pnode.NewMessage(
		p2pnode.MessageType("not_content_event"), "peer", "", wireEvent{Topic: "guard.topic", Payload: []byte("x")}))
	if err != nil {
		t.Fatalf("new wrong-type message: %v", err)
	}
	c.handleMessage(nil, wrongType)

	// unparsable payload (raw bytes that are not the wireEvent JSON shape)
	badPayload := &p2pnode.Message{
		Type:      contentEventMessageType,
		From:      "peer",
		Timestamp: time.Now(),
		Payload:   p2pnode.RawMessage("not-json"),
	}
	c.handleMessage(nil, badPayload)

	// empty topic in an otherwise valid wireEvent
	emptyTopic, err := contentbusResultValue[*p2pnode.Message](p2pnode.NewMessage(
		contentEventMessageType, "peer", "", wireEvent{Topic: "", Payload: []byte("x")}))
	if err != nil {
		t.Fatalf("new empty-topic message: %v", err)
	}
	c.handleMessage(nil, emptyTopic)

	select {
	case event := <-ch:
		t.Fatalf("expected no delivery, got %+v", event)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestController_HandleMessage_Ugly drops a valid message when the controller
// is already closed, and back-fills a zero timestamp with the receive time.
func TestController_HandleMessage_Ugly(t *testing.T) {
	c := newRawContentbusController(t)

	ch, err := contentbusResultValue[<-chan Event](c.Subscribe("zero.ts"))
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// Zero timestamp is back-filled to now.
	zeroTS := &p2pnode.Message{
		Type:    contentEventMessageType,
		From:    "peer",
		Payload: mustWirePayload(t, wireEvent{Topic: "zero.ts", Payload: []byte("y")}),
	}
	c.handleMessage(nil, zeroTS)

	event := readEvent(t, ch)
	if event.Timestamp.IsZero() {
		t.Fatal("zero timestamp should be back-filled")
	}

	// After Close, a delivered message must be dropped (closed guard).
	if err := contentbusResultErr(c.Close()); err != nil {
		t.Fatalf("close: %v", err)
	}
	afterClose := &p2pnode.Message{
		Type:      contentEventMessageType,
		From:      "peer",
		Timestamp: time.Now(),
		Payload:   mustWirePayload(t, wireEvent{Topic: "zero.ts", Payload: []byte("z")}),
	}
	c.handleMessage(nil, afterClose) // must not panic on the closed channel
}

func mustWirePayload(t *testing.T, w wireEvent) p2pnode.RawMessage {
	t.Helper()
	msg, err := contentbusResultValue[*p2pnode.Message](p2pnode.NewMessage(
		contentEventMessageType, "peer", "", w))
	if err != nil {
		t.Fatalf("new message for payload: %v", err)
	}
	return msg.Payload
}

// TestController_Publish_Bad rejects an empty topic and rejects publishing
// after the controller has been closed.
func TestController_Publish_Bad(t *testing.T) {
	c := newRawContentbusController(t)

	if err := contentbusResultErr(c.Publish("", []byte("x"))); err == nil {
		t.Fatal("expected invalid topic error for empty topic")
	}

	if err := contentbusResultErr(c.Close()); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := contentbusResultErr(c.Publish("topic", []byte("x"))); err == nil {
		t.Fatal("expected closed error after Close")
	}
}

// TestController_Subscribe_Bad rejects an empty topic and rejects subscribing
// after the controller has been closed.
func TestController_Subscribe_Bad(t *testing.T) {
	c := newRawContentbusController(t)

	if err := contentbusResultErr(c.Subscribe("")); err == nil {
		t.Fatal("expected invalid topic error for empty topic")
	}

	if err := contentbusResultErr(c.Close()); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := contentbusResultErr(c.Subscribe("topic")); err == nil {
		t.Fatal("expected closed error after Close")
	}
}

// TestController_Close_Idempotent confirms Close is safe to call repeatedly.
func TestController_Close_Idempotent(t *testing.T) {
	c := newRawContentbusController(t)

	if err := contentbusResultErr(c.Close()); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := contentbusResultErr(c.Close()); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

// TestController_NewController_OwnsTransport creates a controller without a
// supplied transport, exercising the owns-transport / owns-registry branch and
// the default-identity loader.
func TestController_NewController_OwnsTransport(t *testing.T) {
	node, _ := newContentbusNodeAndRegistry(t)

	controller, err := contentbusResultValue[Controller](NewController(WithNodeManager(node)))
	if err != nil {
		t.Fatalf("NewController owns-transport: %v", err)
	}
	if controller == nil {
		t.Fatal("expected controller")
	}
	if err := contentbusResultErr(controller.Close()); err != nil {
		t.Fatalf("close owns-transport controller: %v", err)
	}
}

// TestController_LoadOrCreateContentIdentity_Good drives the default-identity
// path used when NewController is given neither a node nor a transport.
func TestController_LoadOrCreateContentIdentity_Good(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	r := loadOrCreateContentIdentity()
	if !r.OK {
		t.Fatalf("loadOrCreateContentIdentity: %v", contentbusResultErr(r))
	}
	nm, ok := r.Value.(*p2pnode.NodeManager)
	if !ok {
		t.Fatalf("expected *NodeManager, got %#v", r.Value)
	}
	if !nm.HasIdentity() {
		t.Fatal("expected an identity after load-or-create")
	}

	// Second call reuses the now-present identity rather than regenerating.
	again := loadOrCreateContentIdentity()
	if !again.OK {
		t.Fatalf("loadOrCreateContentIdentity (reuse): %v", contentbusResultErr(again))
	}
}

// TestController_Publish_NilPayload publishes a nil payload, exercising the
// cloneBytes nil branch, and confirms a local subscriber still receives the
// event with a nil payload.
func TestController_Publish_NilPayload(t *testing.T) {
	c := newRawContentbusController(t)

	ch, err := contentbusResultValue[<-chan Event](c.Subscribe("nil.payload"))
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	if err := contentbusResultErr(c.Publish("nil.payload", nil)); err != nil {
		t.Fatalf("publish nil payload: %v", err)
	}

	event := readEvent(t, ch)
	if event.Payload != nil {
		t.Fatalf("payload: got %q, want nil", event.Payload)
	}
}

// TestController_StartTransport_Publish_Good starts the owned transport on an
// ephemeral port and publishes, exercising the broadcast (zero-peer) path.
func TestController_StartTransport_Publish_Good(t *testing.T) {
	node, registry := newContentbusNodeAndRegistry(t)
	cfg := p2pnode.DefaultTransportConfig()
	cfg.ListenAddr = "127.0.0.1:0"

	controller, err := contentbusResultValue[Controller](NewController(
		WithNodeManager(node),
		WithPeerRegistry(registry),
		WithTransportConfig(cfg),
		WithStartTransport(true),
	))
	if err != nil {
		t.Fatalf("NewController with start transport: %v", err)
	}
	t.Cleanup(func() {
		if err := contentbusResultErr(controller.Close()); err != nil {
			t.Fatalf("close started controller: %v", err)
		}
	})

	ch, err := contentbusResultValue[<-chan Event](controller.Subscribe("broadcast.topic"))
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	if err := contentbusResultErr(controller.Publish("broadcast.topic", []byte("broadcast"))); err != nil {
		t.Fatalf("publish over started transport: %v", err)
	}

	event := readEvent(t, ch)
	if !core.DeepEqual(event.Payload, []byte("broadcast")) {
		t.Fatalf("payload: got %q, want %q", event.Payload, []byte("broadcast"))
	}
}

// ExampleController_Subscribe shows the subscribe/publish round-trip shape.
func ExampleController_Subscribe() {
	_ = Controller.Subscribe
	core.Println("Subscribe")
	// Output: Subscribe
}

// ExampleController_Publish shows the publish entry-point shape.
func ExampleController_Publish() {
	_ = Controller.Publish
	core.Println("Publish")
	// Output: Publish
}
