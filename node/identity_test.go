package node

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// setupTestNodeManager creates a NodeManager with paths in a temp directory.
func setupTestNodeManager(t *testing.T) (*NodeManager, func()) {
	tmpDir, err := os.MkdirTemp("", "node-identity-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	keyPath := filepath.Join(tmpDir, "private.key")
	configPath := filepath.Join(tmpDir, "node.json")

	nm, err := NewNodeManagerWithPaths(keyPath, configPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create node manager: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return nm, cleanup
}

func TestNodeIdentity(t *testing.T) {
	t.Run("NewNodeManager", func(t *testing.T) {
		nm, cleanup := setupTestNodeManager(t)
		defer cleanup()

		if nm.HasIdentity() {
			t.Error("new node manager should not have identity")
		}
	})

	t.Run("GenerateIdentity", func(t *testing.T) {
		nm, cleanup := setupTestNodeManager(t)
		defer cleanup()

		err := nm.GenerateIdentity("test-node", RoleDual)
		if err != nil {
			t.Fatalf("failed to generate identity: %v", err)
		}

		if !nm.HasIdentity() {
			t.Error("node manager should have identity after generation")
		}

		identity := nm.GetIdentity()
		if identity == nil {
			t.Fatal("identity should not be nil")
		}

		if identity.Name != "test-node" {
			t.Errorf("expected name 'test-node', got '%s'", identity.Name)
		}

		if identity.Role != RoleDual {
			t.Errorf("expected role Dual, got '%s'", identity.Role)
		}

		if identity.ID == "" {
			t.Error("identity ID should not be empty")
		}

		if identity.PublicKey == "" {
			t.Error("public key should not be empty")
		}
	})

	t.Run("PrivateKeyPermissions", func(t *testing.T) {
		nm, cleanup := setupTestNodeManager(t)
		defer cleanup()

		err := nm.GenerateIdentity("permission-test", RoleDual)
		if err != nil {
			t.Fatalf("failed to generate identity: %v", err)
		}

		info, err := os.Stat(nm.keyPath)
		if err != nil {
			t.Fatalf("failed to stat private key: %v", err)
		}

		if got := info.Mode().Perm(); got != 0600 {
			t.Fatalf("expected private key permissions 0600, got %04o", got)
		}
	})

	t.Run("LoadExistingIdentity", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "node-load-test")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		keyPath := filepath.Join(tmpDir, "private.key")
		configPath := filepath.Join(tmpDir, "node.json")

		// First, create an identity
		nm1, err := NewNodeManagerWithPaths(keyPath, configPath)
		if err != nil {
			t.Fatalf("failed to create first node manager: %v", err)
		}

		err = nm1.GenerateIdentity("persistent-node", RoleWorker)
		if err != nil {
			t.Fatalf("failed to generate identity: %v", err)
		}

		originalID := nm1.GetIdentity().ID
		originalPubKey := nm1.GetIdentity().PublicKey

		// Create a new manager - should load existing identity
		nm2, err := NewNodeManagerWithPaths(keyPath, configPath)
		if err != nil {
			t.Fatalf("failed to create second node manager: %v", err)
		}

		if !nm2.HasIdentity() {
			t.Error("second node manager should have loaded existing identity")
		}

		identity := nm2.GetIdentity()
		if identity.ID != originalID {
			t.Errorf("expected ID '%s', got '%s'", originalID, identity.ID)
		}

		if identity.PublicKey != originalPubKey {
			t.Error("public key mismatch after reload")
		}
	})

	t.Run("DeriveSharedSecret", func(t *testing.T) {
		// Create two node managers with separate temp directories
		tmpDir1, _ := os.MkdirTemp("", "node1")
		tmpDir2, _ := os.MkdirTemp("", "node2")
		defer os.RemoveAll(tmpDir1)
		defer os.RemoveAll(tmpDir2)

		// Node 1
		nm1, err := NewNodeManagerWithPaths(
			filepath.Join(tmpDir1, "private.key"),
			filepath.Join(tmpDir1, "node.json"),
		)
		if err != nil {
			t.Fatalf("failed to create node manager 1: %v", err)
		}
		err = nm1.GenerateIdentity("node1", RoleDual)
		if err != nil {
			t.Fatalf("failed to generate identity 1: %v", err)
		}

		// Node 2
		nm2, err := NewNodeManagerWithPaths(
			filepath.Join(tmpDir2, "private.key"),
			filepath.Join(tmpDir2, "node.json"),
		)
		if err != nil {
			t.Fatalf("failed to create node manager 2: %v", err)
		}
		err = nm2.GenerateIdentity("node2", RoleDual)
		if err != nil {
			t.Fatalf("failed to generate identity 2: %v", err)
		}

		// Derive shared secrets - should be identical
		secret1, err := nm1.DeriveSharedSecret(nm2.GetIdentity().PublicKey)
		if err != nil {
			t.Fatalf("failed to derive shared secret from node 1: %v", err)
		}

		secret2, err := nm2.DeriveSharedSecret(nm1.GetIdentity().PublicKey)
		if err != nil {
			t.Fatalf("failed to derive shared secret from node 2: %v", err)
		}

		if len(secret1) != len(secret2) {
			t.Errorf("shared secrets have different lengths: %d vs %d", len(secret1), len(secret2))
		}

		for i := range secret1 {
			if secret1[i] != secret2[i] {
				t.Error("shared secrets do not match")
				break
			}
		}
	})

	t.Run("DeleteIdentity", func(t *testing.T) {
		nm, cleanup := setupTestNodeManager(t)
		defer cleanup()

		err := nm.GenerateIdentity("delete-me", RoleDual)
		if err != nil {
			t.Fatalf("failed to generate identity: %v", err)
		}

		if !nm.HasIdentity() {
			t.Error("should have identity before delete")
		}

		err = nm.Delete()
		if err != nil {
			t.Fatalf("failed to delete identity: %v", err)
		}

		if nm.HasIdentity() {
			t.Error("should not have identity after delete")
		}
	})

	t.Run("LoadOrCreateIdentityWithPaths", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "node-load-or-create-test")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		keyPath := filepath.Join(tmpDir, "private.key")
		configPath := filepath.Join(tmpDir, "node.json")

		nm, err := LoadOrCreateIdentityWithPaths(keyPath, configPath)
		if err != nil {
			t.Fatalf("failed to load or create identity: %v", err)
		}

		if !nm.HasIdentity() {
			t.Fatal("expected identity to be initialised")
		}

		identity := nm.GetIdentity()
		if identity == nil {
			t.Fatal("identity should not be nil")
		}

		if identity.Name == "" {
			t.Error("identity name should be populated")
		}

		if identity.Role != RoleDual {
			t.Errorf("expected default role dual, got %s", identity.Role)
		}

		if _, err := os.Stat(keyPath); err != nil {
			t.Fatalf("expected private key to be persisted: %v", err)
		}

		if _, err := os.Stat(configPath); err != nil {
			t.Fatalf("expected identity config to be persisted: %v", err)
		}
	})
}

