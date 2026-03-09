package node

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupTestEnv sets up a temporary environment for testing and returns cleanup function
func setupTestEnv(t *testing.T) func() {
	tmpDir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "config"))
	os.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "data"))
	return func() {
		os.Unsetenv("XDG_CONFIG_HOME")
		os.Unsetenv("XDG_DATA_HOME")
	}
}

func TestNewWorker(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("failed to create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("test-worker", RoleWorker); err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}

	pr, err := NewPeerRegistryWithPath(t.TempDir() + "/peers.json")
	if err != nil {
		t.Fatalf("failed to create peer registry: %v", err)
	}

	transport := NewTransport(nm, pr, DefaultTransportConfig())
	worker := NewWorker(nm, transport)
	worker.DataDir = t.TempDir()

	if worker == nil {
		t.Fatal("NewWorker returned nil")
	}
	if worker.node != nm {
		t.Error("worker.node not set correctly")
	}
	if worker.transport != transport {
		t.Error("worker.transport not set correctly")
	}
}

func TestWorker_SetMinerManager(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("failed to create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("test-worker", RoleWorker); err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}

	pr, err := NewPeerRegistryWithPath(t.TempDir() + "/peers.json")
	if err != nil {
		t.Fatalf("failed to create peer registry: %v", err)
	}

	transport := NewTransport(nm, pr, DefaultTransportConfig())
	worker := NewWorker(nm, transport)
	worker.DataDir = t.TempDir()

	mockManager := &mockMinerManager{}
	worker.SetMinerManager(mockManager)

	if worker.minerManager != mockManager {
		t.Error("minerManager not set correctly")
	}
}

func TestWorker_SetProfileManager(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("failed to create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("test-worker", RoleWorker); err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}

	pr, err := NewPeerRegistryWithPath(t.TempDir() + "/peers.json")
	if err != nil {
		t.Fatalf("failed to create peer registry: %v", err)
	}

	transport := NewTransport(nm, pr, DefaultTransportConfig())
	worker := NewWorker(nm, transport)
	worker.DataDir = t.TempDir()

	mockProfile := &mockProfileManager{}
	worker.SetProfileManager(mockProfile)

	if worker.profileManager != mockProfile {
		t.Error("profileManager not set correctly")
	}
}

func TestWorker_HandlePing(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("failed to create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("test-worker", RoleWorker); err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}

	pr, err := NewPeerRegistryWithPath(t.TempDir() + "/peers.json")
	if err != nil {
		t.Fatalf("failed to create peer registry: %v", err)
	}

	transport := NewTransport(nm, pr, DefaultTransportConfig())
	worker := NewWorker(nm, transport)
	worker.DataDir = t.TempDir()

	// Create a ping message
	identity := nm.GetIdentity()
	if identity == nil {
		t.Fatal("expected identity to be generated")
	}
	pingPayload := PingPayload{SentAt: time.Now().UnixMilli()}
	pingMsg, err := NewMessage(MsgPing, "sender-id", identity.ID, pingPayload)
	if err != nil {
		t.Fatalf("failed to create ping message: %v", err)
	}

	// Call handlePing directly
	response, err := worker.handlePing(pingMsg)
	if err != nil {
		t.Fatalf("handlePing returned error: %v", err)
	}

	if response == nil {
		t.Fatal("handlePing returned nil response")
	}

	if response.Type != MsgPong {
		t.Errorf("expected response type %s, got %s", MsgPong, response.Type)
	}

	var pong PongPayload
	if err := response.ParsePayload(&pong); err != nil {
		t.Fatalf("failed to parse pong payload: %v", err)
	}

	if pong.SentAt != pingPayload.SentAt {
		t.Errorf("pong SentAt mismatch: expected %d, got %d", pingPayload.SentAt, pong.SentAt)
	}

	if pong.ReceivedAt == 0 {
		t.Error("pong ReceivedAt not set")
	}
}

func TestWorker_HandleGetStats(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("failed to create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("test-worker", RoleWorker); err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}

	pr, err := NewPeerRegistryWithPath(t.TempDir() + "/peers.json")
	if err != nil {
		t.Fatalf("failed to create peer registry: %v", err)
	}

	transport := NewTransport(nm, pr, DefaultTransportConfig())
	worker := NewWorker(nm, transport)
	worker.DataDir = t.TempDir()

	// Create a get_stats message
	identity := nm.GetIdentity()
	if identity == nil {
		t.Fatal("expected identity to be generated")
	}
	msg, err := NewMessage(MsgGetStats, "sender-id", identity.ID, nil)
	if err != nil {
		t.Fatalf("failed to create get_stats message: %v", err)
	}

	// Call handleGetStats directly (without miner manager)
	response, err := worker.handleGetStats(msg)
	if err != nil {
		t.Fatalf("handleGetStats returned error: %v", err)
	}

	if response == nil {
		t.Fatal("handleGetStats returned nil response")
	}

	if response.Type != MsgStats {
		t.Errorf("expected response type %s, got %s", MsgStats, response.Type)
	}

	var stats StatsPayload
	if err := response.ParsePayload(&stats); err != nil {
		t.Fatalf("failed to parse stats payload: %v", err)
	}

	if stats.NodeID != identity.ID {
		t.Errorf("stats NodeID mismatch: expected %s, got %s", identity.ID, stats.NodeID)
	}

	if stats.NodeName != identity.Name {
		t.Errorf("stats NodeName mismatch: expected %s, got %s", identity.Name, stats.NodeName)
	}
}

func TestWorker_HandleStartMiner_NoManager(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("failed to create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("test-worker", RoleWorker); err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}

	pr, err := NewPeerRegistryWithPath(t.TempDir() + "/peers.json")
	if err != nil {
		t.Fatalf("failed to create peer registry: %v", err)
	}

	transport := NewTransport(nm, pr, DefaultTransportConfig())
	worker := NewWorker(nm, transport)
	worker.DataDir = t.TempDir()

	// Create a start_miner message
	identity := nm.GetIdentity()
	if identity == nil {
		t.Fatal("expected identity to be generated")
	}
	payload := StartMinerPayload{MinerType: "xmrig", ProfileID: "test-profile"}
	msg, err := NewMessage(MsgStartMiner, "sender-id", identity.ID, payload)
	if err != nil {
		t.Fatalf("failed to create start_miner message: %v", err)
	}

	// Without miner manager, should return error
	_, err = worker.handleStartMiner(msg)
	if err == nil {
		t.Error("expected error when miner manager is nil")
	}
}

func TestWorker_HandleStopMiner_NoManager(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("failed to create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("test-worker", RoleWorker); err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}

	pr, err := NewPeerRegistryWithPath(t.TempDir() + "/peers.json")
	if err != nil {
		t.Fatalf("failed to create peer registry: %v", err)
	}

	transport := NewTransport(nm, pr, DefaultTransportConfig())
	worker := NewWorker(nm, transport)
	worker.DataDir = t.TempDir()

	// Create a stop_miner message
	identity := nm.GetIdentity()
	if identity == nil {
		t.Fatal("expected identity to be generated")
	}
	payload := StopMinerPayload{MinerName: "test-miner"}
	msg, err := NewMessage(MsgStopMiner, "sender-id", identity.ID, payload)
	if err != nil {
		t.Fatalf("failed to create stop_miner message: %v", err)
	}

	// Without miner manager, should return error
	_, err = worker.handleStopMiner(msg)
	if err == nil {
		t.Error("expected error when miner manager is nil")
	}
}

func TestWorker_HandleGetLogs_NoManager(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("failed to create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("test-worker", RoleWorker); err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}

	pr, err := NewPeerRegistryWithPath(t.TempDir() + "/peers.json")
	if err != nil {
		t.Fatalf("failed to create peer registry: %v", err)
	}

	transport := NewTransport(nm, pr, DefaultTransportConfig())
	worker := NewWorker(nm, transport)
	worker.DataDir = t.TempDir()

	// Create a get_logs message
	identity := nm.GetIdentity()
	if identity == nil {
		t.Fatal("expected identity to be generated")
	}
	payload := GetLogsPayload{MinerName: "test-miner", Lines: 100}
	msg, err := NewMessage(MsgGetLogs, "sender-id", identity.ID, payload)
	if err != nil {
		t.Fatalf("failed to create get_logs message: %v", err)
	}

	// Without miner manager, should return error
	_, err = worker.handleGetLogs(msg)
	if err == nil {
		t.Error("expected error when miner manager is nil")
	}
}

func TestWorker_HandleDeploy_Profile(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("failed to create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("test-worker", RoleWorker); err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}

	pr, err := NewPeerRegistryWithPath(t.TempDir() + "/peers.json")
	if err != nil {
		t.Fatalf("failed to create peer registry: %v", err)
	}

	transport := NewTransport(nm, pr, DefaultTransportConfig())
	worker := NewWorker(nm, transport)
	worker.DataDir = t.TempDir()

	// Create a deploy message for profile
	identity := nm.GetIdentity()
	if identity == nil {
		t.Fatal("expected identity to be generated")
	}
	payload := DeployPayload{
		BundleType: "profile",
		Data:       []byte(`{"id": "test", "name": "Test Profile"}`),
		Name:       "test-profile",
	}
	msg, err := NewMessage(MsgDeploy, "sender-id", identity.ID, payload)
	if err != nil {
		t.Fatalf("failed to create deploy message: %v", err)
	}

	// Without profile manager, should return error
	_, err = worker.handleDeploy(nil, msg)
	if err == nil {
		t.Error("expected error when profile manager is nil")
	}
}

func TestWorker_HandleDeploy_UnknownType(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("failed to create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("test-worker", RoleWorker); err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}

	pr, err := NewPeerRegistryWithPath(t.TempDir() + "/peers.json")
	if err != nil {
		t.Fatalf("failed to create peer registry: %v", err)
	}

	transport := NewTransport(nm, pr, DefaultTransportConfig())
	worker := NewWorker(nm, transport)
	worker.DataDir = t.TempDir()

	// Create a deploy message with unknown type
	identity := nm.GetIdentity()
	if identity == nil {
		t.Fatal("expected identity to be generated")
	}
	payload := DeployPayload{
		BundleType: "unknown",
		Data:       []byte(`{}`),
		Name:       "test",
	}
	msg, err := NewMessage(MsgDeploy, "sender-id", identity.ID, payload)
	if err != nil {
		t.Fatalf("failed to create deploy message: %v", err)
	}

	_, err = worker.handleDeploy(nil, msg)
	if err == nil {
		t.Error("expected error for unknown bundle type")
	}
}

func TestConvertMinerStats(t *testing.T) {
	tests := []struct {
		name     string
		rawStats any
		wantHash float64
	}{
		{
			name: "MapWithHashrate",
			rawStats: map[string]any{
				"hashrate":  100.5,
				"shares":    10,
				"rejected":  2,
				"uptime":    3600,
				"pool":      "test-pool",
				"algorithm": "rx/0",
			},
			wantHash: 100.5,
		},
		{
			name:     "EmptyMap",
			rawStats: map[string]any{},
			wantHash: 0,
		},
		{
			name:     "NonMap",
			rawStats: "not a map",
			wantHash: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMinerInstance{name: "test", minerType: "xmrig"}
			result := convertMinerStats(mock, tt.rawStats)

			if result.Name != "test" {
				t.Errorf("expected name 'test', got '%s'", result.Name)
			}
			if result.Hashrate != tt.wantHash {
				t.Errorf("expected hashrate %f, got %f", tt.wantHash, result.Hashrate)
			}
		})
	}
}

// Mock implementations for testing

type mockMinerManager struct {
	miners []MinerInstance
}

func (m *mockMinerManager) StartMiner(minerType string, config any) (MinerInstance, error) {
	return nil, nil
}

func (m *mockMinerManager) StopMiner(name string) error {
	return nil
}

func (m *mockMinerManager) ListMiners() []MinerInstance {
	return m.miners
}

func (m *mockMinerManager) GetMiner(name string) (MinerInstance, error) {
	for _, miner := range m.miners {
		if miner.GetName() == name {
			return miner, nil
		}
	}
	return nil, nil
}

type mockMinerInstance struct {
	name      string
	minerType string
	stats     any
}

func (m *mockMinerInstance) GetName() string                      { return m.name }
func (m *mockMinerInstance) GetType() string                      { return m.minerType }
func (m *mockMinerInstance) GetStats() (any, error)               { return m.stats, nil }
func (m *mockMinerInstance) GetConsoleHistory(lines int) []string { return []string{} }

type mockProfileManager struct{}

func (m *mockProfileManager) GetProfile(id string) (any, error) {
	return nil, nil
}

func (m *mockProfileManager) SaveProfile(profile any) error {
	return nil
}

// --- Enhanced worker handler tests for full code path coverage ---

// mockMinerManagerFailing always returns errors from StartMiner.
type mockMinerManagerFailing struct {
	mockMinerManager
}

func (m *mockMinerManagerFailing) StartMiner(minerType string, config any) (MinerInstance, error) {
	return nil, fmt.Errorf("mining hardware not available")
}

func (m *mockMinerManagerFailing) StopMiner(name string) error {
	return fmt.Errorf("miner %s not found", name)
}

func (m *mockMinerManagerFailing) GetMiner(name string) (MinerInstance, error) {
	return nil, fmt.Errorf("miner %s not found", name)
}

// mockProfileManagerFull implements ProfileManager that returns real data.
type mockProfileManagerFull struct {
	profiles map[string]any
}

func (m *mockProfileManagerFull) GetProfile(id string) (any, error) {
	p, ok := m.profiles[id]
	if !ok {
		return nil, fmt.Errorf("profile %s not found", id)
	}
	return p, nil
}

func (m *mockProfileManagerFull) SaveProfile(profile any) error {
	return nil
}

// mockProfileManagerFailing always returns errors.
type mockProfileManagerFailing struct{}

func (m *mockProfileManagerFailing) GetProfile(id string) (any, error) {
	return nil, fmt.Errorf("profile %s not found", id)
}

func (m *mockProfileManagerFailing) SaveProfile(profile any) error {
	return fmt.Errorf("save failed")
}

func TestWorker_HandleStartMiner_WithManager(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("failed to create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("test-worker", RoleWorker); err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}

	pr, err := NewPeerRegistryWithPath(t.TempDir() + "/peers.json")
	if err != nil {
		t.Fatalf("failed to create peer registry: %v", err)
	}

	transport := NewTransport(nm, pr, DefaultTransportConfig())
	worker := NewWorker(nm, transport)
	worker.DataDir = t.TempDir()

	mm := &mockMinerManager{
		miners: []MinerInstance{},
	}
	// Override StartMiner to return a real instance
	mmFull := &mockMinerManagerWithStart{}
	worker.SetMinerManager(mmFull)

	identity := nm.GetIdentity()

	t.Run("WithConfigOverride", func(t *testing.T) {
		payload := StartMinerPayload{
			MinerType: "xmrig",
			Config:    json.RawMessage(`{"pool":"test:3333"}`),
		}
		msg, err := NewMessage(MsgStartMiner, "sender-id", identity.ID, payload)
		if err != nil {
			t.Fatalf("failed to create message: %v", err)
		}

		response, err := worker.handleStartMiner(msg)
		if err != nil {
			t.Fatalf("handleStartMiner returned error: %v", err)
		}

		if response.Type != MsgMinerAck {
			t.Errorf("expected type %s, got %s", MsgMinerAck, response.Type)
		}

		var ack MinerAckPayload
		if err := response.ParsePayload(&ack); err != nil {
			t.Fatalf("failed to parse ack: %v", err)
		}
		if !ack.Success {
			t.Errorf("expected success, got error: %s", ack.Error)
		}
		if ack.MinerName == "" {
			t.Error("expected miner name in ack")
		}
	})

	t.Run("EmptyMinerType", func(t *testing.T) {
		payload := StartMinerPayload{
			MinerType: "",
			Config:    json.RawMessage(`{}`),
		}
		msg, err := NewMessage(MsgStartMiner, "sender-id", identity.ID, payload)
		if err != nil {
			t.Fatalf("failed to create message: %v", err)
		}

		_, err = worker.handleStartMiner(msg)
		if err == nil {
			t.Error("expected error for empty miner type")
		}
	})

	t.Run("WithProfileManager", func(t *testing.T) {
		pm := &mockProfileManagerFull{
			profiles: map[string]any{
				"test-profile": map[string]any{"pool": "pool.test:3333"},
			},
		}
		worker.SetProfileManager(pm)

		payload := StartMinerPayload{
			MinerType: "xmrig",
			ProfileID: "test-profile",
		}
		msg, err := NewMessage(MsgStartMiner, "sender-id", identity.ID, payload)
		if err != nil {
			t.Fatalf("failed to create message: %v", err)
		}

		response, err := worker.handleStartMiner(msg)
		if err != nil {
			t.Fatalf("handleStartMiner returned error: %v", err)
		}

		var ack MinerAckPayload
		response.ParsePayload(&ack)
		if !ack.Success {
			t.Errorf("expected success, got error: %s", ack.Error)
		}
	})

	t.Run("ProfileNotFound", func(t *testing.T) {
		pm := &mockProfileManagerFailing{}
		worker.SetProfileManager(pm)

		payload := StartMinerPayload{
			MinerType: "xmrig",
			ProfileID: "missing-profile",
		}
		msg, err := NewMessage(MsgStartMiner, "sender-id", identity.ID, payload)
		if err != nil {
			t.Fatalf("failed to create message: %v", err)
		}

		_, err = worker.handleStartMiner(msg)
		if err == nil {
			t.Error("expected error for missing profile")
		}
	})

	t.Run("StartFailsReturnsAck", func(t *testing.T) {
		worker.SetMinerManager(&mockMinerManagerFailing{})
		worker.SetProfileManager(nil)

		payload := StartMinerPayload{
			MinerType: "xmrig",
			Config:    json.RawMessage(`{}`),
		}
		msg, err := NewMessage(MsgStartMiner, "sender-id", identity.ID, payload)
		if err != nil {
			t.Fatalf("failed to create message: %v", err)
		}

		response, err := worker.handleStartMiner(msg)
		if err != nil {
			t.Fatalf("handleStartMiner should not return error when start fails: %v", err)
		}

		var ack MinerAckPayload
		response.ParsePayload(&ack)
		if ack.Success {
			t.Error("expected failure ack")
		}
		if ack.Error == "" {
			t.Error("expected error message in ack")
		}
	})

	_ = mm // suppress lint
}

// mockMinerManagerWithStart returns real instances from StartMiner.
type mockMinerManagerWithStart struct {
	mockMinerManager
	counter int
}

func (m *mockMinerManagerWithStart) StartMiner(minerType string, config any) (MinerInstance, error) {
	m.counter++
	name := fmt.Sprintf("%s-%d", minerType, m.counter)
	return &mockMinerInstance{name: name, minerType: minerType}, nil
}

func TestWorker_HandleStopMiner_WithManager(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("failed to create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("test-worker", RoleWorker); err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}
	pr, err := NewPeerRegistryWithPath(t.TempDir() + "/peers.json")
	if err != nil {
		t.Fatalf("failed to create peer registry: %v", err)
	}
	transport := NewTransport(nm, pr, DefaultTransportConfig())
	worker := NewWorker(nm, transport)
	worker.DataDir = t.TempDir()
	identity := nm.GetIdentity()

	t.Run("Success", func(t *testing.T) {
		worker.SetMinerManager(&mockMinerManager{})

		payload := StopMinerPayload{MinerName: "test-miner"}
		msg, _ := NewMessage(MsgStopMiner, "sender-id", identity.ID, payload)

		response, err := worker.handleStopMiner(msg)
		if err != nil {
			t.Fatalf("handleStopMiner returned error: %v", err)
		}

		var ack MinerAckPayload
		response.ParsePayload(&ack)
		if !ack.Success {
			t.Errorf("expected success, got error: %s", ack.Error)
		}
		if ack.MinerName != "test-miner" {
			t.Errorf("expected miner name 'test-miner', got '%s'", ack.MinerName)
		}
	})

	t.Run("StopFails", func(t *testing.T) {
		worker.SetMinerManager(&mockMinerManagerFailing{})

		payload := StopMinerPayload{MinerName: "missing-miner"}
		msg, _ := NewMessage(MsgStopMiner, "sender-id", identity.ID, payload)

		response, err := worker.handleStopMiner(msg)
		if err != nil {
			t.Fatalf("handleStopMiner should not return error: %v", err)
		}

		var ack MinerAckPayload
		response.ParsePayload(&ack)
		if ack.Success {
			t.Error("expected failure ack")
		}
		if ack.Error == "" {
			t.Error("expected error message in ack")
		}
	})
}

func TestWorker_HandleGetLogs_WithManager(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("failed to create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("test-worker", RoleWorker); err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}
	pr, err := NewPeerRegistryWithPath(t.TempDir() + "/peers.json")
	if err != nil {
		t.Fatalf("failed to create peer registry: %v", err)
	}
	transport := NewTransport(nm, pr, DefaultTransportConfig())
	worker := NewWorker(nm, transport)
	worker.DataDir = t.TempDir()
	identity := nm.GetIdentity()

	t.Run("Success", func(t *testing.T) {
		mm := &mockMinerManager{
			miners: []MinerInstance{
				&mockMinerInstance{
					name:      "test-miner",
					minerType: "xmrig",
				},
			},
		}
		worker.SetMinerManager(mm)

		payload := GetLogsPayload{MinerName: "test-miner", Lines: 100}
		msg, _ := NewMessage(MsgGetLogs, "sender-id", identity.ID, payload)

		response, err := worker.handleGetLogs(msg)
		if err != nil {
			t.Fatalf("handleGetLogs returned error: %v", err)
		}

		if response.Type != MsgLogs {
			t.Errorf("expected type %s, got %s", MsgLogs, response.Type)
		}

		var logs LogsPayload
		response.ParsePayload(&logs)
		if logs.MinerName != "test-miner" {
			t.Errorf("expected miner name 'test-miner', got '%s'", logs.MinerName)
		}
	})

	t.Run("MinerNotFound", func(t *testing.T) {
		// Use a manager that returns error for GetMiner
		mm := &mockMinerManagerFailing{}
		worker.SetMinerManager(mm)

		payload := GetLogsPayload{MinerName: "non-existent", Lines: 50}
		msg, _ := NewMessage(MsgGetLogs, "sender-id", identity.ID, payload)

		_, err := worker.handleGetLogs(msg)
		if err == nil {
			t.Error("expected error for non-existent miner")
		}
	})

	t.Run("NegativeLines", func(t *testing.T) {
		mm := &mockMinerManager{
			miners: []MinerInstance{
				&mockMinerInstance{name: "test-miner", minerType: "xmrig"},
			},
		}
		worker.SetMinerManager(mm)

		payload := GetLogsPayload{MinerName: "test-miner", Lines: -1}
		msg, _ := NewMessage(MsgGetLogs, "sender-id", identity.ID, payload)

		response, err := worker.handleGetLogs(msg)
		if err != nil {
			t.Fatalf("handleGetLogs returned error: %v", err)
		}
		// Lines <= 0 should be clamped to maxLogLines
		if response.Type != MsgLogs {
			t.Errorf("expected %s, got %s", MsgLogs, response.Type)
		}
	})

	t.Run("ExcessiveLines", func(t *testing.T) {
		mm := &mockMinerManager{
			miners: []MinerInstance{
				&mockMinerInstance{name: "test-miner", minerType: "xmrig"},
			},
		}
		worker.SetMinerManager(mm)

		payload := GetLogsPayload{MinerName: "test-miner", Lines: 999999}
		msg, _ := NewMessage(MsgGetLogs, "sender-id", identity.ID, payload)

		response, err := worker.handleGetLogs(msg)
		if err != nil {
			t.Fatalf("handleGetLogs returned error: %v", err)
		}
		if response.Type != MsgLogs {
			t.Errorf("expected %s, got %s", MsgLogs, response.Type)
		}
	})
}

func TestWorker_HandleGetStats_WithMinerManager(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("failed to create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("test-worker", RoleWorker); err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}
	pr, err := NewPeerRegistryWithPath(t.TempDir() + "/peers.json")
	if err != nil {
		t.Fatalf("failed to create peer registry: %v", err)
	}
	transport := NewTransport(nm, pr, DefaultTransportConfig())
	worker := NewWorker(nm, transport)
	worker.DataDir = t.TempDir()
	identity := nm.GetIdentity()

	// Set miner manager with miners that have real stats
	mm := &mockMinerManager{
		miners: []MinerInstance{
			&mockMinerInstance{
				name:      "miner-1",
				minerType: "xmrig",
				stats: map[string]any{
					"hashrate":  500.0,
					"shares":    25,
					"rejected":  1,
					"uptime":    3600,
					"pool":      "pool.test:3333",
					"algorithm": "rx/0",
				},
			},
			&mockMinerInstance{
				name:      "miner-2",
				minerType: "tt-miner",
				stats: map[string]any{
					"hashrate": 1200.0,
				},
			},
		},
	}
	worker.SetMinerManager(mm)

	msg, _ := NewMessage(MsgGetStats, "sender-id", identity.ID, nil)
	response, err := worker.handleGetStats(msg)
	if err != nil {
		t.Fatalf("handleGetStats returned error: %v", err)
	}

	var stats StatsPayload
	response.ParsePayload(&stats)

	if len(stats.Miners) != 2 {
		t.Errorf("expected 2 miners in stats, got %d", len(stats.Miners))
	}
}

func TestWorker_HandleMessage_UnknownType(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("failed to create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("test-worker", RoleWorker); err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}
	pr, err := NewPeerRegistryWithPath(t.TempDir() + "/peers.json")
	if err != nil {
		t.Fatalf("failed to create peer registry: %v", err)
	}
	transport := NewTransport(nm, pr, DefaultTransportConfig())
	worker := NewWorker(nm, transport)
	worker.DataDir = t.TempDir()

	identity := nm.GetIdentity()
	msg, _ := NewMessage("unknown_type", "sender-id", identity.ID, nil)

	// HandleMessage with unknown type should return silently (no panic)
	worker.HandleMessage(nil, msg)
}

func TestWorker_HandleDeploy_ProfileWithManager(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("failed to create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("test-worker", RoleWorker); err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}
	pr, err := NewPeerRegistryWithPath(t.TempDir() + "/peers.json")
	if err != nil {
		t.Fatalf("failed to create peer registry: %v", err)
	}
	transport := NewTransport(nm, pr, DefaultTransportConfig())
	worker := NewWorker(nm, transport)
	worker.DataDir = t.TempDir()

	pm := &mockProfileManagerFull{profiles: make(map[string]any)}
	worker.SetProfileManager(pm)

	identity := nm.GetIdentity()

	// Create an unencrypted profile bundle for deploy
	profileJSON := []byte(`{"id": "deploy-test", "name": "Test Profile"}`)
	bundle, err := CreateProfileBundleUnencrypted(profileJSON, "deploy-test")
	if err != nil {
		t.Fatalf("failed to create bundle: %v", err)
	}

	payload := DeployPayload{
		BundleType: string(BundleProfile),
		Data:       bundle.Data,
		Checksum:   bundle.Checksum,
		Name:       "deploy-test",
	}
	msg, _ := NewMessage(MsgDeploy, "sender-id", identity.ID, payload)

	response, err := worker.handleDeploy(nil, msg)
	if err != nil {
		t.Fatalf("handleDeploy returned error: %v", err)
	}

	var ack DeployAckPayload
	response.ParsePayload(&ack)
	if !ack.Success {
		t.Errorf("expected success, got error: %s", ack.Error)
	}
	if ack.Name != "deploy-test" {
		t.Errorf("expected name 'deploy-test', got '%s'", ack.Name)
	}
}

func TestWorker_HandleDeploy_ProfileSaveFails(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("failed to create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("test-worker", RoleWorker); err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}
	pr, err := NewPeerRegistryWithPath(t.TempDir() + "/peers.json")
	if err != nil {
		t.Fatalf("failed to create peer registry: %v", err)
	}
	transport := NewTransport(nm, pr, DefaultTransportConfig())
	worker := NewWorker(nm, transport)
	worker.DataDir = t.TempDir()
	worker.SetProfileManager(&mockProfileManagerFailing{})

	identity := nm.GetIdentity()

	profileJSON := []byte(`{"id": "fail-test"}`)
	bundle, _ := CreateProfileBundleUnencrypted(profileJSON, "fail-test")

	payload := DeployPayload{
		BundleType: string(BundleProfile),
		Data:       bundle.Data,
		Checksum:   bundle.Checksum,
		Name:       "fail-test",
	}
	msg, _ := NewMessage(MsgDeploy, "sender-id", identity.ID, payload)

	response, err := worker.handleDeploy(nil, msg)
	if err != nil {
		t.Fatalf("handleDeploy should return ack with error, not error: %v", err)
	}

	var ack DeployAckPayload
	response.ParsePayload(&ack)
	if ack.Success {
		t.Error("expected failure ack when save fails")
	}
}

func TestWorker_HandleDeploy_MinerBundle(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("failed to create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("test-worker", RoleWorker); err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}
	pr, err := NewPeerRegistryWithPath(t.TempDir() + "/peers.json")
	if err != nil {
		t.Fatalf("failed to create peer registry: %v", err)
	}
	transport := NewTransport(nm, pr, DefaultTransportConfig())
	worker := NewWorker(nm, transport)
	worker.DataDir = t.TempDir()
	pm := &mockProfileManagerFull{profiles: make(map[string]any)}
	worker.SetProfileManager(pm)

	identity := nm.GetIdentity()

	tmpDir := t.TempDir()
	minerPath := filepath.Join(tmpDir, "test-miner")
	os.WriteFile(minerPath, []byte("fake miner binary"), 0755)

	profileJSON := []byte(`{"pool":"test:3333"}`)

	// The handler extracts password as base64(conn.SharedSecret).
	// Create bundle with matching password.
	sharedSecret := []byte("shared-secret-32")
	bundlePassword := base64.StdEncoding.EncodeToString(sharedSecret)

	bundle, err := CreateMinerBundle(minerPath, profileJSON, "deploy-miner", bundlePassword)
	if err != nil {
		t.Fatalf("failed to create miner bundle: %v", err)
	}

	payload := DeployPayload{
		BundleType: string(BundleMiner),
		Data:       bundle.Data,
		Checksum:   bundle.Checksum,
		Name:       "deploy-miner",
	}
	msg, _ := NewMessage(MsgDeploy, "sender-id", identity.ID, payload)

	conn := &PeerConnection{
		SharedSecret: sharedSecret,
	}

	response, err := worker.handleDeploy(conn, msg)
	if err != nil {
		t.Fatalf("handleDeploy returned error: %v", err)
	}

	var ack DeployAckPayload
	response.ParsePayload(&ack)
	if !ack.Success {
		t.Errorf("expected success, got error: %s", ack.Error)
	}
}

func TestWorker_HandleDeploy_FullBundle(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("failed to create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("test-worker", RoleWorker); err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}
	pr, err := NewPeerRegistryWithPath(t.TempDir() + "/peers.json")
	if err != nil {
		t.Fatalf("failed to create peer registry: %v", err)
	}
	transport := NewTransport(nm, pr, DefaultTransportConfig())
	worker := NewWorker(nm, transport)
	worker.DataDir = t.TempDir()

	identity := nm.GetIdentity()

	tmpDir := t.TempDir()
	minerPath := filepath.Join(tmpDir, "test-miner")
	os.WriteFile(minerPath, []byte("miner binary"), 0755)

	sharedSecret := []byte("full-secret-key!")
	bundlePassword := base64.StdEncoding.EncodeToString(sharedSecret)

	bundle, err := CreateMinerBundle(minerPath, nil, "full-deploy", bundlePassword)
	if err != nil {
		t.Fatalf("failed to create miner bundle: %v", err)
	}

	payload := DeployPayload{
		BundleType: string(BundleFull),
		Data:       bundle.Data,
		Checksum:   bundle.Checksum,
		Name:       "full-deploy",
	}
	msg, _ := NewMessage(MsgDeploy, "sender-id", identity.ID, payload)

	conn := &PeerConnection{SharedSecret: sharedSecret}

	response, err := worker.handleDeploy(conn, msg)
	if err != nil {
		t.Fatalf("handleDeploy for full bundle returned error: %v", err)
	}

	var ack DeployAckPayload
	response.ParsePayload(&ack)
	if !ack.Success {
		t.Errorf("expected success, got error: %s", ack.Error)
	}
}

func TestWorker_HandleDeploy_MinerBundle_WithProfileManager(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	if err != nil {
		t.Fatalf("failed to create node manager: %v", err)
	}
	if err := nm.GenerateIdentity("test-worker", RoleWorker); err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}
	pr, err := NewPeerRegistryWithPath(t.TempDir() + "/peers.json")
	if err != nil {
		t.Fatalf("failed to create peer registry: %v", err)
	}
	transport := NewTransport(nm, pr, DefaultTransportConfig())
	worker := NewWorker(nm, transport)
	worker.DataDir = t.TempDir()

	// Set a failing profile manager to exercise the warn-and-continue path
	worker.SetProfileManager(&mockProfileManagerFailing{})

	identity := nm.GetIdentity()

	tmpDir := t.TempDir()
	minerPath := filepath.Join(tmpDir, "test-miner")
	os.WriteFile(minerPath, []byte("miner binary"), 0755)

	profileJSON := []byte(`{"pool":"test:3333"}`)
	sharedSecret := []byte("profile-secret!!")
	bundlePassword := base64.StdEncoding.EncodeToString(sharedSecret)

	bundle, err := CreateMinerBundle(minerPath, profileJSON, "deploy-with-profile", bundlePassword)
	if err != nil {
		t.Fatalf("failed to create miner bundle: %v", err)
	}

	payload := DeployPayload{
		BundleType: string(BundleMiner),
		Data:       bundle.Data,
		Checksum:   bundle.Checksum,
		Name:       "deploy-with-profile",
	}
	msg, _ := NewMessage(MsgDeploy, "sender-id", identity.ID, payload)

	conn := &PeerConnection{SharedSecret: sharedSecret}

	response, err := worker.handleDeploy(conn, msg)
	if err != nil {
		t.Fatalf("handleDeploy returned error: %v", err)
	}

	var ack DeployAckPayload
	response.ParsePayload(&ack)
	// Deploy should still succeed even if profile save fails
	if !ack.Success {
		t.Errorf("expected success despite profile save failure, got: %s", ack.Error)
	}
}

func TestWorker_HandleDeploy_InvalidPayload(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	dir := t.TempDir()
	nm, _ := NewNodeManagerWithPaths(
		filepath.Join(dir, "private.key"),
		filepath.Join(dir, "node.json"),
	)
	nm.GenerateIdentity("test", RoleWorker)
	pr, _ := NewPeerRegistryWithPath(t.TempDir() + "/peers.json")
	transport := NewTransport(nm, pr, DefaultTransportConfig())
	worker := NewWorker(nm, transport)
	worker.DataDir = t.TempDir()
	identity := nm.GetIdentity()

	// Create a message with invalid payload
	msg, _ := NewMessage(MsgDeploy, "sender-id", identity.ID, "invalid-payload-not-struct")

	_, err := worker.handleDeploy(nil, msg)
	if err == nil {
		t.Error("expected error for invalid deploy payload")
	}
}

func TestWorker_HandleGetStats_NoIdentity(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()

	nm, _ := NewNodeManagerWithPaths(
		filepath.Join(t.TempDir(), "priv.key"),
		filepath.Join(t.TempDir(), "node.json"),
	)
	// Don't generate identity
	pr, _ := NewPeerRegistryWithPath(t.TempDir() + "/peers.json")
	transport := NewTransport(nm, pr, DefaultTransportConfig())
	worker := NewWorker(nm, transport)
	worker.DataDir = t.TempDir()

	msg, _ := NewMessage(MsgGetStats, "sender-id", "target-id", nil)
	_, err := worker.handleGetStats(msg)
	if err == nil {
		t.Error("expected error when identity is not initialized")
	}
}

func TestWorker_HandleMessage_IntegrationViaWebSocket(t *testing.T) {
	// Test HandleMessage through real WebSocket -- exercises error response sending path
	tp := setupTestTransportPair(t)

	worker := NewWorker(tp.ServerNode, tp.Server)
	// No miner manager set -- start_miner will fail and send error response
	worker.RegisterWithTransport()

	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)
	tp.connectClient(t)
	time.Sleep(50 * time.Millisecond)

	serverID := tp.ServerNode.GetIdentity().ID

	// Send start_miner which will fail because no manager is set.
	// The worker should send an error response via the connection.
	err := controller.StartRemoteMiner(serverID, "xmrig", "", json.RawMessage(`{}`))
	// Should get an error back (either protocol error or operation failed)
	if err == nil {
		t.Error("expected error when worker has no miner manager")
	}
}

func TestWorker_HandleMessage_GetStats_IntegrationViaWebSocket(t *testing.T) {
	// HandleMessage dispatch for get_stats through real WebSocket
	tp := setupTestTransportPair(t)

	worker := NewWorker(tp.ServerNode, tp.Server)
	mm := &mockMinerManager{
		miners: []MinerInstance{
			&mockMinerInstance{
				name:      "test-miner",
				minerType: "xmrig",
				stats: map[string]any{
					"hashrate":  500.0,
					"shares":    25,
					"rejected":  1,
					"uptime":    3600,
					"pool":      "pool.test:3333",
					"algorithm": "rx/0",
				},
			},
		},
	}
	worker.SetMinerManager(mm)
	worker.RegisterWithTransport()

	controller := NewController(tp.ClientNode, tp.ClientReg, tp.Client)
	tp.connectClient(t)
	time.Sleep(50 * time.Millisecond)

	serverID := tp.ServerNode.GetIdentity().ID

	stats, err := controller.GetRemoteStats(serverID)
	if err != nil {
		t.Fatalf("GetRemoteStats failed: %v", err)
	}
	if len(stats.Miners) != 1 {
		t.Errorf("expected 1 miner, got %d", len(stats.Miners))
	}
}
