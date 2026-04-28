package node

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func setupTestPeerRegistry(t *testing.T) (*PeerRegistry, func()) {
	tmpDir, err := os.MkdirTemp("", "peer-registry-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	peersPath := filepath.Join(tmpDir, "peers.json")

	pr, err := NewPeerRegistryWithPath(peersPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create peer registry: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return pr, cleanup
}

func tripletPeerRegistry(t *testing.T) *PeerRegistry {
	t.Helper()
	registry, cleanup := setupTestPeerRegistry(t)
	t.Cleanup(func() {
		_ = registry.Close()
		cleanup()
	})
	return registry
}

func tripletPeer(id string) *Peer {
	return &Peer{
		ID:        id,
		Name:      "peer-" + id,
		PublicKey: "pub-" + id,
		Address:   "127.0.0.1:0",
		Role:      RoleWorker,
		Score:     ScoreDefault,
	}
}

func TestPeerRegistry_NewPeerRegistry(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	if pr.Count() != 0 {
		t.Errorf("expected 0 peers, got %d", pr.Count())
	}
}

func TestPeer_NewPeerRegistry_Good(t *testing.T) {
	path := filepath.Join(t.TempDir(), "peers.json")
	registry, err := NewPeerRegistry(path)
	if err != nil {
		t.Fatalf("NewPeerRegistry: %v", err)
	}
	t.Cleanup(func() { registry.Close() })
	if registry.path != path {
		t.Fatalf("path: got %q", registry.path)
	}
}

func TestPeer_NewPeerRegistry_Bad(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()
	registry, err := NewPeerRegistry()
	if err != nil {
		t.Fatalf("NewPeerRegistry default: %v", err)
	}
	t.Cleanup(func() { registry.Close() })
	if registry.Count() != 0 {
		t.Fatalf("count: got %d", registry.Count())
	}
}

func TestPeer_NewPeerRegistry_Ugly(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()
	registry, err := NewPeerRegistry("")
	if err != nil {
		t.Fatalf("NewPeerRegistry empty path: %v", err)
	}
	t.Cleanup(func() { registry.Close() })
	if registry.GetAuthMode() != PeerAuthOpen {
		t.Fatalf("auth mode: got %v", registry.GetAuthMode())
	}
}

func TestPeer_NewPeerRegistryWithPath_Good(t *testing.T) {
	path := filepath.Join(t.TempDir(), "peers.json")
	registry, err := NewPeerRegistryWithPath(path)
	if err != nil {
		t.Fatalf("NewPeerRegistryWithPath: %v", err)
	}
	t.Cleanup(func() { registry.Close() })
	if registry.allowlistPath != path+".allowlist.json" {
		t.Fatalf("allowlist path: got %q", registry.allowlistPath)
	}
}

func TestPeer_NewPeerRegistryWithPath_Bad(t *testing.T) {
	registry, err := NewPeerRegistryWithPath("")
	if err != nil {
		t.Fatalf("NewPeerRegistryWithPath empty: %v", err)
	}
	t.Cleanup(func() { registry.Close() })
	if registry.path != "" {
		t.Fatalf("path: got %q", registry.path)
	}
}

func TestPeer_NewPeerRegistryWithPath_Ugly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "peers.json")
	registry, err := NewPeerRegistryWithPath(path)
	if err != nil {
		t.Fatalf("NewPeerRegistryWithPath nested: %v", err)
	}
	t.Cleanup(func() { registry.Close() })
	if registry.Count() != 0 {
		t.Fatalf("count: got %d", registry.Count())
	}
}

func TestPeer_PeerRegistry_SetAuthMode_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.SetAuthMode(PeerAuthAllowlist)
	if registry.GetAuthMode() != PeerAuthAllowlist {
		t.Fatalf("auth mode: got %v", registry.GetAuthMode())
	}
}

func TestPeer_PeerRegistry_SetAuthMode_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.SetAuthMode(PeerAuthMode(99))
	if registry.GetAuthMode() != PeerAuthMode(99) {
		t.Fatalf("auth mode: got %v", registry.GetAuthMode())
	}
}

func TestPeer_PeerRegistry_SetAuthMode_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.SetAuthMode(PeerAuthAllowlist)
	registry.SetAuthMode(PeerAuthOpen)
	if registry.GetAuthMode() != PeerAuthOpen {
		t.Fatalf("auth mode: got %v", registry.GetAuthMode())
	}
}

func TestPeer_PeerRegistry_GetAuthMode_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	if registry.GetAuthMode() != PeerAuthOpen {
		t.Fatalf("auth mode: got %v", registry.GetAuthMode())
	}
}

func TestPeer_PeerRegistry_GetAuthMode_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.SetAuthMode(PeerAuthAllowlist)
	if registry.GetAuthMode() == PeerAuthOpen {
		t.Fatal("expected allowlist mode")
	}
}

func TestPeer_PeerRegistry_GetAuthMode_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.SetAuthMode(PeerAuthMode(-1))
	if registry.GetAuthMode() != PeerAuthMode(-1) {
		t.Fatalf("auth mode: got %v", registry.GetAuthMode())
	}
}

func TestPeer_PeerRegistry_AllowPublicKey_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.AllowPublicKey("pub")
	if !registry.IsPublicKeyAllowed("pub") {
		t.Fatal("public key not allowed")
	}
}

func TestPeer_PeerRegistry_AllowPublicKey_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.AllowPublicKey("")
	if !registry.IsPublicKeyAllowed("") {
		t.Fatal("empty key should be stored")
	}
}

func TestPeer_PeerRegistry_AllowPublicKey_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.AllowPublicKey("pub")
	registry.AllowPublicKey("pub")
	if len(registry.ListAllowedPublicKeys()) != 1 {
		t.Fatalf("keys: %#v", registry.ListAllowedPublicKeys())
	}
}

func TestPeer_PeerRegistry_RevokePublicKey_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.AllowPublicKey("pub")
	registry.RevokePublicKey("pub")
	if registry.IsPublicKeyAllowed("pub") {
		t.Fatal("public key should be revoked")
	}
}

func TestPeer_PeerRegistry_RevokePublicKey_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.RevokePublicKey("missing")
	if registry.IsPublicKeyAllowed("missing") {
		t.Fatal("missing key should not be allowed")
	}
}

func TestPeer_PeerRegistry_RevokePublicKey_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.AllowPublicKey("")
	registry.RevokePublicKey("")
	if registry.IsPublicKeyAllowed("") {
		t.Fatal("empty key should be revoked")
	}
}

func TestPeer_PeerRegistry_IsPublicKeyAllowed_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.AllowPublicKey("pub")
	if !registry.IsPublicKeyAllowed("pub") {
		t.Fatal("expected key allowed")
	}
}

func TestPeer_PeerRegistry_IsPublicKeyAllowed_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	if registry.IsPublicKeyAllowed("missing") {
		t.Fatal("missing key should not be allowed")
	}
}

func TestPeer_PeerRegistry_IsPublicKeyAllowed_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	if registry.IsPublicKeyAllowed("") {
		t.Fatal("empty key should start disallowed")
	}
}

func TestPeer_PeerRegistry_IsPeerAllowed_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	if !registry.IsPeerAllowed("peer", "pub") {
		t.Fatal("open auth should allow peer")
	}
}

func TestPeer_PeerRegistry_IsPeerAllowed_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.SetAuthMode(PeerAuthAllowlist)
	if registry.IsPeerAllowed("peer", "pub") {
		t.Fatal("unregistered peer should be rejected")
	}
}