func TestIdentity_GenerateChallenge_Good(t *testing.T) {
	challenge, err := GenerateChallenge()
	if err != nil {
		t.Fatalf("GenerateChallenge: %v", err)
	}
	if len(challenge) != ChallengeSize {
		t.Fatalf("challenge length: got %d", len(challenge))
	}
}

func TestIdentity_GenerateChallenge_Bad(t *testing.T) {
	challenge, err := GenerateChallenge()
	if err != nil {
		t.Fatalf("GenerateChallenge: %v", err)
	}
	if challenge == nil {
		t.Fatal("challenge should not be nil")
	}
}

func TestIdentity_GenerateChallenge_Ugly(t *testing.T) {
	first, err := GenerateChallenge()
	if err != nil {
		t.Fatalf("GenerateChallenge first: %v", err)
	}
	second, err := GenerateChallenge()
	if err != nil {
		t.Fatalf("GenerateChallenge second: %v", err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("two random challenges unexpectedly matched")
	}
}

func TestIdentity_SignChallenge_Good(t *testing.T) {
	signature := SignChallenge([]byte("challenge"), []byte("secret"))
	if len(signature) != 32 {
		t.Fatalf("signature length: got %d", len(signature))
	}
	if !VerifyChallenge([]byte("challenge"), signature, []byte("secret")) {
		t.Fatal("signature should verify")
	}
}

func TestIdentity_SignChallenge_Bad(t *testing.T) {
	signature := SignChallenge([]byte("challenge"), nil)
	if len(signature) != 32 {
		t.Fatalf("signature length: got %d", len(signature))
	}
	if VerifyChallenge([]byte("challenge"), signature, []byte("other")) {
		t.Fatal("signature should not verify with other secret")
	}
}

func TestIdentity_SignChallenge_Ugly(t *testing.T) {
	signature := SignChallenge(nil, nil)
	if len(signature) != 32 {
		t.Fatalf("signature length: got %d", len(signature))
	}
	if !VerifyChallenge(nil, signature, nil) {
		t.Fatal("empty challenge and secret should verify against same inputs")
	}
}

func TestIdentity_VerifyChallenge_Good(t *testing.T) {
	response := SignChallenge([]byte("challenge"), []byte("secret"))
	if !VerifyChallenge([]byte("challenge"), response, []byte("secret")) {
		t.Fatal("expected challenge verification")
	}
	if len(response) == 0 {
		t.Fatal("expected response")
	}
}

func TestIdentity_VerifyChallenge_Bad(t *testing.T) {
	response := SignChallenge([]byte("challenge"), []byte("secret"))
	if VerifyChallenge([]byte("challenge"), response, []byte("wrong")) {
		t.Fatal("wrong secret should not verify")
	}
	if VerifyChallenge([]byte("other"), response, []byte("secret")) {
		t.Fatal("wrong challenge should not verify")
	}
}

func TestIdentity_VerifyChallenge_Ugly(t *testing.T) {
	if !VerifyChallenge(nil, SignChallenge(nil, nil), nil) {
		t.Fatal("nil inputs should verify when signed the same way")
	}
	if VerifyChallenge(nil, nil, nil) {
		t.Fatal("nil response should not verify")
	}
}

func TestIdentity_NewNodeManager_Good(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()
	nm, err := NewNodeManager()
	if err != nil {
		t.Fatalf("NewNodeManager: %v", err)
	}
	if nm == nil {
		t.Fatal("expected node manager")
	}
}

func TestIdentity_NewNodeManager_Bad(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()
	nm, err := NewNodeManager()
	if err != nil {
		t.Fatalf("NewNodeManager: %v", err)
	}
	if nm.keyPath == "" {
		t.Fatal("expected key path")
	}
	if nm.configPath == "" {
		t.Fatal("expected config path")
	}
}

func TestIdentity_NewNodeManager_Ugly(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()
	first, err := NewNodeManager()
	if err != nil {
		t.Fatalf("NewNodeManager first: %v", err)
	}
	second, err := NewNodeManager()
	if err != nil {
		t.Fatalf("NewNodeManager second: %v", err)
	}
	if first.configPath != second.configPath {
		t.Fatal("default config path should be stable")
	}
}

func TestIdentity_NewNodeManagerWithPaths_Good(t *testing.T) {
	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(filepath.Join(dir, "private.key"), filepath.Join(dir, "node.json"))
	if err != nil {
		t.Fatalf("NewNodeManagerWithPaths: %v", err)
	}
	if nm.keyPath == "" || nm.configPath == "" {
		t.Fatalf("paths not set: %#v", nm)
	}
}

func TestIdentity_NewNodeManagerWithPaths_Bad(t *testing.T) {
	nm, err := NewNodeManagerWithPaths("", "")
	if err != nil {
		t.Fatalf("NewNodeManagerWithPaths empty paths: %v", err)
	}
	if nm.HasIdentity() {
		t.Fatal("empty-path manager should not have identity")
	}
}

func TestIdentity_NewNodeManagerWithPaths_Ugly(t *testing.T) {
	dir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(filepath.Join(dir, "nested", "private.key"), filepath.Join(dir, "nested", "node.json"))
	if err != nil {
		t.Fatalf("NewNodeManagerWithPaths nested: %v", err)
	}
	if nm.GetIdentity() != nil {
		t.Fatal("new nested manager should not have identity")
	}
}

func TestIdentity_LoadOrCreateIdentity_Good(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()
	nm, err := LoadOrCreateIdentity()
	if err != nil {
		t.Fatalf("LoadOrCreateIdentity: %v", err)
	}
	if !nm.HasIdentity() {
		t.Fatal("expected identity")
	}
}

func TestIdentity_LoadOrCreateIdentity_Bad(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()
	nm, err := LoadOrCreateIdentity()
	if err != nil {
		t.Fatalf("LoadOrCreateIdentity: %v", err)
	}
	if !nm.HasIdentity() {
		t.Fatal("expected identity")
	}
	if nm.GetIdentity().ID == "" {
		t.Fatal("expected identity ID")
	}
}

func TestIdentity_LoadOrCreateIdentity_Ugly(t *testing.T) {
	cleanup := setupTestEnv(t)
	defer cleanup()
	first, err := LoadOrCreateIdentity()
	if err != nil {
		t.Fatalf("LoadOrCreateIdentity first: %v", err)
	}
	second, err := LoadOrCreateIdentity()
	if err != nil {
		t.Fatalf("LoadOrCreateIdentity second: %v", err)
	}
	if first.GetIdentity().ID != second.GetIdentity().ID {
		t.Fatal("expected persisted identity to reload")
	}
}

func TestIdentity_LoadOrCreateIdentityWithPaths_Good(t *testing.T) {
	dir := t.TempDir()
	nm, err := LoadOrCreateIdentityWithPaths(filepath.Join(dir, "private.key"), filepath.Join(dir, "node.json"))
	if err != nil {
		t.Fatalf("LoadOrCreateIdentityWithPaths: %v", err)
	}
	if !nm.HasIdentity() {
		t.Fatal("expected identity")
	}
}

func TestIdentity_LoadOrCreateIdentityWithPaths_Bad(t *testing.T) {
	dir := t.TempDir()
	err := os.Mkdir(filepath.Join(dir, "private.key"), 0755)
	if err != nil {
		t.Fatalf("mkdir private key path: %v", err)
	}
	nm, err := LoadOrCreateIdentityWithPaths(filepath.Join(dir, "private.key"), filepath.Join(dir, "node.json"))
	if err == nil {
		t.Fatal("expected write error")
	}
	if nm != nil {
		t.Fatalf("manager: got %#v, want nil", nm)
	}
}

func TestIdentity_LoadOrCreateIdentityWithPaths_Ugly(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "private.key")
	configPath := filepath.Join(dir, "node.json")
	first, err := LoadOrCreateIdentityWithPaths(keyPath, configPath)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	second, err := LoadOrCreateIdentityWithPaths(keyPath, configPath)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if first.GetIdentity().ID != second.GetIdentity().ID {
		t.Fatal("expected same identity")
	}
}

