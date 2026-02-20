package node

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"forge.lthn.ai/core/go-p2p/ueps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Full Integration Test — Phase 5
//
// Exercises the complete node lifecycle on localhost:
//   1. Identity creation for two nodes
//   2. WebSocket handshake with challenge-response authentication
//   3. Encrypted message exchange (ping/pong, stats)
//   4. UEPS packet routing via the Dispatcher
//   5. Graceful shutdown with disconnect messages
// ============================================================================

func TestIntegration_FullNodeLifecycle(t *testing.T) {
	// ----------------------------------------------------------------
	// Step 1: Identity creation
	// ----------------------------------------------------------------
	controllerNM := testNode(t, "integration-controller", RoleController)
	workerNM := testNode(t, "integration-worker", RoleWorker)

	controllerIdentity := controllerNM.GetIdentity()
	workerIdentity := workerNM.GetIdentity()
	require.NotNil(t, controllerIdentity, "controller identity should be initialised")
	require.NotNil(t, workerIdentity, "worker identity should be initialised")
	assert.NotEmpty(t, controllerIdentity.ID, "controller ID should be non-empty")
	assert.NotEmpty(t, workerIdentity.ID, "worker ID should be non-empty")
	assert.NotEqual(t, controllerIdentity.ID, workerIdentity.ID,
		"two independently generated identities must differ")
	assert.Equal(t, RoleController, controllerIdentity.Role)
	assert.Equal(t, RoleWorker, workerIdentity.Role)

	// ----------------------------------------------------------------
	// Step 2: Set up transports, registries, worker, and controller
	// ----------------------------------------------------------------
	workerReg := testRegistry(t)
	controllerReg := testRegistry(t)

	workerCfg := DefaultTransportConfig()
	workerCfg.PingInterval = 2 * time.Second
	workerCfg.PongTimeout = 2 * time.Second
	controllerCfg := DefaultTransportConfig()
	controllerCfg.PingInterval = 2 * time.Second
	controllerCfg.PongTimeout = 2 * time.Second

	workerTransport := NewTransport(workerNM, workerReg, workerCfg)
	controllerTransport := NewTransport(controllerNM, controllerReg, controllerCfg)

	// Register a Worker on the server side.
	worker := NewWorker(workerNM, workerTransport)
	worker.SetMinerManager(&mockMinerManagerFull{
		miners: map[string]*mockMinerFull{
			"integration-miner": {
				name:      "integration-miner",
				minerType: "xmrig",
				stats: map[string]interface{}{
					"hashrate": 5000.0,
					"shares":   250,
				},
				consoleHistory: []string{
					"[2026-02-20 12:00:00] miner started",
				},
			},
		},
	})
	worker.RegisterWithTransport()

	// Start the worker transport behind httptest.
	mux := http.NewServeMux()
	mux.HandleFunc(workerCfg.WSPath, workerTransport.handleWSUpgrade)
	ts := httptest.NewServer(mux)
	t.Cleanup(func() {
		controllerTransport.Stop()
		workerTransport.Stop()
		ts.Close()
	})

	u, _ := url.Parse(ts.URL)
	workerAddr := u.Host

	// Register the worker peer in the controller's registry.
	workerPeer := &Peer{
		ID:      workerIdentity.ID,
		Name:    "integration-worker",
		Address: workerAddr,
		Role:    RoleWorker,
	}
	require.NoError(t, controllerReg.AddPeer(workerPeer))

	// Create the controller (registers handleResponse on the transport).
	controller := NewController(controllerNM, controllerReg, controllerTransport)

	// ----------------------------------------------------------------
	// Step 3: WebSocket handshake (challenge-response)
	// ----------------------------------------------------------------
	pc, err := controllerTransport.Connect(workerPeer)
	require.NoError(t, err, "handshake should succeed")
	require.NotNil(t, pc, "peer connection should be returned")
	assert.NotEmpty(t, pc.SharedSecret, "shared secret should be derived after handshake")

	// Allow server-side goroutines to register the connection.
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 1, controllerTransport.ConnectedPeers(),
		"controller should have 1 connected peer")
	assert.Equal(t, 1, workerTransport.ConnectedPeers(),
		"worker should have 1 connected peer")

	// Verify the peer's real identity is stored.
	serverPeerID := workerNM.GetIdentity().ID
	conn := controllerTransport.GetConnection(serverPeerID)
	require.NotNil(t, conn, "controller should hold a connection keyed by server's real ID")
	assert.Equal(t, "integration-worker", conn.Peer.Name)

	// ----------------------------------------------------------------
	// Step 4: Encrypted message exchange — Ping/Pong
	// ----------------------------------------------------------------
	rtt, err := controller.PingPeer(serverPeerID)
	require.NoError(t, err, "PingPeer should succeed")
	assert.Greater(t, rtt, 0.0, "RTT should be positive")
	assert.Less(t, rtt, 1000.0, "RTT on loopback should be well under 1s")

	// Verify registry metrics were updated.
	peerAfterPing := controllerReg.GetPeer(serverPeerID)
	require.NotNil(t, peerAfterPing)
	assert.Greater(t, peerAfterPing.PingMS, 0.0, "PingMS should be updated")

	// ----------------------------------------------------------------
	// Step 5: Encrypted message exchange — GetRemoteStats
	// ----------------------------------------------------------------
	stats, err := controller.GetRemoteStats(serverPeerID)
	require.NoError(t, err, "GetRemoteStats should succeed")
	require.NotNil(t, stats)
	assert.Equal(t, workerIdentity.ID, stats.NodeID)
	assert.Equal(t, "integration-worker", stats.NodeName)
	assert.Len(t, stats.Miners, 1, "worker should report 1 miner")
	assert.Equal(t, "integration-miner", stats.Miners[0].Name)
	assert.Equal(t, 5000.0, stats.Miners[0].Hashrate)

	// ----------------------------------------------------------------
	// Step 6: UEPS packet routing via the Dispatcher
	// ----------------------------------------------------------------
	dispatcher := NewDispatcher()

	var handshakeReceived, computeReceived atomic.Int32
	dispatcher.RegisterHandler(IntentHandshake, func(pkt *ueps.ParsedPacket) error {
		handshakeReceived.Add(1)
		return nil
	})
	dispatcher.RegisterHandler(IntentCompute, func(pkt *ueps.ParsedPacket) error {
		computeReceived.Add(1)
		return nil
	})

	// Build UEPS packets, sign them with the shared secret, parse and dispatch.
	sharedSecret := pc.SharedSecret

	// 6a. Handshake intent.
	pb := ueps.NewBuilder(IntentHandshake, []byte("hello-from-controller"))
	wireData, err := pb.MarshalAndSign(sharedSecret)
	require.NoError(t, err, "MarshalAndSign should succeed")

	parsed, err := ueps.ReadAndVerify(bufio.NewReader(bytes.NewReader(wireData)), sharedSecret)
	require.NoError(t, err, "ReadAndVerify should succeed")
	require.NoError(t, dispatcher.Dispatch(parsed), "dispatch handshake should succeed")
	assert.Equal(t, int32(1), handshakeReceived.Load())

	// 6b. Compute intent.
	pb2 := ueps.NewBuilder(IntentCompute, []byte(`{"job":"mine-block-42"}`))
	wireData2, err := pb2.MarshalAndSign(sharedSecret)
	require.NoError(t, err)

	parsed2, err := ueps.ReadAndVerify(bufio.NewReader(bytes.NewReader(wireData2)), sharedSecret)
	require.NoError(t, err)
	require.NoError(t, dispatcher.Dispatch(parsed2))
	assert.Equal(t, int32(1), computeReceived.Load())

	// 6c. High-threat packet should be rejected by the circuit breaker.
	pb3 := ueps.NewBuilder(IntentCompute, []byte("hostile"))
	pb3.Header.ThreatScore = ThreatScoreThreshold + 1
	wireData3, err := pb3.MarshalAndSign(sharedSecret)
	require.NoError(t, err)

	parsed3, err := ueps.ReadAndVerify(bufio.NewReader(bytes.NewReader(wireData3)), sharedSecret)
	require.NoError(t, err)
	err = dispatcher.Dispatch(parsed3)
	assert.ErrorIs(t, err, ErrThreatScoreExceeded,
		"high-threat packet should be dropped by circuit breaker")
	// Compute handler should NOT have been called again.
	assert.Equal(t, int32(1), computeReceived.Load())

	// ----------------------------------------------------------------
	// Step 7: Graceful shutdown
	// ----------------------------------------------------------------
	disconnectReceived := make(chan *Message, 1)
	workerTransport.OnMessage(func(conn *PeerConnection, msg *Message) {
		if msg.Type == MsgDisconnect {
			disconnectReceived <- msg
		}
	})

	// Gracefully close from the controller side.
	pc.GracefulClose("integration test complete", DisconnectNormal)

	select {
	case msg := <-disconnectReceived:
		assert.Equal(t, MsgDisconnect, msg.Type)
		var payload DisconnectPayload
		require.NoError(t, msg.ParsePayload(&payload))
		assert.Equal(t, "integration test complete", payload.Reason)
		assert.Equal(t, DisconnectNormal, payload.Code)
	case <-time.After(3 * time.Second):
		t.Error("timeout waiting for disconnect message on the worker side")
	}

	// Allow cleanup to propagate.
	time.Sleep(200 * time.Millisecond)

	// After graceful close, the controller should have 0 peers.
	assert.Equal(t, 0, controllerTransport.ConnectedPeers(),
		"controller should have 0 peers after graceful close")
}

