package node

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err, "PingPeer should succeed")
	assert.Greater(t, rtt, 0.0, "RTT should be positive")
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
	require.NoError(t, err)

	start := time.Now()
	_, err = controller.sendRequest(serverID, msg, 200*time.Millisecond)
	elapsed := time.Since(start)

	require.Error(t, err, "request should time out when peer does not reply")
	assert.Contains(t, err.Error(), "timeout", "error message should mention timeout")
	assert.Less(t, elapsed, 1*time.Second, "should return quickly after the deadline")
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
	assert.Equal(t, 0, tp.Client.ConnectedPeers(), "should have no connections initially")

	// Send a request — controller should auto-connect via transport before sending.
	rtt, err := controller.PingPeer(serverIdentity.ID)
	require.NoError(t, err, "PingPeer with auto-connect should succeed")
	assert.Greater(t, rtt, 0.0, "RTT should be positive after auto-connect")

	// Verify connection was established.
	assert.Equal(t, 1, tp.Client.ConnectedPeers(), "should have 1 connection after auto-connect")
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
		require.NoError(t, err, "connecting to worker %d should succeed", i)
	}

	time.Sleep(100 * time.Millisecond) // Allow connections to stabilise.

	controller := NewController(controllerNM, controllerReg, controllerTransport)

	// GetAllStats fetches stats from all connected peers in parallel.
	stats := controller.GetAllStats()
	assert.Len(t, stats, numWorkers, "should get stats from all connected workers")

	for _, wID := range workerIDs {
		peerStats, exists := stats[wID]
		assert.True(t, exists, "stats should contain worker %s", wID)
		if peerStats != nil {
			assert.NotEmpty(t, peerStats.NodeID, "stats should include the node ID")
			assert.GreaterOrEqual(t, peerStats.Uptime, int64(0), "uptime should be non-negative")
		}
	}
}

func TestController_PingPeerRTT(t *testing.T) {
	controller, _, tp := setupControllerPair(t)
	serverID := tp.ServerNode.GetIdentity().ID

	// Record initial peer metrics.
	peerBefore := tp.ClientReg.GetPeer(serverID)
	require.NotNil(t, peerBefore, "server peer should exist in the client registry")
	initialPingMS := peerBefore.PingMS

	// Send a ping.
	rtt, err := controller.PingPeer(serverID)
	require.NoError(t, err, "PingPeer should succeed")
	assert.Greater(t, rtt, 0.0, "RTT should be positive")
	assert.Less(t, rtt, 1000.0, "RTT on loopback should be well under 1000ms")

	// Verify the peer registry was updated with the measured latency.
	peerAfter := tp.ClientReg.GetPeer(serverID)
	require.NotNil(t, peerAfter, "server peer should still exist after ping")
	assert.NotEqual(t, initialPingMS, peerAfter.PingMS,
		"PingMS should be updated after a successful ping")
	assert.Greater(t, peerAfter.PingMS, 0.0, "PingMS should be positive")
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
		require.NoError(t, err, "connecting to worker %d should succeed", i)
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
		assert.NoError(t, err, "PingPeer to peer %d should succeed", i)
		assert.Greater(t, results[i], 0.0, "RTT for peer %d should be positive", i)
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
	require.NoError(t, err)

	_, err = controller.sendRequest(serverID, msg, 100*time.Millisecond)
	require.Error(t, err, "request should time out")
	assert.Contains(t, err.Error(), "timeout")

	// The defer block inside sendRequest should have cleaned up the pending entry.
	time.Sleep(50 * time.Millisecond)

	controller.mu.RLock()
	pendingCount := len(controller.pending)
	controller.mu.RUnlock()

	assert.Equal(t, 0, pendingCount,
		"pending map should be empty after timeout — no goroutine/memory leak")
}

// --- Additional edge-case tests ---