func TestIdentity_NodeManager_HasIdentity_Good(t *testing.T) {
	nm, cleanup := setupTestNodeManager(t)
	defer cleanup()
	if err := nm.GenerateIdentity("node", RoleDual); err != nil {
		t.Fatalf("GenerateIdentity: %v", err)
	}
	if !nm.HasIdentity() {
		t.Fatal("expected identity")
	}
}

func TestIdentity_NodeManager_HasIdentity_Bad(t *testing.T) {
	nm, cleanup := setupTestNodeManager(t)
	defer cleanup()
	if nm.HasIdentity() {
		t.Fatal("new manager should not have identity")
	}
	if nm.GetIdentity() != nil {
		t.Fatal("identity should be nil")
	}
}

func TestIdentity_NodeManager_HasIdentity_Ugly(t *testing.T) {
	nm, cleanup := setupTestNodeManager(t)
	defer cleanup()
	_ = nm.Delete()
	if nm.HasIdentity() {
		t.Fatal("deleted manager should not have identity")
	}
}

func TestIdentity_NodeManager_GetIdentity_Good(t *testing.T) {
	nm, cleanup := setupTestNodeManager(t)
	defer cleanup()
	if err := nm.GenerateIdentity("node", RoleWorker); err != nil {
		t.Fatalf("GenerateIdentity: %v", err)
	}
	identity := nm.GetIdentity()
	if identity == nil || identity.Name != "node" {
		t.Fatalf("identity: %#v", identity)
	}
}