// TestIntegration_SharedSecretAgreement verifies that two independently created
// nodes derive the same shared secret via ECDH.
func TestIntegration_SharedSecretAgreement(t *testing.T) {
	nodeA := testNode(t, "secret-node-a", RoleDual)
	nodeB := testNode(t, "secret-node-b", RoleDual)

	pubKeyA := nodeA.GetIdentity().PublicKey
	pubKeyB := nodeB.GetIdentity().PublicKey

	secretFromA, err := nodeA.DeriveSharedSecret(pubKeyB)
	require.NoError(t, err)

	secretFromB, err := nodeB.DeriveSharedSecret(pubKeyA)
	require.NoError(t, err)

	assert.Equal(t, secretFromA, secretFromB,
		"both nodes should derive identical shared secrets via ECDH")
	assert.Equal(t, 32, len(secretFromA), "shared secret should be 32 bytes")
}

// TestIntegration_TwoNodeBidirectionalMessages verifies that both nodes
// can send and receive encrypted messages after the handshake.
func TestIntegration_TwoNodeBidirectionalMessages(t *testing.T) {
	controller, _, tp := setupControllerPair(t)
	serverID := tp.ServerNode.GetIdentity().ID

	// Controller -> Worker: Ping
	rtt, err := controller.PingPeer(serverID)
	require.NoError(t, err)
	assert.Greater(t, rtt, 0.0)

	// Controller -> Worker: GetStats
	stats, err := controller.GetRemoteStats(serverID)
	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.NotEmpty(t, stats.NodeID)

	// Verify multiple sequential round-trips work.
	for i := 0; i < 5; i++ {
		rtt, err := controller.PingPeer(serverID)
		require.NoError(t, err, "sequential ping %d should succeed", i)
		assert.Greater(t, rtt, 0.0)
	}
}