func TestController_MultipleSequentialPings(t *testing.T) {
	// Ensures sequential requests to the same peer are correctly correlated.
	controller, _, tp := setupControllerPair(t)
	serverID := tp.ServerNode.GetIdentity().ID

	for i := range 5 {
		rtt, err := controller.PingPeer(serverID)
		require.NoError(t, err, "iteration %d should succeed", i)
		assert.Greater(t, rtt, 0.0, "iteration %d RTT should be positive", i)
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
	assert.Equal(t, int32(goroutines), successCount.Load(),
		"all concurrent requests to the same peer should succeed")
}

func TestController_GetRemoteStats(t *testing.T) {
	controller, _, tp := setupControllerPair(t)
	serverID := tp.ServerNode.GetIdentity().ID

	stats, err := controller.GetRemoteStats(serverID)
	require.NoError(t, err, "GetRemoteStats should succeed")
	require.NotNil(t, stats)

	assert.NotEmpty(t, stats.NodeID, "stats should contain the node ID")
	assert.NotEmpty(t, stats.NodeName, "stats should contain the node name")
	assert.NotNil(t, stats.Miners, "miners list should not be nil")
	assert.GreaterOrEqual(t, stats.Uptime, int64(0), "uptime should be non-negative")
}

func TestController_ConnectToPeerUnknown(t *testing.T) {
	tp := setupTestTransportPair(t)
	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)

	err := controller.ConnectToPeer("non-existent-peer-id")
	require.Error(t, err, "connecting to an unknown peer should fail")
	assert.Contains(t, err.Error(), "not found")
}

func TestController_DisconnectFromPeer(t *testing.T) {
	controller, _, tp := setupControllerPair(t)
	serverID := tp.ServerNode.GetIdentity().ID

	assert.Equal(t, 1, tp.Client.ConnectedPeers(), "should have 1 connection")

	err := controller.DisconnectFromPeer(serverID)
	require.NoError(t, err, "DisconnectFromPeer should succeed")
}

func TestController_DisconnectFromPeerNotConnected(t *testing.T) {
	tp := setupTestTransportPair(t)
	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)

	err := controller.DisconnectFromPeer("non-existent-peer-id")
	require.Error(t, err, "disconnecting from a non-connected peer should fail")
	assert.Contains(t, err.Error(), "not connected")
}

func TestController_SendRequestPeerNotFound(t *testing.T) {
	tp := setupTestTransportPair(t)
	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)

	clientID := tp.ClientNode.GetIdentity().ID
	msg, err := NewMessage(MsgPing, clientID, "ghost-peer", PingPayload{
		SentAt: time.Now().UnixMilli(),
	})
	require.NoError(t, err)

	// Peer is neither connected nor in the registry — sendRequest should fail.
	_, err = controller.sendRequest("ghost-peer", msg, 1*time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "peer not found")
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
		return fmt.Errorf("miner %s not found", name)
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
		return nil, fmt.Errorf("miner %s not found", name)
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
	start := strings.IndexByte(line, '[')
	end := strings.IndexByte(line, ']')
	if start != 0 || end <= start+1 {
		return true
	}

	ts, err := time.Parse("2006-01-02 15:04:05", line[start+1:end])
	if err != nil {
		return true
	}
	return ts.After(since) || ts.Equal(since)
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
	configOverride := json.RawMessage(`{"pool":"pool.example.com:3333"}`)
	err := controller.StartRemoteMiner(serverID, "xmrig", "profile-1", configOverride)

	require.NoError(t, err, "StartRemoteMiner should succeed")
}

func TestController_StartRemoteMiner_WithConfig(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	serverID := tp.ServerNode.GetIdentity().ID

	configOverride := json.RawMessage(`{"pool":"custom-pool:3333","threads":4}`)
	err := controller.StartRemoteMiner(serverID, "xmrig", "", configOverride)
	require.NoError(t, err, "StartRemoteMiner with config override should succeed")
}

func TestController_StartRemoteMiner_EmptyType(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	serverID := tp.ServerNode.GetIdentity().ID

	err := controller.StartRemoteMiner(serverID, "", "profile-1", nil)
	require.Error(t, err, "StartRemoteMiner with empty miner type should fail")
	assert.Contains(t, err.Error(), "miner type is required")
}

func TestController_StartRemoteMiner_NoIdentity(t *testing.T) {
	tp := setupTestTransportPair(t)

	// Create a node without identity
	nmNoID, err := NewNodeManagerWithPaths(
		filepath.Join(t.TempDir(), "priv.key"),
		filepath.Join(t.TempDir(), "node.json"),
	)
	require.NoError(t, err)

	controller := NewController(nmNoID, tp.ClientReg, tp.Client)

	err = controller.StartRemoteMiner("some-peer", "xmrig", "profile-1", nil)
	require.Error(t, err, "should fail without identity")
	assert.Contains(t, err.Error(), "identity not initialized")
}

func TestController_StopRemoteMiner(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	serverID := tp.ServerNode.GetIdentity().ID

	err := controller.StopRemoteMiner(serverID, "running-miner")
	require.NoError(t, err, "StopRemoteMiner should succeed for existing miner")
}