func TestPeer_PeerRegistry_IsPeerAllowed_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.SetAuthMode(PeerAuthAllowlist)
	registry.AllowPublicKey("pub")
	if !registry.IsPeerAllowed("peer", "pub") {
		t.Fatal("allowlisted public key should be allowed")
	}
}

func TestPeer_PeerRegistry_ListAllowedPublicKeys_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.AllowPublicKey("pub")
	keys := registry.ListAllowedPublicKeys()
	if len(keys) != 1 || keys[0] != "pub" {
		t.Fatalf("keys: %#v", keys)
	}
}

func TestPeer_PeerRegistry_ListAllowedPublicKeys_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	keys := registry.ListAllowedPublicKeys()
	if len(keys) != 0 {
		t.Fatalf("keys: %#v", keys)
	}
}

func TestPeer_PeerRegistry_ListAllowedPublicKeys_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.AllowPublicKey("b")
	registry.AllowPublicKey("a")
	keys := registry.ListAllowedPublicKeys()
	if len(keys) != 2 {
		t.Fatalf("keys: %#v", keys)
	}
}

func TestPeer_PeerRegistry_AllowedPublicKeys_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.AllowPublicKey("pub")
	count := 0
	for range registry.AllowedPublicKeys() {
		count++
	}
	if count != 1 {
		t.Fatalf("key count: got %d", count)
	}
}

func TestPeer_PeerRegistry_AllowedPublicKeys_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	count := 0
	for range registry.AllowedPublicKeys() {
		count++
	}
	if count != 0 {
		t.Fatalf("key count: got %d", count)
	}
}

func TestPeer_PeerRegistry_AllowedPublicKeys_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.AllowPublicKey("pub")
	count := 0
	for range registry.AllowedPublicKeys() {
		count++
		break
	}
	if count != 1 {
		t.Fatalf("key count: got %d", count)
	}
}

func TestPeer_PeerRegistry_AddPeer_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	err := registry.AddPeer(tripletPeer("a"))
	if err != nil {
		t.Fatalf("AddPeer: %v", err)
	}
	if registry.Count() != 1 {
		t.Fatalf("count: got %d", registry.Count())
	}
}

func TestPeer_PeerRegistry_AddPeer_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	err := registry.AddPeer(&Peer{})
	if err == nil {
		t.Fatal("expected missing ID error")
	}
	if registry.Count() != 0 {
		t.Fatalf("count: got %d", registry.Count())
	}
}

func TestPeer_PeerRegistry_AddPeer_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	peer := tripletPeer("a")
	peer.Name = "-bad"
	err := registry.AddPeer(peer)
	if err == nil {
		t.Fatal("expected invalid name error")
	}
	if registry.Count() != 0 {
		t.Fatalf("count: got %d", registry.Count())
	}
}

func TestPeer_PeerRegistry_UpdatePeer_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	peer := tripletPeer("a")
	_ = registry.AddPeer(peer)
	peer.Name = "renamed"
	err := registry.UpdatePeer(peer)
	if err != nil {
		t.Fatalf("UpdatePeer: %v", err)
	}
	if registry.GetPeer("a").Name != "renamed" {
		t.Fatal("peer not updated")
	}
}

func TestPeer_PeerRegistry_UpdatePeer_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	err := registry.UpdatePeer(tripletPeer("missing"))
	if err == nil {
		t.Fatal("expected missing peer error")
	}
	if registry.Count() != 0 {
		t.Fatalf("count: got %d", registry.Count())
	}
}

func TestPeer_PeerRegistry_UpdatePeer_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	peer := tripletPeer("a")
	_ = registry.AddPeer(peer)
	peer.Score = 0
	err := registry.UpdatePeer(peer)
	if err != nil {
		t.Fatalf("UpdatePeer: %v", err)
	}
	if registry.GetPeer("a").Score != 0 {
		t.Fatal("score not updated")
	}
}

func TestPeer_PeerRegistry_RemovePeer_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	err := registry.RemovePeer("a")
	if err != nil {
		t.Fatalf("RemovePeer: %v", err)
	}
	if registry.Count() != 0 {
		t.Fatalf("count: got %d", registry.Count())
	}
}

func TestPeer_PeerRegistry_RemovePeer_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	err := registry.RemovePeer("missing")
	if err == nil {
		t.Fatal("expected missing peer error")
	}
	if registry.Count() != 0 {
		t.Fatalf("count: got %d", registry.Count())
	}
}

func TestPeer_PeerRegistry_RemovePeer_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	_ = registry.RemovePeer("a")
	err := registry.RemovePeer("a")
	if err == nil {
		t.Fatal("expected second remove error")
	}
}

func TestPeer_PeerRegistry_GetPeer_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	peer := registry.GetPeer("a")
	if peer == nil || peer.ID != "a" {
		t.Fatalf("peer: %#v", peer)
	}
}

func TestPeer_PeerRegistry_GetPeer_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	peer := registry.GetPeer("missing")
	if peer != nil {
		t.Fatalf("peer: got %#v, want nil", peer)
	}
}

func TestPeer_PeerRegistry_GetPeer_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	peer := registry.GetPeer("a")
	peer.Name = "mutated"
	if registry.GetPeer("a").Name == "mutated" {
		t.Fatal("GetPeer should return a copy")
	}
}

func TestPeer_PeerRegistry_ListPeers_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	if len(registry.ListPeers()) != 1 {
		t.Fatalf("peers: %#v", registry.ListPeers())
	}
}

func TestPeer_PeerRegistry_ListPeers_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	if len(registry.ListPeers()) != 0 {
		t.Fatalf("peers: %#v", registry.ListPeers())
	}
}

func TestPeer_PeerRegistry_ListPeers_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	peers := registry.ListPeers()
	peers[0].Name = "mutated"
	if registry.GetPeer("a").Name == "mutated" {
		t.Fatal("ListPeers should return copies")
	}
}

func TestPeer_PeerRegistry_Peers_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	count := 0
	for range registry.Peers() {
		count++
	}
	if count != 1 {
		t.Fatalf("peer count: got %d", count)
	}
}

func TestPeer_PeerRegistry_Peers_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	count := 0
	for range registry.Peers() {
		count++
	}
	if count != 0 {
		t.Fatalf("peer count: got %d", count)
	}
}

func TestPeer_PeerRegistry_Peers_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	count := 0
	for range registry.Peers() {
		count++
		break
	}
	if count != 1 {
		t.Fatalf("peer count: got %d", count)
	}
}

func TestPeer_PeerRegistry_UpdateMetrics_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	err := registry.UpdateMetrics("a", 10, 20, 1)
	if err != nil {
		t.Fatalf("UpdateMetrics: %v", err)
	}
	if registry.GetPeer("a").PingMS != 10 {
		t.Fatal("metrics not updated")
	}
}

func TestPeer_PeerRegistry_UpdateMetrics_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	err := registry.UpdateMetrics("missing", 10, 20, 1)
	if err == nil {
		t.Fatal("expected missing peer error")
	}
	if registry.Count() != 0 {
		t.Fatalf("count: got %d", registry.Count())
	}
}

func TestPeer_PeerRegistry_UpdateMetrics_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	err := registry.UpdateMetrics("a", -1, -2, -3)
	if err != nil {
		t.Fatalf("UpdateMetrics negative: %v", err)
	}
	if registry.GetPeer("a").Hops != -3 {
		t.Fatal("negative hops not stored")
	}
}