// TestIntegration_MultiPeerTopology verifies that a controller can
// simultaneously communicate with multiple workers.
func TestIntegration_MultiPeerTopology(t *testing.T) {
	controllerNM := testNode(t, "multi-controller", RoleController)
	controllerReg := testRegistry(t)
	controllerTransport := NewTransport(controllerNM, controllerReg, DefaultTransportConfig())
	t.Cleanup(func() { controllerTransport.Stop() })

	const numWorkers = 3
	workerIDs := make([]string, numWorkers)

	for i := 0; i < numWorkers; i++ {
		nm, addr, _ := makeWorkerServer(t)
		wID := nm.GetIdentity().ID
		workerIDs[i] = wID

		peer := &Peer{
			ID:      wID,
			Name:    "multi-worker",
			Address: addr,
			Role:    RoleWorker,
		}
		require.NoError(t, controllerReg.AddPeer(peer))

		_, err := controllerTransport.Connect(peer)
		require.NoError(t, err, "connecting to worker %d should succeed", i)
	}

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, numWorkers, controllerTransport.ConnectedPeers(),
		"controller should be connected to all workers")

	controller := NewController(controllerNM, controllerReg, controllerTransport)

	// Ping all workers concurrently.
	var wg sync.WaitGroup
	results := make([]float64, numWorkers)
	errs := make([]error, numWorkers)

	for i, wID := range workerIDs {
		wg.Add(1)
		go func(idx int, peerID string) {
			defer wg.Done()
			results[idx], errs[idx] = controller.PingPeer(peerID)
		}(i, wID)
	}
	wg.Wait()

	for i := range numWorkers {
		require.NoError(t, errs[i], "ping to worker %d should succeed", i)
		assert.Greater(t, results[i], 0.0, "RTT for worker %d should be positive", i)
	}

	// Fetch stats from all workers in parallel.
	allStats := controller.GetAllStats()
	assert.Len(t, allStats, numWorkers, "should get stats from all workers")
}