func TestController_StopRemoteMiner_NotFound(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	serverID := tp.ServerNode.GetIdentity().ID

	err := controller.StopRemoteMiner(serverID, "non-existent-miner")
	require.Error(t, err, "StopRemoteMiner should fail for non-existent miner")
}

func TestController_StopRemoteMiner_NoIdentity(t *testing.T) {
	tp := setupTestTransportPair(t)
	nmNoID, err := NewNodeManagerWithPaths(
		filepath.Join(t.TempDir(), "priv.key"),
		filepath.Join(t.TempDir(), "node.json"),
	)
	require.NoError(t, err)

	controller := NewController(nmNoID, tp.ClientReg, tp.Client)

	err = controller.StopRemoteMiner("some-peer", "any-miner")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "identity not initialized")
}

func TestController_GetRemoteLogs(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	serverID := tp.ServerNode.GetIdentity().ID

	lines, err := controller.GetRemoteLogs(serverID, "running-miner", 10)
	require.NoError(t, err, "GetRemoteLogs should succeed")
	require.NotNil(t, lines)
	assert.Len(t, lines, 3, "should return all 3 console history lines")
	assert.Contains(t, lines[0], "started")
}

func TestController_GetRemoteLogs_LimitedLines(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	serverID := tp.ServerNode.GetIdentity().ID

	lines, err := controller.GetRemoteLogs(serverID, "running-miner", 1)
	require.NoError(t, err, "GetRemoteLogs with limited lines should succeed")
	assert.Len(t, lines, 1, "should return only 1 line")
}

func TestController_GetRemoteLogsSince(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	serverID := tp.ServerNode.GetIdentity().ID

	since, err := time.Parse("2006-01-02 15:04:05", "2026-02-20 10:00:01")
	require.NoError(t, err)

	lines, err := controller.GetRemoteLogsSince(serverID, "running-miner", 10, since)
	require.NoError(t, err, "GetRemoteLogsSince should succeed")
	require.Len(t, lines, 2, "should return only log lines on or after the requested timestamp")
	assert.Contains(t, lines[0], "connected to pool")
	assert.Contains(t, lines[1], "new job received")
}

func TestController_GetRemoteLogs_NoIdentity(t *testing.T) {
	tp := setupTestTransportPair(t)
	nmNoID, err := NewNodeManagerWithPaths(
		filepath.Join(t.TempDir(), "priv.key"),
		filepath.Join(t.TempDir(), "node.json"),
	)
	require.NoError(t, err)

	controller := NewController(nmNoID, tp.ClientReg, tp.Client)

	_, err = controller.GetRemoteLogs("some-peer", "any-miner", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "identity not initialized")
}

func TestController_GetRemoteStats_WithMiners(t *testing.T) {
	controller, _, tp := setupControllerPairWithMiner(t)
	serverID := tp.ServerNode.GetIdentity().ID

	stats, err := controller.GetRemoteStats(serverID)
	require.NoError(t, err, "GetRemoteStats should succeed")
	require.NotNil(t, stats)
	assert.NotEmpty(t, stats.NodeID)
	// The worker has a miner manager with 1 running miner
	assert.Len(t, stats.Miners, 1, "should list the running miner")
	assert.Equal(t, "running-miner", stats.Miners[0].Name)
	assert.Equal(t, 1234.5, stats.Miners[0].Hashrate)
}

func TestController_GetRemoteStats_NoIdentity(t *testing.T) {
	tp := setupTestTransportPair(t)
	nmNoID, err := NewNodeManagerWithPaths(
		filepath.Join(t.TempDir(), "priv.key"),
		filepath.Join(t.TempDir(), "node.json"),
	)
	require.NoError(t, err)

	controller := NewController(nmNoID, tp.ClientReg, tp.Client)

	_, err = controller.GetRemoteStats("some-peer")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "identity not initialized")
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
	require.NoError(t, err, "ConnectToPeer should succeed")

	assert.Equal(t, 1, tp.Client.ConnectedPeers(), "should have 1 connection after ConnectToPeer")
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
	assert.Equal(t, 0, count)
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
	assert.False(t, exists, "pending entry should be removed after handling")
}

func TestController_PingPeer_NoIdentity(t *testing.T) {
	tp := setupTestTransportPair(t)
	nmNoID, _ := NewNodeManagerWithPaths(
		filepath.Join(t.TempDir(), "priv.key"),
		filepath.Join(t.TempDir(), "node.json"),
	)
	controller := NewController(nmNoID, tp.ClientReg, tp.Client)

	_, err := controller.PingPeer("some-peer")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "identity not initialized")
}