func TestPeer_PeerRegistry_UpdateScore_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	err := registry.UpdateScore("a", 80)
	if err != nil {
		t.Fatalf("UpdateScore: %v", err)
	}
	if registry.GetPeer("a").Score != 80 {
		t.Fatal("score not updated")
	}
}

func TestPeer_PeerRegistry_UpdateScore_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	err := registry.UpdateScore("missing", 80)
	if err == nil {
		t.Fatal("expected missing peer error")
	}
	if registry.Count() != 0 {
		t.Fatalf("count: got %d", registry.Count())
	}
}

func TestPeer_PeerRegistry_UpdateScore_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	_ = registry.UpdateScore("a", 200)
	if registry.GetPeer("a").Score != ScoreMaximum {
		t.Fatalf("score: got %f", registry.GetPeer("a").Score)
	}
}

func TestPeer_PeerRegistry_SetConnected_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	registry.SetConnected("a", true)
	if !registry.GetPeer("a").Connected {
		t.Fatal("peer should be connected")
	}
}

func TestPeer_PeerRegistry_SetConnected_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.SetConnected("missing", true)
	if registry.Count() != 0 {
		t.Fatalf("count: got %d", registry.Count())
	}
}

func TestPeer_PeerRegistry_SetConnected_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	registry.SetConnected("a", true)
	registry.SetConnected("a", false)
	if registry.GetPeer("a").Connected {
		t.Fatal("peer should be disconnected")
	}
}

func TestPeer_PeerRegistry_MarkSeen_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	peer := tripletPeer("a")
	_ = registry.AddPeer(peer)
	before := registry.GetPeer("a").LastSeen
	registry.MarkSeen("a")
	if !registry.GetPeer("a").LastSeen.After(before) && registry.GetPeer("a").LastSeen.Equal(before) {
		t.Fatal("LastSeen not updated")
	}
}

func TestPeer_PeerRegistry_MarkSeen_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.MarkSeen("missing")
	if registry.Count() != 0 {
		t.Fatalf("count: got %d", registry.Count())
	}
}

func TestPeer_PeerRegistry_MarkSeen_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	peer := tripletPeer("a")
	peer.LastSeen = time.Time{}
	_ = registry.AddPeer(peer)
	registry.MarkSeen("a")
	if registry.GetPeer("a").LastSeen.IsZero() {
		t.Fatal("LastSeen should be set")
	}
}

func TestPeer_PeerRegistry_RecordSuccess_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	peer := tripletPeer("a")
	peer.Score = 50
	_ = registry.AddPeer(peer)
	registry.RecordSuccess("a")
	if registry.GetPeer("a").Score <= 50 {
		t.Fatal("score should increase")
	}
}

func TestPeer_PeerRegistry_RecordSuccess_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.RecordSuccess("missing")
	if registry.Count() != 0 {
		t.Fatalf("count: got %d", registry.Count())
	}
}

func TestPeer_PeerRegistry_RecordSuccess_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	peer := tripletPeer("a")
	peer.Score = ScoreMaximum
	_ = registry.AddPeer(peer)
	registry.RecordSuccess("a")
	if registry.GetPeer("a").Score != ScoreMaximum {
		t.Fatal("score should clamp at maximum")
	}
}

func TestPeer_PeerRegistry_RecordFailure_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	peer := tripletPeer("a")
	peer.Score = 50
	_ = registry.AddPeer(peer)
	registry.RecordFailure("a")
	if registry.GetPeer("a").Score >= 50 {
		t.Fatal("score should decrease")
	}
}

func TestPeer_PeerRegistry_RecordFailure_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.RecordFailure("missing")
	if registry.Count() != 0 {
		t.Fatalf("count: got %d", registry.Count())
	}
}

func TestPeer_PeerRegistry_RecordFailure_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	peer := tripletPeer("a")
	peer.Score = 1
	_ = registry.AddPeer(peer)
	registry.RecordFailure("a")
	if registry.GetPeer("a").Score != ScoreMinimum {
		t.Fatal("score should clamp at minimum")
	}
}

func TestPeer_PeerRegistry_RecordTimeout_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	peer := tripletPeer("a")
	peer.Score = 50
	_ = registry.AddPeer(peer)
	registry.RecordTimeout("a")
	if registry.GetPeer("a").Score >= 50 {
		t.Fatal("score should decrease")
	}
}

func TestPeer_PeerRegistry_RecordTimeout_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	registry.RecordTimeout("missing")
	if registry.Count() != 0 {
		t.Fatalf("count: got %d", registry.Count())
	}
}

func TestPeer_PeerRegistry_RecordTimeout_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	peer := tripletPeer("a")
	peer.Score = 1
	_ = registry.AddPeer(peer)
	registry.RecordTimeout("a")
	if registry.GetPeer("a").Score != ScoreMinimum {
		t.Fatal("score should clamp at minimum")
	}
}

func TestPeer_PeerRegistry_GetPeersByScore_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	a := tripletPeer("a")
	b := tripletPeer("b")
	a.Score = 10
	b.Score = 90
	_ = registry.AddPeer(a)
	_ = registry.AddPeer(b)
	peers := registry.GetPeersByScore()
	if peers[0].ID != "b" {
		t.Fatalf("peers: %#v", peers)
	}
}

func TestPeer_PeerRegistry_GetPeersByScore_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	peers := registry.GetPeersByScore()
	if len(peers) != 0 {
		t.Fatalf("peers: %#v", peers)
	}
}

func TestPeer_PeerRegistry_GetPeersByScore_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	peers := registry.GetPeersByScore()
	peers[0].Name = "mutated"
	if registry.GetPeer("a").Name == "mutated" {
		t.Fatal("GetPeersByScore should return copies")
	}
}

func TestPeer_PeerRegistry_PeersByScore_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	count := 0
	for range registry.PeersByScore() {
		count++
	}
	if count != 1 {
		t.Fatalf("peer count: got %d", count)
	}
}

func TestPeer_PeerRegistry_PeersByScore_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	count := 0
	for range registry.PeersByScore() {
		count++
	}
	if count != 0 {
		t.Fatalf("peer count: got %d", count)
	}
}

func TestPeer_PeerRegistry_PeersByScore_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	count := 0
	for range registry.PeersByScore() {
		count++
		break
	}
	if count != 1 {
		t.Fatalf("peer count: got %d", count)
	}
}

func TestPeer_PeerRegistry_SelectOptimalPeer_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	peer := tripletPeer("a")
	peer.Score = 100
	_ = registry.AddPeer(peer)
	if registry.SelectOptimalPeer().ID != "a" {
		t.Fatal("expected optimal peer")
	}
}

func TestPeer_PeerRegistry_SelectOptimalPeer_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	if registry.SelectOptimalPeer() != nil {
		t.Fatal("empty registry should have no optimal peer")
	}
}

func TestPeer_PeerRegistry_SelectOptimalPeer_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	peer := registry.SelectOptimalPeer()
	peer.Name = "mutated"
	if registry.GetPeer("a").Name == "mutated" {
		t.Fatal("SelectOptimalPeer should return a copy")
	}
}

func TestPeer_PeerRegistry_SelectNearestPeers_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	peers := registry.SelectNearestPeers(1)
	if len(peers) != 1 {
		t.Fatalf("peers: %#v", peers)
	}
}

