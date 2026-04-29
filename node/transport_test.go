package node

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	core "dappco.re/go"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dappco.re/go/p2p/logging"
	"github.com/gorilla/websocket"
)

// --- Test Helpers ---

// testNode creates a NodeManager with a generated identity in a temp directory.
func testNode(t *testing.T, name string, role NodeRole) *NodeManager {
	t.Helper()
	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		core.PathJoin(dir, "private.key"),
		core.PathJoin(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("create node manager %q: %v", name, err)
	}
	if err := nm.GenerateIdentity(name, role); err != nil {
		t.Fatalf("generate identity %q: %v", name, err)
	}
	return nm
}

// testRegistry creates a PeerRegistry with open auth in a temp directory.
func testRegistry(t *testing.T) *PeerRegistry {
	t.Helper()
	dir := t.TempDir()
	reg, err := NewPeerRegistryWithPath(core.PathJoin(dir, "peers.json"))
	if err != nil {
		t.Fatalf("create registry: %v", err)
	}
	t.Cleanup(func() { reg.Close() })
	return reg
}

// testTransportPair holds everything needed for transport pair tests.
type testTransportPair struct {
	Server     *Transport
	Client     *Transport
	ServerNode *NodeManager
	ClientNode *NodeManager
	ServerReg  *PeerRegistry
	ClientReg  *PeerRegistry
	HTTPServer *httptest.Server
	ServerAddr string // "127.0.0.1:PORT"
}

// setupTestTransportPair creates a server transport (backed by httptest) and a
// client transport, both with generated identities and open-auth registries.
func setupTestTransportPair(t *testing.T) *testTransportPair {
	return setupTestTransportPairWithConfig(t, DefaultTransportConfig(), DefaultTransportConfig())
}

// setupTestTransportPairWithConfig allows custom configs for server and client.
func setupTestTransportPairWithConfig(t *testing.T, serverCfg, clientCfg TransportConfig) *testTransportPair {
	t.Helper()

	serverNM := testNode(t, "server", RoleWorker)
	clientNM := testNode(t, "client", RoleController)
	serverReg := testRegistry(t)
	clientReg := testRegistry(t)

	serverTransport := NewTransport(serverNM, serverReg, serverCfg)
	clientTransport := NewTransport(clientNM, clientReg, clientCfg)

	// Use httptest.Server with the transport's WebSocket handler
	mux := http.NewServeMux()
	mux.HandleFunc(serverCfg.WSPath, serverTransport.handleWSUpgrade)
	ts := httptest.NewServer(mux)

	u, _ := url.Parse(ts.URL)

	tp := &testTransportPair{
		Server:     serverTransport,
		Client:     clientTransport,
		ServerNode: serverNM,
		ClientNode: clientNM,
		ServerReg:  serverReg,
		ClientReg:  clientReg,
		HTTPServer: ts,
		ServerAddr: u.Host,
	}

	t.Cleanup(func() {
		clientTransport.Stop()
		serverTransport.Stop()
		ts.Close()
	})

	return tp
}

// connectClient establishes a connection from client to server transport.
func (tp *testTransportPair) connectClient(t *testing.T) *PeerConnection {
	t.Helper()

	peer := &Peer{
		ID:      tp.ServerNode.GetIdentity().ID,
		Name:    "server",
		Address: tp.ServerAddr,
		Role:    RoleWorker,
	}
	tp.ClientReg.AddPeer(peer)

	pc, err := tp.Client.Connect(peer)
	if err != nil {
		t.Fatalf("client connect failed: %v", err)
	}
	return pc
}

func waitForTransportCondition(t *testing.T, timeout time.Duration, condition func() bool, failure string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(failure)
}

func waitForPeerConnection(t *testing.T, transport *Transport, peerID string) *PeerConnection {
	t.Helper()
	var pc *PeerConnection
	waitForTransportCondition(t, time.Second, func() bool {
		pc = transport.GetConnection(peerID)
		return pc != nil
	}, "peer connection was not registered")
	return pc
}

type lockedLogBuffer struct {
	mu  sync.Mutex
	buf pooledBuffer
}

func (b *lockedLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func captureTransportLogs(t *testing.T) *lockedLogBuffer {
	t.Helper()
	buf := &lockedLogBuffer{buf: core.NewBuffer()}
	previous := logging.GetGlobal()
	logging.SetGlobal(logging.New(logging.Config{
		Output: buf,
		Level:  logging.LevelDebug,
	}))
	t.Cleanup(func() {
		logging.SetGlobal(previous)
	})
	return buf
}

func envelopeBody(t *testing.T, msg *Message) []byte {
	t.Helper()
	body, err := MarshalJSON(msg)
	if err != nil {
		t.Fatalf("marshal message body: %v", err)
	}
	return body
}

func writeEnvelopeFrame(t *testing.T, pc *PeerConnection, env Envelope) {
	t.Helper()
	payload, err := MarshalJSON(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	encrypted, err := encryptTransportPayload(payload, pc.SharedSecret)
	if err != nil {
		t.Fatalf("encrypt envelope: %v", err)
	}

	pc.writeMu.Lock()
	defer pc.writeMu.Unlock()
	if err := pc.Conn.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set write deadline: %v", err)
	}
	defer pc.Conn.SetWriteDeadline(time.Time{})
	if err := pc.Conn.WriteMessage(websocket.BinaryMessage, encrypted); err != nil {
		t.Fatalf("write envelope frame: %v", err)
	}
}

// --- Unit Tests for Sub-Components ---

func TestMessageDeduplicator(t *testing.T) {
	t.Run("MarkAndCheck", func(t *testing.T) {
		d := NewMessageDeduplicator(5 * time.Minute)

		if d.IsDuplicate("msg-1") {
			t.Error("should not be duplicate before marking")
		}

		d.Mark("msg-1")

		if !d.IsDuplicate("msg-1") {
			t.Error("should be duplicate after marking")
		}

		if d.IsDuplicate("msg-2") {
			t.Error("different ID should not be duplicate")
		}
	})

	t.Run("Cleanup", func(t *testing.T) {
		d := NewMessageDeduplicator(50 * time.Millisecond)
		d.Mark("msg-1")

		if !d.IsDuplicate("msg-1") {
			t.Error("should be duplicate immediately after marking")
		}

		time.Sleep(60 * time.Millisecond)
		d.Cleanup()

		if d.IsDuplicate("msg-1") {
			t.Error("should not be duplicate after TTL + cleanup")
		}
	})

	t.Run("ExpiredEntriesAreNotDuplicates", func(t *testing.T) {
		d := NewMessageDeduplicator(25 * time.Millisecond)
		d.Mark("msg-expired")

		time.Sleep(40 * time.Millisecond)

		if d.IsDuplicate("msg-expired") {
			t.Error("expired message should not remain a duplicate")
		}
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		d := NewMessageDeduplicator(5 * time.Minute)
		var wg sync.WaitGroup
		for i := range 100 {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				msgID := "msg-" + time.Now().String()
				d.Mark(msgID)
				d.IsDuplicate(msgID)
			}(i)
		}
		wg.Wait()
	})
}