// TestIntegration_IdentityPersistenceAndReload verifies that a node identity
// can be generated, persisted, and reloaded from disk.
func TestIntegration_IdentityPersistenceAndReload(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "private.key")
	configPath := filepath.Join(dir, "node.json")

	// Create and persist identity.
	nm1, err := NewNodeManagerWithPaths(keyPath, configPath)
	require.NoError(t, err)
	require.NoError(t, nm1.GenerateIdentity("persistent-node", RoleDual))

	original := nm1.GetIdentity()
	require.NotNil(t, original)

	// Reload from disk.
	nm2, err := NewNodeManagerWithPaths(keyPath, configPath)
	require.NoError(t, err)
	require.True(t, nm2.HasIdentity(), "identity should be loaded from disk")

	reloaded := nm2.GetIdentity()
	require.NotNil(t, reloaded)

	assert.Equal(t, original.ID, reloaded.ID, "ID should persist")
	assert.Equal(t, original.Name, reloaded.Name, "Name should persist")
	assert.Equal(t, original.PublicKey, reloaded.PublicKey, "PublicKey should persist")
	assert.Equal(t, original.Role, reloaded.Role, "Role should persist")

	// Verify the reloaded key can derive the same shared secret.
	kp, err := stmfGenerateKeyPair()
	require.NoError(t, err)

	secret1, err := nm1.DeriveSharedSecret(kp)
	require.NoError(t, err)

	secret2, err := nm2.DeriveSharedSecret(kp)
	require.NoError(t, err)

	assert.Equal(t, secret1, secret2,
		"shared secrets derived from original and reloaded keys should match")
}

// stmfGenerateKeyPair is a helper that generates a keypair and returns
// the public key as base64 (for use in DeriveSharedSecret tests).
func stmfGenerateKeyPair() (string, error) {
	dir, _ := filepath.Abs("/tmp/stmf-test-" + time.Now().Format("20060102150405.000"))
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	if err != nil {
		return "", err
	}
	if err := nm.GenerateIdentity("temp-peer", RoleWorker); err != nil {
		return "", err
	}
	return nm.GetIdentity().PublicKey, nil
}

// TestIntegration_UEPSFullRoundTrip exercises a complete UEPS packet
// lifecycle: build, sign, transmit (simulated), read, verify, dispatch.
func TestIntegration_UEPSFullRoundTrip(t *testing.T) {
	nodeA := testNode(t, "ueps-node-a", RoleController)
	nodeB := testNode(t, "ueps-node-b", RoleWorker)

	bPubKey := nodeB.GetIdentity().PublicKey
	sharedSecret, err := nodeA.DeriveSharedSecret(bPubKey)
	require.NoError(t, err, "shared secret derivation should succeed")
	require.Len(t, sharedSecret, 32, "shared secret should be 32 bytes (SHA-256)")

	// Build and sign a UEPS packet.
	payload := []byte(`{"intent":"compute","job_id":"block-99"}`)
	pb := ueps.NewBuilder(IntentCompute, payload)
	pb.Header.ThreatScore = 100

	wireData, err := pb.MarshalAndSign(sharedSecret)
	require.NoError(t, err)
	require.NotEmpty(t, wireData)

	// Node B derives the same shared secret from A's public key.
	aPubKey := nodeA.GetIdentity().PublicKey
	sharedSecretB, err := nodeB.DeriveSharedSecret(aPubKey)
	require.NoError(t, err)
	assert.Equal(t, sharedSecret, sharedSecretB,
		"both sides should derive the same shared secret via ECDH")

	parsed, err := ueps.ReadAndVerify(
		bufio.NewReader(bytes.NewReader(wireData)),
		sharedSecretB,
	)
	require.NoError(t, err, "ReadAndVerify should succeed with the matching shared secret")

	assert.Equal(t, byte(0x09), parsed.Header.Version)
	assert.Equal(t, IntentCompute, parsed.Header.IntentID)
	assert.Equal(t, uint16(100), parsed.Header.ThreatScore)
	assert.Equal(t, payload, parsed.Payload)

	// Dispatch through the dispatcher.
	dispatcher := NewDispatcher()
	var dispatched bool
	dispatcher.RegisterHandler(IntentCompute, func(pkt *ueps.ParsedPacket) error {
		dispatched = true
		assert.Equal(t, payload, pkt.Payload)
		return nil
	})

	require.NoError(t, dispatcher.Dispatch(parsed))
	assert.True(t, dispatched, "handler should have been called")
}