func TestIdentity_NodeManager_GetIdentity_Bad(t *testing.T) {
	nm, cleanup := setupTestNodeManager(t)
	defer cleanup()
	identity := nm.GetIdentity()
	if identity != nil {
		t.Fatalf("identity: got %#v, want nil", identity)
	}
}

func TestIdentity_NodeManager_GetIdentity_Ugly(t *testing.T) {
	nm, cleanup := setupTestNodeManager(t)
	defer cleanup()
	if err := nm.GenerateIdentity("node", RoleDual); err != nil {
		t.Fatalf("GenerateIdentity: %v", err)
	}
	identity := nm.GetIdentity()
	identity.Name = "mutated"
	if nm.GetIdentity().Name == "mutated" {
		t.Fatal("GetIdentity should return a copy")
	}
}

func TestIdentity_NodeManager_GenerateIdentity_Good(t *testing.T) {
	nm, cleanup := setupTestNodeManager(t)
	defer cleanup()
	err := nm.GenerateIdentity("node", RoleController)
	if err != nil {
		t.Fatalf("GenerateIdentity: %v", err)
	}
	if nm.GetIdentity().Role != RoleController {
		t.Fatalf("role: got %s", nm.GetIdentity().Role)
	}
}

func TestIdentity_NodeManager_GenerateIdentity_Bad(t *testing.T) {
	dir := t.TempDir()
	err := os.Mkdir(filepath.Join(dir, "private.key"), 0755)
	if err != nil {
		t.Fatalf("mkdir private key path: %v", err)
	}
	nm, err := NewNodeManagerWithPaths(filepath.Join(dir, "private.key"), filepath.Join(dir, "node.json"))
	if err != nil {
		t.Fatalf("NewNodeManagerWithPaths: %v", err)
	}
	err = nm.GenerateIdentity("node", RoleDual)
	if err == nil {
		t.Fatal("expected save private key error")
	}
}

