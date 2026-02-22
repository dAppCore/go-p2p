package node

import (
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/Snider/Borg/pkg/smsg"
)

// BenchmarkIdentityGenerate measures Ed25519/X25519 keypair generation and
// identity derivation (SHA-256 hash of public key).
func BenchmarkIdentityGenerate(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		dir := b.TempDir()
		nm, err := NewNodeManagerWithPaths(
			filepath.Join(dir, "private.key"),
			filepath.Join(dir, "node.json"),
		)
		if err != nil {
			b.Fatalf("create node manager: %v", err)
		}
		if err := nm.GenerateIdentity("bench-node", RoleDual); err != nil {
			b.Fatalf("generate identity: %v", err)
		}
	}
}

// BenchmarkDeriveSharedSecret measures X25519 ECDH + SHA-256 key derivation.
func BenchmarkDeriveSharedSecret(b *testing.B) {
	dir1 := b.TempDir()
	dir2 := b.TempDir()

	nm1, _ := NewNodeManagerWithPaths(filepath.Join(dir1, "k"), filepath.Join(dir1, "n"))
	nm1.GenerateIdentity("node1", RoleDual)

	nm2, _ := NewNodeManagerWithPaths(filepath.Join(dir2, "k"), filepath.Join(dir2, "n"))
	nm2.GenerateIdentity("node2", RoleDual)

	peerPubKey := nm2.GetIdentity().PublicKey

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, err := nm1.DeriveSharedSecret(peerPubKey)
		if err != nil {
			b.Fatalf("derive shared secret: %v", err)
		}
	}
}

// BenchmarkMessageSerialise measures Message creation + JSON marshalling.
func BenchmarkMessageSerialise(b *testing.B) {
	payload := StatsPayload{
		NodeID:   "bench-node-id-1234567890abcdef",
		NodeName: "bench-node",
		Miners: []MinerStatsItem{
			{
				Name:      "xmrig-0",
				Type:      "xmrig",
				Hashrate:  1234.56,
				Shares:    1000,
				Rejected:  5,
				Uptime:    86400,
				Pool:      "pool.example.com:3333",
				Algorithm: "rx/0",
			},
		},
		Uptime: 172800,
	}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		msg, err := NewMessage(MsgStats, "sender-id", "receiver-id", payload)
		if err != nil {
			b.Fatalf("create message: %v", err)
		}

		data, err := MarshalJSON(msg)
		if err != nil {
			b.Fatalf("marshal message: %v", err)
		}

		var restored Message
		if err := json.Unmarshal(data, &restored); err != nil {
			b.Fatalf("unmarshal message: %v", err)
		}
	}
}

// BenchmarkMessageCreateOnly measures just Message struct creation (no marshal).
func BenchmarkMessageCreateOnly(b *testing.B) {
	payload := PingPayload{SentAt: time.Now().UnixMilli()}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_, err := NewMessage(MsgPing, "sender", "receiver", payload)
		if err != nil {
			b.Fatalf("create message: %v", err)
		}
	}
}

// BenchmarkMarshalJSON measures the pooled JSON encoder against stdlib.
func BenchmarkMarshalJSON(b *testing.B) {
	data := map[string]any{
		"id":        "test-id-1234",
		"type":      "stats",
		"from":      "node-a",
		"to":        "node-b",
		"timestamp": time.Now(),
		"payload": map[string]any{
			"hashrate": 1234.56,
			"shares":   1000,
		},
	}

	b.Run("Pooled", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_, err := MarshalJSON(data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Stdlib", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_, err := json.Marshal(data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkSMSGEncryptDecrypt measures full SMSG encrypt + decrypt cycle.
func BenchmarkSMSGEncryptDecrypt(b *testing.B) {
	// Set up two nodes for shared secret derivation
	dir1 := b.TempDir()
	dir2 := b.TempDir()

	nm1, _ := NewNodeManagerWithPaths(filepath.Join(dir1, "k"), filepath.Join(dir1, "n"))
	nm1.GenerateIdentity("node1", RoleDual)

	nm2, _ := NewNodeManagerWithPaths(filepath.Join(dir2, "k"), filepath.Join(dir2, "n"))
	nm2.GenerateIdentity("node2", RoleDual)

	sharedSecret, _ := nm1.DeriveSharedSecret(nm2.GetIdentity().PublicKey)
	password := base64.StdEncoding.EncodeToString(sharedSecret)

	// Prepare a message to encrypt
	plaintext := `{"id":"bench-msg","type":"ping","from":"node1","to":"node2","ts":"2026-02-20T00:00:00Z","payload":{"sentAt":1740000000000}}`

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		msg := smsg.NewMessage(plaintext)
		encrypted, err := smsg.Encrypt(msg, password)
		if err != nil {
			b.Fatalf("encrypt: %v", err)
		}

		_, err = smsg.Decrypt(encrypted, password)
		if err != nil {
			b.Fatalf("decrypt: %v", err)
		}
	}
}

// BenchmarkChallengeSignVerify measures the HMAC challenge-response cycle.
func BenchmarkChallengeSignVerify(b *testing.B) {
	challenge, _ := GenerateChallenge()
	sharedSecret := make([]byte, 32)
	// Use a deterministic secret for reproducibility
	for i := range sharedSecret {
		sharedSecret[i] = byte(i)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		sig := SignChallenge(challenge, sharedSecret)
		if !VerifyChallenge(challenge, sig, sharedSecret) {
			b.Fatal("verification failed")
		}
	}
}

// BenchmarkPeerScoring measures KD-tree rebuild and peer selection.
func BenchmarkPeerScoring(b *testing.B) {
	dir := b.TempDir()
	reg, err := NewPeerRegistryWithPath(filepath.Join(dir, "peers.json"))
	if err != nil {
		b.Fatalf("create registry: %v", err)
	}
	defer reg.Close()

	// Add 50 peers with varied metrics
	for i := range 50 {
		peer := &Peer{
			ID:      filepath.Join("peer", string(rune('A'+i%26)), string(rune('0'+i/26))),
			Name:    "peer",
			PingMS:  float64(i*10 + 5),
			Hops:    i%5 + 1,
			GeoKM:   float64(i * 100),
			Score:   float64(50 + i%50),
			AddedAt: time.Now(),
		}
		// Bypass AddPeer's duplicate check by adding directly
		reg.mu.Lock()
		reg.peers[peer.ID] = peer
		reg.mu.Unlock()
	}
	// Rebuild KD-tree once with all peers
	reg.mu.Lock()
	reg.rebuildKDTree()
	reg.mu.Unlock()

	b.Run("SelectOptimalPeer", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			peer := reg.SelectOptimalPeer()
			if peer == nil {
				b.Fatal("SelectOptimalPeer returned nil")
			}
		}
	})

	b.Run("SelectNearestPeers_5", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			peers := reg.SelectNearestPeers(5)
			if len(peers) == 0 {
				b.Fatal("SelectNearestPeers returned empty")
			}
		}
	})

	b.Run("RebuildKDTree_50peers", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			reg.mu.Lock()
			reg.rebuildKDTree()
			reg.mu.Unlock()
		}
	})
}

// BenchmarkBufPool measures buffer pool get/put throughput.
func BenchmarkBufPool(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		buf := getBuffer()
		buf.WriteString(`{"type":"ping","from":"node-a","to":"node-b"}`)
		putBuffer(buf)
	}
}

// BenchmarkGenerateChallenge measures random challenge generation.
func BenchmarkGenerateChallenge(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, err := GenerateChallenge()
		if err != nil {
			b.Fatal(err)
		}
	}
}
