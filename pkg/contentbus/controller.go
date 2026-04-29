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
	Subscribe(topic string) (<-chan Event, error)
	Publish(topic string, payload []byte) error
	Close() error
}

// Option configures a Controller created by NewController.
type Option func(*options) error

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
	closeErr    error
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
func NewController(opts ...Option) (Controller, error) {
	cfg := options{
		transportConfig: p2pnode.DefaultTransportConfig(),
		channelBuffer:   16,
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&cfg); err != nil {
			return nil, coreerr.E("contentbus.NewController", "apply option", err)
		}
	}

	ownsRegistry := false
	ownsTransport := false

	if cfg.transport != nil && cfg.node == nil {
		return nil, coreerr.E("contentbus.NewController", "node manager is required when providing a transport", nil)
	}

	if cfg.node == nil {
		nm, err := loadOrCreateContentIdentity()
		if err != nil {
			return nil, coreerr.E("contentbus.NewController", "load node identity", err)
		}
		cfg.node = nm
	}

	if cfg.transport == nil {
		if cfg.registry == nil {
			registry, err := p2pnode.NewPeerRegistry()
			if err != nil {
				return nil, coreerr.E("contentbus.NewController", "create peer registry", err)
			}
			cfg.registry = registry
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
		if err := c.transport.Start(); err != nil {
			startErr := err
			if c.ownsTransport {
				if stopErr := c.transport.Stop(); stopErr != nil {
					startErr = core.ErrorJoin(startErr, stopErr)
				}
			}
			if c.ownsRegistry {
				if closeErr := c.registry.Close(); closeErr != nil {
					startErr = core.ErrorJoin(startErr, closeErr)
				}
			}
			return nil, coreerr.E("contentbus.NewController", "start transport", startErr)
		}
		c.startedTransport = true
	}

	return c, nil
}

func loadOrCreateContentIdentity() (*p2pnode.NodeManager, error) {
	nm, err := p2pnode.NewNodeManager()
	if err != nil {
		return nil, err
	}
	if nm.HasIdentity() {
		return nm, nil
	}
	if err := nm.GenerateIdentity("contentbus-node", p2pnode.RoleController); err != nil {
		return nil, err
	}
	return nm, nil
}

// WithNodeManager uses an existing node identity manager.
func WithNodeManager(node *p2pnode.NodeManager) Option {
	return func(opts *options) error {
		if node == nil {
			return coreerr.E("contentbus.WithNodeManager", "node manager is nil", nil)
		}
		opts.node = node
		return nil
	}
}

// WithPeerRegistry uses an existing peer registry when NewController creates
// its own transport.
func WithPeerRegistry(registry *p2pnode.PeerRegistry) Option {
	return func(opts *options) error {
		if registry == nil {
			return coreerr.E("contentbus.WithPeerRegistry", "peer registry is nil", nil)
		}
		opts.registry = registry
		return nil
	}
}

// WithTransport uses an existing P2P transport. Pair this with WithNodeManager
// for the same node identity used by the transport.
func WithTransport(transport *p2pnode.Transport) Option {
	return func(opts *options) error {
		if transport == nil {
			return coreerr.E("contentbus.WithTransport", "transport is nil", nil)
		}
		opts.transport = transport
		return nil
	}
}

// WithTransportConfig sets the config used when NewController creates its own
// transport.
func WithTransportConfig(config p2pnode.TransportConfig) Option {
	return func(opts *options) error {
		opts.transportConfig = config
		return nil
	}
}

// WithChannelBuffer sets the per-subscriber channel buffer size.
func WithChannelBuffer(size int) Option {
	return func(opts *options) error {
		if size < 1 {
			return coreerr.E("contentbus.WithChannelBuffer", "channel buffer must be at least 1", nil)
		}
		opts.channelBuffer = size
		return nil
	}
}

// WithStartTransport controls whether NewController starts the configured
// transport before returning.
func WithStartTransport(start bool) Option {
	return func(opts *options) error {
		opts.startTransport = start
		return nil
	}
}

// Subscribe registers for events published on topic. The returned channel is
// closed when the controller is closed.
func (c *controller) Subscribe(topic string) (<-chan Event, error) {
	if topic == "" {
		return nil, ErrInvalidTopic
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return nil, ErrClosed
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

	return ch, nil
}

// Publish delivers payload to local subscribers of topic and broadcasts it to
// connected P2P peers.
func (c *controller) Publish(topic string, payload []byte) error {
	if topic == "" {
		return ErrInvalidTopic
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return ErrClosed
	}

	identity := c.node.GetIdentity()
	if identity == nil {
		return p2pnode.ErrIdentityNotInitialized
	}

	event := Event{
		Topic:     topic,
		Payload:   cloneBytes(payload),
		PeerID:    identity.ID,
		Timestamp: time.Now(),
	}

	c.deliver(event)

	msg, err := p2pnode.NewMessage(contentEventMessageType, identity.ID, "", wireEvent{
		Topic:   topic,
		Payload: event.Payload,
	})
	if err != nil {
		return coreerr.E("contentbus.Controller.Publish", "create content event message", err)
	}

	if err := c.transport.Broadcast(msg); err != nil {
		return coreerr.E("contentbus.Controller.Publish", "broadcast content event", err)
	}

	return nil
}

// Close releases controller-owned resources and closes all subscription
// channels. It is safe to call multiple times.
func (c *controller) Close() error {
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
			if err := c.transport.Stop(); err != nil {
				c.closeErr = coreerr.E("contentbus.Controller.Close", "stop transport", err)
				return
			}
		}

		if c.ownsRegistry {
			if err := c.registry.Close(); err != nil {
				c.closeErr = coreerr.E("contentbus.Controller.Close", "close peer registry", err)
			}
		}
	})

	return c.closeErr
}

func (c *controller) handleMessage(_ *p2pnode.PeerConnection, msg *p2pnode.Message) {
	if msg == nil || msg.Type != contentEventMessageType {
		return
	}

	var payload wireEvent
	if err := msg.ParsePayload(&payload); err != nil || payload.Topic == "" {
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