func TestIdentity_NodeManager_GenerateIdentity_Ugly(t *testing.T) {
	nm, cleanup := setupTestNodeManager(t)
	defer cleanup()
	err := nm.GenerateIdentity("", "")
	if err != nil {
		t.Fatalf("GenerateIdentity empty fields: %v", err)
	}
	if nm.GetIdentity().Name != "" || nm.GetIdentity().Role != "" {
		t.Fatalf("identity: %#v", nm.GetIdentity())
	}
}

func TestIdentity_NodeManager_DeriveSharedSecret_Good(t *testing.T) {
	left, cleanupLeft := setupTestNodeManager(t)
	defer cleanupLeft()
	right, cleanupRight := setupTestNodeManager(t)
	defer cleanupRight()
	_ = left.GenerateIdentity("left", RoleDual)
	_ = right.GenerateIdentity("right", RoleDual)
	secret, err := left.DeriveSharedSecret(right.GetIdentity().PublicKey)
	if err != nil {
		t.Fatalf("DeriveSharedSecret: %v", err)
	}
	if len(secret) != 32 {
		t.Fatalf("secret length: got %d", len(secret))
	}
}

func TestIdentity_NodeManager_DeriveSharedSecret_Bad(t *testing.T) {
	nm, cleanup := setupTestNodeManager(t)
	defer cleanup()
	secret, err := nm.DeriveSharedSecret("invalid")
	if err == nil {
		t.Fatal("expected identity error")
	}
	if secret != nil {
		t.Fatalf("secret: got %x, want nil", secret)
	}
}