func TestPeer_PeerRegistry_SelectNearestPeers_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	peers := registry.SelectNearestPeers(1)
	if peers != nil {
		t.Fatalf("peers: %#v", peers)
	}
}

func TestPeer_PeerRegistry_SelectNearestPeers_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	peers := registry.SelectNearestPeers(0)
	if len(peers) != 0 {
		t.Fatalf("peers: %#v", peers)
	}
}

func TestPeer_PeerRegistry_FindNearby_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	peer := tripletPeer("a")
	peer.Latitude = 1
	peer.Longitude = 1
	_ = registry.AddPeer(peer)
	peers, err := registry.FindNearby(1, 1, 0, 1)
	if err != nil || len(peers) != 1 {
		t.Fatalf("peers=%#v err=%v", peers, err)
	}
}

func TestPeer_PeerRegistry_FindNearby_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	peers, err := registry.FindNearby(0, 0, 0, 0)
	if err != nil || len(peers) != 0 {
		t.Fatalf("peers=%#v err=%v", peers, err)
	}
}

func TestPeer_PeerRegistry_FindNearby_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	peers, err := registry.FindNearby(0, 0, 0, 5)
	if err != nil || len(peers) != 1 {
		t.Fatalf("peers=%#v err=%v", peers, err)
	}
}

func TestPeer_PeerRegistry_GetConnectedPeers_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	registry.SetConnected("a", true)
	if len(registry.GetConnectedPeers()) != 1 {
		t.Fatalf("connected: %#v", registry.GetConnectedPeers())
	}
}

func TestPeer_PeerRegistry_GetConnectedPeers_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	if len(registry.GetConnectedPeers()) != 0 {
		t.Fatalf("connected: %#v", registry.GetConnectedPeers())
	}
}

func TestPeer_PeerRegistry_GetConnectedPeers_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	registry.SetConnected("a", false)
	if len(registry.GetConnectedPeers()) != 0 {
		t.Fatalf("connected: %#v", registry.GetConnectedPeers())
	}
}

func TestPeer_PeerRegistry_ConnectedPeers_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	registry.SetConnected("a", true)
	count := 0
	for range registry.ConnectedPeers() {
		count++
	}
	if count != 1 {
		t.Fatalf("connected count: got %d", count)
	}
}

func TestPeer_PeerRegistry_ConnectedPeers_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	count := 0
	for range registry.ConnectedPeers() {
		count++
	}
	if count != 0 {
		t.Fatalf("connected count: got %d", count)
	}
}

func TestPeer_PeerRegistry_ConnectedPeers_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	registry.SetConnected("a", true)
	count := 0
	for range registry.ConnectedPeers() {
		count++
		break
	}
	if count != 1 {
		t.Fatalf("connected count: got %d", count)
	}
}

func TestPeer_PeerRegistry_Count_Good(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	if registry.Count() != 1 {
		t.Fatalf("count: got %d", registry.Count())
	}
}

func TestPeer_PeerRegistry_Count_Bad(t *testing.T) {
	registry := tripletPeerRegistry(t)
	if registry.Count() != 0 {
		t.Fatalf("count: got %d", registry.Count())
	}
}

func TestPeer_PeerRegistry_Count_Ugly(t *testing.T) {
	registry := tripletPeerRegistry(t)
	_ = registry.AddPeer(tripletPeer("a"))
	_ = registry.RemovePeer("a")
	if registry.Count() != 0 {
		t.Fatalf("count: got %d", registry.Count())
	}
}

func TestPeer_PeerRegistry_Close_Good(t *testing.T) {
	registry, cleanup := setupTestPeerRegistry(t)
	defer cleanup()
	_ = registry.AddPeer(tripletPeer("a"))
	if err := registry.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if registry.dirty {
		t.Fatal("dirty flag should be cleared")
	}
}

func TestPeer_PeerRegistry_Close_Bad(t *testing.T) {
	registry, cleanup := setupTestPeerRegistry(t)
	defer cleanup()
	if err := registry.Close(); err != nil {
		t.Fatalf("Close clean: %v", err)
	}
	if registry.dirty {
		t.Fatal("dirty flag should remain false")
	}
}

