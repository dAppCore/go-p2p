// Package node provides P2P node identity and communication for multi-node mining management.
package node

import (
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	core "dappco.re/go"
	coreio "dappco.re/go/io"
	coreerr "dappco.re/go/log"

	"github.com/Snider/Borg/pkg/stmf"
	"github.com/adrg/xdg" // Note: intrinsic - XDG data directory resolution; c.Fs() does not expose XDG paths.
	"golang.org/x/sys/unix"
)

// ChallengeSize is the size of the challenge in bytes
const ChallengeSize = 32

// GenerateChallenge creates a random challenge for authentication.
func GenerateChallenge() core.Result {
	challenge := make([]byte, ChallengeSize)
	if _, err := rand.Read(challenge); err != nil {
		return core.Fail(coreerr.E("GenerateChallenge", "failed to generate challenge", err))
	}
	return core.Ok(challenge)
}

// SignChallenge creates an HMAC signature of a challenge using a shared secret.
// The signature proves possession of the shared secret without revealing it.
func SignChallenge(challenge []byte, sharedSecret []byte) []byte {
	mac := hmac.New(sha256.New, sharedSecret)
	mac.Write(challenge)
	return mac.Sum(nil)
}

// VerifyChallenge verifies that a challenge response was signed with the correct shared secret.
func VerifyChallenge(challenge, response, sharedSecret []byte) bool {
	expected := SignChallenge(challenge, sharedSecret)
	return hmac.Equal(response, expected)
}

// NodeRole defines the operational mode of a node.
type NodeRole string

const (
	// RoleController manages remote worker nodes.
	RoleController NodeRole = "controller"
	// RoleWorker receives commands and runs miners.
	RoleWorker NodeRole = "worker"
	// RoleDual operates as both controller and worker (default).
	RoleDual NodeRole = "dual"
)

// NodeIdentity represents the public identity of a node.
type NodeIdentity struct {
	ID        string    `json:"id"`        // Derived from public key (first 16 bytes hex)
	Name      string    `json:"name"`      // Human-friendly name
	PublicKey string    `json:"publicKey"` // X25519 base64
	CreatedAt time.Time `json:"createdAt"`
	Role      NodeRole  `json:"role"`
}

// NodeManager handles node identity operations including key generation and storage.
type NodeManager struct {
	identity   *NodeIdentity
	privateKey []byte // Never serialized to JSON
	keyPair    *stmf.KeyPair
	keyPath    string // ~/.local/share/lethean-desktop/node/private.key
	configPath string // ~/.config/lethean-desktop/node.json
	mu         core.RWMutex
}

// NewNodeManager creates a new NodeManager, loading existing identity if available.
func NewNodeManager() core.Result {
	keyPath, err := xdg.DataFile("lethean-desktop/node/private.key")
	if err != nil {
		return core.Fail(coreerr.E("NodeManager.New", "failed to get key path", err))
	}

	configPath, err := xdg.ConfigFile("lethean-desktop/node.json")
	if err != nil {
		return core.Fail(coreerr.E("NodeManager.New", "failed to get config path", err))
	}

	return NewNodeManagerWithPaths(keyPath, configPath)
}

// NewNodeManagerWithPaths creates a NodeManager with custom paths.
// This is primarily useful for testing to avoid xdg path caching issues.
func NewNodeManagerWithPaths(keyPath, configPath string) core.Result {
	nm := &NodeManager{
		keyPath:    keyPath,
		configPath: configPath,
	}

	// Try to load existing identity
	if r := nm.loadIdentity(); !r.OK {
		// Identity doesn't exist yet, that's ok
		return core.Ok(nm)
	}

	return core.Ok(nm)
}

