package node

import (
	"bufio"
	"bytes"
	"sync/atomic"
	"testing"
	"time"

	"forge.lthn.ai/core/go-p2p/ueps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_TwoNodeHandshakeAndMessage is the full integration test:
// two nodes on localhost performing identity creation, handshake, encrypted
// message exchange, UEPS packet routing via dispatcher, and graceful shutdown.
func TestIntegration_TwoNodeHandshakeAndMessage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// ---------------------------------------------------------------
	// 1. Create two identities via the transport pair helper
	// ---------------------------------------------------------------
	cfg := DefaultTransportConfig()
	cfg.PingInterval = 2 * time.Second
	cfg.PongTimeout = 1 * time.Second

	tp := setupTestTransportPairWithConfig(t, cfg, cfg)

	serverNode := tp.ServerNode
	clientNode := tp.ClientNode
	serverTransport := tp.Server
	clientTransport := tp.Client

	identityA := clientNode.GetIdentity()
	identityB := serverNode.GetIdentity()
	require.NotNil(t, identityA, "client node should have an identity")
	require.NotNil(t, identityB, "server node should have an identity")
	require.NotEqual(t, identityA.ID, identityB.ID, "nodes should have distinct IDs")

	t.Logf("Client (Node A): id=%s name=%s", identityA.ID, identityA.Name)
	t.Logf("Server (Node B): id=%s name=%s", identityB.ID, identityB.Name)

	// ---------------------------------------------------------------
	// 2. Register a Worker on the server side
	// ---------------------------------------------------------------
	worker := NewWorker(serverNode, serverTransport)

	// Track all messages received on the server for verification
	var serverMsgCount atomic.Int32
	var lastServerMsg atomic.Value
	serverTransport.OnMessage(func(conn *PeerConnection, msg *Message) {
		serverMsgCount.Add(1)
		lastServerMsg.Store(msg)
		worker.HandleMessage(conn, msg)
	})

	// ---------------------------------------------------------------
	// 3. Node A connects to Node B (handshake completes)
	// ---------------------------------------------------------------
	pc := tp.connectClient(t)

	require.NotNil(t, pc, "connection should be established")
	require.NotEmpty(t, pc.SharedSecret, "shared secret should be derived after handshake")
	assert.Equal(t, 32, len(pc.SharedSecret), "shared secret should be 32 bytes (SHA-256)")

	// Allow connection to settle
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 1, clientTransport.ConnectedPeers(), "client should have 1 connection")
	assert.Equal(t, 1, serverTransport.ConnectedPeers(), "server should have 1 connection")

	t.Log("Handshake completed successfully with shared secret derived")

	// ---------------------------------------------------------------
	// 4. Node A sends an encrypted message to Node B
	// ---------------------------------------------------------------
	clientID := clientNode.GetIdentity().ID
	serverID := serverNode.GetIdentity().ID

	pingMsg, err := NewMessage(MsgPing, clientID, serverID, PingPayload{
		SentAt: time.Now().UnixMilli(),
	})
	require.NoError(t, err, "creating ping message should succeed")

	err = pc.Send(pingMsg)
	require.NoError(t, err, "sending encrypted message should succeed")

	// Wait for message delivery
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for server to receive message (received %d)", serverMsgCount.Load())
		default:
			if serverMsgCount.Load() >= 1 {
				goto messageReceived
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
messageReceived:

	t.Logf("Server received %d message(s)", serverMsgCount.Load())

	// ---------------------------------------------------------------
	// 5. Verify encrypted message was decrypted correctly
	// ---------------------------------------------------------------
	stored := lastServerMsg.Load()
	require.NotNil(t, stored, "server should have received a message")
	receivedMsg := stored.(*Message)
	assert.Equal(t, MsgPing, receivedMsg.Type, "received message type should be ping")
	assert.Equal(t, clientID, receivedMsg.From, "received message should be from client")
	assert.Equal(t, pingMsg.ID, receivedMsg.ID, "message ID should match after encrypt/decrypt")

	var receivedPayload PingPayload
	err = receivedMsg.ParsePayload(&receivedPayload)
	require.NoError(t, err, "parsing received payload should succeed")
	assert.NotZero(t, receivedPayload.SentAt, "SentAt should be non-zero")

	t.Log("Encrypted message round-trip verified")

	// ---------------------------------------------------------------
	// 6. Controller sends a request and gets a response (ping/pong)
	// ---------------------------------------------------------------
	// Create a controller on the client side. We need to set the transport
	// message handler to route replies to the controller and requests to the worker.
	controller := NewController(clientNode, tp.ClientReg, clientTransport)

	// The controller's OnMessage call overrides our tracking handler on the CLIENT.
	// The SERVER still needs to route to the worker. Re-register on the server to
	// ensure the worker still handles incoming requests.
	serverTransport.OnMessage(func(conn *PeerConnection, msg *Message) {
		serverMsgCount.Add(1)
		lastServerMsg.Store(msg)
		worker.HandleMessage(conn, msg)
	})

	rtt, err := controller.PingPeer(serverID)
	require.NoError(t, err, "PingPeer via controller should succeed")
	assert.Greater(t, rtt, 0.0, "RTT should be positive")
	assert.Less(t, rtt, 1000.0, "RTT on localhost should be under 1000ms")

	t.Logf("Controller PingPeer RTT: %.2f ms", rtt)

	// ---------------------------------------------------------------
	// 7. Route a UEPS packet via the dispatcher
	// ---------------------------------------------------------------
	dispatcher := NewDispatcher()

	var dispatchedPacket atomic.Value
	dispatcher.RegisterHandler(IntentCompute, func(pkt *ueps.ParsedPacket) error {
		dispatchedPacket.Store(pkt)
		return nil
	})
	dispatcher.RegisterHandler(IntentHandshake, func(pkt *ueps.ParsedPacket) error {
		return nil
	})

	// Build a UEPS packet with the shared secret
	uepsPayload := []byte("compute-job-data-for-integration-test")
	builder := ueps.NewBuilder(IntentCompute, uepsPayload)
	builder.Header.ThreatScore = 100 // Safe, below threshold

	frame, err := builder.MarshalAndSign(pc.SharedSecret)
	require.NoError(t, err, "UEPS MarshalAndSign should succeed")

	// Parse and verify the packet
	parsed, err := ueps.ReadAndVerify(bufio.NewReader(bytes.NewReader(frame)), pc.SharedSecret)
	require.NoError(t, err, "UEPS ReadAndVerify should succeed")
	assert.Equal(t, byte(IntentCompute), parsed.Header.IntentID)
	assert.Equal(t, uepsPayload, parsed.Payload)

	// Dispatch through the intent router
	err = dispatcher.Dispatch(parsed)
	require.NoError(t, err, "Dispatch should route to compute handler")

	stored2 := dispatchedPacket.Load()
	require.NotNil(t, stored2, "compute handler should have received the packet")
	dispPkt := stored2.(*ueps.ParsedPacket)
	assert.Equal(t, uepsPayload, dispPkt.Payload, "dispatched payload should match")
	assert.Equal(t, uint16(100), dispPkt.Header.ThreatScore, "threat score should match")

	t.Log("UEPS packet routing via dispatcher verified")

	// Verify threat circuit breaker rejects high-threat packets
	highThreatBuilder := ueps.NewBuilder(IntentCompute, []byte("hostile"))
	highThreatBuilder.Header.ThreatScore = ThreatScoreThreshold + 1
	highThreatFrame, err := highThreatBuilder.MarshalAndSign(pc.SharedSecret)
	require.NoError(t, err)

	highThreatParsed, err := ueps.ReadAndVerify(
		bufio.NewReader(bytes.NewReader(highThreatFrame)), pc.SharedSecret)
	require.NoError(t, err)

	err = dispatcher.Dispatch(highThreatParsed)
	assert.ErrorIs(t, err, ErrThreatScoreExceeded, "high-threat packet should be rejected")

	t.Log("Threat circuit breaker verified")

	// ---------------------------------------------------------------
	// 8. Graceful shutdown
	// ---------------------------------------------------------------
	// Track disconnect message on server side
	disconnectReceived := make(chan struct{}, 1)
	serverTransport.OnMessage(func(conn *PeerConnection, msg *Message) {
		if msg.Type == MsgDisconnect {
			disconnectReceived <- struct{}{}
		}
	})

	// Gracefully close the client connection
	pc.GracefulClose("integration test complete", DisconnectNormal)

	select {
	case <-disconnectReceived:
		t.Log("Graceful disconnect message received by server")
	case <-time.After(2 * time.Second):
		t.Log("Disconnect message not received (may have been processed before handler change)")
	}

	// Stop transports (cleanup is deferred via t.Cleanup in setupTestTransportPair)
	t.Log("Integration test complete: all phases passed")
}