func TestIdentity_NodeManager_DeriveSharedSecret_Ugly(t *testing.T) {
	nm, cleanup := setupTestNodeManager(t)
	defer cleanup()
	_ = nm.GenerateIdentity("node", RoleDual)
	secret, err := nm.DeriveSharedSecret("invalid")
	if err == nil {
		t.Fatal("expected invalid public key error")
	}
	if secret != nil {
		t.Fatalf("secret: got %x, want nil", secret)
	}
}

func TestIdentity_NodeManager_Delete_Good(t *testing.T) {
	nm, cleanup := setupTestNodeManager(t)
	defer cleanup()
	_ = nm.GenerateIdentity("node", RoleDual)
	err := nm.Delete()
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if nm.HasIdentity() {
		t.Fatal("identity should be cleared")
	}
}

func TestIdentity_NodeManager_Delete_Bad(t *testing.T) {
	nm, cleanup := setupTestNodeManager(t)
	defer cleanup()
	err := nm.Delete()
	if err != nil {
		t.Fatalf("Delete without files: %v", err)
	}
	if nm.GetIdentity() != nil {
		t.Fatal("identity should be nil")
	}
}

func TestIdentity_NodeManager_Delete_Ugly(t *testing.T) {
	nm, cleanup := setupTestNodeManager(t)
	defer cleanup()
	_ = nm.GenerateIdentity("node", RoleDual)
	_ = nm.Delete()
	err := nm.Delete()
	if err != nil {
		t.Fatalf("second Delete: %v", err)
	}
}

func TestNodeRoles(t *testing.T) {
	tests := []struct {
		role     NodeRole
		expected string
	}{
		{RoleController, "controller"},
		{RoleWorker, "worker"},
		{RoleDual, "dual"},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			if string(tt.role) != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, string(tt.role))
			}
		})
	}
}

