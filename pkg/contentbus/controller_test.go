// SPDX-License-Identifier: EUPL-1.2

package contentbus

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	p2pnode "dappco.re/go/p2p/node"
)

func newTestController(t *testing.T) Controller {
	t.Helper()

	dir := t.TempDir()
	nm, err := p2pnode.NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("contentbus-test", p2pnode.RoleController); err != nil {
		t.Fatalf("generate identity: %v", err)
	}

	registry, err := p2pnode.NewPeerRegistryWithPath(filepath.Join(dir, "peers.json"))
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

func TestController_SubscribePublishSameTopic_Good(t *testing.T) {
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
	if !bytes.Equal(event.Payload, []byte("hello")) {
		t.Fatalf("payload: got %q, want %q", event.Payload, []byte("hello"))
	}
	if event.PeerID == "" {
		t.Fatal("peer id should be set")
	}
	if event.Timestamp.IsZero() {
		t.Fatal("timestamp should be set")
	}
}

func TestController_SubscribeTopicAPublishTopicB_Good(t *testing.T) {
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

func TestController_CloseWhileSubscribed_Good(t *testing.T) {
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

func TestController_TwoConcurrentSubscribersSameTopic_Good(t *testing.T) {
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

	if !bytes.Equal(event1.Payload, []byte("payload")) {
		t.Fatalf("first payload: got %q, want %q", event1.Payload, []byte("payload"))
	}
	if !bytes.Equal(event2.Payload, []byte("payload")) {
		t.Fatalf("second payload: got %q, want %q", event2.Payload, []byte("payload"))
	}
}