// LoadOrCreateIdentity loads the node identity from the default XDG paths or
// generates a new dual-role identity when none exists yet.
func LoadOrCreateIdentity() core.Result {
	keyPath, err := xdg.DataFile("lethean-desktop/node/private.key")
	if err != nil {
		return core.Fail(coreerr.E("LoadOrCreateIdentity", "failed to get key path", err))
	}

	configPath, err := xdg.ConfigFile("lethean-desktop/node.json")
	if err != nil {
		return core.Fail(coreerr.E("LoadOrCreateIdentity", "failed to get config path", err))
	}

	return LoadOrCreateIdentityWithPaths(keyPath, configPath)
}

// LoadOrCreateIdentityWithPaths loads an existing identity from the supplied
// paths or creates a new dual-role identity if no persisted identity exists.
// The generated identity name falls back to the host name, then a stable
// project-specific default if the host name cannot be determined.
func LoadOrCreateIdentityWithPaths(keyPath, configPath string) core.Result {
	nmResult := NewNodeManagerWithPaths(keyPath, configPath)
	if !nmResult.OK {
		return nmResult
	}
	nm := nmResult.Value.(*NodeManager)

	if nm.HasIdentity() {
		return core.Ok(nm)
	}

	name := core.Env("HOSTNAME")
	if name == "" {
		name = "lethean-node"
	}

	if r := nm.GenerateIdentity(name, RoleDual); !r.OK {
		err, _ := r.Value.(error)
		return core.Fail(coreerr.E("LoadOrCreateIdentityWithPaths", "failed to generate identity", err))
	}

	return core.Ok(nm)
}

// HasIdentity returns true if a node identity has been initialized.
func (n *NodeManager) HasIdentity() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.identity != nil
}

// GetIdentity returns the node's public identity.
func (n *NodeManager) GetIdentity() *NodeIdentity {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.identity == nil {
		return nil
	}
	// Return a copy to prevent mutation
	identity := *n.identity
	return &identity
}

// GenerateIdentity creates a new node identity with the given name and role.
func (n *NodeManager) GenerateIdentity(name string, role NodeRole) core.Result {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Generate X25519 keypair using STMF
	keyPair, err := stmf.GenerateKeyPair()
	if err != nil {
		return core.Fail(coreerr.E("NodeManager.GenerateIdentity", "failed to generate keypair", err))
	}

	// Derive node ID from public key (first 16 bytes as hex = 32 char ID)
	pubKeyBytes := keyPair.PublicKey()
	hash := sha256.Sum256(pubKeyBytes)
	nodeID := hex.EncodeToString(hash[:16])

	n.identity = &NodeIdentity{
		ID:        nodeID,
		Name:      name,
		PublicKey: keyPair.PublicKeyBase64(),
		CreatedAt: time.Now(),
		Role:      role,
	}

	n.keyPair = keyPair
	n.privateKey = keyPair.PrivateKey()

	// Save private key
	if r := n.savePrivateKey(); !r.OK {
		err, _ := r.Value.(error)
		return core.Fail(coreerr.E("NodeManager.GenerateIdentity", "failed to save private key", err))
	}

	// Save identity config
	if r := n.saveIdentity(); !r.OK {
		err, _ := r.Value.(error)
		return core.Fail(coreerr.E("NodeManager.GenerateIdentity", "failed to save identity", err))
	}

	return core.Ok(nil)
}

// DeriveSharedSecret derives a shared secret with a peer using X25519 ECDH.
// The result is hashed with SHA-256 for use as a symmetric key.
func (n *NodeManager) DeriveSharedSecret(peerPubKeyBase64 string) core.Result {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if n.privateKey == nil {
		return core.Fail(ErrIdentityNotInitialized)
	}

	// Load peer's public key
	peerPubKey, err := stmf.LoadPublicKeyBase64(peerPubKeyBase64)
	if err != nil {
		return core.Fail(coreerr.E("NodeManager.DeriveSharedSecret", "failed to load peer public key", err))
	}

	// Load our private key
	privateKey, err := ecdh.X25519().NewPrivateKey(n.privateKey)
	if err != nil {
		return core.Fail(coreerr.E("NodeManager.DeriveSharedSecret", "failed to load private key", err))
	}

	// Derive shared secret using ECDH
	sharedSecret, err := privateKey.ECDH(peerPubKey)
	if err != nil {
		return core.Fail(coreerr.E("NodeManager.DeriveSharedSecret", "failed to derive shared secret", err))
	}

	// Hash the shared secret using SHA-256 (same pattern as Borg/trix)
	hash := sha256.Sum256(sharedSecret)
	return core.Ok(hash[:])
}