// TestIntegration_UEPSIntegrityFailure verifies that a tampered UEPS packet
// is rejected by HMAC verification.
func TestIntegration_UEPSIntegrityFailure(t *testing.T) {
	nodeA := testNode(t, "integrity-a", RoleController)
	nodeB := testNode(t, "integrity-b", RoleWorker)

	bPubKey := nodeB.GetIdentity().PublicKey
	sharedSecret, err := nodeA.DeriveSharedSecret(bPubKey)
	require.NoError(t, err)

	pb := ueps.NewBuilder(IntentHandshake, []byte("legitimate data"))
	wireData, err := pb.MarshalAndSign(sharedSecret)
	require.NoError(t, err)

	// Tamper with the payload (last bytes).
	tampered := make([]byte, len(wireData))
	copy(tampered, wireData)
	tampered[len(tampered)-1] ^= 0xFF

	aPubKey := nodeA.GetIdentity().PublicKey
	sharedSecretB, err := nodeB.DeriveSharedSecret(aPubKey)
	require.NoError(t, err)

	_, err = ueps.ReadAndVerify(
		bufio.NewReader(bytes.NewReader(tampered)),
		sharedSecretB,
	)
	assert.Error(t, err, "tampered packet should fail HMAC verification")
	assert.Contains(t, err.Error(), "HMAC mismatch")
}

// TestIntegration_AllowlistHandshakeRejection verifies that a peer not in the
// allowlist is rejected during the WebSocket handshake.
func TestIntegration_AllowlistHandshakeRejection(t *testing.T) {
	workerNM := testNode(t, "allowlist-worker", RoleWorker)
	workerReg := testRegistry(t)
	workerReg.SetAuthMode(PeerAuthAllowlist)

	workerTransport := NewTransport(workerNM, workerReg, DefaultTransportConfig())

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", workerTransport.handleWSUpgrade)
	ts := httptest.NewServer(mux)
	t.Cleanup(func() {
		workerTransport.Stop()
		ts.Close()
	})

	u, _ := url.Parse(ts.URL)

	controllerNM := testNode(t, "rejected-controller", RoleController)
	controllerReg := testRegistry(t)
	controllerTransport := NewTransport(controllerNM, controllerReg, DefaultTransportConfig())
	t.Cleanup(func() { controllerTransport.Stop() })

	peer := &Peer{
		ID:      workerNM.GetIdentity().ID,
		Name:    "worker",
		Address: u.Host,
		Role:    RoleWorker,
	}
	controllerReg.AddPeer(peer)

	_, err := controllerTransport.Connect(peer)
	require.Error(t, err, "connection should be rejected by allowlist")
	assert.Contains(t, err.Error(), "rejected")
}

// TestIntegration_AllowlistHandshakeAccepted verifies that an allowlisted
// peer can connect successfully.
func TestIntegration_AllowlistHandshakeAccepted(t *testing.T) {
	workerNM := testNode(t, "allowlist-worker-ok", RoleWorker)
	workerReg := testRegistry(t)
	workerReg.SetAuthMode(PeerAuthAllowlist)

	controllerNM := testNode(t, "allowed-controller", RoleController)
	controllerReg := testRegistry(t)

	workerReg.AllowPublicKey(controllerNM.GetIdentity().PublicKey)

	workerTransport := NewTransport(workerNM, workerReg, DefaultTransportConfig())
	worker := NewWorker(workerNM, workerTransport)
	worker.RegisterWithTransport()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", workerTransport.handleWSUpgrade)
	ts := httptest.NewServer(mux)
	t.Cleanup(func() {
		workerTransport.Stop()
		ts.Close()
	})

	u, _ := url.Parse(ts.URL)

	controllerTransport := NewTransport(controllerNM, controllerReg, DefaultTransportConfig())
	t.Cleanup(func() { controllerTransport.Stop() })

	peer := &Peer{
		ID:      workerNM.GetIdentity().ID,
		Name:    "worker",
		Address: u.Host,
		Role:    RoleWorker,
	}
	controllerReg.AddPeer(peer)

	pc, err := controllerTransport.Connect(peer)
	require.NoError(t, err, "allowlisted peer should connect successfully")
	assert.NotEmpty(t, pc.SharedSecret)
}

