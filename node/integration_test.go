package node

import (
	"bufio"
	core "dappco.re/go"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dappco.re/go/p2p/ueps"
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
	if controllerIdentity == nil {
		t.Fatal("expected non-nil")
	}
	if workerIdentity == nil {
		t.Fatal("expected non-nil")
	}
	if len(controllerIdentity.ID) == 0 {
		t.Fatal("expected non-empty")
	}
	if len(workerIdentity.ID) == 0 {
		t.Fatal("expected non-empty")
	}
	if reflect.DeepEqual(controllerIdentity.ID, workerIdentity.ID) {
		t.Fatalf("did not want %v", workerIdentity.ID)
	}
	if !reflect.DeepEqual(RoleController, controllerIdentity.Role) {
		t.Fatalf("want %v, got %v", RoleController, controllerIdentity.Role)
	}
	if !reflect.DeepEqual(RoleWorker, workerIdentity.Role) {
		t.Fatalf("want %v, got %v", RoleWorker, workerIdentity.Role)
	}

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
				stats: map[string]any{
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
	if err := controllerReg.AddPeer(workerPeer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create the controller (registers handleResponse on the transport).
	controller := NewController(controllerNM, controllerReg, controllerTransport)

	// ----------------------------------------------------------------
	// Step 3: WebSocket handshake (challenge-response)
	// ----------------------------------------------------------------
	pc, err := controllerTransport.Connect(workerPeer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pc == nil {
		t.Fatal("expected non-nil")
	}
	if len(pc.SharedSecret) == 0 {
		t.Fatal("expected non-empty")
	}

	// Allow server-side goroutines to register the connection.
	time.Sleep(100 * time.Millisecond)

	if !reflect.DeepEqual(1, controllerTransport.ConnectedPeers()) {
		t.Fatalf("want %v, got %v", 1, controllerTransport.ConnectedPeers())
	}
	if !reflect.DeepEqual(1, workerTransport.ConnectedPeers()) {
		t.Fatalf("want %v, got %v", 1, workerTransport.ConnectedPeers())
	}

	// Verify the peer's real identity is stored.
	serverPeerID := workerNM.GetIdentity().ID
	conn := controllerTransport.GetConnection(serverPeerID)
	if conn == nil {
		t.Fatal("expected non-nil")
	}
	if !reflect.DeepEqual("integration-worker", conn.Peer.Name) {
		t.Fatalf("want %v, got %v", "integration-worker", conn.Peer.Name)
	}

	// ----------------------------------------------------------------
	// Step 4: Encrypted message exchange — Ping/Pong
	// ----------------------------------------------------------------
	rtt, err := controller.PingPeer(serverPeerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !(rtt > 0.0) {
		t.Fatalf("expected %v to be greater than %v", rtt, 0.0)
	}
	if !(rtt < 1000.0) {
		t.Fatalf("expected %v to be less than %v", rtt, 1000.0)
	}

	// Verify registry metrics were updated.
	peerAfterPing := controllerReg.GetPeer(serverPeerID)
	if peerAfterPing == nil {
		t.Fatal("expected non-nil")
	}
	if !(peerAfterPing.PingMS > 0.0) {
		t.Fatalf("expected %v to be greater than %v", peerAfterPing.PingMS, 0.0)
	}

	// ----------------------------------------------------------------
	// Step 5: Encrypted message exchange — GetRemoteStats
	// ----------------------------------------------------------------
	stats, err := controller.GetRemoteStats(serverPeerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats == nil {
		t.Fatal("expected non-nil")
	}
	if !reflect.DeepEqual(workerIdentity.ID, stats.NodeID) {
		t.Fatalf("want %v, got %v", workerIdentity.ID, stats.NodeID)
	}
	if !reflect.DeepEqual("integration-worker", stats.NodeName) {
		t.Fatalf("want %v, got %v", "integration-worker", stats.NodeName)
	}
	if len(stats.Miners) != 1 {
		t.Fatalf("want len %v, got %v", 1, len(stats.Miners))
	}
	if !reflect.DeepEqual("integration-miner", stats.Miners[0].Name) {
		t.Fatalf("want %v, got %v", "integration-miner", stats.Miners[0].Name)
	}
	if !reflect.DeepEqual(5000.0, stats.Miners[0].Hashrate) {
		t.Fatalf("want %v, got %v", 5000.0, stats.Miners[0].Hashrate)
	}

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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parsed, err := ueps.ReadAndVerify(bufio.NewReader(core.NewBuffer(wireData)), sharedSecret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := dispatcher.Dispatch(parsed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(int32(1), handshakeReceived.Load()) {
		t.Fatalf("want %v, got %v", int32(1), handshakeReceived.Load())
	}

	// 6b. Compute intent.
	pb2 := ueps.NewBuilder(IntentCompute, []byte(`{"job":"mine-block-42"}`))
	wireData2, err := pb2.MarshalAndSign(sharedSecret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parsed2, err := ueps.ReadAndVerify(bufio.NewReader(core.NewBuffer(wireData2)), sharedSecret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := dispatcher.Dispatch(parsed2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(int32(1), computeReceived.Load()) {
		t.Fatalf("want %v, got %v", int32(1), computeReceived.Load())
	}

	// 6c. High-threat packet should be rejected by the circuit breaker.
	pb3 := ueps.NewBuilder(IntentCompute, []byte("hostile"))
	pb3.Header.ThreatScore = ThreatScoreThreshold + 1
	wireData3, err := pb3.MarshalAndSign(sharedSecret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parsed3, err := ueps.ReadAndVerify(bufio.NewReader(core.NewBuffer(wireData3)), sharedSecret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = dispatcher.Dispatch(parsed3)
	if !core.Is(err, ErrThreatScoreExceeded) {
		t.Fatalf("expected error %v, got %v", ErrThreatScoreExceeded, err)
	}
	// Compute handler should NOT have been called again.
	if !reflect.DeepEqual(int32(1), computeReceived.Load()) {
		t.Fatalf("want %v, got %v", int32(1), computeReceived.Load())
	}

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
		if !reflect.DeepEqual(MsgDisconnect, msg.Type) {
			t.Fatalf("want %v, got %v", MsgDisconnect, msg.Type)
		}
		var payload DisconnectPayload
		if err := msg.ParsePayload(&payload); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual("integration test complete", payload.Reason) {
			t.Fatalf("want %v, got %v", "integration test complete", payload.Reason)
		}
		if !reflect.DeepEqual(DisconnectNormal, payload.Code) {
			t.Fatalf("want %v, got %v", DisconnectNormal, payload.Code)
		}
	case <-time.After(3 * time.Second):
		t.Error("timeout waiting for disconnect message on the worker side")
	}

	// Allow cleanup to propagate.
	time.Sleep(200 * time.Millisecond)

	// After graceful close, the controller should have 0 peers.
	if !reflect.DeepEqual(0, controllerTransport.ConnectedPeers()) {
		t.Fatalf("want %v, got %v", 0, controllerTransport.ConnectedPeers())
	}
}

// TestIntegration_SharedSecretAgreement verifies that two independently created
// nodes derive the same shared secret via ECDH.
func TestIntegration_SharedSecretAgreement(t *testing.T) {
	nodeA := testNode(t, "secret-node-a", RoleDual)
	nodeB := testNode(t, "secret-node-b", RoleDual)

	pubKeyA := nodeA.GetIdentity().PublicKey
	pubKeyB := nodeB.GetIdentity().PublicKey

	secretFromA, err := nodeA.DeriveSharedSecret(pubKeyB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	secretFromB, err := nodeB.DeriveSharedSecret(pubKeyA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(secretFromA, secretFromB) {
		t.Fatalf("want %v, got %v", secretFromA, secretFromB)
	}
	if !reflect.DeepEqual(32, len(secretFromA)) {
		t.Fatalf("want %v, got %v", 32, len(secretFromA))
	}
}

// TestIntegration_TwoNodeBidirectionalMessages verifies that both nodes
// can send and receive encrypted messages after the handshake.
func TestIntegration_TwoNodeBidirectionalMessages(t *testing.T) {
	controller, _, tp := setupControllerPair(t)
	serverID := tp.ServerNode.GetIdentity().ID

	// Controller -> Worker: Ping
	rtt, err := controller.PingPeer(serverID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !(rtt > 0.0) {
		t.Fatalf("expected %v to be greater than %v", rtt, 0.0)
	}

	// Controller -> Worker: GetStats
	stats, err := controller.GetRemoteStats(serverID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats == nil {
		t.Fatal("expected non-nil")
	}
	if len(stats.NodeID) == 0 {
		t.Fatal("expected non-empty")
	}

	// Verify multiple sequential round-trips work.
	for range 5 {
		rtt, err := controller.PingPeer(serverID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !(rtt > 0.0) {
			t.Fatalf("expected %v to be greater than %v", rtt, 0.0)
		}
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

	for i := range numWorkers {
		nm, addr, _ := makeWorkerServer(t)
		wID := nm.GetIdentity().ID
		workerIDs[i] = wID

		peer := &Peer{
			ID:      wID,
			Name:    "multi-worker",
			Address: addr,
			Role:    RoleWorker,
		}
		if err := controllerReg.AddPeer(peer); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err := controllerTransport.Connect(peer)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	time.Sleep(100 * time.Millisecond)
	if !reflect.DeepEqual(numWorkers, controllerTransport.ConnectedPeers()) {
		t.Fatalf("want %v, got %v", numWorkers, controllerTransport.ConnectedPeers())
	}

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
		if err := errs[i]; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !(results[i] > 0.0) {
			t.Fatalf("expected %v to be greater than %v", results[i], 0.0)
		}
	}

	// Fetch stats from all workers in parallel.
	allStats := controller.GetAllStats()
	if len(allStats) != numWorkers {
		t.Fatalf("want len %v, got %v", numWorkers, len(allStats))
	}
}

// TestIntegration_IdentityPersistenceAndReload verifies that a node identity
// can be generated, persisted, and reloaded from disk.
func TestIntegration_IdentityPersistenceAndReload(t *testing.T) {
	dir := t.TempDir()
	keyPath := core.PathJoin(dir, "private.key")
	configPath := core.PathJoin(dir, "node.json")

	// Create and persist identity.
	nm1, err := NewNodeManagerWithPaths(keyPath, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := nm1.GenerateIdentity("persistent-node", RoleDual); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	original := nm1.GetIdentity()
	if original == nil {
		t.Fatal("expected non-nil")
	}

	// Reload from disk.
	nm2, err := NewNodeManagerWithPaths(keyPath, configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !(nm2.HasIdentity()) {
		t.Fatal("expected true")
	}

	reloaded := nm2.GetIdentity()
	if reloaded == nil {
		t.Fatal("expected non-nil")
	}

	if !reflect.DeepEqual(original.ID, reloaded.ID) {
		t.Fatalf("want %v, got %v", original.ID, reloaded.ID)
	}
	if !reflect.DeepEqual(original.Name, reloaded.Name) {
		t.Fatalf("want %v, got %v", original.Name, reloaded.Name)
	}
	if !reflect.DeepEqual(original.PublicKey, reloaded.PublicKey) {
		t.Fatalf("want %v, got %v", original.PublicKey, reloaded.PublicKey)
	}
	if !reflect.DeepEqual(original.Role, reloaded.Role) {
		t.Fatalf("want %v, got %v", original.Role, reloaded.Role)
	}

	// Verify the reloaded key can derive the same shared secret.
	kp, err := stmfGenerateKeyPair(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	secret1, err := nm1.DeriveSharedSecret(kp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	secret2, err := nm2.DeriveSharedSecret(kp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(secret1, secret2) {
		t.Fatalf("want %v, got %v", secret1, secret2)
	}
}

// stmfGenerateKeyPair is a helper that generates a keypair and returns
// the public key as base64 (for use in DeriveSharedSecret tests).
func stmfGenerateKeyPair(dir string) (string, error) {
	nm, err := NewNodeManagerWithPaths(
		core.PathJoin(dir, "private.key"),
		core.PathJoin(dir, "node.json"),
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sharedSecret) != 32 {
		t.Fatalf("want len %v, got %v", 32, len(sharedSecret))
	}

	// Build and sign a UEPS packet.
	payload := []byte(`{"intent":"compute","job_id":"block-99"}`)
	pb := ueps.NewBuilder(IntentCompute, payload)
	pb.Header.ThreatScore = 100

	wireData, err := pb.MarshalAndSign(sharedSecret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wireData) == 0 {
		t.Fatal("expected non-empty")
	}

	// Node B derives the same shared secret from A's public key.
	aPubKey := nodeA.GetIdentity().PublicKey
	sharedSecretB, err := nodeB.DeriveSharedSecret(aPubKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(sharedSecret, sharedSecretB) {
		t.Fatalf("want %v, got %v", sharedSecret, sharedSecretB)
	}

	parsed, err := ueps.ReadAndVerify(
		bufio.NewReader(core.NewBuffer(wireData)),
		sharedSecretB,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(byte(0x09), parsed.Header.Version) {
		t.Fatalf("want %v, got %v", byte(0x09), parsed.Header.Version)
	}
	if !reflect.DeepEqual(IntentCompute, parsed.Header.IntentID) {
		t.Fatalf("want %v, got %v", IntentCompute, parsed.Header.IntentID)
	}
	if !reflect.DeepEqual(uint16(100), parsed.Header.ThreatScore) {
		t.Fatalf("want %v, got %v", uint16(100), parsed.Header.ThreatScore)
	}
	if !reflect.DeepEqual(payload, parsed.Payload) {
		t.Fatalf("want %v, got %v", payload, parsed.Payload)
	}

	// Dispatch through the dispatcher.
	dispatcher := NewDispatcher()
	var dispatched bool
	dispatcher.RegisterHandler(IntentCompute, func(pkt *ueps.ParsedPacket) error {
		dispatched = true
		if !reflect.DeepEqual(payload, pkt.Payload) {
			t.Fatalf("want %v, got %v", payload, pkt.Payload)
		}
		return nil
	})

	if err := dispatcher.Dispatch(parsed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !(dispatched) {
		t.Fatal("expected true")
	}
}

// TestIntegration_UEPSIntegrityFailure verifies that a tampered UEPS packet
// is rejected by HMAC verification.
func TestIntegration_UEPSIntegrityFailure(t *testing.T) {
	nodeA := testNode(t, "integrity-a", RoleController)
	nodeB := testNode(t, "integrity-b", RoleWorker)

	bPubKey := nodeB.GetIdentity().PublicKey
	sharedSecret, err := nodeA.DeriveSharedSecret(bPubKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pb := ueps.NewBuilder(IntentHandshake, []byte("legitimate data"))
	wireData, err := pb.MarshalAndSign(sharedSecret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Tamper with the payload (last bytes).
	tampered := make([]byte, len(wireData))
	copy(tampered, wireData)
	tampered[len(tampered)-1] ^= 0xFF

	aPubKey := nodeA.GetIdentity().PublicKey
	sharedSecretB, err := nodeB.DeriveSharedSecret(aPubKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = ueps.ReadAndVerify(
		bufio.NewReader(core.NewBuffer(tampered)),
		sharedSecretB,
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !core.Contains(err.Error(), "HMAC mismatch") {
		t.Fatalf("expected %q to contain %q", err.Error(), "HMAC mismatch")
	}
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
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !core.Contains(err.Error(), "rejected") {
		t.Fatalf("expected %q to contain %q", err.Error(), "rejected")
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pc.SharedSecret) == 0 {
		t.Fatal("expected non-empty")
	}
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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			parsed, err := ueps.ReadAndVerify(
				bufio.NewReader(core.NewBuffer(wireData)),
				sharedSecret,
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := dispatcher.Dispatch(parsed); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			val, ok := results.Load(intent.id)
			if !(ok) {
				t.Fatal("expected true")
			}
			if !reflect.DeepEqual(intent.payload, val) {
				t.Fatalf("want %v, got %v", intent.payload, val)
			}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	original.ReplyTo = "parent-msg-id-12345"

	encrypted, err := tp.Client.encryptMessage(original, pc.SharedSecret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(encrypted) == 0 {
		t.Fatal("expected non-empty")
	}

	decrypted, err := tp.Client.decryptMessage(encrypted, pc.SharedSecret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(original.ID, decrypted.ID) {
		t.Fatalf("want %v, got %v", original.ID, decrypted.ID)
	}
	if !reflect.DeepEqual(original.Type, decrypted.Type) {
		t.Fatalf("want %v, got %v", original.Type, decrypted.Type)
	}
	if !reflect.DeepEqual(original.From, decrypted.From) {
		t.Fatalf("want %v, got %v", original.From, decrypted.From)
	}
	if !reflect.DeepEqual(original.To, decrypted.To) {
		t.Fatalf("want %v, got %v", original.To, decrypted.To)
	}
	if !reflect.DeepEqual(original.ReplyTo, decrypted.ReplyTo) {
		t.Fatalf("want %v, got %v", original.ReplyTo, decrypted.ReplyTo)
	}

	var originalStats, decryptedStats StatsPayload
	if err := testJSONUnmarshal(original.Payload, &originalStats); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := testJSONUnmarshal(decrypted.Payload, &decryptedStats); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(originalStats, decryptedStats) {
		t.Fatalf("want %v, got %v", originalStats, decryptedStats)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats == nil {
		t.Fatal("expected non-nil")
	}
	if !reflect.DeepEqual(serverID, stats.NodeID) {
		t.Fatalf("want %v, got %v", serverID, stats.NodeID)
	}
	if !reflect.DeepEqual("server", stats.NodeName) {
		t.Fatalf("want %v, got %v", "server", stats.NodeName)
	}
	if !(stats.Uptime >= int64(0)) {
		t.Fatalf("expected %v to be greater than or equal to %v", stats.Uptime, int64(0))
	}
}