func TestChallengeResponse(t *testing.T) {
	t.Run("GenerateChallenge", func(t *testing.T) {
		challenge, err := GenerateChallenge()
		if err != nil {
			t.Fatalf("failed to generate challenge: %v", err)
		}

		if len(challenge) != ChallengeSize {
			t.Errorf("expected challenge size %d, got %d", ChallengeSize, len(challenge))
		}

		// Ensure challenges are unique (not all zeros)
		allZero := true
		for _, b := range challenge {
			if b != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			t.Error("challenge should not be all zeros")
		}

		// Generate another and ensure they're different
		challenge2, err := GenerateChallenge()
		if err != nil {
			t.Fatalf("failed to generate second challenge: %v", err)
		}

		same := true
		for i := range challenge {
			if challenge[i] != challenge2[i] {
				same = false
				break
			}
		}
		if same {
			t.Error("two generated challenges should be different")
		}
	})

	t.Run("SignAndVerifyChallenge", func(t *testing.T) {
		challenge, _ := GenerateChallenge()
		sharedSecret := []byte("test-secret-key-32-bytes-long!!")

		// Sign the challenge
		signature := SignChallenge(challenge, sharedSecret)

		if len(signature) == 0 {
			t.Error("signature should not be empty")
		}

		// Verify should succeed with correct parameters
		if !VerifyChallenge(challenge, signature, sharedSecret) {
			t.Error("verification should succeed with correct parameters")
		}

		// Verify should fail with wrong challenge
		wrongChallenge, _ := GenerateChallenge()
		if VerifyChallenge(wrongChallenge, signature, sharedSecret) {
			t.Error("verification should fail with wrong challenge")
		}

		// Verify should fail with wrong secret
		wrongSecret := []byte("wrong-secret-key-32-bytes-long!")
		if VerifyChallenge(challenge, signature, wrongSecret) {
			t.Error("verification should fail with wrong secret")
		}

		// Verify should fail with tampered signature
		tamperedSig := make([]byte, len(signature))
		copy(tamperedSig, signature)
		tamperedSig[0] ^= 0xFF // Flip bits
		if VerifyChallenge(challenge, tamperedSig, sharedSecret) {
			t.Error("verification should fail with tampered signature")
		}
	})

	t.Run("SignatureIsDeterministic", func(t *testing.T) {
		challenge := []byte("fixed-challenge-for-testing")
		sharedSecret := []byte("fixed-secret-key-for-testing")

		sig1 := SignChallenge(challenge, sharedSecret)
		sig2 := SignChallenge(challenge, sharedSecret)

		if len(sig1) != len(sig2) {
			t.Fatal("signatures should have same length")
		}

		for i := range sig1 {
			if sig1[i] != sig2[i] {
				t.Fatal("signatures should be identical for same inputs")
			}
		}
	})

	t.Run("IntegrationWithSharedSecret", func(t *testing.T) {
		// Create two nodes and test end-to-end challenge-response
		tmpDir1, _ := os.MkdirTemp("", "node-challenge-1")
		tmpDir2, _ := os.MkdirTemp("", "node-challenge-2")
		defer os.RemoveAll(tmpDir1)
		defer os.RemoveAll(tmpDir2)

		nm1, _ := NewNodeManagerWithPaths(
			filepath.Join(tmpDir1, "private.key"),
			filepath.Join(tmpDir1, "node.json"),
		)
		nm1.GenerateIdentity("challenger", RoleDual)

		nm2, _ := NewNodeManagerWithPaths(
			filepath.Join(tmpDir2, "private.key"),
			filepath.Join(tmpDir2, "node.json"),
		)
		nm2.GenerateIdentity("responder", RoleDual)

		// Challenger generates challenge
		challenge, err := GenerateChallenge()
		if err != nil {
			t.Fatalf("failed to generate challenge: %v", err)
		}

		// Both derive the same shared secret
		secret1, _ := nm1.DeriveSharedSecret(nm2.GetIdentity().PublicKey)
		secret2, _ := nm2.DeriveSharedSecret(nm1.GetIdentity().PublicKey)

		// Responder signs challenge with their derived secret
		response := SignChallenge(challenge, secret2)

		// Challenger verifies with their derived secret
		if !VerifyChallenge(challenge, response, secret1) {
			t.Error("challenge-response should verify with matching shared secrets")
		}
	})
}

func TestNodeManager_DeriveSharedSecret_NoIdentity(t *testing.T) {
	nm, cleanup := setupTestNodeManager(t)
	defer cleanup()

	// No identity generated
	_, err := nm.DeriveSharedSecret("some-key")
	if err == nil {
		t.Error("expected error when identity not initialized")
	}
}

func TestNodeManager_GetIdentity_NilWhenNoIdentity(t *testing.T) {
	nm, cleanup := setupTestNodeManager(t)
	defer cleanup()

	identity := nm.GetIdentity()
	if identity != nil {
		t.Error("expected nil identity before generation")
	}
}

func TestNodeManager_Delete_NoFiles(t *testing.T) {
	tmpDir := t.TempDir()
	nm, err := NewNodeManagerWithPaths(
		filepath.Join(tmpDir, "nonexistent.key"),
		filepath.Join(tmpDir, "nonexistent.json"),
	)
	if err != nil {
		t.Fatalf("failed to create node manager: %v", err)
	}

	// Delete when no files exist should succeed
	err = nm.Delete()
	if err != nil {
		t.Errorf("Delete should not error when files don't exist: %v", err)
	}
}
