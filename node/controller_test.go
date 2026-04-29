package node

import (
	core "dappco.re/go"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// setupControllerPair creates a controller (client-side) connected to a worker
// (server-side) over a real WebSocket transport pair.  Returns the controller,
// worker, and the underlying testTransportPair so callers can inspect internal state.
func setupControllerPair(t *testing.T) (*Controller, *Worker, *testTransportPair) {
	t.Helper()

	tp := setupTestTransportPair(t)

	// Server side: register a Worker to handle incoming requests.
	worker := NewWorker(tp.ServerNode, tp.Server)
	worker.RegisterWithTransport()

	// Client side: create a Controller (registers handleResponse via OnMessage).
	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)

	// Establish the WebSocket connection and complete the handshake.
	tp.connectClient(t)

	// Allow connection to fully settle on both sides.
	time.Sleep(50 * time.Millisecond)

	return controller, worker, tp
}

// makeWorkerServer spins up an independent server transport with a Worker
// registered, returning the server's NodeManager, address, and a cleanup func.
// Useful for multi-peer tests (GetAllStats, ConcurrentRequests).
func makeWorkerServer(t *testing.T) (*NodeManager, string, *Transport) {
	t.Helper()

	nm := testNode(t, "worker", RoleWorker)
	reg := testRegistry(t)
	cfg := DefaultTransportConfig()
	srv := NewTransport(nm, reg, cfg)

	mux := http.NewServeMux()
	mux.HandleFunc(cfg.WSPath, srv.handleWSUpgrade)
	ts := httptest.NewServer(mux)

	u, _ := url.Parse(ts.URL)

	worker := NewWorker(nm, srv)
	worker.RegisterWithTransport()

	t.Cleanup(func() {
		// Brief pause to let in-flight readLoop/Send operations finish before
		// Stop() calls GracefulClose.  Without this, the race detector flags a
		// pre-existing race in transport.go (GracefulClose vs Send on
		// SetWriteDeadline — see FINDINGS.md).
		time.Sleep(50 * time.Millisecond)
		srv.Stop()
		ts.Close()
	})

	return nm, u.Host, srv
}

// --- Controller Tests ---

func TestController_RequestResponseCorrelation(t *testing.T) {
	controller, _, tp := setupControllerPair(t)
	serverID := tp.ServerNode.GetIdentity().ID

	// Send a ping request via the controller; the server-side worker
	// replies with MsgPong, setting ReplyTo to the original message ID.
	rtt, err := controller.PingPeer(serverID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !(rtt > 0.0) {
		t.Fatalf("expected %v to be greater than %v", rtt, 0.0)
	}
}

func TestController_NewController_Good(t *testing.T) {
	tp := setupTestTransportPair(t)
	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)
	if controller == nil {
		t.Fatal("expected controller")
	}
	if tp.Client.handler == nil {
		t.Fatal("response handler not registered")
	}
}

func TestController_NewController_Bad(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for nil transport")
		}
	}()
	_ = NewController(nil, nil, nil)
}

func TestController_NewController_Ugly(t *testing.T) {
	tp := setupTestTransportPair(t)
	first := NewController(tp.ClientNode, tp.ClientReg, tp.Client)
	second := NewController(tp.ClientNode, tp.ClientReg, tp.Client)
	if first == second {
		t.Fatal("expected distinct controllers")
	}
	if tp.Client.handler == nil {
		t.Fatal("handler should remain registered")
	}
}

func TestController_Controller_GetRemoteStats_Good(t *testing.T) {
	controller, _, tp := setupControllerPair(t)
	stats, err := controller.GetRemoteStats(tp.ServerNode.GetIdentity().ID)
	if err != nil {
		t.Fatalf("GetRemoteStats: %v", err)
	}
	if stats == nil || stats.NodeID == "" {
		t.Fatalf("stats: %#v", stats)
	}
}

func TestController_Controller_GetRemoteStats_Bad(t *testing.T) {
	tp := setupTestTransportPair(t)
	nm, _ := NewNodeManagerWithPaths(core.PathJoin(t.TempDir(), "private.key"), core.PathJoin(t.TempDir(), "node.json"))
	controller := NewController(nm, tp.ClientReg, tp.Client)
	stats, err := controller.GetRemoteStats("peer")
	if err == nil {
		t.Fatal("expected identity error")
	}
	if stats != nil {
		t.Fatalf("stats: got %#v, want nil", stats)
	}
}

func TestController_Controller_GetRemoteStats_Ugly(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	stats, err := controller.GetRemoteStats(tp.ServerNode.GetIdentity().ID)
	if err != nil {
		t.Fatalf("GetRemoteStats with miners: %v", err)
	}
	if len(stats.Miners) != 1 {
		t.Fatalf("miners: %#v", stats.Miners)
	}
}