// savePrivateKey saves the private key to disk with restricted permissions.
func (n *NodeManager) savePrivateKey() core.Result {
	// Ensure directory exists
	dir := core.PathDir(n.keyPath)
	if err := coreio.Local.EnsureDir(dir); err != nil {
		return core.Fail(coreerr.E("NodeManager.savePrivateKey", "failed to create key directory", err))
	}

	if err := coreio.Local.WriteMode(n.keyPath, string(n.privateKey), 0600); err != nil {
		return core.Fail(coreerr.E("NodeManager.savePrivateKey", "failed to write private key", err))
	}
	if err := unix.Chmod(n.keyPath, 0600); err != nil {
		return core.Fail(coreerr.E("NodeManager.savePrivateKey", "failed to set private key permissions", err))
	}

	return core.Ok(nil)
}

// saveIdentity saves the public identity to the config file.
func (n *NodeManager) saveIdentity() core.Result {
	// Ensure directory exists
	dir := core.PathDir(n.configPath)
	if err := coreio.Local.EnsureDir(dir); err != nil {
		return core.Fail(coreerr.E("NodeManager.saveIdentity", "failed to create config directory", err))
	}

	result := core.JSONMarshal(n.identity)
	if !result.OK {
		err, _ := result.Value.(error)
		return core.Fail(coreerr.E("NodeManager.saveIdentity", "failed to marshal identity", err))
	}
	data := result.Value.([]byte)

	if err := coreio.Local.Write(n.configPath, string(data)); err != nil {
		return core.Fail(coreerr.E("NodeManager.saveIdentity", "failed to write identity", err))
	}

	return core.Ok(nil)
}

// loadIdentity loads the node identity from disk.
func (n *NodeManager) loadIdentity() core.Result {
	// Load identity config
	content, err := coreio.Local.Read(n.configPath)
	if err != nil {
		return core.Fail(coreerr.E("NodeManager.loadIdentity", "failed to read identity", err))
	}

	var identity NodeIdentity
	if result := core.JSONUnmarshal([]byte(content), &identity); !result.OK {
		err, _ := result.Value.(error)
		return core.Fail(coreerr.E("NodeManager.loadIdentity", "failed to unmarshal identity", err))
	}

	// Load private key
	keyContent, err := coreio.Local.Read(n.keyPath)
	if err != nil {
		return core.Fail(coreerr.E("NodeManager.loadIdentity", "failed to read private key", err))
	}
	privateKey := []byte(keyContent)

	// Reconstruct keypair from private key
	keyPair, err := stmf.LoadKeyPair(privateKey)
	if err != nil {
		return core.Fail(coreerr.E("NodeManager.loadIdentity", "failed to load keypair", err))
	}

	n.identity = &identity
	n.privateKey = privateKey
	n.keyPair = keyPair

	return core.Ok(nil)
}

// Delete removes the node identity and keys from disk.
func (n *NodeManager) Delete() core.Result {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Remove private key (ignore if already absent)
	if coreio.Local.Exists(n.keyPath) {
		if err := coreio.Local.Delete(n.keyPath); err != nil {
			return core.Fail(coreerr.E("NodeManager.Delete", "failed to remove private key", err))
		}
	}

	// Remove identity config (ignore if already absent)
	if coreio.Local.Exists(n.configPath) {
		if err := coreio.Local.Delete(n.configPath); err != nil {
			return core.Fail(coreerr.E("NodeManager.Delete", "failed to remove identity", err))
		}
	}

	n.identity = nil
	n.privateKey = nil
	n.keyPair = nil

	return core.Ok(nil)
}
