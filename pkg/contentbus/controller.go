// SPDX-License-Identifier: EUPL-1.2

// Package contentbus exposes a small publish/subscribe controller for P2P
// consumers that do not need the miner control surface.
package contentbus

import (
	"sync"
	"time"

	core "dappco.re/go"
	coreerr "dappco.re/go/log"
	p2pnode "dappco.re/go/p2p/node"
)

const contentEventMessageType p2pnode.MessageType = "content_event"

// Event is a content-bus message delivered to topic subscribers.
type Event struct {
	Topic     string
	Payload   []byte
	PeerID    string
	Timestamp time.Time
}

// Controller is the public content-bus API for non-mining P2P consumers.
type Controller interface {
	Subscribe(topic string) core.Result
	Publish(topic string, payload []byte) core.Result
	Close() core.Result
}

// Option configures a Controller created by NewController.
type Option func(*options) core.Result

type options struct {
	node            *p2pnode.NodeManager
	registry        *p2pnode.PeerRegistry
	transport       *p2pnode.Transport
	transportConfig p2pnode.TransportConfig
	channelBuffer   int
	startTransport  bool
}

type wireEvent struct {
	Topic   string `json:"topic"`
	Payload []byte `json:"payload"`
}

type controller struct {
	node             *p2pnode.NodeManager
	registry         *p2pnode.PeerRegistry
	transport        *p2pnode.Transport
	ownsRegistry     bool
	ownsTransport    bool
	startedTransport bool
	channelBuffer    int

	mu          sync.RWMutex
	subsMu      sync.RWMutex
	subscribers map[string]map[uint64]chan Event
	nextSubID   uint64
	closed      bool
	closeErr    core.Result
	closeOnce   sync.Once
}

var _ Controller = (*controller)(nil)

// ErrClosed is returned when an operation is attempted after Close.
var ErrClosed = coreerr.E("contentbus.Controller", "controller closed", nil)

// ErrInvalidTopic is returned when a caller passes an empty topic.
var ErrInvalidTopic = coreerr.E("contentbus.Controller", "topic is required", nil)

// NewController creates a content-bus controller backed by the existing P2P
// node transport. If no transport is supplied, NewController creates one and
// owns its shutdown. A supplied transport is used as-is and its OnMessage
// handler is set to the content-bus handler.
func NewController(opts ...Option) core.Result {
	cfg := options{
		transportConfig: p2pnode.DefaultTransportConfig(),
		channelBuffer:   16,
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if r := opt(&cfg); !r.OK {
			err, _ := r.Value.(error)
			return core.Fail(coreerr.E("contentbus.NewController", "apply option", err))
		}
	}

	ownsRegistry := false
	ownsTransport := false

	if cfg.transport != nil && cfg.node == nil {
		return core.Fail(coreerr.E("contentbus.NewController", "node manager is required when providing a transport", nil))
	}

	if cfg.node == nil {
		nmResult := loadOrCreateContentIdentity()
		if !nmResult.OK {
			err, _ := nmResult.Value.(error)
			return core.Fail(coreerr.E("contentbus.NewController", "load node identity", err))
		}
		cfg.node = nmResult.Value.(*p2pnode.NodeManager)
	}

	if cfg.transport == nil {
		if cfg.registry == nil {
			registryResult := p2pnode.NewPeerRegistry()
			if !registryResult.OK {
				err, _ := registryResult.Value.(error)
				return core.Fail(coreerr.E("contentbus.NewController", "create peer registry", err))
			}
			cfg.registry = registryResult.Value.(*p2pnode.PeerRegistry)
			ownsRegistry = true
		}

		cfg.transport = p2pnode.NewTransport(cfg.node, cfg.registry, cfg.transportConfig)
		ownsTransport = true
	}

	c := &controller{
		node:          cfg.node,
		registry:      cfg.registry,
		transport:     cfg.transport,
		ownsRegistry:  ownsRegistry,
		ownsTransport: ownsTransport,
		channelBuffer: cfg.channelBuffer,
		subscribers:   make(map[string]map[uint64]chan Event),
	}

	c.transport.OnMessage(c.handleMessage)

	if cfg.startTransport {
		if r := c.transport.Start(); !r.OK {
			startErr, _ := r.Value.(error)
			if c.ownsTransport {
				if stopResult := c.transport.Stop(); !stopResult.OK {
					stopErr, _ := stopResult.Value.(error)
					startErr = core.ErrorJoin(startErr, stopErr)
				}
			}
			if c.ownsRegistry {
				if closeResult := c.registry.Close(); !closeResult.OK {
					closeErr, _ := closeResult.Value.(error)
					startErr = core.ErrorJoin(startErr, closeErr)
				}
			}
			return core.Fail(coreerr.E("contentbus.NewController", "start transport", startErr))
		}
		c.startedTransport = true
	}

	return core.Ok(Controller(c))
}

func loadOrCreateContentIdentity() core.Result {
	nmResult := p2pnode.NewNodeManager()
	if !nmResult.OK {
		return nmResult
	}
	nm := nmResult.Value.(*p2pnode.NodeManager)
	if nm.HasIdentity() {
		return core.Ok(nm)
	}
	if r := nm.GenerateIdentity("contentbus-node", p2pnode.RoleController); !r.OK {
		return r
	}
	return core.Ok(nm)
}

// WithNodeManager uses an existing node identity manager.
func WithNodeManager(node *p2pnode.NodeManager) Option {
	return func(opts *options) core.Result {
		if node == nil {
			return core.Fail(coreerr.E("contentbus.WithNodeManager", "node manager is nil", nil))
		}
		opts.node = node
		return core.Ok(nil)
	}
}