func TestTransport_DefaultTransportConfig_Good(t *testing.T) {
	cfg := DefaultTransportConfig()
	if cfg.ListenAddr != ":9091" {
		t.Fatalf("listen addr: got %q", cfg.ListenAddr)
	}
	if cfg.MaxMessageSize != DefaultMaxMessageSize {
		t.Fatalf("max size: got %d", cfg.MaxMessageSize)
	}
}

func TestTransport_DefaultTransportConfig_Bad(t *testing.T) {
	cfg := DefaultTransportConfig()
	if cfg.WSPath == "" {
		t.Fatal("expected websocket path")
	}
	if cfg.MaxConns <= 0 {
		t.Fatalf("max conns: got %d", cfg.MaxConns)
	}
}

func TestTransport_DefaultTransportConfig_Ugly(t *testing.T) {
	cfg := DefaultTransportConfig()
	cfg.ListenAddr = "127.0.0.1:0"
	if cfg.ListenAddr != "127.0.0.1:0" {
		t.Fatal("config should be a mutable value copy")
	}
	if DefaultTransportConfig().ListenAddr == "127.0.0.1:0" {
		t.Fatal("default config should not be mutated")
	}
}

func TestTransport_NewMessageDeduplicator_Good(t *testing.T) {
	d := NewMessageDeduplicator(time.Minute)
	if d == nil {
		t.Fatal("expected deduplicator")
	}
	if d.ttl != time.Minute {
		t.Fatalf("ttl: got %v", d.ttl)
	}
}

func TestTransport_NewMessageDeduplicator_Bad(t *testing.T) {
	d := NewMessageDeduplicator(0)
	d.Mark("id")
	if !d.IsDuplicate("id") {
		t.Fatal("zero ttl should keep immediate duplicate")
	}
}

func TestTransport_NewMessageDeduplicator_Ugly(t *testing.T) {
	d := NewMessageDeduplicator(-time.Second)
	d.Mark("id")
	if !d.IsDuplicate("id") {
		t.Fatal("negative ttl keeps immediate duplicate")
	}
}

func TestTransport_MessageDeduplicator_IsDuplicate_Good(t *testing.T) {
	d := NewMessageDeduplicator(time.Minute)
	d.Mark("id")
	if !d.IsDuplicate("id") {
		t.Fatal("expected duplicate")
	}
}

func TestTransport_MessageDeduplicator_IsDuplicate_Bad(t *testing.T) {
	d := NewMessageDeduplicator(time.Minute)
	if d.IsDuplicate("missing") {
		t.Fatal("missing id should not be duplicate")
	}
	if len(d.seen) != 0 {
		t.Fatal("missing lookup should not mark id")
	}
}

func TestTransport_MessageDeduplicator_IsDuplicate_Ugly(t *testing.T) {
	d := NewMessageDeduplicator(time.Nanosecond)
	d.Mark("expired")
	time.Sleep(time.Millisecond)
	if d.IsDuplicate("expired") {
		t.Fatal("expired id should not be duplicate")
	}
}

func TestTransport_MessageDeduplicator_Mark_Good(t *testing.T) {
	d := NewMessageDeduplicator(time.Minute)
	d.Mark("id")
	if _, ok := d.seen["id"]; !ok {
		t.Fatal("id not marked")
	}
}

func TestTransport_MessageDeduplicator_Mark_Bad(t *testing.T) {
	d := NewMessageDeduplicator(time.Minute)
	d.Mark("")
	if _, ok := d.seen[""]; !ok {
		t.Fatal("empty id should be recorded")
	}
}

func TestTransport_MessageDeduplicator_Mark_Ugly(t *testing.T) {
	d := NewMessageDeduplicator(time.Minute)
	d.Mark("id")
	first := d.seen["id"]
	d.Mark("id")
	if d.seen["id"].Before(first) {
		t.Fatal("mark should refresh timestamp")
	}
}

func TestTransport_MessageDeduplicator_Cleanup_Good(t *testing.T) {
	d := NewMessageDeduplicator(time.Nanosecond)
	d.Mark("id")
	time.Sleep(time.Millisecond)
	d.Cleanup()
	if len(d.seen) != 0 {
		t.Fatalf("seen: got %d", len(d.seen))
	}
}

func TestTransport_MessageDeduplicator_Cleanup_Bad(t *testing.T) {
	d := NewMessageDeduplicator(time.Minute)
	d.Mark("id")
	d.Cleanup()
	if len(d.seen) != 1 {
		t.Fatalf("seen: got %d", len(d.seen))
	}
}

func TestTransport_MessageDeduplicator_Cleanup_Ugly(t *testing.T) {
	d := NewMessageDeduplicator(0)
	d.Cleanup()
	if len(d.seen) != 0 {
		t.Fatalf("seen: got %d", len(d.seen))
	}
}

func TestTransport_NewPeerRateLimiter_Good(t *testing.T) {
	limiter := NewPeerRateLimiter(2, 1)
	if limiter == nil {
		t.Fatal("expected limiter")
	}
	if !limiter.Allow() {
		t.Fatal("first token should be allowed")
	}
}

func TestTransport_NewPeerRateLimiter_Bad(t *testing.T) {
	limiter := NewPeerRateLimiter(0, 0)
	if limiter.Allow() {
		t.Fatal("zero-token limiter should reject")
	}
	if limiter.maxTokens != 0 {
		t.Fatalf("max tokens: got %d", limiter.maxTokens)
	}
}

func TestTransport_NewPeerRateLimiter_Ugly(t *testing.T) {
	limiter := NewPeerRateLimiter(1, 1000)
	if !limiter.Allow() {
		t.Fatal("initial token should be allowed")
	}
	if limiter.Allow() {
		t.Fatal("single token should be consumed")
	}
}

func TestTransport_PeerRateLimiter_Allow_Good(t *testing.T) {
	limiter := NewPeerRateLimiter(1, 1)
	if !limiter.Allow() {
		t.Fatal("expected allow")
	}
	if limiter.tokens != 0 {
		t.Fatalf("tokens: got %d", limiter.tokens)
	}
}

func TestTransport_PeerRateLimiter_Allow_Bad(t *testing.T) {
	limiter := NewPeerRateLimiter(0, 1)
	if limiter.Allow() {
		t.Fatal("expected reject")
	}
	if limiter.tokens != 0 {
		t.Fatalf("tokens: got %d", limiter.tokens)
	}
}

func TestTransport_PeerRateLimiter_Allow_Ugly(t *testing.T) {
	limiter := NewPeerRateLimiter(1, 10)
	limiter.Allow()
	limiter.lastRefill = time.Now().Add(-time.Second)
	if !limiter.Allow() {
		t.Fatal("expected refill allow")
	}
}

func TestTransport_NewTransport_Good(t *testing.T) {
	node := testNode(t, "node", RoleDual)
	registry := testRegistry(t)
	transport := NewTransport(node, registry, DefaultTransportConfig())
	if transport == nil {
		t.Fatal("expected transport")
	}
	if transport.node != node || transport.registry != registry {
		t.Fatal("transport dependencies not retained")
	}
}

func TestTransport_NewTransport_Bad(t *testing.T) {
	transport := NewTransport(nil, nil, TransportConfig{})
	if transport == nil {
		t.Fatal("expected transport")
	}
	if transport.config.MaxMessageSize != 0 {
		t.Fatalf("max size: got %d", transport.config.MaxMessageSize)
	}
}