func TestPeer_PeerRegistry_Close_Ugly(t *testing.T) {
	registry, cleanup := setupTestPeerRegistry(t)
	defer cleanup()
	_ = registry.Close()
	if err := registry.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestPeerRegistry_AddPeer(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	peer := &Peer{
		ID:        "test-peer-1",
		Name:      "Test Peer",
		PublicKey: "testkey123",
		Address:   "192.168.1.100:9091",
		Role:      RoleWorker,
		Score:     75,
	}

	err := pr.AddPeer(peer)
	if err != nil {
		t.Fatalf("failed to add peer: %v", err)
	}

	if pr.Count() != 1 {
		t.Errorf("expected 1 peer, got %d", pr.Count())
	}

	// Try to add duplicate
	err = pr.AddPeer(peer)
	if err == nil {
		t.Error("expected error when adding duplicate peer")
	}
}

func TestPeerRegistry_GetPeer(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	peer := &Peer{
		ID:        "get-test-peer",
		Name:      "Get Test",
		PublicKey: "getkey123",
		Address:   "10.0.0.1:9091",
		Role:      RoleDual,
	}

	pr.AddPeer(peer)

	retrieved := pr.GetPeer("get-test-peer")
	if retrieved == nil {
		t.Fatal("failed to retrieve peer")
	}

	if retrieved.Name != "Get Test" {
		t.Errorf("expected name 'Get Test', got '%s'", retrieved.Name)
	}

	// Non-existent peer
	nonExistent := pr.GetPeer("non-existent")
	if nonExistent != nil {
		t.Error("expected nil for non-existent peer")
	}
}

func TestPeerRegistry_ListPeers(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	peers := []*Peer{
		{ID: "list-1", Name: "Peer 1", Address: "1.1.1.1:9091", Role: RoleWorker},
		{ID: "list-2", Name: "Peer 2", Address: "2.2.2.2:9091", Role: RoleWorker},
		{ID: "list-3", Name: "Peer 3", Address: "3.3.3.3:9091", Role: RoleController},
	}

	for _, p := range peers {
		pr.AddPeer(p)
	}

	listed := pr.ListPeers()
	if len(listed) != 3 {
		t.Errorf("expected 3 peers, got %d", len(listed))
	}
}

func TestPeerRegistry_RemovePeer(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	peer := &Peer{
		ID:      "remove-test",
		Name:    "Remove Me",
		Address: "5.5.5.5:9091",
		Role:    RoleWorker,
	}

	pr.AddPeer(peer)

	if pr.Count() != 1 {
		t.Error("peer should exist before removal")
	}

	err := pr.RemovePeer("remove-test")
	if err != nil {
		t.Fatalf("failed to remove peer: %v", err)
	}

	if pr.Count() != 0 {
		t.Error("peer should be removed")
	}

	// Remove non-existent
	err = pr.RemovePeer("non-existent")
	if err == nil {
		t.Error("expected error when removing non-existent peer")
	}
}

func TestPeerRegistry_UpdateMetrics(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	peer := &Peer{
		ID:      "metrics-test",
		Name:    "Metrics Peer",
		Address: "6.6.6.6:9091",
		Role:    RoleWorker,
	}

	pr.AddPeer(peer)

	err := pr.UpdateMetrics("metrics-test", 50.5, 100.2, 3)
	if err != nil {
		t.Fatalf("failed to update metrics: %v", err)
	}

	updated := pr.GetPeer("metrics-test")
	if updated == nil {
		t.Fatal("expected peer to exist")
	}
	if updated.PingMS != 50.5 {
		t.Errorf("expected ping 50.5, got %f", updated.PingMS)
	}
	if updated.GeoKM != 100.2 {
		t.Errorf("expected geo 100.2, got %f", updated.GeoKM)
	}
	if updated.Hops != 3 {
		t.Errorf("expected hops 3, got %d", updated.Hops)
	}
}

func TestPeerRegistry_UpdateScore(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	peer := &Peer{
		ID:    "score-test",
		Name:  "Score Peer",
		Score: 50,
	}

	pr.AddPeer(peer)

	err := pr.UpdateScore("score-test", 85.5)
	if err != nil {
		t.Fatalf("failed to update score: %v", err)
	}

	updated := pr.GetPeer("score-test")
	if updated == nil {
		t.Fatal("expected peer to exist")
	}
	if updated.Score != 85.5 {
		t.Errorf("expected score 85.5, got %f", updated.Score)
	}

	// Test clamping - over 100
	err = pr.UpdateScore("score-test", 150)
	if err != nil {
		t.Fatalf("failed to update score: %v", err)
	}

	updated = pr.GetPeer("score-test")
	if updated == nil {
		t.Fatal("expected peer to exist")
	}
	if updated.Score != 100 {
		t.Errorf("expected score clamped to 100, got %f", updated.Score)
	}

	// Test clamping - below 0
	err = pr.UpdateScore("score-test", -50)
	if err != nil {
		t.Fatalf("failed to update score: %v", err)
	}

	updated = pr.GetPeer("score-test")
	if updated == nil {
		t.Fatal("expected peer to exist")
	}
	if updated.Score != 0 {
		t.Errorf("expected score clamped to 0, got %f", updated.Score)
	}
}

func TestPeerRegistry_SetConnected(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	peer := &Peer{
		ID:        "connect-test",
		Name:      "Connect Peer",
		Connected: false,
	}

	pr.AddPeer(peer)

	pr.SetConnected("connect-test", true)

	updated := pr.GetPeer("connect-test")
	if updated == nil {
		t.Fatal("expected peer to exist")
	}
	if !updated.Connected {
		t.Error("peer should be connected")
	}
	if updated.LastSeen.IsZero() {
		t.Error("LastSeen should be set when connected")
	}

	pr.SetConnected("connect-test", false)
	updated = pr.GetPeer("connect-test")
	if updated == nil {
		t.Fatal("expected peer to exist")
	}
	if updated.Connected {
		t.Error("peer should be disconnected")
	}
}

func TestPeerRegistry_GetConnectedPeers(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	peers := []*Peer{
		{ID: "conn-1", Name: "Peer 1"},
		{ID: "conn-2", Name: "Peer 2"},
		{ID: "conn-3", Name: "Peer 3"},
	}

	for _, p := range peers {
		pr.AddPeer(p)
	}

	pr.SetConnected("conn-1", true)
	pr.SetConnected("conn-3", true)

	connected := pr.GetConnectedPeers()
	if len(connected) != 2 {
		t.Errorf("expected 2 connected peers, got %d", len(connected))
	}
}

func TestPeerRegistry_SelectOptimalPeer(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	// Add peers with different metrics
	peers := []*Peer{
		{ID: "opt-1", Name: "Slow Peer", PingMS: 200, Hops: 5, GeoKM: 1000, Score: 50},
		{ID: "opt-2", Name: "Fast Peer", PingMS: 10, Hops: 1, GeoKM: 50, Score: 90},
		{ID: "opt-3", Name: "Medium Peer", PingMS: 50, Hops: 2, GeoKM: 200, Score: 70},
	}

	for _, p := range peers {
		pr.AddPeer(p)
	}

	optimal := pr.SelectOptimalPeer()
	if optimal == nil {
		t.Fatal("expected to find an optimal peer")
	}

	// The "Fast Peer" should be selected as optimal
	if optimal.ID != "opt-2" {
		t.Errorf("expected 'opt-2' (Fast Peer) to be optimal, got '%s' (%s)", optimal.ID, optimal.Name)
	}
}

func TestPeerRegistry_SelectNearestPeers(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	peers := []*Peer{
		{ID: "near-1", Name: "Peer 1", PingMS: 100, Score: 50},
		{ID: "near-2", Name: "Peer 2", PingMS: 10, Score: 90},
		{ID: "near-3", Name: "Peer 3", PingMS: 50, Score: 70},
		{ID: "near-4", Name: "Peer 4", PingMS: 200, Score: 30},
	}

	for _, p := range peers {
		pr.AddPeer(p)
	}

	nearest := pr.SelectNearestPeers(2)
	if len(nearest) != 2 {
		t.Errorf("expected 2 nearest peers, got %d", len(nearest))
	}
}

func TestPeerRegistry_Persistence(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "persist-test")
	defer os.RemoveAll(tmpDir)

	peersPath := filepath.Join(tmpDir, "peers.json")

	// Create and save
	pr1, err := NewPeerRegistryWithPath(peersPath)
	if err != nil {
		t.Fatalf("failed to create first registry: %v", err)
	}

	peer := &Peer{
		ID:      "persist-test",
		Name:    "Persistent Peer",
		Address: "7.7.7.7:9091",
		Role:    RoleDual,
		AddedAt: time.Now(),
	}

	pr1.AddPeer(peer)

	// Flush pending changes before reloading
	if err := pr1.Close(); err != nil {
		t.Fatalf("failed to close first registry: %v", err)
	}

	// Load in new registry from same path
	pr2, err := NewPeerRegistryWithPath(peersPath)
	if err != nil {
		t.Fatalf("failed to create second registry: %v", err)
	}

	if pr2.Count() != 1 {
		t.Errorf("expected 1 peer after reload, got %d", pr2.Count())
	}

	loaded := pr2.GetPeer("persist-test")
	if loaded == nil {
		t.Fatal("peer should exist after reload")
	}

	if loaded.Name != "Persistent Peer" {
		t.Errorf("expected name 'Persistent Peer', got '%s'", loaded.Name)
	}
}

func TestPeerRegistry_AllowlistPersistence(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "allowlist-persist-test")
	defer os.RemoveAll(tmpDir)

	peersPath := filepath.Join(tmpDir, "peers.json")

	pr1, err := NewPeerRegistryWithPath(peersPath)
	if err != nil {
		t.Fatalf("failed to create first registry: %v", err)
	}

	key := "allowlist-key-1234567890"
	pr1.AllowPublicKey(key)

	if err := pr1.Close(); err != nil {
		t.Fatalf("failed to close first registry: %v", err)
	}

	pr2, err := NewPeerRegistryWithPath(peersPath)
	if err != nil {
		t.Fatalf("failed to create second registry: %v", err)
	}

	if !pr2.IsPublicKeyAllowed(key) {
		t.Fatal("expected allowlisted key to survive reload")
	}

	keys := pr2.ListAllowedPublicKeys()
	if !slices.Contains(keys, key) {
		t.Fatalf("expected allowlisted key to be listed after reload, got %v", keys)
	}
}

// --- Security Feature Tests ---