func TestController_Controller_StartRemoteMiner_Good(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	err := controller.StartRemoteMiner(tp.ServerNode.GetIdentity().ID, "xmrig", "", RawMessage(`{"pool":"p"}`))
	if err != nil {
		t.Fatalf("StartRemoteMiner: %v", err)
	}
}

func TestController_Controller_StartRemoteMiner_Bad(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	err := controller.StartRemoteMiner(tp.ServerNode.GetIdentity().ID, "", "", nil)
	if err == nil {
		t.Fatal("expected miner type error")
	}
}

func TestController_Controller_StartRemoteMiner_Ugly(t *testing.T) {
	tp := setupTestTransportPair(t)
	nm, _ := NewNodeManagerWithPaths(core.PathJoin(t.TempDir(), "private.key"), core.PathJoin(t.TempDir(), "node.json"))
	controller := NewController(nm, tp.ClientReg, tp.Client)
	err := controller.StartRemoteMiner("peer", "xmrig", "", nil)
	if err == nil {
		t.Fatal("expected identity error")
	}
}

func TestController_Controller_StopRemoteMiner_Good(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	err := controller.StopRemoteMiner(tp.ServerNode.GetIdentity().ID, "running-miner")
	if err != nil {
		t.Fatalf("StopRemoteMiner: %v", err)
	}
}

func TestController_Controller_StopRemoteMiner_Bad(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	err := controller.StopRemoteMiner(tp.ServerNode.GetIdentity().ID, "missing")
	if err == nil {
		t.Fatal("expected missing miner error")
	}
}

func TestController_Controller_StopRemoteMiner_Ugly(t *testing.T) {
	tp := setupTestTransportPair(t)
	nm, _ := NewNodeManagerWithPaths(core.PathJoin(t.TempDir(), "private.key"), core.PathJoin(t.TempDir(), "node.json"))
	controller := NewController(nm, tp.ClientReg, tp.Client)
	err := controller.StopRemoteMiner("peer", "")
	if err == nil {
		t.Fatal("expected identity error")
	}
}

func TestController_Controller_GetRemoteLogs_Good(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	lines, err := controller.GetRemoteLogs(tp.ServerNode.GetIdentity().ID, "running-miner", 2)
	if err != nil {
		t.Fatalf("GetRemoteLogs: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("lines: %#v", lines)
	}
}

func TestController_Controller_GetRemoteLogs_Bad(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	lines, err := controller.GetRemoteLogs(tp.ServerNode.GetIdentity().ID, "missing", 2)
	if err == nil {
		t.Fatal("expected missing miner error")
	}
	if lines != nil {
		t.Fatalf("lines: %#v", lines)
	}
}

func TestController_Controller_GetRemoteLogs_Ugly(t *testing.T) {
	tp := setupTestTransportPair(t)
	nm, _ := NewNodeManagerWithPaths(core.PathJoin(t.TempDir(), "private.key"), core.PathJoin(t.TempDir(), "node.json"))
	controller := NewController(nm, tp.ClientReg, tp.Client)
	lines, err := controller.GetRemoteLogs("peer", "", 0)
	if err == nil {
		t.Fatal("expected identity error")
	}
	if lines != nil {
		t.Fatalf("lines: %#v", lines)
	}
}

func TestController_Controller_GetRemoteLogsSince_Good(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	since, _ := time.Parse("2006-01-02 15:04:05", "2026-02-20 10:00:01")
	lines, err := controller.GetRemoteLogsSince(tp.ServerNode.GetIdentity().ID, "running-miner", 10, since)
	if err != nil {
		t.Fatalf("GetRemoteLogsSince: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("lines: %#v", lines)
	}
}

func TestController_Controller_GetRemoteLogsSince_Bad(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	lines, err := controller.GetRemoteLogsSince(tp.ServerNode.GetIdentity().ID, "missing", 10, time.Now())
	if err == nil {
		t.Fatal("expected missing miner error")
	}
	if lines != nil {
		t.Fatalf("lines: %#v", lines)
	}
}

func TestController_Controller_GetRemoteLogsSince_Ugly(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	lines, err := controller.GetRemoteLogsSince(tp.ServerNode.GetIdentity().ID, "running-miner", 0, time.Time{})
	if err != nil {
		t.Fatalf("GetRemoteLogsSince zero lines: %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("expected capped log lines")
	}
}

func TestController_Controller_GetAllStats_Good(t *testing.T) {
	controller, _, tp := setupControllerPair(t)
	stats := controller.GetAllStats()
	if len(stats) != 1 {
		t.Fatalf("stats: %#v", stats)
	}
	if stats[tp.ServerNode.GetIdentity().ID] == nil {
		t.Fatal("expected server stats")
	}
}

func TestController_Controller_GetAllStats_Bad(t *testing.T) {
	tp := setupTestTransportPair(t)
	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)
	stats := controller.GetAllStats()
	if len(stats) != 0 {
		t.Fatalf("stats: %#v", stats)
	}
}

func TestController_Controller_GetAllStats_Ugly(t *testing.T) {
	controller, _, tp := setupControllerPair(t)
	_ = tp.Client.Stop()
	stats := controller.GetAllStats()
	if len(stats) != 0 {
		t.Fatalf("stats after stop: %#v", stats)
	}
}

func TestController_Controller_PingPeer_Good(t *testing.T) {
	controller, _, tp := setupControllerPair(t)
	rtt, err := controller.PingPeer(tp.ServerNode.GetIdentity().ID)
	if err != nil {
		t.Fatalf("PingPeer: %v", err)
	}
	if rtt <= 0 {
		t.Fatalf("rtt: got %f", rtt)
	}
}

func TestController_Controller_PingPeer_Bad(t *testing.T) {
	tp := setupTestTransportPair(t)
	nm, _ := NewNodeManagerWithPaths(core.PathJoin(t.TempDir(), "private.key"), core.PathJoin(t.TempDir(), "node.json"))
	controller := NewController(nm, tp.ClientReg, tp.Client)
	rtt, err := controller.PingPeer("peer")
	if err == nil {
		t.Fatal("expected identity error")
	}
	if rtt != 0 {
		t.Fatalf("rtt: got %f", rtt)
	}
}

func TestController_Controller_PingPeer_Ugly(t *testing.T) {
	tp := setupTestTransportPair(t)
	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)
	rtt, err := controller.PingPeer("missing")
	if err == nil {
		t.Fatal("expected missing peer error")
	}
	if rtt != 0 {
		t.Fatalf("rtt: got %f", rtt)
	}
}

func TestController_Controller_ConnectToPeer_Good(t *testing.T) {
	tp := setupTestTransportPair(t)
	worker := NewWorker(tp.ServerNode, tp.Server)
	worker.RegisterWithTransport()
	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)
	peer := &Peer{ID: tp.ServerNode.GetIdentity().ID, Name: "server", Address: tp.ServerAddr, Role: RoleWorker}
	_ = tp.ClientReg.AddPeer(peer)
	if err := controller.ConnectToPeer(peer.ID); err != nil {
		t.Fatalf("ConnectToPeer: %v", err)
	}
}