// WithPeerRegistry uses an existing peer registry when NewController creates
// its own transport.
func WithPeerRegistry(registry *p2pnode.PeerRegistry) Option {
	return func(opts *options) core.Result {
		if registry == nil {
			return core.Fail(coreerr.E("contentbus.WithPeerRegistry", "peer registry is nil", nil))
		}
		opts.registry = registry
		return core.Ok(nil)
	}
}

// WithTransport uses an existing P2P transport. Pair this with WithNodeManager
// for the same node identity used by the transport.
func WithTransport(transport *p2pnode.Transport) Option {
	return func(opts *options) core.Result {
		if transport == nil {
			return core.Fail(coreerr.E("contentbus.WithTransport", "transport is nil", nil))
		}
		opts.transport = transport
		return core.Ok(nil)
	}
}

// WithTransportConfig sets the config used when NewController creates its own
// transport.
func WithTransportConfig(config p2pnode.TransportConfig) Option {
	return func(opts *options) core.Result {
		opts.transportConfig = config
		return core.Ok(nil)
	}
}

// WithChannelBuffer sets the per-subscriber channel buffer size.
func WithChannelBuffer(size int) Option {
	return func(opts *options) core.Result {
		if size < 1 {
			return core.Fail(coreerr.E("contentbus.WithChannelBuffer", "channel buffer must be at least 1", nil))
		}
		opts.channelBuffer = size
		return core.Ok(nil)
	}
}

// WithStartTransport controls whether NewController starts the configured
// transport before returning.
func WithStartTransport(start bool) Option {
	return func(opts *options) core.Result {
		opts.startTransport = start
		return core.Ok(nil)
	}
}

// Subscribe registers for events published on topic. The returned channel is
// closed when the controller is closed.
func (c *controller) Subscribe(topic string) core.Result {
	if topic == "" {
		return core.Fail(ErrInvalidTopic)
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return core.Fail(ErrClosed)
	}

	ch := make(chan Event, c.channelBuffer)

	c.subsMu.Lock()
	defer c.subsMu.Unlock()

	topicSubs := c.subscribers[topic]
	if topicSubs == nil {
		topicSubs = make(map[uint64]chan Event)
		c.subscribers[topic] = topicSubs
	}

	id := c.nextSubID
	c.nextSubID++
	topicSubs[id] = ch

	return core.Ok((<-chan Event)(ch))
}

// Publish delivers payload to local subscribers of topic and broadcasts it to
// connected P2P peers.
func (c *controller) Publish(topic string, payload []byte) core.Result {
	if topic == "" {
		return core.Fail(ErrInvalidTopic)
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return core.Fail(ErrClosed)
	}

	identity := c.node.GetIdentity()
	if identity == nil {
		return core.Fail(p2pnode.ErrIdentityNotInitialized)
	}

	event := Event{
		Topic:     topic,
		Payload:   cloneBytes(payload),
		PeerID:    identity.ID,
		Timestamp: time.Now(),
	}

	c.deliver(event)

	msgResult := p2pnode.NewMessage(contentEventMessageType, identity.ID, "", wireEvent{
		Topic:   topic,
		Payload: event.Payload,
	})
	if !msgResult.OK {
		err, _ := msgResult.Value.(error)
		return core.Fail(coreerr.E("contentbus.Controller.Publish", "create content event message", err))
	}
	msg := msgResult.Value.(*p2pnode.Message)

	if r := c.transport.Broadcast(msg); !r.OK {
		err, _ := r.Value.(error)
		return core.Fail(coreerr.E("contentbus.Controller.Publish", "broadcast content event", err))
	}

	return core.Ok(nil)
}

// Close releases controller-owned resources and closes all subscription
// channels. It is safe to call multiple times.
func (c *controller) Close() core.Result {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.closed = true

		c.subsMu.Lock()
		for topic, topicSubs := range c.subscribers {
			for id, ch := range topicSubs {
				close(ch)
				delete(topicSubs, id)
			}
			delete(c.subscribers, topic)
		}
		c.subsMu.Unlock()
		c.mu.Unlock()

		if c.ownsTransport || c.startedTransport {
			if r := c.transport.Stop(); !r.OK {
				err, _ := r.Value.(error)
				c.closeErr = core.Fail(coreerr.E("contentbus.Controller.Close", "stop transport", err))
				return
			}
		}

		if c.ownsRegistry {
			if r := c.registry.Close(); !r.OK {
				err, _ := r.Value.(error)
				c.closeErr = core.Fail(coreerr.E("contentbus.Controller.Close", "close peer registry", err))
			}
		}
	})

	if c.closeErr.Value == nil && !c.closeErr.OK {
		return core.Ok(nil)
	}
	return c.closeErr
}

func (c *controller) handleMessage(_ *p2pnode.PeerConnection, msg *p2pnode.Message) {
	if msg == nil || msg.Type != contentEventMessageType {
		return
	}

	var payload wireEvent
	if r := msg.ParsePayload(&payload); !r.OK || payload.Topic == "" {
		return
	}

	timestamp := msg.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return
	}

	c.deliver(Event{
		Topic:     payload.Topic,
		Payload:   cloneBytes(payload.Payload),
		PeerID:    msg.From,
		Timestamp: timestamp,
	})
}

func (c *controller) deliver(event Event) {
	c.subsMu.RLock()
	defer c.subsMu.RUnlock()

	for _, ch := range c.subscribers[event.Topic] {
		select {
		case ch <- cloneEvent(event):
		default:
		}
	}
}

func cloneEvent(event Event) Event {
	event.Payload = cloneBytes(event.Payload)
	return event
}

func cloneBytes(payload []byte) []byte {
	if payload == nil {
		return nil
	}
	return append([]byte(nil), payload...)
}