func TestPeerRegistry_AuthMode(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	// Default should be Open
	if pr.GetAuthMode() != PeerAuthOpen {
		t.Errorf("expected default auth mode to be Open, got %d", pr.GetAuthMode())
	}

	// Set to Allowlist
	pr.SetAuthMode(PeerAuthAllowlist)
	if pr.GetAuthMode() != PeerAuthAllowlist {
		t.Errorf("expected auth mode to be Allowlist after setting, got %d", pr.GetAuthMode())
	}

	// Set back to Open
	pr.SetAuthMode(PeerAuthOpen)
	if pr.GetAuthMode() != PeerAuthOpen {
		t.Errorf("expected auth mode to be Open after resetting, got %d", pr.GetAuthMode())
	}
}

func TestPeerRegistry_PublicKeyAllowlist(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	testKey := "base64PublicKeyExample1234567890123456"

	// Initially key should not be allowed
	if pr.IsPublicKeyAllowed(testKey) {
		t.Error("key should not be allowed before adding")
	}

	// Add key to allowlist
	pr.AllowPublicKey(testKey)
	if !pr.IsPublicKeyAllowed(testKey) {
		t.Error("key should be allowed after adding")
	}

	// List should contain the key
	keys := pr.ListAllowedPublicKeys()
	found := slices.Contains(keys, testKey)
	if !found {
		t.Error("ListAllowedPublicKeys should contain the added key")
	}

	// Revoke key
	pr.RevokePublicKey(testKey)
	if pr.IsPublicKeyAllowed(testKey) {
		t.Error("key should not be allowed after revoking")
	}

	// List should be empty
	keys = pr.ListAllowedPublicKeys()
	if len(keys) != 0 {
		t.Errorf("expected 0 keys after revoke, got %d", len(keys))
	}
}

func TestPeerRegistry_IsPeerAllowed_OpenMode(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	pr.SetAuthMode(PeerAuthOpen)

	// In Open mode, any peer should be allowed
	if !pr.IsPeerAllowed("unknown-peer", "unknown-key") {
		t.Error("in Open mode, all peers should be allowed")
	}

	if !pr.IsPeerAllowed("", "") {
		t.Error("in Open mode, even empty IDs should be allowed")
	}
}

func TestPeerRegistry_IsPeerAllowed_AllowlistMode(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	pr.SetAuthMode(PeerAuthAllowlist)

	// Unknown peer with unknown key should be rejected
	if pr.IsPeerAllowed("unknown-peer", "unknown-key") {
		t.Error("in Allowlist mode, unknown peers should be rejected")
	}

	// Pre-registered peer should be allowed
	peer := &Peer{
		ID:        "registered-peer",
		Name:      "Registered",
		PublicKey: "registered-key",
	}
	pr.AddPeer(peer)

	if !pr.IsPeerAllowed("registered-peer", "any-key") {
		t.Error("pre-registered peer should be allowed in Allowlist mode")
	}

	// Peer with allowlisted public key should be allowed
	pr.AllowPublicKey("allowed-key-1234567890")
	if !pr.IsPeerAllowed("new-peer", "allowed-key-1234567890") {
		t.Error("peer with allowlisted key should be allowed")
	}

	// Unknown peer with non-allowlisted key should still be rejected
	if pr.IsPeerAllowed("another-peer", "not-allowed-key") {
		t.Error("peer without allowlisted key should be rejected")
	}
}

func TestPeerRegistry_PeerNameValidation(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	testCases := []struct {
		name      string
		peerName  string
		shouldErr bool
	}{
		{"empty name allowed", "", false},
		{"single char", "A", false},
		{"simple name", "MyPeer", false},
		{"name with hyphen", "my-peer", false},
		{"name with underscore", "my_peer", false},
		{"name with space", "My Peer", false},
		{"name with numbers", "Peer123", false},
		{"max length name", "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789AB", false},
		{"too long name", "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789ABC", true},
		{"starts with hyphen", "-peer", true},
		{"ends with hyphen", "peer-", true},
		{"special chars", "peer@host", true},
		{"unicode chars", "peer\u0000name", true},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			peer := &Peer{
				ID:   "test-peer-" + string(rune('A'+i)),
				Name: tc.peerName,
			}
			err := pr.AddPeer(peer)
			if tc.shouldErr && err == nil {
				t.Errorf("expected error for name '%s' but got none", tc.peerName)
			} else if !tc.shouldErr && err != nil {
				t.Errorf("unexpected error for name '%s': %v", tc.peerName, err)
			}
			// Clean up for next test
			if err == nil {
				pr.RemovePeer(peer.ID)
			}
		})
	}
}

func TestPeerRegistry_ScoreRecording(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	peer := &Peer{
		ID:    "score-record-test",
		Name:  "Score Peer",
		Score: 50, // Start at neutral
	}
	pr.AddPeer(peer)

	// Record successes - score should increase
	for range 5 {
		pr.RecordSuccess("score-record-test")
	}
	updated := pr.GetPeer("score-record-test")
	if updated.Score <= 50 {
		t.Errorf("score should increase after successes, got %f", updated.Score)
	}

	// Record failures - score should decrease
	initialScore := updated.Score
	for range 3 {
		pr.RecordFailure("score-record-test")
	}
	updated = pr.GetPeer("score-record-test")
	if updated.Score >= initialScore {
		t.Errorf("score should decrease after failures, got %f (was %f)", updated.Score, initialScore)
	}

	// Record timeouts - score should decrease
	initialScore = updated.Score
	pr.RecordTimeout("score-record-test")
	updated = pr.GetPeer("score-record-test")
	if updated.Score >= initialScore {
		t.Errorf("score should decrease after timeout, got %f (was %f)", updated.Score, initialScore)
	}

	// Score should be clamped to min/max
	for range 100 {
		pr.RecordSuccess("score-record-test")
	}
	updated = pr.GetPeer("score-record-test")
	if updated.Score > ScoreMaximum {
		t.Errorf("score should be clamped to max %f, got %f", ScoreMaximum, updated.Score)
	}

	for range 100 {
		pr.RecordFailure("score-record-test")
	}
	updated = pr.GetPeer("score-record-test")
	if updated.Score < ScoreMinimum {
		t.Errorf("score should be clamped to min %f, got %f", ScoreMinimum, updated.Score)
	}
}

func TestPeerRegistry_GetPeersByScore(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	// Add peers with different scores
	peers := []*Peer{
		{ID: "low-score", Name: "Low", Score: 20},
		{ID: "high-score", Name: "High", Score: 90},
		{ID: "mid-score", Name: "Mid", Score: 50},
	}

	for _, p := range peers {
		pr.AddPeer(p)
	}

	sorted := pr.GetPeersByScore()
	if len(sorted) != 3 {
		t.Fatalf("expected 3 peers, got %d", len(sorted))
	}

	// Should be sorted by score descending
	if sorted[0].ID != "high-score" {
		t.Errorf("first peer should be high-score, got %s", sorted[0].ID)
	}
	if sorted[1].ID != "mid-score" {
		t.Errorf("second peer should be mid-score, got %s", sorted[1].ID)
	}
	if sorted[2].ID != "low-score" {
		t.Errorf("third peer should be low-score, got %s", sorted[2].ID)
	}

	// Returned peers should be copies, not live pointers into the registry.
	sorted[0].Name = "mutated"
	if original := pr.GetPeer("high-score"); original == nil || original.Name != "High" {
		t.Fatalf("registry peer should not be mutated through GetPeersByScore copy")
	}
}