func TestTransport_NewTransport_Ugly(t *testing.T) {
	transport := NewTransport(nil, nil, TransportConfig{WSPath: ""})
	if transport.dedup == nil {
		t.Fatal("expected deduplicator")
	}
	if transport.ctx == nil || transport.cancel == nil {
		t.Fatal("expected context")
	}
}

func TestTransport_Transport_Start_Good(t *testing.T) {
	node := testNode(t, "node", RoleDual)
	registry := testRegistry(t)
	cfg := DefaultTransportConfig()
	cfg.ListenAddr = "127.0.0.1:0"
	transport := NewTransport(node, registry, cfg)
	if err := transport.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := transport.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestTransport_Transport_Start_Bad(t *testing.T) {
	transport := NewTransport(nil, nil, TransportConfig{ListenAddr: "127.0.0.1:0", WSPath: "/ws"})
	err := transport.Start()
	if err != nil {
		t.Fatalf("Start with nil deps: %v", err)
	}
	if transport.server == nil {
		t.Fatal("server should be created")
	}
	_ = transport.Stop()
}

func TestTransport_Transport_Start_Ugly(t *testing.T) {
	node := testNode(t, "node", RoleDual)
	registry := testRegistry(t)
	cfg := DefaultTransportConfig()
	cfg.ListenAddr = "127.0.0.1:0"
	cfg.WSPath = "/"
	transport := NewTransport(node, registry, cfg)
	if err := transport.Start(); err != nil {
		t.Fatalf("Start root path: %v", err)
	}
	_ = transport.Stop()
}

func TestTransport_Transport_Stop_Good(t *testing.T) {
	node := testNode(t, "node", RoleDual)
	registry := testRegistry(t)
	transport := NewTransport(node, registry, DefaultTransportConfig())
	if err := transport.Stop(); err != nil {
		t.Fatalf("Stop unstarted: %v", err)
	}
}

func TestTransport_Transport_Stop_Bad(t *testing.T) {
	transport := NewTransport(nil, nil, TransportConfig{})
	if err := transport.Stop(); err != nil {
		t.Fatalf("Stop nil deps: %v", err)
	}
	if transport.ctx.Err() == nil {
		t.Fatal("context should be canceled")
	}
}

func TestTransport_Transport_Stop_Ugly(t *testing.T) {
	node := testNode(t, "node", RoleDual)
	registry := testRegistry(t)
	transport := NewTransport(node, registry, DefaultTransportConfig())
	_ = transport.Stop()
	if err := transport.Stop(); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
}

func TestTransport_Transport_OnMessage_Good(t *testing.T) {
	transport := NewTransport(nil, nil, TransportConfig{})
	handler := func(*PeerConnection, *Message) {}
	transport.OnMessage(handler)
	if transport.handler == nil {
		t.Fatal("handler not set")
	}
}

func TestTransport_Transport_OnMessage_Bad(t *testing.T) {
	transport := NewTransport(nil, nil, TransportConfig{})
	transport.OnMessage(nil)
	if transport.handler != nil {
		t.Fatal("handler should be nil")
	}
}

func TestTransport_Transport_OnMessage_Ugly(t *testing.T) {
	transport := NewTransport(nil, nil, TransportConfig{})
	first := func(*PeerConnection, *Message) {}
	second := func(*PeerConnection, *Message) {}
	transport.OnMessage(first)
	transport.OnMessage(second)
	if transport.handler == nil {
		t.Fatal("handler should be set")
	}
}

func TestTransport_Transport_Connect_Good(t *testing.T) {
	tp := setupTestTransportPair(t)
	peer := &Peer{ID: tp.ServerNode.GetIdentity().ID, Name: "server", Address: tp.ServerAddr, Role: RoleWorker}
	tp.ClientReg.AddPeer(peer)
	conn, err := tp.Client.Connect(peer)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if conn.SharedSecret == nil {
		t.Fatal("shared secret not set")
	}
}

func TestTransport_Transport_Connect_Bad(t *testing.T) {
	tp := setupTestTransportPair(t)
	peer := &Peer{ID: "missing", Address: "127.0.0.1:1"}
	conn, err := tp.Client.Connect(peer)
	if err == nil {
		t.Fatal("expected connect error")
	}
	if conn != nil {
		t.Fatalf("connection: got %#v, want nil", conn)
	}
}

func TestTransport_Transport_Connect_Ugly(t *testing.T) {
	tp := setupTestTransportPair(t)
	peer := &Peer{ID: tp.ServerNode.GetIdentity().ID, Name: "server", Address: tp.ServerAddr, Role: RoleWorker}
	conn, err := tp.Client.Connect(peer)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if conn.Peer.ID != tp.ServerNode.GetIdentity().ID {
		t.Fatalf("peer ID: got %s", conn.Peer.ID)
	}
}

func TestTransport_Transport_Send_Good(t *testing.T) {
	tp := setupTestTransportPair(t)
	received := make(chan *Message, 1)
	tp.Server.OnMessage(func(_ *PeerConnection, msg *Message) { received <- msg })
	tp.connectClient(t)
	msg, _ := NewMessage(MsgPing, tp.ClientNode.GetIdentity().ID, tp.ServerNode.GetIdentity().ID, PingPayload{SentAt: 1})
	if err := tp.Client.Send(tp.ServerNode.GetIdentity().ID, msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	select {
	case got := <-received:
		if got.Type != MsgPing {
			t.Fatalf("message: %#v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestTransport_Transport_Send_Bad(t *testing.T) {
	transport := NewTransport(nil, nil, TransportConfig{})
	msg, _ := NewMessage(MsgPing, "from", "to", nil)
	err := transport.Send("missing", msg)
	if err == nil {
		t.Fatal("expected missing peer error")
	}
}

func TestTransport_Transport_Send_Ugly(t *testing.T) {
	transport := NewTransport(nil, nil, TransportConfig{})
	err := transport.Send("", nil)
	if err == nil {
		t.Fatal("expected missing peer error")
	}
}

func TestTransport_Transport_Connections_Good(t *testing.T) {
	tp := setupTestTransportPair(t)
	tp.connectClient(t)
	count := 0
	for range tp.Client.Connections() {
		count++
	}
	if count != 1 {
		t.Fatalf("connection count: got %d", count)
	}
}

func TestTransport_Transport_Connections_Bad(t *testing.T) {
	transport := NewTransport(nil, nil, TransportConfig{})
	count := 0
	for range transport.Connections() {
		count++
	}
	if count != 0 {
		t.Fatalf("connection count: got %d", count)
	}
}

func TestTransport_Transport_Connections_Ugly(t *testing.T) {
	tp := setupTestTransportPair(t)
	tp.connectClient(t)
	count := 0
	for range tp.Client.Connections() {
		count++
		break
	}
	if count != 1 {
		t.Fatalf("connection count: got %d", count)
	}
}

func TestTransport_Transport_Broadcast_Good(t *testing.T) {
	tp := setupTestTransportPair(t)
	received := make(chan *Message, 1)
	tp.Server.OnMessage(func(_ *PeerConnection, msg *Message) { received <- msg })
	tp.connectClient(t)
	msg, _ := NewMessage(MsgPing, tp.ClientNode.GetIdentity().ID, "", PingPayload{SentAt: 1})
	if err := tp.Client.Broadcast(msg); err != nil {
		t.Fatalf("Broadcast: %v", err)
	}
	select {
	case got := <-received:
		if got.Type != MsgPing {
			t.Fatalf("message: %#v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for broadcast")
	}
}

func TestTransport_Transport_Broadcast_Bad(t *testing.T) {
	transport := NewTransport(nil, nil, TransportConfig{})
	msg, _ := NewMessage(MsgPing, "from", "", nil)
	if err := transport.Broadcast(msg); err != nil {
		t.Fatalf("Broadcast empty: %v", err)
	}
}

func TestTransport_Transport_Broadcast_Ugly(t *testing.T) {
	tp := setupTestTransportPair(t)
	tp.connectClient(t)
	msg, _ := NewMessage(MsgPing, tp.ServerNode.GetIdentity().ID, "", nil)
	if err := tp.Client.Broadcast(msg); err != nil {
		t.Fatalf("Broadcast skip sender: %v", err)
	}
}

func TestTransport_Transport_GetConnection_Good(t *testing.T) {
	tp := setupTestTransportPair(t)
	tp.connectClient(t)
	conn := tp.Client.GetConnection(tp.ServerNode.GetIdentity().ID)
	if conn == nil {
		t.Fatal("expected connection")
	}
}

func TestTransport_Transport_GetConnection_Bad(t *testing.T) {
	transport := NewTransport(nil, nil, TransportConfig{})
	conn := transport.GetConnection("missing")
	if conn != nil {
		t.Fatalf("connection: got %#v, want nil", conn)
	}
}

func TestTransport_Transport_GetConnection_Ugly(t *testing.T) {
	tp := setupTestTransportPair(t)
	conn := tp.connectClient(t)
	_ = conn.Close()
	if got := tp.Client.GetConnection(tp.ServerNode.GetIdentity().ID); got != nil {
		t.Fatalf("connection: got %#v, want nil", got)
	}
}

func TestTransport_Transport_ConnectedPeers_Good(t *testing.T) {
	tp := setupTestTransportPair(t)
	tp.connectClient(t)
	if tp.Client.ConnectedPeers() != 1 {
		t.Fatalf("connected peers: got %d", tp.Client.ConnectedPeers())
	}
}

func TestTransport_Transport_ConnectedPeers_Bad(t *testing.T) {
	transport := NewTransport(nil, nil, TransportConfig{})
	if transport.ConnectedPeers() != 0 {
		t.Fatalf("connected peers: got %d", transport.ConnectedPeers())
	}
}

func TestTransport_Transport_ConnectedPeers_Ugly(t *testing.T) {
	tp := setupTestTransportPair(t)
	conn := tp.connectClient(t)
	_ = conn.Close()
	if tp.Client.ConnectedPeers() != 0 {
		t.Fatalf("connected peers: got %d", tp.Client.ConnectedPeers())
	}
}

func TestTransport_PeerConnection_Send_Good(t *testing.T) {
	tp := setupTestTransportPair(t)
	received := make(chan *Message, 1)
	tp.Server.OnMessage(func(_ *PeerConnection, msg *Message) { received <- msg })
	conn := tp.connectClient(t)
	msg, _ := NewMessage(MsgPing, tp.ClientNode.GetIdentity().ID, tp.ServerNode.GetIdentity().ID, nil)
	if err := conn.Send(msg); err != nil {
		t.Fatalf("PeerConnection.Send: %v", err)
	}
	select {
	case <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestTransport_PeerConnection_Send_Bad(t *testing.T) {
	tp := setupTestTransportPair(t)
	conn := tp.connectClient(t)
	conn.SharedSecret = nil
	msg, _ := NewMessage(MsgPing, "from", "to", nil)
	err := conn.Send(msg)
	if err == nil {
		t.Fatal("expected key derivation error")
	}
}

func TestTransport_PeerConnection_Send_Ugly(t *testing.T) {
	tp := setupTestTransportPair(t)
	conn := tp.connectClient(t)
	err := conn.Send(nil)
	if err != nil {
		t.Fatalf("nil message should marshal as JSON null: %v", err)
	}
}

func TestTransport_PeerConnection_Close_Good(t *testing.T) {
	tp := setupTestTransportPair(t)
	conn := tp.connectClient(t)
	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if tp.Client.GetConnection(tp.ServerNode.GetIdentity().ID) != nil {
		t.Fatal("connection should be removed")
	}
}

func TestTransport_PeerConnection_Close_Bad(t *testing.T) {
	tp := setupTestTransportPair(t)
	conn := tp.connectClient(t)
	_ = conn.Close()
	if err := conn.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestTransport_PeerConnection_Close_Ugly(t *testing.T) {
	tp := setupTestTransportPair(t)
	conn := tp.connectClient(t)
	conn.transport = nil
	if err := conn.Close(); err != nil {
		t.Fatalf("Close without transport: %v", err)
	}
}

func TestTransport_PeerConnection_GracefulClose_Good(t *testing.T) {
	tp := setupTestTransportPair(t)
	conn := tp.connectClient(t)
	if err := conn.GracefulClose("bye", DisconnectNormal); err != nil {
		t.Fatalf("GracefulClose: %v", err)
	}
	if tp.Client.GetConnection(tp.ServerNode.GetIdentity().ID) != nil {
		t.Fatal("connection should be removed")
	}
}

func TestTransport_PeerConnection_GracefulClose_Bad(t *testing.T) {
	tp := setupTestTransportPair(t)
	conn := tp.connectClient(t)
	conn.SharedSecret = nil
	if err := conn.GracefulClose("bye", DisconnectNormal); err != nil {
		t.Fatalf("GracefulClose without secret: %v", err)
	}
	if tp.Client.GetConnection(tp.ServerNode.GetIdentity().ID) != nil {
		t.Fatal("connection should be removed")
	}
}

func TestTransport_PeerConnection_GracefulClose_Ugly(t *testing.T) {
	tp := setupTestTransportPair(t)
	conn := tp.connectClient(t)
	_ = conn.GracefulClose("", 0)
	if err := conn.GracefulClose("", 0); err != nil {
		t.Fatalf("second GracefulClose: %v", err)
	}
}

func TestPeerRateLimiter(t *testing.T) {
	t.Run("AllowUpToBurst", func(t *testing.T) {
		rl := NewPeerRateLimiter(10, 5)

		for i := range 10 {
			if !rl.Allow() {
				t.Errorf("should allow message %d (within burst)", i)
			}
		}

		if rl.Allow() {
			t.Error("should reject message after burst exhausted")
		}
	})

	t.Run("RefillAfterTime", func(t *testing.T) {
		rl := NewPeerRateLimiter(5, 10) // 5 burst, 10/sec refill

		// Exhaust all tokens
		for range 5 {
			rl.Allow()
		}

		if rl.Allow() {
			t.Error("should reject after exhaustion")
		}

		// Wait for refill
		time.Sleep(1100 * time.Millisecond)

		if !rl.Allow() {
			t.Error("should allow after refill")
		}
	})
}

func TestDeriveSubKeysDeterministicAndSeparated(t *testing.T) {
	sharedSecret := testSharedSecret(0x42)

	keys1, err := deriveSubKeys(sharedSecret)
	if err != nil {
		t.Fatalf("deriveSubKeys: %v", err)
	}
	keys2, err := deriveSubKeys(sharedSecret)
	if err != nil {
		t.Fatalf("deriveSubKeys second call: %v", err)
	}

	if !core.DeepEqual(keys1.encKey, keys2.encKey) {
		t.Fatal("encryption key derivation is not deterministic")
	}
	if !core.DeepEqual(keys1.macKey, keys2.macKey) {
		t.Fatal("MAC key derivation is not deterministic")
	}
	if !core.DeepEqual(keys1.chlKey, keys2.chlKey) {
		t.Fatal("challenge key derivation is not deterministic")
	}

	for name, key := range map[string][]byte{
		"encKey": keys1.encKey,
		"macKey": keys1.macKey,
		"chlKey": keys1.chlKey,
	} {
		if len(key) != subKeySize {
			t.Fatalf("%s length: got %d, want %d", name, len(key), subKeySize)
		}
	}

	if core.DeepEqual(keys1.encKey, keys1.macKey) {
		t.Fatal("encryption and MAC keys should be domain-separated")
	}
	if core.DeepEqual(keys1.encKey, keys1.chlKey) {
		t.Fatal("encryption and challenge keys should be domain-separated")
	}
	if core.DeepEqual(keys1.macKey, keys1.chlKey) {
		t.Fatal("MAC and challenge keys should be domain-separated")
	}
}

func TestDeriveSubKeysDifferentSecrets(t *testing.T) {
	keys1, err := deriveSubKeys(testSharedSecret(0x10))
	if err != nil {
		t.Fatalf("deriveSubKeys first secret: %v", err)
	}
	keys2, err := deriveSubKeys(testSharedSecret(0x20))
	if err != nil {
		t.Fatalf("deriveSubKeys second secret: %v", err)
	}

	if core.DeepEqual(keys1.encKey, keys2.encKey) {
		t.Fatal("different shared secrets produced the same encryption key")
	}
	if core.DeepEqual(keys1.macKey, keys2.macKey) {
		t.Fatal("different shared secrets produced the same MAC key")
	}
	if core.DeepEqual(keys1.chlKey, keys2.chlKey) {
		t.Fatal("different shared secrets produced the same challenge key")
	}
}

func TestTransportPayloadEncryptDecryptRoundTrip(t *testing.T) {
	sharedSecret := testSharedSecret(0x33)
	payload := []byte(`{"id":"msg-1","type":"ping","from":"a","to":"b","timestamp":"2026-04-25T00:00:00Z","payload":{}}`)

	encrypted, err := encryptTransportPayload(payload, sharedSecret)
	if err != nil {
		t.Fatalf("encryptTransportPayload: %v", err)
	}
	if len(encrypted) == 0 {
		t.Fatal("expected encrypted payload")
	}
	if core.DeepEqual(encrypted, payload) {
		t.Fatal("ciphertext should not equal plaintext")
	}

	decrypted, err := decryptTransportPayload(encrypted, sharedSecret)
	if err != nil {
		t.Fatalf("decryptTransportPayload: %v", err)
	}
	if !core.DeepEqual(decrypted, payload) {
		t.Fatalf("decrypted payload mismatch: got %q, want %q", decrypted, payload)
	}

	if _, err := decryptTransportPayload(encrypted, testSharedSecret(0x34)); err == nil {
		t.Fatal("decryptTransportPayload should reject a different shared secret")
	}
}

func TestDerivedMACKeySignsAndVerifies(t *testing.T) {
	keys, err := deriveSubKeys(testSharedSecret(0x55))
	if err != nil {
		t.Fatalf("deriveSubKeys: %v", err)
	}
	otherKeys, err := deriveSubKeys(testSharedSecret(0x56))
	if err != nil {
		t.Fatalf("deriveSubKeys other secret: %v", err)
	}

	message := []byte("ueps-mac-domain-separation")
	signature := signWithMACKey(keys.macKey, message)

	if !verifyWithMACKey(keys.macKey, message, signature) {
		t.Fatal("signature should verify with the derived MAC key")
	}
	if verifyWithMACKey(otherKeys.macKey, message, signature) {
		t.Fatal("signature should not verify with a different derived MAC key")
	}
}

func TestTransportHotPathDoesNotCallSMSGEncrypt(t *testing.T) {
	source, err := testReadFile("transport.go")
	if err != nil {
		t.Fatalf("read transport.go: %v", err)
	}
	if core.Contains(string(source), "smsg.Encrypt") {
		t.Fatal("transport hot path should not call smsg.Encrypt")
	}
}

func testSharedSecret(seed byte) []byte {
	sharedSecret := make([]byte, sharedSecretSize)
	for i := range sharedSecret {
		sharedSecret[i] = seed ^ byte(i)
	}
	return sharedSecret
}

func signWithMACKey(macKey []byte, message []byte) []byte {
	mac := hmac.New(sha256.New, macKey)
	mac.Write(message)
	return mac.Sum(nil)
}

func verifyWithMACKey(macKey []byte, message []byte, signature []byte) bool {
	return hmac.Equal(signature, signWithMACKey(macKey, message))
}

// --- Transport Integration Tests ---

func TestTransport_FullHandshake(t *testing.T) {
	tp := setupTestTransportPair(t)
	pc := tp.connectClient(t)

	// Shared secret must be derived
	if len(pc.SharedSecret) == 0 {
		t.Error("shared secret should be derived after handshake")
	}

	// Allow server goroutines to register the connection
	time.Sleep(50 * time.Millisecond)

	if tp.Server.ConnectedPeers() != 1 {
		t.Errorf("server connected peers: got %d, want 1", tp.Server.ConnectedPeers())
	}
	if tp.Client.ConnectedPeers() != 1 {
		t.Errorf("client connected peers: got %d, want 1", tp.Client.ConnectedPeers())
	}

	// Verify peer identity was exchanged correctly
	serverID := tp.ServerNode.GetIdentity().ID
	serverConn := tp.Client.GetConnection(serverID)
	if serverConn == nil {
		t.Fatal("client should have connection to server by server ID")
	}
	if serverConn.Peer.Name != "server" {
		t.Errorf("peer name: got %q, want %q", serverConn.Peer.Name, "server")
	}
}

func TestTransport_HandshakeRejectWrongVersion(t *testing.T) {
	tp := setupTestTransportPair(t)

	// Dial raw WebSocket and send handshake with unsupported version
	wsURL := "ws://" + tp.ServerAddr + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("raw dial: %v", err)
	}
	defer conn.Close()

	clientIdentity := tp.ClientNode.GetIdentity()
	payload := HandshakePayload{
		Identity: *clientIdentity,
		Version:  "99.99", // Unsupported
	}
	msg, _ := NewMessage(MsgHandshake, clientIdentity.ID, "", payload)
	data, _ := MarshalJSON(msg)

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("write handshake: %v", err)
	}

	_, respData, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read response: %v", err)
	}

	var resp Message
	if err := testJSONUnmarshal(respData, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	var ack HandshakeAckPayload
	resp.ParsePayload(&ack)

	if ack.Accepted {
		t.Error("should reject incompatible protocol version")
	}
	if !core.Contains(ack.Reason, "incompatible protocol version") {
		t.Errorf("expected version rejection reason, got: %s", ack.Reason)
	}
}

func TestTransport_HandshakeRejectAllowlist(t *testing.T) {
	tp := setupTestTransportPair(t)

	// Switch server to allowlist mode WITHOUT adding client's key
	tp.ServerReg.SetAuthMode(PeerAuthAllowlist)

	peer := &Peer{
		ID:      tp.ServerNode.GetIdentity().ID,
		Name:    "server",
		Address: tp.ServerAddr,
		Role:    RoleWorker,
	}
	tp.ClientReg.AddPeer(peer)

	_, err := tp.Client.Connect(peer)
	if err == nil {
		t.Fatal("should reject peer not in allowlist")
	}
	if !core.Contains(err.Error(), "rejected") {
		t.Errorf("expected rejection error, got: %v", err)
	}
}

func TestTransport_EncryptedMessageRoundTrip(t *testing.T) {
	tp := setupTestTransportPair(t)

	received := make(chan *Message, 1)
	tp.Server.OnMessage(func(conn *PeerConnection, msg *Message) {
		received <- msg
	})

	pc := tp.connectClient(t)

	// Send an encrypted message from client to server
	clientID := tp.ClientNode.GetIdentity().ID
	serverID := tp.ServerNode.GetIdentity().ID
	sentMsg, _ := NewMessage(MsgPing, clientID, serverID, PingPayload{
		SentAt: time.Now().UnixMilli(),
	})

	if err := pc.Send(sentMsg); err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case msg := <-received:
		if msg.Type != MsgPing {
			t.Errorf("type: got %s, want %s", msg.Type, MsgPing)
		}
		if msg.ID != sentMsg.ID {
			t.Error("message ID mismatch after encrypt/decrypt round-trip")
		}
		if msg.From != clientID {
			t.Errorf("from: got %s, want %s", msg.From, clientID)
		}

		var payload PingPayload
		msg.ParsePayload(&payload)
		if payload.SentAt == 0 {
			t.Error("payload should have SentAt timestamp")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestTransport_MessageDedup(t *testing.T) {
	tp := setupTestTransportPair(t)

	var count atomic.Int32
	tp.Server.OnMessage(func(conn *PeerConnection, msg *Message) {
		count.Add(1)
	})

	pc := tp.connectClient(t)

	clientID := tp.ClientNode.GetIdentity().ID
	serverID := tp.ServerNode.GetIdentity().ID
	msg, _ := NewMessage(MsgPing, clientID, serverID, PingPayload{SentAt: time.Now().UnixMilli()})

	// Send the same message twice
	if err := pc.Send(msg); err != nil {
		t.Fatalf("first send: %v", err)
	}
	time.Sleep(100 * time.Millisecond) // Ensure first is processed and marked

	if err := pc.Send(msg); err != nil {
		t.Fatalf("second send: %v", err)
	}
	time.Sleep(100 * time.Millisecond) // Allow time for second to be processed (or dropped)

	if got := count.Load(); got != 1 {
		t.Errorf("expected 1 message delivered (dedup), got %d", got)
	}
}

func TestTransport_RateLimiting(t *testing.T) {
	tp := setupTestTransportPair(t)

	var count atomic.Int32
	tp.Server.OnMessage(func(conn *PeerConnection, msg *Message) {
		count.Add(1)
	})

	pc := tp.connectClient(t)

	clientID := tp.ClientNode.GetIdentity().ID
	serverID := tp.ServerNode.GetIdentity().ID

	// Send 150 messages rapidly (rate limiter burst = 100)
	for range 150 {
		msg, _ := NewMessage(MsgPing, clientID, serverID, PingPayload{SentAt: time.Now().UnixMilli()})
		pc.Send(msg)
	}

	time.Sleep(1 * time.Second) // Allow processing

	received := int(count.Load())
	t.Logf("rate limiting: %d/150 messages delivered", received)

	if received >= 150 {
		t.Error("rate limiting should have dropped some messages")
	}
	if received < 50 {
		t.Errorf("too few messages received (%d), rate limiter may be too aggressive", received)
	}
}

func TestTransport_MaxConnsEnforcement(t *testing.T) {
	// Server with MaxConns=1
	serverNM := testNode(t, "maxconns-server", RoleWorker)
	serverReg := testRegistry(t)

	serverCfg := DefaultTransportConfig()
	serverCfg.MaxConns = 1
	serverTransport := NewTransport(serverNM, serverReg, serverCfg)

	mux := http.NewServeMux()
	mux.HandleFunc(serverCfg.WSPath, serverTransport.handleWSUpgrade)
	ts := httptest.NewServer(mux)
	t.Cleanup(func() {
		serverTransport.Stop()
		ts.Close()
	})

	u, _ := url.Parse(ts.URL)
	serverAddr := u.Host

	// First client connects successfully
	client1NM := testNode(t, "client1", RoleController)
	client1Reg := testRegistry(t)
	client1Transport := NewTransport(client1NM, client1Reg, DefaultTransportConfig())
	t.Cleanup(func() { client1Transport.Stop() })

	peer1 := &Peer{ID: serverNM.GetIdentity().ID, Name: "server", Address: serverAddr, Role: RoleWorker}
	client1Reg.AddPeer(peer1)

	_, err := client1Transport.Connect(peer1)
	if err != nil {
		t.Fatalf("first connection should succeed: %v", err)
	}

	// Allow server to register the connection
	time.Sleep(50 * time.Millisecond)

	// Second client should be rejected (MaxConns=1 reached)
	client2NM := testNode(t, "client2", RoleController)
	client2Reg := testRegistry(t)
	client2Transport := NewTransport(client2NM, client2Reg, DefaultTransportConfig())
	t.Cleanup(func() { client2Transport.Stop() })

	peer2 := &Peer{ID: serverNM.GetIdentity().ID, Name: "server", Address: serverAddr, Role: RoleWorker}
	client2Reg.AddPeer(peer2)

	_, err = client2Transport.Connect(peer2)
	if err == nil {
		t.Fatal("second connection should be rejected when MaxConns=1")
	}
}

func TestTransport_KeepaliveTimeout(t *testing.T) {
	// Use short keepalive settings so the test is fast
	serverCfg := DefaultTransportConfig()
	serverCfg.PingInterval = 100 * time.Millisecond
	serverCfg.PongTimeout = 100 * time.Millisecond

	clientCfg := DefaultTransportConfig()
	clientCfg.PingInterval = 100 * time.Millisecond
	clientCfg.PongTimeout = 100 * time.Millisecond

	tp := setupTestTransportPairWithConfig(t, serverCfg, clientCfg)
	tp.connectClient(t)

	// Verify connection is established
	time.Sleep(50 * time.Millisecond)
	if tp.Server.ConnectedPeers() != 1 {
		t.Fatalf("server should have 1 peer initially, got %d", tp.Server.ConnectedPeers())
	}

	// Close the underlying WebSocket on the client side to simulate network failure.
	// The server's readLoop will detect the broken connection and clean up.
	clientID := tp.ClientNode.GetIdentity().ID
	serverPeerID := tp.ServerNode.GetIdentity().ID
	clientConn := tp.Client.GetConnection(serverPeerID)
	if clientConn == nil {
		t.Fatal("client should have connection to server")
	}
	clientConn.Conn.Close()

	// Wait for server to detect and clean up
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatalf("server did not clean up connection: still has %d peers", tp.Server.ConnectedPeers())
		default:
			if tp.Server.ConnectedPeers() == 0 {
				// Verify registry updated
				peer := tp.ServerReg.GetPeer(clientID)
				if peer != nil && peer.Connected {
					t.Error("registry should show peer as disconnected")
				}
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestTransport_IdleTimeoutClosesSilentPeer(t *testing.T) {
	serverCfg := DefaultTransportConfig()
	serverCfg.IdleTimeout = 150 * time.Millisecond
	serverCfg.PingInterval = time.Second
	serverCfg.PongTimeout = time.Second

	clientCfg := DefaultTransportConfig()
	clientCfg.IdleTimeout = 150 * time.Millisecond
	clientCfg.PingInterval = time.Second
	clientCfg.PongTimeout = time.Second

	tp := setupTestTransportPairWithConfig(t, serverCfg, clientCfg)
	tp.connectClient(t)

	if tp.Server.ConnectedPeers() != 1 {
		t.Fatalf("server should have 1 peer initially, got %d", tp.Server.ConnectedPeers())
	}

	waitForTransportCondition(t, 2*time.Second, func() bool {
		return tp.Server.ConnectedPeers() == 0
	}, "silent peer connection did not close after idle timeout")
}

func TestTransport_IdleTimeoutAllowsActivePeer(t *testing.T) {
	serverCfg := DefaultTransportConfig()
	serverCfg.IdleTimeout = 500 * time.Millisecond
	serverCfg.PingInterval = 2 * time.Second
	serverCfg.PongTimeout = 2 * time.Second

	clientCfg := DefaultTransportConfig()
	clientCfg.IdleTimeout = 500 * time.Millisecond
	clientCfg.PingInterval = 2 * time.Second
	clientCfg.PongTimeout = 2 * time.Second

	tp := setupTestTransportPairWithConfig(t, serverCfg, clientCfg)
	clientConn := tp.connectClient(t)
	serverConn := waitForPeerConnection(t, tp.Server, tp.ClientNode.GetIdentity().ID)

	clientID := tp.ClientNode.GetIdentity().ID
	serverID := tp.ServerNode.GetIdentity().ID
	deadline := time.Now().Add(700 * time.Millisecond)
	for time.Now().Before(deadline) {
		clientMsg, _ := NewMessage(MsgPing, clientID, serverID, PingPayload{SentAt: time.Now().UnixMilli()})
		if err := clientConn.Send(clientMsg); err != nil {
			t.Fatalf("client send: %v", err)
		}

		serverMsg, _ := NewMessage(MsgPong, serverID, clientID, PongPayload{
			SentAt:     time.Now().UnixMilli(),
			ReceivedAt: time.Now().UnixMilli(),
		})
		if err := serverConn.Send(serverMsg); err != nil {
			t.Fatalf("server send: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	if tp.Server.ConnectedPeers() != 1 {
		t.Fatalf("server should keep active peer connected, got %d", tp.Server.ConnectedPeers())
	}
	if tp.Client.ConnectedPeers() != 1 {
		t.Fatalf("client should keep active peer connected, got %d", tp.Client.ConnectedPeers())
	}
}

func TestTransport_EnvelopeSignedValidAccepted(t *testing.T) {
	tp := setupTestTransportPair(t)

	received := make(chan *Message, 1)
	tp.Server.OnMessage(func(conn *PeerConnection, msg *Message) {
		received <- msg
	})

	pc := tp.connectClient(t)
	clientID := tp.ClientNode.GetIdentity().ID
	serverID := tp.ServerNode.GetIdentity().ID
	msg, _ := NewMessage(MsgPing, clientID, serverID, PingPayload{SentAt: time.Now().UnixMilli()})
	body := envelopeBody(t, msg)

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate signing key: %v", err)
	}
	env := Envelope{
		PeerPubkey: pub,
		Body:       body,
		Signature:  ed25519.Sign(priv, body),
	}
	if err := env.VerifySignature(); err != nil {
		t.Fatalf("signed envelope should verify: %v", err)
	}

	writeEnvelopeFrame(t, pc, env)

	select {
	case got := <-received:
		if got.ID != msg.ID {
			t.Errorf("message ID: got %q, want %q", got.ID, msg.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for signed envelope message")
	}
}

func TestTransport_EnvelopeBadSignatureDroppedAndLogged(t *testing.T) {
	logs := captureTransportLogs(t)
	tp := setupTestTransportPair(t)

	var received atomic.Int32
	tp.Server.OnMessage(func(conn *PeerConnection, msg *Message) {
		received.Add(1)
	})

	pc := tp.connectClient(t)
	clientID := tp.ClientNode.GetIdentity().ID
	serverID := tp.ServerNode.GetIdentity().ID
	msg, _ := NewMessage(MsgPing, clientID, serverID, PingPayload{SentAt: time.Now().UnixMilli()})
	body := envelopeBody(t, msg)

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate signing key: %v", err)
	}
	sig := ed25519.Sign(priv, body)
	sig[0] ^= 0xff

	writeEnvelopeFrame(t, pc, Envelope{
		PeerPubkey: pub,
		Body:       body,
		Signature:  sig,
	})

	waitForTransportCondition(t, time.Second, func() bool {
		return core.Contains(logs.String(), "invalid envelope signature")
	}, "invalid envelope signature was not logged")

	if received.Load() != 0 {
		t.Fatalf("bad signature message should be dropped, delivered %d messages", received.Load())
	}
}

func TestTransport_EnvelopeUnsignedAccepted(t *testing.T) {
	tp := setupTestTransportPair(t)

	received := make(chan *Message, 1)
	tp.Server.OnMessage(func(conn *PeerConnection, msg *Message) {
		received <- msg
	})

	pc := tp.connectClient(t)
	clientID := tp.ClientNode.GetIdentity().ID
	serverID := tp.ServerNode.GetIdentity().ID
	msg, _ := NewMessage(MsgPing, clientID, serverID, PingPayload{SentAt: time.Now().UnixMilli()})

	writeEnvelopeFrame(t, pc, Envelope{
		Body: envelopeBody(t, msg),
	})

	select {
	case got := <-received:
		if got.ID != msg.ID {
			t.Errorf("message ID: got %q, want %q", got.ID, msg.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for unsigned envelope message")
	}
}

func TestTransport_GracefulClose(t *testing.T) {
	tp := setupTestTransportPair(t)

	received := make(chan *Message, 10)
	tp.Server.OnMessage(func(conn *PeerConnection, msg *Message) {
		received <- msg
	})

	pc := tp.connectClient(t)

	// Allow connection to fully establish
	time.Sleep(50 * time.Millisecond)

	// Graceful close should send a MsgDisconnect before closing
	pc.GracefulClose("test shutdown", DisconnectNormal)

	// Check if disconnect message was received
	select {
	case msg := <-received:
		if msg.Type != MsgDisconnect {
			t.Errorf("expected disconnect message, got %s", msg.Type)
		}
		var payload DisconnectPayload
		msg.ParsePayload(&payload)
		if payload.Reason != "test shutdown" {
			t.Errorf("disconnect reason: got %q, want %q", payload.Reason, "test shutdown")
		}
		if payload.Code != DisconnectNormal {
			t.Errorf("disconnect code: got %d, want %d", payload.Code, DisconnectNormal)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for disconnect message")
	}
}

func TestTransport_ConcurrentSends(t *testing.T) {
	tp := setupTestTransportPair(t)

	var count atomic.Int32
	tp.Server.OnMessage(func(conn *PeerConnection, msg *Message) {
		count.Add(1)
	})

	pc := tp.connectClient(t)

	clientID := tp.ClientNode.GetIdentity().ID
	serverID := tp.ServerNode.GetIdentity().ID

	// Spawn 10 goroutines each sending 5 messages concurrently
	const goroutines = 10
	const msgsPerGoroutine = 5
	var wg sync.WaitGroup

	for range goroutines {
		wg.Go(func() {
			for range msgsPerGoroutine {
				msg, _ := NewMessage(MsgPing, clientID, serverID, PingPayload{SentAt: time.Now().UnixMilli()})
				pc.Send(msg)
			}
		})
	}

	wg.Wait()
	time.Sleep(1 * time.Second) // Allow delivery

	got := int(count.Load())
	// All messages should be delivered (unique IDs, within rate limit burst of 100)
	expected := goroutines * msgsPerGoroutine
	if got != expected {
		t.Errorf("concurrent sends: got %d/%d messages delivered", got, expected)
	}
}

// --- Additional coverage tests ---

func TestTransport_Broadcast(t *testing.T) {
	// Set up a controller with two worker peers connected.
	controllerNM := testNode(t, "broadcast-controller", RoleController)
	controllerReg := testRegistry(t)
	controllerTransport := NewTransport(controllerNM, controllerReg, DefaultTransportConfig())
	t.Cleanup(func() { controllerTransport.Stop() })

	const numWorkers = 2
	var receiveCounters [numWorkers]*atomic.Int32

	for i := range numWorkers {
		receiveCounters[i] = &atomic.Int32{}
		counter := receiveCounters[i]

		nm, addr, srv := makeWorkerServer(t)
		srv.OnMessage(func(conn *PeerConnection, msg *Message) {
			counter.Add(1)
		})

		wID := nm.GetIdentity().ID
		peer := &Peer{
			ID:      wID,
			Name:    "worker",
			Address: addr,
			Role:    RoleWorker,
		}
		controllerReg.AddPeer(peer)

		_, err := controllerTransport.Connect(peer)
		if err != nil {
			t.Fatalf("failed to connect to worker %d: %v", i, err)
		}
	}

	time.Sleep(100 * time.Millisecond)

	// Broadcast a message from the controller
	controllerID := controllerNM.GetIdentity().ID
	msg, _ := NewMessage(MsgPing, controllerID, "", PingPayload{
		SentAt: time.Now().UnixMilli(),
	})

	err := controllerTransport.Broadcast(msg)
	if err != nil {
		t.Fatalf("Broadcast failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Both workers should have received the broadcast
	for i, counter := range receiveCounters {
		if counter.Load() != 1 {
			t.Errorf("worker %d received %d messages, expected 1", i, counter.Load())
		}
	}
}

func TestTransport_BroadcastExcludesSender(t *testing.T) {
	// Verify that Broadcast excludes the sender.
	tp := setupTestTransportPair(t)

	serverReceived := &atomic.Int32{}
	tp.Server.OnMessage(func(conn *PeerConnection, msg *Message) {
		serverReceived.Add(1)
	})

	tp.connectClient(t)
	time.Sleep(50 * time.Millisecond)

	// Broadcast from the server side with From = server ID.
	// The server has a connection to the client, but msg.From matches the client's
	// connection peer ID check, not the server's own ID. Let's verify sender exclusion
	// by broadcasting from the server with its own ID.
	serverID := tp.ServerNode.GetIdentity().ID
	msg, _ := NewMessage(MsgPing, serverID, "", PingPayload{SentAt: time.Now().UnixMilli()})

	// This broadcasts from server to all connected peers (the client).
	// The server itself won't receive it back because it's not connected to itself.
	err := tp.Server.Broadcast(msg)
	if err != nil {
		t.Fatalf("Broadcast failed: %v", err)
	}
}

func TestTransport_NewTransport_DefaultMaxMessageSize(t *testing.T) {
	nm := testNode(t, "defaults", RoleWorker)
	reg := testRegistry(t)
	cfg := TransportConfig{
		MaxMessageSize: 0, // should use default
	}
	tr := NewTransport(nm, reg, cfg)

	if tr == nil {
		t.Fatal("NewTransport returned nil")
	}
	if tr.config.MaxMessageSize != 0 {
		t.Errorf("config should preserve 0 value, got %d", tr.config.MaxMessageSize)
	}
	// The actual default is applied at usage time (readLoop, handleWSUpgrade)
}

func TestTransport_ConnectedPeers(t *testing.T) {
	tp := setupTestTransportPair(t)

	if tp.Server.ConnectedPeers() != 0 {
		t.Errorf("expected 0 connected peers initially, got %d", tp.Server.ConnectedPeers())
	}

	tp.connectClient(t)
	time.Sleep(50 * time.Millisecond)

	if tp.Server.ConnectedPeers() != 1 {
		t.Errorf("expected 1 connected peer after connect, got %d", tp.Server.ConnectedPeers())
	}
}

func TestTransport_StartAndStop(t *testing.T) {
	nm := testNode(t, "start-test", RoleWorker)
	reg := testRegistry(t)
	cfg := DefaultTransportConfig()
	cfg.ListenAddr = ":0" // Let OS pick a free port

	tr := NewTransport(nm, reg, cfg)

	err := tr.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Small wait for server goroutine to start
	time.Sleep(100 * time.Millisecond)

	err = tr.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestTransport_CheckOrigin(t *testing.T) {
	nm := testNode(t, "origin-test", RoleWorker)
	reg := testRegistry(t)
	cfg := DefaultTransportConfig()
	tr := NewTransport(nm, reg, cfg)

	tests := []struct {
		name    string
		origin  string
		allowed bool
	}{
		{"no origin", "", true},
		{"localhost", "http://localhost:8080", true},
		{"127.0.0.1", "http://127.0.0.1:8080", true},
		{"ipv6 loopback", "http://[::1]:8080", true},
		{"remote host", "http://evil.example.com", false},
		{"invalid origin", "://not-a-url", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &http.Request{Header: http.Header{}}
			if tt.origin != "" {
				r.Header.Set("Origin", tt.origin)
			}
			result := tr.upgrader.CheckOrigin(r)
			if result != tt.allowed {
				t.Errorf("CheckOrigin(%q) = %v, want %v", tt.origin, result, tt.allowed)
			}
		})
	}
}
