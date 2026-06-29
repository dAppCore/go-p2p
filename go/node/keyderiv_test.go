package node

import (
	"testing"

	core "dappco.re/go"
)

// TestKeyderiv_deriveSubKeys_Good derives three distinct sub-keys from a valid
// 32-byte shared secret.
func TestKeyderiv_deriveSubKeys_Good(t *testing.T) {
	secret := make([]byte, sharedSecretSize)
	for i := range secret {
		secret[i] = byte(i)
	}

	keys, err := resultValue[transportSubKeys](deriveSubKeys(secret))
	if err != nil {
		t.Fatalf("deriveSubKeys: %v", err)
	}
	if len(keys.encKey) != subKeySize || len(keys.macKey) != subKeySize || len(keys.chlKey) != subKeySize {
		t.Fatalf("sub-key sizes: enc=%d mac=%d chl=%d, want %d each",
			len(keys.encKey), len(keys.macKey), len(keys.chlKey), subKeySize)
	}
	// Distinct info strings must yield distinct keys.
	if core.DeepEqual(keys.encKey, keys.macKey) || core.DeepEqual(keys.encKey, keys.chlKey) || core.DeepEqual(keys.macKey, keys.chlKey) {
		t.Fatal("sub-keys must be distinct across encrypt/mac/challenge contexts")
	}
}

// TestKeyderiv_deriveSubKeys_Bad rejects a shared secret of the wrong length.
func TestKeyderiv_deriveSubKeys_Bad(t *testing.T) {
	if r := deriveSubKeys(make([]byte, sharedSecretSize-1)); r.OK {
		t.Fatal("expected failure for short shared secret")
	}
	if r := deriveSubKeys(nil); r.OK {
		t.Fatal("expected failure for nil shared secret")
	}
}

// TestKeyderiv_deriveSubKeys_Ugly is deterministic: the same secret yields the
// same key material on every call.
func TestKeyderiv_deriveSubKeys_Ugly(t *testing.T) {
	secret := make([]byte, sharedSecretSize)
	first, err := resultValue[transportSubKeys](deriveSubKeys(secret))
	if err != nil {
		t.Fatalf("first deriveSubKeys: %v", err)
	}
	second, err := resultValue[transportSubKeys](deriveSubKeys(secret))
	if err != nil {
		t.Fatalf("second deriveSubKeys: %v", err)
	}
	if !core.DeepEqual(first.encKey, second.encKey) {
		t.Fatal("derivation must be deterministic for the same secret")
	}
}

// TestKeyderiv_deriveSubKey_Good derives a single sub-key of the expected size.
func TestKeyderiv_deriveSubKey_Good(t *testing.T) {
	key, err := resultValue[[]byte](deriveSubKey(make([]byte, sharedSecretSize), keyInfoEncryptV1))
	if err != nil {
		t.Fatalf("deriveSubKey: %v", err)
	}
	if len(key) != subKeySize {
		t.Fatalf("key size: got %d, want %d", len(key), subKeySize)
	}
}