// TestIntegration_DispatcherWithRealUEPSPackets builds real UEPS packets
// from wire bytes and routes them through the dispatcher.
func TestIntegration_DispatcherWithRealUEPSPackets(t *testing.T) {
	sharedSecret := make([]byte, 32)
	for i := range sharedSecret {
		sharedSecret[i] = byte(i ^ 0x42)
	}

	dispatcher := NewDispatcher()
	var results sync.Map

	intents := []struct {
		id      byte
		name    string
		payload string
	}{
		{IntentHandshake, "handshake", "hello"},
		{IntentCompute, "compute", `{"job":"123"}`},
		{IntentRehab, "rehab", "pause"},
		{IntentCustom, "custom", "app-specific-data"},
	}

	for _, intent := range intents {
		intentID := intent.id
		dispatcher.RegisterHandler(intentID, func(pkt *ueps.ParsedPacket) error {
			results.Store(pkt.Header.IntentID, string(pkt.Payload))
			return nil
		})
	}

	for _, intent := range intents {
		t.Run(intent.name, func(t *testing.T) {
			pb := ueps.NewBuilder(intent.id, []byte(intent.payload))
			wireData, err := pb.MarshalAndSign(sharedSecret)
			require.NoError(t, err)

			parsed, err := ueps.ReadAndVerify(
				bufio.NewReader(bytes.NewReader(wireData)),
				sharedSecret,
			)
			require.NoError(t, err)
			require.NoError(t, dispatcher.Dispatch(parsed))

			val, ok := results.Load(intent.id)
			require.True(t, ok, "handler for %s should have been called", intent.name)
			assert.Equal(t, intent.payload, val)
		})
	}
}

// TestIntegration_MessageSerialiseDeserialise verifies that messages survive
// the full serialisation/encryption/decryption/deserialisation pipeline
// with all fields intact.
func TestIntegration_MessageSerialiseDeserialise(t *testing.T) {
	tp := setupTestTransportPair(t)
	pc := tp.connectClient(t)

	original, err := NewMessage(MsgStats, tp.ClientNode.GetIdentity().ID, tp.ServerNode.GetIdentity().ID, StatsPayload{
		NodeID:   "test-node",
		NodeName: "test-name",
		Miners: []MinerStatsItem{
			{
				Name:       "miner-0",
				Type:       "xmrig",
				Hashrate:   9999.9,
				Shares:     500,
				Rejected:   3,
				Uptime:     7200,
				Pool:       "pool.example.com:3333",
				Algorithm:  "rx/0",
				CPUThreads: 8,
			},
		},
		Uptime: 86400,
	})
	require.NoError(t, err)
	original.ReplyTo = "parent-msg-id-12345"

	encrypted, err := tp.Client.encryptMessage(original, pc.SharedSecret)
	require.NoError(t, err)
	require.NotEmpty(t, encrypted)

	decrypted, err := tp.Client.decryptMessage(encrypted, pc.SharedSecret)
	require.NoError(t, err)

	assert.Equal(t, original.ID, decrypted.ID)
	assert.Equal(t, original.Type, decrypted.Type)
	assert.Equal(t, original.From, decrypted.From)
	assert.Equal(t, original.To, decrypted.To)
	assert.Equal(t, original.ReplyTo, decrypted.ReplyTo)

	var originalStats, decryptedStats StatsPayload
	require.NoError(t, json.Unmarshal(original.Payload, &originalStats))
	require.NoError(t, json.Unmarshal(decrypted.Payload, &decryptedStats))
	assert.Equal(t, originalStats, decryptedStats)
}

// TestIntegration_GetRemoteStats_EndToEnd tests the full stats retrieval flow
// across a real WebSocket connection.
func TestIntegration_GetRemoteStats_EndToEnd(t *testing.T) {
	tp := setupTestTransportPair(t)

	worker := NewWorker(tp.ServerNode, tp.Server)
	worker.RegisterWithTransport()

	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)

	tp.connectClient(t)
	time.Sleep(100 * time.Millisecond)

	serverID := tp.ServerNode.GetIdentity().ID

	stats, err := controller.GetRemoteStats(serverID)
	require.NoError(t, err, "GetRemoteStats should succeed end-to-end")
	require.NotNil(t, stats)
	assert.Equal(t, serverID, stats.NodeID)
	assert.Equal(t, "server", stats.NodeName)
	assert.GreaterOrEqual(t, stats.Uptime, int64(0))
}