func TestPeerRegistry_OptimalPeerRebuildsAfterScoreChange(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	peers := []*Peer{
		{ID: "peer-a", Name: "Peer A", PingMS: 10, Hops: 1, GeoKM: 5, Score: 80},
		{ID: "peer-b", Name: "Peer B", PingMS: 10, Hops: 1, GeoKM: 5, Score: 90},
	}

	for _, p := range peers {
		if err := pr.AddPeer(p); err != nil {
			t.Fatalf("failed to add peer %s: %v", p.ID, err)
		}
	}

	if got := pr.SelectOptimalPeer(); got == nil || got.ID != "peer-b" {
		t.Fatalf("expected peer-b to be optimal initially, got %#v", got)
	}

	for range 5 {
		pr.RecordFailure("peer-b")
	}

	if got := pr.SelectOptimalPeer(); got == nil || got.ID != "peer-a" {
		t.Fatalf("expected peer-a to become optimal after score changes, got %#v", got)
	}
}

// --- Additional coverage tests for peer.go ---

func TestSafeKeyPrefix(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{"long key", "abcdefghijklmnopqrstuvwxyz", "abcdefghijklmnop..."},
		{"exactly 16", "1234567890123456", "1234567890123456..."},
		{"short key", "abc", "abc"},
		{"single char", "x", "x"},
		{"empty key", "", "(empty)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := safeKeyPrefix(tt.key)
			if result != tt.expected {
				t.Errorf("safeKeyPrefix(%q) = %q, want %q", tt.key, result, tt.expected)
			}
		})
	}
}

func TestValidatePeerName(t *testing.T) {
	tests := []struct {
		name      string
		peerName  string
		shouldErr bool
	}{
		{"empty allowed", "", false},
		{"single alphanumeric", "A", false},
		{"simple alphanumeric", "TestPeer", false},
		{"with hyphens", "test-peer", false},
		{"with underscores", "test_peer", false},
		{"with spaces", "Test Peer", false},
		{"max length", "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789AB", false},
		{"too long", "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789ABC", true},
		{"starts with hyphen", "-peer", true},
		{"ends with space", "peer ", true},
		{"special chars", "peer@host", true},
		{"starts with space", " peer", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePeerName(tt.peerName)
			if tt.shouldErr && err == nil {
				t.Errorf("expected error for name %q", tt.peerName)
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("unexpected error for name %q: %v", tt.peerName, err)
			}
		})
	}
}

func TestPeerRegistry_AddPeer_EmptyID(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	peer := &Peer{ID: "", Name: "no-id"}
	err := pr.AddPeer(peer)
	if err == nil {
		t.Error("expected error for empty peer ID")
	}
}

func TestPeerRegistry_UpdatePeer(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	// UpdatePeer for non-existent peer
	err := pr.UpdatePeer(&Peer{ID: "non-existent"})
	if err == nil {
		t.Error("expected error when updating non-existent peer")
	}

	// Add then update
	peer := &Peer{ID: "update-test", Name: "Original", Score: 50}
	pr.AddPeer(peer)

	peer.Name = "Updated"
	peer.Score = 80
	err = pr.UpdatePeer(peer)
	if err != nil {
		t.Fatalf("failed to update peer: %v", err)
	}

	updated := pr.GetPeer("update-test")
	if updated == nil {
		t.Fatal("expected peer to exist after update")
	}
	if updated.Name != "Updated" {
		t.Errorf("expected name 'Updated', got '%s'", updated.Name)
	}
	if updated.Score != 80 {
		t.Errorf("expected score 80, got %f", updated.Score)
	}
}

func TestPeerRegistry_UpdateMetrics_NotFound(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	err := pr.UpdateMetrics("ghost", 10.0, 100.0, 1)
	if err == nil {
		t.Error("expected error updating metrics for non-existent peer")
	}
}

func TestPeerRegistry_UpdateScore_NotFound(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	err := pr.UpdateScore("ghost", 75.0)
	if err == nil {
		t.Error("expected error updating score for non-existent peer")
	}
}

func TestPeerRegistry_RecordSuccess_NotFound(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	// Should not panic for non-existent peer
	pr.RecordSuccess("ghost-peer")
}

func TestPeerRegistry_RecordFailure_NotFound(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	pr.RecordFailure("ghost-peer")
}

func TestPeerRegistry_RecordTimeout_NotFound(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	pr.RecordTimeout("ghost-peer")
}

func TestPeerRegistry_SelectOptimalPeer_EmptyRegistry(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	optimal := pr.SelectOptimalPeer()
	if optimal != nil {
		t.Error("expected nil for empty registry")
	}
}

func TestPeerRegistry_SelectNearestPeers_EmptyRegistry(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	nearest := pr.SelectNearestPeers(5)
	if nearest != nil {
		t.Error("expected nil for empty registry")
	}
}

func TestPeerRegistry_SetConnected_NonExistent(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	// Should not panic for non-existent peer
	pr.SetConnected("ghost-peer", true)
}

func TestPeerRegistry_Close_NoDirtyData(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	// Close without any changes should succeed
	err := pr.Close()
	if err != nil {
		t.Errorf("Close with no dirty data should not error: %v", err)
	}
}

func TestPeerRegistry_Close_WithDirtyData(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "close-dirty-test")
	defer os.RemoveAll(tmpDir)

	peersPath := filepath.Join(tmpDir, "peers.json")
	pr, err := NewPeerRegistryWithPath(peersPath)
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}

	// Add a peer which triggers scheduleSave (dirty flag)
	pr.AddPeer(&Peer{ID: "dirty-peer", Name: "Dirty"})

	// Close should flush dirty data
	err = pr.Close()
	if err != nil {
		t.Errorf("Close should not error: %v", err)
	}

	// Verify data was saved
	pr2, err := NewPeerRegistryWithPath(peersPath)
	if err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	if pr2.Count() != 1 {
		t.Errorf("expected 1 peer after close+reload, got %d", pr2.Count())
	}
}

func TestPeerRegistry_ScheduleSave_Debounce(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "debounce-test")
	defer os.RemoveAll(tmpDir)

	peersPath := filepath.Join(tmpDir, "peers.json")
	pr, err := NewPeerRegistryWithPath(peersPath)
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}

	// Multiple rapid saves should be debounced
	for range 10 {
		pr.scheduleSave()
	}

	// Close should flush
	err = pr.Close()
	if err != nil {
		t.Errorf("Close should not error: %v", err)
	}
}

func TestPeerRegistry_SaveNow(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "savenow-test")
	defer os.RemoveAll(tmpDir)

	peersPath := filepath.Join(tmpDir, "subdir", "peers.json")
	pr, err := NewPeerRegistryWithPath(peersPath)
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}

	pr.AddPeer(&Peer{ID: "save-test", Name: "SaveTest"})

	// Direct saveNow call
	pr.mu.RLock()
	err = pr.saveNow()
	pr.mu.RUnlock()
	if err != nil {
		t.Fatalf("saveNow failed: %v", err)
	}

	// Verify the file was written
	if _, err := os.Stat(peersPath); os.IsNotExist(err) {
		t.Error("peers.json should exist after saveNow")
	}
}