func TestController_Controller_ConnectToPeer_Bad(t *testing.T) {
	tp := setupTestTransportPair(t)
	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)
	err := controller.ConnectToPeer("missing")
	if err == nil {
		t.Fatal("expected missing peer error")
	}
}

func TestController_Controller_ConnectToPeer_Ugly(t *testing.T) {
	tp := setupTestTransportPair(t)
	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)
	peer := &Peer{ID: "bad", Address: "127.0.0.1:1"}
	_ = tp.ClientReg.AddPeer(peer)
	err := controller.ConnectToPeer("bad")
	if err == nil {
		t.Fatal("expected connect error")
	}
}

func TestController_Controller_DisconnectFromPeer_Good(t *testing.T) {
	controller, _, tp := setupControllerPair(t)
	err := controller.DisconnectFromPeer(tp.ServerNode.GetIdentity().ID)
	if err != nil {
		t.Fatalf("DisconnectFromPeer: %v", err)
	}
}

func TestController_Controller_DisconnectFromPeer_Bad(t *testing.T) {
	tp := setupTestTransportPair(t)
	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)
	err := controller.DisconnectFromPeer("missing")
	if err == nil {
		t.Fatal("expected not connected error")
	}
}

func TestController_Controller_DisconnectFromPeer_Ugly(t *testing.T) {
	controller, _, tp := setupControllerPair(t)
	_ = controller.DisconnectFromPeer(tp.ServerNode.GetIdentity().ID)
	err := controller.DisconnectFromPeer(tp.ServerNode.GetIdentity().ID)
	if err == nil {
		t.Fatal("expected second disconnect error")
	}
}

