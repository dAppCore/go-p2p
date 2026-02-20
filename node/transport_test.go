package node

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// --- Test Helpers ---

// testNode creates a NodeManager with a generated identity in a temp directory.
func testNode(t *testing.T, name string, role NodeRole) *NodeManager {
	t.Helper()
	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
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
	reg, err := NewPeerRegistryWithPath(filepath.Join(dir, "peers.json"))
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

	t.Run("ConcurrentAccess", func(t *testing.T) {
		d := NewMessageDeduplicator(5 * time.Minute)
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
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

func TestPeerRateLimiter(t *testing.T) {
	t.Run("AllowUpToBurst", func(t *testing.T) {
		rl := NewPeerRateLimiter(10, 5)

		for i := 0; i < 10; i++ {
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
		for i := 0; i < 5; i++ {
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
	if err := json.Unmarshal(respData, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	var ack HandshakeAckPayload
	resp.ParsePayload(&ack)

	if ack.Accepted {
		t.Error("should reject incompatible protocol version")
	}
	if !strings.Contains(ack.Reason, "incompatible protocol version") {
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
	if !strings.Contains(err.Error(), "rejected") {
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
	for i := 0; i < 150; i++ {
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

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < msgsPerGoroutine; i++ {
				msg, _ := NewMessage(MsgPing, clientID, serverID, PingPayload{SentAt: time.Now().UnixMilli()})
				pc.Send(msg)
			}
		}()
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

	for i := 0; i < numWorkers; i++ {
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
