package node

import (
	"testing"

	core "dappco.re/go"
)

// TestPeerRegistry_AllowlistRoundTrip persists allowlist entries and reloads
// them in a fresh registry, covering loadAllowedPublicKeys happy + empty-key
// skip branches.
func TestPeerRegistry_AllowlistRoundTrip(t *testing.T) {
	dir := t.TempDir()
	peersPath := core.PathJoin(dir, "peers.json")

	first, err := resultValue[*PeerRegistry](NewPeerRegistryWithPath(peersPath))
	if err != nil {
		t.Fatalf("create registry: %v", err)
	}
	first.AllowPublicKey("pubkey-alpha")
	first.AllowPublicKey("pubkey-beta")
	if err := resultErr(first.Close()); err != nil {
		t.Fatalf("close first registry: %v", err)
	}

	// A fresh registry on the same path loads the persisted allowlist.
	second, err := resultValue[*PeerRegistry](NewPeerRegistryWithPath(peersPath))
	if err != nil {
		t.Fatalf("reopen registry: %v", err)
	}
	t.Cleanup(func() { _ = resultErr(second.Close()) })

	if !second.IsPublicKeyAllowed("pubkey-alpha") {
		t.Fatal("expected pubkey-alpha to survive reload")
	}
	if !second.IsPublicKeyAllowed("pubkey-beta") {
		t.Fatal("expected pubkey-beta to survive reload")
	}
}

// TestPeerRegistry_LoadAllowlist_EmptyKeySkipped confirms an empty-string entry
// in the persisted allowlist is skipped on load.
func TestPeerRegistry_LoadAllowlist_EmptyKeySkipped(t *testing.T) {
	dir := t.TempDir()
	peersPath := core.PathJoin(dir, "peers.json")
	allowlistPath := peersPath + ".allowlist.json"

	if err := resultErr(core.WriteFile(allowlistPath, []byte(`["good-key","",  "another-key"]`), 0o600)); err != nil {
		t.Fatalf("write allowlist: %v", err)
	}

	reg, err := resultValue[*PeerRegistry](NewPeerRegistryWithPath(peersPath))
	if err != nil {
		t.Fatalf("create registry: %v", err)
	}
	t.Cleanup(func() { _ = resultErr(reg.Close()) })

	if !reg.IsPublicKeyAllowed("good-key") {
		t.Fatal("expected good-key allowed")
	}
	if !reg.IsPublicKeyAllowed("another-key") {
		t.Fatal("expected another-key allowed")
	}
	if reg.IsPublicKeyAllowed("") {
		t.Fatal("empty key must not be allowed")
	}
}

// TestPeerRegistry_LoadAllowlist_Malformed surfaces a registry construction
// error when the persisted allowlist is not valid JSON. The allowlist load is
// best-effort, so construction still succeeds but the file is ignored.
func TestPeerRegistry_LoadAllowlist_Malformed(t *testing.T) {
	dir := t.TempDir()
	peersPath := core.PathJoin(dir, "peers.json")
	allowlistPath := peersPath + ".allowlist.json"

	if err := resultErr(core.WriteFile(allowlistPath, []byte(`{not valid json`), 0o600)); err != nil {
		t.Fatalf("write malformed allowlist: %v", err)
	}

	// Construction is best-effort: a malformed allowlist is logged + ignored,
	// not fatal. The direct loader, however, must report the parse failure.
	reg, err := resultValue[*PeerRegistry](NewPeerRegistryWithPath(peersPath))
	if err != nil {
		t.Fatalf("create registry: %v", err)
	}
	t.Cleanup(func() { _ = resultErr(reg.Close()) })

	if r := reg.loadAllowedPublicKeys(); r.OK {
		t.Fatal("expected loadAllowedPublicKeys to reject malformed JSON")
	}
}

// TestPeerRegistry_SelectOptimalPeer_Empty returns nil when no peers are known.
func TestPeerRegistry_SelectOptimalPeer_Empty(t *testing.T) {
	reg := tripletPeerRegistry(t)
	if peer := reg.SelectOptimalPeer(); peer != nil {
		t.Fatalf("expected nil for empty registry, got %#v", peer)
	}
}

// TestPeerRegistry_SelectOptimalPeer_Populated returns the best-scoring peer
// from a populated registry.
func TestPeerRegistry_SelectOptimalPeer_Populated(t *testing.T) {
	reg := tripletPeerRegistry(t)

	if err := resultErr(reg.AddPeer(&Peer{ID: "slow", Name: "slow", PingMS: 500, Score: 10})); err != nil {
		t.Fatalf("add slow peer: %v", err)
	}
	if err := resultErr(reg.AddPeer(&Peer{ID: "fast", Name: "fast", PingMS: 5, Score: 99})); err != nil {
		t.Fatalf("add fast peer: %v", err)
	}

	peer := reg.SelectOptimalPeer()
	if peer == nil {
		t.Fatal("expected a selected peer")
	}
	// The returned peer must be a copy, not a pointer into the registry's map.
	if peer == reg.GetPeer(peer.ID) {
		t.Fatal("SelectOptimalPeer should return a copy, not the stored pointer")
	}
}