func TestController_RequestTimeout(t *testing.T) {
	tp := setupTestTransportPair(t)

	// Register a handler on the server that deliberately ignores all messages,
	// so no reply will come back.
	tp.Server.OnMessage(func(_ *PeerConnection, _ *Message) {
		// Intentionally do nothing — simulate an unresponsive peer.
	})

	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)
	tp.connectClient(t)
	time.Sleep(50 * time.Millisecond)

	serverID := tp.ServerNode.GetIdentity().ID
	clientID := tp.ClientNode.GetIdentity().ID

	// Use sendRequest directly with a short deadline (PingPeer uses 5s internally).
	msg, err := NewMessage(MsgPing, clientID, serverID, PingPayload{
		SentAt: time.Now().UnixMilli(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	start := time.Now()
	_, err = controller.sendRequest(serverID, msg, 200*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !core.Contains(err.Error(), "timeout") {
		t.Fatalf("expected %q to contain %q", err.Error(), "timeout")
	}
	if !(elapsed < 1*time.Second) {
		t.Fatalf("expected %v to be less than %v", elapsed, 1*time.Second)
	}
}

func TestController_AutoConnect(t *testing.T) {
	tp := setupTestTransportPair(t)

	// Register worker on the server side.
	worker := NewWorker(tp.ServerNode, tp.Server)
	worker.RegisterWithTransport()

	// Create controller WITHOUT establishing a connection first.
	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)

	// Add the server peer to the client registry so auto-connect can resolve it.
	serverIdentity := tp.ServerNode.GetIdentity()
	peer := &Peer{
		ID:      serverIdentity.ID,
		Name:    "server",
		Address: tp.ServerAddr,
		Role:    RoleWorker,
	}
	tp.ClientReg.AddPeer(peer)

	// Confirm no connection exists yet.
	if !reflect.DeepEqual(0, tp.Client.ConnectedPeers()) {
		t.Fatalf("want %v, got %v", 0, tp.Client.ConnectedPeers())
	}

	// Send a request — controller should auto-connect via transport before sending.
	rtt, err := controller.PingPeer(serverIdentity.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !(rtt > 0.0) {
		t.Fatalf("expected %v to be greater than %v", rtt, 0.0)
	}

	// Verify connection was established.
	if !reflect.DeepEqual(1, tp.Client.ConnectedPeers()) {
		t.Fatalf("want %v, got %v", 1, tp.Client.ConnectedPeers())
	}
}

func TestController_GetAllStats(t *testing.T) {
	// Controller node with connections to two independent worker servers.
	controllerNM := testNode(t, "controller", RoleController)
	controllerReg := testRegistry(t)
	controllerTransport := NewTransport(controllerNM, controllerReg, DefaultTransportConfig())
	t.Cleanup(func() { controllerTransport.Stop() })

	const numWorkers = 2
	workerIDs := make([]string, numWorkers)

	for i := range numWorkers {
		nm, addr, _ := makeWorkerServer(t)
		wID := nm.GetIdentity().ID
		workerIDs[i] = wID

		peer := &Peer{
			ID:      wID,
			Name:    "worker",
			Address: addr,
			Role:    RoleWorker,
		}
		controllerReg.AddPeer(peer)

		_, err := controllerTransport.Connect(peer)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	time.Sleep(100 * time.Millisecond) // Allow connections to stabilise.

	controller := NewController(controllerNM, controllerReg, controllerTransport)

	// GetAllStats fetches stats from all connected peers in parallel.
	stats := controller.GetAllStats()
	if len(stats) != numWorkers {
		t.Fatalf("want len %v, got %v", numWorkers, len(stats))
	}

	for _, wID := range workerIDs {
		peerStats, exists := stats[wID]
		if !(exists) {
			t.Fatal("expected true")
		}
		if peerStats != nil {
			if len(peerStats.NodeID) == 0 {
				t.Fatal("expected non-empty")
			}
			if !(peerStats.Uptime >= int64(0)) {
				t.Fatalf("expected %v to be greater than or equal to %v", peerStats.Uptime, int64(0))
			}
		}
	}
}

func TestController_PingPeerRTT(t *testing.T) {
	controller, _, tp := setupControllerPair(t)
	serverID := tp.ServerNode.GetIdentity().ID

	// Record initial peer metrics.
	peerBefore := tp.ClientReg.GetPeer(serverID)
	if peerBefore == nil {
		t.Fatal("expected non-nil")
	}
	initialPingMS := peerBefore.PingMS

	// Send a ping.
	rtt, err := controller.PingPeer(serverID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !(rtt > 0.0) {
		t.Fatalf("expected %v to be greater than %v", rtt, 0.0)
	}
	if !(rtt < 1000.0) {
		t.Fatalf("expected %v to be less than %v", rtt, 1000.0)
	}

	// Verify the peer registry was updated with the measured latency.
	peerAfter := tp.ClientReg.GetPeer(serverID)
	if peerAfter == nil {
		t.Fatal("expected non-nil")
	}
	if reflect.DeepEqual(initialPingMS, peerAfter.PingMS) {
		t.Fatalf("did not want %v", peerAfter.PingMS)
	}
	if !(peerAfter.PingMS > 0.0) {
		t.Fatalf("expected %v to be greater than %v", peerAfter.PingMS, 0.0)
	}
}

func TestController_ConcurrentRequests(t *testing.T) {
	// Multiple goroutines send pings to different peers simultaneously.
	// Verify correct correlation — no cross-talk between responses.
	controllerNM := testNode(t, "controller", RoleController)
	controllerReg := testRegistry(t)
	controllerTransport := NewTransport(controllerNM, controllerReg, DefaultTransportConfig())
	t.Cleanup(func() { controllerTransport.Stop() })

	const numPeers = 3
	peerIDs := make([]string, numPeers)

	for i := range numPeers {
		nm, addr, _ := makeWorkerServer(t)
		pID := nm.GetIdentity().ID
		peerIDs[i] = pID

		peer := &Peer{
			ID:      pID,
			Name:    "worker",
			Address: addr,
			Role:    RoleWorker,
		}
		controllerReg.AddPeer(peer)

		_, err := controllerTransport.Connect(peer)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	time.Sleep(100 * time.Millisecond)

	controller := NewController(controllerNM, controllerReg, controllerTransport)

	var wg sync.WaitGroup
	results := make([]float64, numPeers)
	errors := make([]error, numPeers)

	for i, pID := range peerIDs {
		wg.Add(1)
		go func(idx int, peerID string) {
			defer wg.Done()
			rtt, err := controller.PingPeer(peerID)
			results[idx] = rtt
			errors[idx] = err
		}(i, pID)
	}

	wg.Wait()

	for i, err := range errors {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !(results[i] > 0.0) {
			t.Fatalf("expected %v to be greater than %v", results[i], 0.0)
		}
	}
}

func TestController_DeadPeerCleanup(t *testing.T) {
	tp := setupTestTransportPair(t)

	// Server deliberately ignores all messages.
	tp.Server.OnMessage(func(_ *PeerConnection, _ *Message) {})

	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)
	tp.connectClient(t)
	time.Sleep(50 * time.Millisecond)

	serverID := tp.ServerNode.GetIdentity().ID
	clientID := tp.ClientNode.GetIdentity().ID

	// Fire off a request that will time out.
	msg, err := NewMessage(MsgPing, clientID, serverID, PingPayload{
		SentAt: time.Now().UnixMilli(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = controller.sendRequest(serverID, msg, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !core.Contains(err.Error(), "timeout") {
		t.Fatalf("expected %q to contain %q", err.Error(), "timeout")
	}

	// The defer block inside sendRequest should have cleaned up the pending entry.
	time.Sleep(50 * time.Millisecond)

	controller.mu.RLock()
	pendingCount := len(controller.pending)
	controller.mu.RUnlock()

	if !reflect.DeepEqual(0, pendingCount) {
		t.Fatalf("want %v, got %v", 0, pendingCount)
	}
}

// --- Additional edge-case tests ---

func TestController_MultipleSequentialPings(t *testing.T) {
	// Ensures sequential requests to the same peer are correctly correlated.
	controller, _, tp := setupControllerPair(t)
	serverID := tp.ServerNode.GetIdentity().ID

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

func TestController_ConcurrentRequestsSamePeer(t *testing.T) {
	// Multiple goroutines sending requests to the SAME peer simultaneously.
	// Tests concurrent pending-map insertions/deletions under contention.
	controller, _, tp := setupControllerPair(t)
	serverID := tp.ServerNode.GetIdentity().ID

	const goroutines = 10
	var wg sync.WaitGroup
	var successCount atomic.Int32

	for range goroutines {
		wg.Go(func() {
			rtt, err := controller.PingPeer(serverID)
			if err == nil && rtt > 0 {
				successCount.Add(1)
			}
		})
	}

	wg.Wait()
	if !reflect.DeepEqual(int32(goroutines), successCount.Load()) {
		t.Fatalf("want %v, got %v", int32(goroutines), successCount.Load())
	}
}

func TestController_GetRemoteStats(t *testing.T) {
	controller, _, tp := setupControllerPair(t)
	serverID := tp.ServerNode.GetIdentity().ID

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
	if len(stats.NodeName) == 0 {
		t.Fatal("expected non-empty")
	}
	if stats.Miners == nil {
		t.Fatal("expected non-nil")
	}
	if !(stats.Uptime >= int64(0)) {
		t.Fatalf("expected %v to be greater than or equal to %v", stats.Uptime, int64(0))
	}
}

func TestController_ConnectToPeerUnknown(t *testing.T) {
	tp := setupTestTransportPair(t)
	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)

	err := controller.ConnectToPeer("non-existent-peer-id")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !core.Contains(err.Error(), "not found") {
		t.Fatalf("expected %q to contain %q", err.Error(), "not found")
	}
}

func TestController_DisconnectFromPeer(t *testing.T) {
	controller, _, tp := setupControllerPair(t)
	serverID := tp.ServerNode.GetIdentity().ID

	if !reflect.DeepEqual(1, tp.Client.ConnectedPeers()) {
		t.Fatalf("want %v, got %v", 1, tp.Client.ConnectedPeers())
	}

	err := controller.DisconnectFromPeer(serverID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestController_DisconnectFromPeerNotConnected(t *testing.T) {
	tp := setupTestTransportPair(t)
	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)

	err := controller.DisconnectFromPeer("non-existent-peer-id")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !core.Contains(err.Error(), "not connected") {
		t.Fatalf("expected %q to contain %q", err.Error(), "not connected")
	}
}

func TestController_SendRequestPeerNotFound(t *testing.T) {
	tp := setupTestTransportPair(t)
	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)

	clientID := tp.ClientNode.GetIdentity().ID
	msg, err := NewMessage(MsgPing, clientID, "ghost-peer", PingPayload{
		SentAt: time.Now().UnixMilli(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Peer is neither connected nor in the registry — sendRequest should fail.
	_, err = controller.sendRequest("ghost-peer", msg, 1*time.Second)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !core.Contains(err.Error(), "peer not found") {
		t.Fatalf("expected %q to contain %q", err.Error(), "peer not found")
	}
}

// --- Tests for StartRemoteMiner, StopRemoteMiner, GetRemoteLogs ---

// setupControllerPairWithMiner creates a controller/worker pair where the worker
// has a fully configured MinerManager so that start/stop/logs handlers work.
func setupControllerPairWithMiner(t *testing.T) (*Controller, *Worker, *testTransportPair) {
	t.Helper()

	tp := setupTestTransportPair(t)

	// Server side: register a Worker with a mock miner manager.
	worker := NewWorker(tp.ServerNode, tp.Server)
	mm := &mockMinerManagerFull{
		miners: map[string]*mockMinerFull{
			"running-miner": {
				name:      "running-miner",
				minerType: "xmrig",
				stats: map[string]any{
					"hashrate":  1234.5,
					"shares":    42,
					"rejected":  2,
					"uptime":    7200,
					"pool":      "pool.example.com:3333",
					"algorithm": "rx/0",
				},
				consoleHistory: []string{
					"[2026-02-20 10:00:00] started",
					"[2026-02-20 10:00:01] connected to pool",
					"[2026-02-20 10:00:05] new job received",
				},
			},
		},
	}
	worker.SetMinerManager(mm)
	worker.RegisterWithTransport()

	// Client side: create a Controller.
	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)

	// Establish the WebSocket connection.
	tp.connectClient(t)
	time.Sleep(50 * time.Millisecond)

	return controller, worker, tp
}

// mockMinerManagerFull implements MinerManager with functional start/stop/list/get.
type mockMinerManagerFull struct {
	mu     sync.Mutex
	miners map[string]*mockMinerFull
}

func (m *mockMinerManagerFull) StartMiner(minerType string, config any) (MinerInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := minerType + "-0"
	miner := &mockMinerFull{
		name:      name,
		minerType: minerType,
		stats: map[string]any{
			"hashrate": 0.0,
			"shares":   0,
		},
		consoleHistory: []string{"started " + minerType},
	}
	m.miners[name] = miner
	return miner, nil
}

func (m *mockMinerManagerFull) StopMiner(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.miners[name]; !exists {
		return core.Errorf("miner %s not found", name)
	}
	delete(m.miners, name)
	return nil
}

func (m *mockMinerManagerFull) ListMiners() []MinerInstance {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]MinerInstance, 0, len(m.miners))
	for _, miner := range m.miners {
		result = append(result, miner)
	}
	return result
}

func (m *mockMinerManagerFull) GetMiner(name string) (MinerInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	miner, exists := m.miners[name]
	if !exists {
		return nil, core.Errorf("miner %s not found", name)
	}
	return miner, nil
}

// mockMinerFull implements MinerInstance with real data.
type mockMinerFull struct {
	name           string
	minerType      string
	stats          any
	consoleHistory []string
}

func (m *mockMinerFull) GetName() string        { return m.name }
func (m *mockMinerFull) GetType() string        { return m.minerType }
func (m *mockMinerFull) GetStats() (any, error) { return m.stats, nil }
func (m *mockMinerFull) GetConsoleHistorySince(lines int, since time.Time) []string {
	if since.IsZero() {
		if lines >= len(m.consoleHistory) {
			return m.consoleHistory
		}
		return m.consoleHistory[:lines]
	}

	filtered := make([]string, 0, len(m.consoleHistory))
	for _, line := range m.consoleHistory {
		if lineAfter(line, since) {
			filtered = append(filtered, line)
		}
	}
	if lines >= len(filtered) {
		return filtered
	}
	return filtered[:lines]
}

func lineAfter(line string, since time.Time) bool {
	start := stringIndexByte(line, '[')
	end := stringIndexByte(line, ']')
	if start != 0 || end <= start+1 {
		return true
	}

	ts, err := time.Parse("2006-01-02 15:04:05", line[start+1:end])
	if err != nil {
		return true
	}
	return ts.After(since) || ts.Equal(since)
}

func stringIndexByte(s string, needle byte) int {
	for i := range len(s) {
		if s[i] == needle {
			return i
		}
	}
	return -1
}

func (m *mockMinerFull) GetConsoleHistory(lines int) []string {
	if lines >= len(m.consoleHistory) {
		return m.consoleHistory
	}
	return m.consoleHistory[:lines]
}

func TestController_StartRemoteMiner(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	serverID := tp.ServerNode.GetIdentity().ID
	configOverride := RawMessage(`{"pool":"pool.example.com:3333"}`)
	err := controller.StartRemoteMiner(serverID, "xmrig", "profile-1", configOverride)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestController_StartRemoteMiner_WithConfig(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	serverID := tp.ServerNode.GetIdentity().ID

	configOverride := RawMessage(`{"pool":"custom-pool:3333","threads":4}`)
	err := controller.StartRemoteMiner(serverID, "xmrig", "", configOverride)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestController_StartRemoteMiner_EmptyType(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	serverID := tp.ServerNode.GetIdentity().ID

	err := controller.StartRemoteMiner(serverID, "", "profile-1", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !core.Contains(err.Error(), "miner type is required") {
		t.Fatalf("expected %q to contain %q", err.Error(), "miner type is required")
	}
}

func TestController_StartRemoteMiner_NoIdentity(t *testing.T) {
	tp := setupTestTransportPair(t)

	// Create a node without identity
	nmNoID, err := NewNodeManagerWithPaths(
		core.PathJoin(t.TempDir(), "priv.key"),
		core.PathJoin(t.TempDir(), "node.json"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	controller := NewController(nmNoID, tp.ClientReg, tp.Client)

	err = controller.StartRemoteMiner("some-peer", "xmrig", "profile-1", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !core.Contains(err.Error(), "identity not initialized") {
		t.Fatalf("expected %q to contain %q", err.Error(), "identity not initialized")
	}
}

func TestController_StopRemoteMiner(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	serverID := tp.ServerNode.GetIdentity().ID

	err := controller.StopRemoteMiner(serverID, "running-miner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestController_StopRemoteMiner_NotFound(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	serverID := tp.ServerNode.GetIdentity().ID

	err := controller.StopRemoteMiner(serverID, "non-existent-miner")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestController_StopRemoteMiner_NoIdentity(t *testing.T) {
	tp := setupTestTransportPair(t)
	nmNoID, err := NewNodeManagerWithPaths(
		core.PathJoin(t.TempDir(), "priv.key"),
		core.PathJoin(t.TempDir(), "node.json"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	controller := NewController(nmNoID, tp.ClientReg, tp.Client)

	err = controller.StopRemoteMiner("some-peer", "any-miner")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !core.Contains(err.Error(), "identity not initialized") {
		t.Fatalf("expected %q to contain %q", err.Error(), "identity not initialized")
	}
}

func TestController_GetRemoteLogs(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	serverID := tp.ServerNode.GetIdentity().ID

	lines, err := controller.GetRemoteLogs(serverID, "running-miner", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lines == nil {
		t.Fatal("expected non-nil")
	}
	if len(lines) != 3 {
		t.Fatalf("want len %v, got %v", 3, len(lines))
	}
	if !core.Contains(lines[0], "started") {
		t.Fatalf("expected %q to contain %q", lines[0], "started")
	}
}

func TestController_GetRemoteLogs_LimitedLines(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	serverID := tp.ServerNode.GetIdentity().ID

	lines, err := controller.GetRemoteLogs(serverID, "running-miner", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("want len %v, got %v", 1, len(lines))
	}
}

func TestController_GetRemoteLogsSince(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	serverID := tp.ServerNode.GetIdentity().ID

	since, err := time.Parse("2006-01-02 15:04:05", "2026-02-20 10:00:01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines, err := controller.GetRemoteLogsSince(serverID, "running-miner", 10, since)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("want len %v, got %v", 2, len(lines))
	}
	if !core.Contains(lines[0], "connected to pool") {
		t.Fatalf("expected %q to contain %q", lines[0], "connected to pool")
	}
	if !core.Contains(lines[1], "new job received") {
		t.Fatalf("expected %q to contain %q", lines[1], "new job received")
	}
}

func TestController_GetRemoteLogs_NoIdentity(t *testing.T) {
	tp := setupTestTransportPair(t)
	nmNoID, err := NewNodeManagerWithPaths(
		core.PathJoin(t.TempDir(), "priv.key"),
		core.PathJoin(t.TempDir(), "node.json"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	controller := NewController(nmNoID, tp.ClientReg, tp.Client)

	_, err = controller.GetRemoteLogs("some-peer", "any-miner", 10)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !core.Contains(err.Error(), "identity not initialized") {
		t.Fatalf("expected %q to contain %q", err.Error(), "identity not initialized")
	}
}

func TestController_GetRemoteStats_WithMiners(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	serverID := tp.ServerNode.GetIdentity().ID

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
	// The worker has a miner manager with 1 running miner
	if len(stats.Miners) != 1 {
		t.Fatalf("want len %v, got %v", 1, len(stats.Miners))
	}
	if !reflect.DeepEqual("running-miner", stats.Miners[0].Name) {
		t.Fatalf("want %v, got %v", "running-miner", stats.Miners[0].Name)
	}
	if !reflect.DeepEqual(1234.5, stats.Miners[0].Hashrate) {
		t.Fatalf("want %v, got %v", 1234.5, stats.Miners[0].Hashrate)
	}
}

func TestController_GetRemoteStats_NoIdentity(t *testing.T) {
	tp := setupTestTransportPair(t)
	nmNoID, err := NewNodeManagerWithPaths(
		core.PathJoin(t.TempDir(), "priv.key"),
		core.PathJoin(t.TempDir(), "node.json"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	controller := NewController(nmNoID, tp.ClientReg, tp.Client)

	_, err = controller.GetRemoteStats("some-peer")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !core.Contains(err.Error(), "identity not initialized") {
		t.Fatalf("expected %q to contain %q", err.Error(), "identity not initialized")
	}
}

func TestController_ConnectToPeer_Success(t *testing.T) {
	tp := setupTestTransportPair(t)

	worker := NewWorker(tp.ServerNode, tp.Server)
	worker.RegisterWithTransport()

	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)

	// Add the server peer to the client registry.
	serverIdentity := tp.ServerNode.GetIdentity()
	peer := &Peer{
		ID:      serverIdentity.ID,
		Name:    "server",
		Address: tp.ServerAddr,
		Role:    RoleWorker,
	}
	tp.ClientReg.AddPeer(peer)

	err := controller.ConnectToPeer(serverIdentity.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(1, tp.Client.ConnectedPeers()) {
		t.Fatalf("want %v, got %v", 1, tp.Client.ConnectedPeers())
	}
}

func TestController_HandleResponse_NonReply(t *testing.T) {
	tp := setupTestTransportPair(t)
	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)

	// handleResponse should ignore messages without ReplyTo
	msg, _ := NewMessage(MsgPing, "sender", "target", PingPayload{SentAt: 123})
	controller.handleResponse(nil, msg)

	// No pending entries should be affected
	controller.mu.RLock()
	count := len(controller.pending)
	controller.mu.RUnlock()
	if !reflect.DeepEqual(0, count) {
		t.Fatalf("want %v, got %v", 0, count)
	}
}

func TestController_HandleResponse_FullChannel(t *testing.T) {
	tp := setupTestTransportPair(t)
	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)

	// Create a pending channel that's already full
	ch := make(chan *Message, 1)
	ch <- &Message{} // Fill the channel

	controller.mu.Lock()
	controller.pending["test-id"] = ch
	controller.mu.Unlock()

	// handleResponse with matching reply should not panic on full channel
	msg, _ := NewMessage(MsgPong, "sender", "target", PongPayload{SentAt: 123})
	msg.ReplyTo = "test-id"
	controller.handleResponse(nil, msg)

	// The pending entry should be removed despite channel being full
	controller.mu.RLock()
	_, exists := controller.pending["test-id"]
	controller.mu.RUnlock()
	if exists {
		t.Fatal("expected false")
	}
}

func TestController_PingPeer_NoIdentity(t *testing.T) {
	tp := setupTestTransportPair(t)
	nmNoID, _ := NewNodeManagerWithPaths(
		core.PathJoin(t.TempDir(), "priv.key"),
		core.PathJoin(t.TempDir(), "node.json"),
	)
	controller := NewController(nmNoID, tp.ClientReg, tp.Client)

	_, err := controller.PingPeer("some-peer")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !core.Contains(err.Error(), "identity not initialized") {
		t.Fatalf("expected %q to contain %q", err.Error(), "identity not initialized")
	}
}