func TestPeerRegistry_ScheduleSave_TimerFires(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping debounce timer test in short mode")
	}

	tmpDir, _ := os.MkdirTemp("", "timer-fire-test")
	defer os.RemoveAll(tmpDir)

	peersPath := filepath.Join(tmpDir, "peers.json")
	pr, err := NewPeerRegistryWithPath(peersPath)
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}

	pr.AddPeer(&Peer{ID: "timer-peer", Name: "Timer"})

	// Wait for the debounce interval to fire (5s + buffer)
	time.Sleep(6 * time.Second)

	// The file should have been saved by the timer
	if _, err := os.Stat(peersPath); os.IsNotExist(err) {
		t.Error("peers.json should exist after debounce timer fires")
	}

	// Reload and verify
	pr2, err := NewPeerRegistryWithPath(peersPath)
	if err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	if pr2.Count() != 1 {
		t.Errorf("expected 1 peer after timer save, got %d", pr2.Count())
	}

	pr.Close()
}

// TestPeerRegistry_MarkSeen_Good verifies that MarkSeen refreshes LastSeen for
// an existing peer. This covers the happy path of RFC §3.3's documented
// registry.MarkSeen(peerID) behaviour.
func TestPeerRegistry_MarkSeen_Good(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	peer := &Peer{ID: "seen-peer", Name: "Seen"}
	if err := pr.AddPeer(peer); err != nil {
		t.Fatalf("add peer: %v", err)
	}

	// Rewind LastSeen under the registry lock so MarkSeen's timestamp update
	// is observable.
	pr.mu.Lock()
	pr.peers["seen-peer"].LastSeen = time.Time{}
	pr.mu.Unlock()

	before := time.Now()
	pr.MarkSeen("seen-peer")
	after := time.Now()

	updated := pr.GetPeer("seen-peer")
	if updated == nil {
		t.Fatal("peer should still exist after MarkSeen")
	}
	if updated.LastSeen.IsZero() {
		t.Fatal("LastSeen should be populated after MarkSeen")
	}
	if updated.LastSeen.Before(before) || updated.LastSeen.After(after) {
		t.Errorf("LastSeen %v not in [%v, %v]", updated.LastSeen, before, after)
	}
}

// TestPeerRegistry_MarkSeen_Bad verifies MarkSeen is a safe no-op for peers
// that are not registered.
func TestPeerRegistry_MarkSeen_Bad(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	// Must not panic or mutate any state.
	pr.MarkSeen("never-added")

	if pr.Count() != 0 {
		t.Errorf("registry should still be empty, got %d peers", pr.Count())
	}
}

// TestPeerRegistry_FindNearby_Good covers the RFC §3.3 documented
// FindNearby(lat, lon, hopCount, maxResults) operation when coordinates are
// populated.
func TestPeerRegistry_FindNearby_Good(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	// Three peers at varying coordinates and hop counts. peer-near is the
	// closest match to (0, 0, 1).
	peers := []*Peer{
		{ID: "peer-far", Name: "Far", Latitude: 40.0, Longitude: 40.0, Hops: 8},
		{ID: "peer-near", Name: "Near", Latitude: 0.5, Longitude: 0.5, Hops: 1},
		{ID: "peer-mid", Name: "Mid", Latitude: 5.0, Longitude: 5.0, Hops: 3},
	}
	for _, p := range peers {
		if err := pr.AddPeer(p); err != nil {
			t.Fatalf("add peer %s: %v", p.ID, err)
		}
	}

	results, err := pr.FindNearby(0, 0, 1, 2)
	if err != nil {
		t.Fatalf("FindNearby: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != "peer-near" {
		t.Errorf("closest peer should be peer-near, got %s", results[0].ID)
	}
	if results[1].ID != "peer-mid" {
		t.Errorf("second closest should be peer-mid, got %s", results[1].ID)
	}

	// Copy semantics: mutating a returned peer must not mutate the registry.
	results[0].Name = "mutated"
	original := pr.GetPeer("peer-near")
	if original == nil || original.Name != "Near" {
		t.Errorf("registry peer should not be mutated through FindNearby copy")
	}
}

// TestPeerRegistry_FindNearby_Bad verifies FindNearby's edge cases: empty
// registry, non-positive maxResults, and fallback to GeoKM when coordinates
// are absent.
func TestPeerRegistry_FindNearby_Bad(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	// Empty registry returns an empty slice, no error.
	results, err := pr.FindNearby(0, 0, 0, 5)
	if err != nil {
		t.Fatalf("empty registry should not error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("empty registry should return no results, got %d", len(results))
	}

	// Non-positive maxResults returns an empty slice, no error.
	if err := pr.AddPeer(&Peer{ID: "peer-a"}); err != nil {
		t.Fatalf("add peer: %v", err)
	}
	results, err = pr.FindNearby(0, 0, 0, 0)
	if err != nil {
		t.Fatalf("maxResults=0 should not error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("maxResults=0 should return no results, got %d", len(results))
	}

	// Peers without coordinates fall back to GeoKM distance. Compared to a
	// distant peer, the closer GeoKM wins.
	prFallback, cleanupFallback := setupTestPeerRegistry(t)
	defer cleanupFallback()
	if err := prFallback.AddPeer(&Peer{ID: "peer-close", GeoKM: 5}); err != nil {
		t.Fatalf("add peer-close: %v", err)
	}
	if err := prFallback.AddPeer(&Peer{ID: "peer-distant", GeoKM: 500}); err != nil {
		t.Fatalf("add peer-distant: %v", err)
	}
	results, err = prFallback.FindNearby(0, 0, 0, 2)
	if err != nil {
		t.Fatalf("FindNearby: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != "peer-close" {
		t.Errorf("closest fallback peer should be peer-close, got %s", results[0].ID)
	}
}

// TestPeerRegistry_PeersByScore_Good verifies that the iterator returned by
// PeersByScore emits peers in descending score order and yields copies rather
// than live pointers.
func TestPeerRegistry_PeersByScore_Good(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	peers := []*Peer{
		{ID: "score-low", Score: 10},
		{ID: "score-high", Score: 95},
		{ID: "score-mid", Score: 55},
	}
	for _, p := range peers {
		if err := pr.AddPeer(p); err != nil {
			t.Fatalf("add peer %s: %v", p.ID, err)
		}
	}

	collected := slices.Collect(pr.PeersByScore())
	if len(collected) != 3 {
		t.Fatalf("expected 3 peers, got %d", len(collected))
	}
	if collected[0].ID != "score-high" {
		t.Errorf("first peer should be score-high, got %s", collected[0].ID)
	}
	if collected[2].ID != "score-low" {
		t.Errorf("last peer should be score-low, got %s", collected[2].ID)
	}

	// Mutating the copy must not reach the registry.
	collected[0].Score = 0
	registry := pr.GetPeer("score-high")
	if registry == nil || registry.Score != 95 {
		t.Errorf("registry score should remain 95 after mutation, got %v", registry)
	}
}

// TestPeerRegistry_PeersByScore_Bad covers early-termination: stopping the
// iterator must not deadlock or yield more peers than requested.
func TestPeerRegistry_PeersByScore_Bad(t *testing.T) {
	pr, cleanup := setupTestPeerRegistry(t)
	defer cleanup()

	for i, score := range []float64{10, 20, 30, 40} {
		p := &Peer{ID: "score-" + string(rune('a'+i)), Score: score}
		if err := pr.AddPeer(p); err != nil {
			t.Fatalf("add peer %s: %v", p.ID, err)
		}
	}

	var count int
	for range pr.PeersByScore() {
		count++
		if count == 2 {
			break
		}
	}
	if count != 2 {
		t.Errorf("expected iterator to stop at 2, got %d", count)
	}
}