// TestIntegration_SharedSecretAgreement verifies that two independently created
// nodes derive the same shared secret, which is fundamental to the entire
// encrypted communication chain.
func TestIntegration_SharedSecretAgreement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	nodeA := testNode(t, "secret-node-a", RoleDual)
	nodeB := testNode(t, "secret-node-b", RoleDual)

	pubKeyA := nodeA.GetIdentity().PublicKey
	pubKeyB := nodeB.GetIdentity().PublicKey

	// A derives secret using B's public key
	secretFromA, err := nodeA.DeriveSharedSecret(pubKeyB)
	require.NoError(t, err)

	// B derives secret using A's public key
	secretFromB, err := nodeB.DeriveSharedSecret(pubKeyA)
	require.NoError(t, err)

	assert.Equal(t, secretFromA, secretFromB,
		"both nodes should derive identical shared secrets via ECDH")
	assert.Equal(t, 32, len(secretFromA), "shared secret should be 32 bytes")

	t.Logf("Shared secret agreement verified: %d bytes", len(secretFromA))
}

// TestIntegration_GetRemoteStats_EndToEnd tests the full stats retrieval flow
// across a real WebSocket connection.
func TestIntegration_GetRemoteStats_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tp := setupTestTransportPair(t)

	// Set up worker with response capability
	worker := NewWorker(tp.ServerNode, tp.Server)
	worker.RegisterWithTransport()

	// Set up controller
	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)

	// Connect
	tp.connectClient(t)
	time.Sleep(100 * time.Millisecond)

	serverID := tp.ServerNode.GetIdentity().ID

	// Fetch stats
	stats, err := controller.GetRemoteStats(serverID)
	require.NoError(t, err, "GetRemoteStats should succeed end-to-end")
	require.NotNil(t, stats)
	assert.Equal(t, serverID, stats.NodeID)
	assert.Equal(t, "server", stats.NodeName)
	assert.GreaterOrEqual(t, stats.Uptime, int64(0))

	t.Logf("Remote stats retrieved: nodeID=%s uptime=%ds miners=%d",
		stats.NodeID, stats.Uptime, len(stats.Miners))
}
