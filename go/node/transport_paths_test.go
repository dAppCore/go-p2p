// SPDX-License-Identifier: EUPL-1.2

package node

import (
	"testing"
	"time"

	core "dappco.re/go"
)

// transportTestSecret is a valid 32-byte shared secret for the transport AEAD.
var transportTestSecret = func() []byte {
	s := make([]byte, sharedSecretSize)
	for i := range s {
		s[i] = byte(i + 1)
	}
	return s
}()

// TestTransport_EncryptDecryptPayload_Good round-trips a payload through the
// transport AEAD.
func TestTransport_EncryptDecryptPayload_Good(t *testing.T) {
	plaintext := []byte("transport payload")

	sealed, err := resultValue[[]byte](encryptTransportPayload(plaintext, transportTestSecret))
	if err != nil {
		t.Fatalf("encryptTransportPayload: %v", err)
	}
	if len(sealed) == 0 || sealed[0] != transportAEADVersion {
		t.Fatalf("sealed frame missing version byte: % x", sealed)
	}

	out, err := resultValue[[]byte](decryptTransportPayload(sealed, transportTestSecret))
	if err != nil {
		t.Fatalf("decryptTransportPayload: %v", err)
	}
	if !core.DeepEqual(out, plaintext) {
		t.Fatalf("round trip: got %q, want %q", out, plaintext)
	}
}

// TestTransport_DecryptPayload_TooShort rejects a frame shorter than the
// header + AEAD overhead.
func TestTransport_DecryptPayload_TooShort(t *testing.T) {
	if r := decryptTransportPayload([]byte{transportAEADVersion, 0x00}, transportTestSecret); r.OK {
		t.Fatal("expected too-short rejection")
	}
}

// TestTransport_DecryptPayload_BadVersion rejects a frame whose version byte is
// not the supported transport AEAD version.
func TestTransport_DecryptPayload_BadVersion(t *testing.T) {
	sealed, err := resultValue[[]byte](encryptTransportPayload([]byte("x"), transportTestSecret))
	if err != nil {
		t.Fatalf("encryptTransportPayload: %v", err)
	}
	sealed[0] = transportAEADVersion + 1 // corrupt version

	if r := decryptTransportPayload(sealed, transportTestSecret); r.OK {
		t.Fatal("expected unsupported-version rejection")
	}
}

// TestTransport_DecryptPayload_TamperedCiphertext rejects a frame whose
// ciphertext has been modified (AEAD authentication failure).
func TestTransport_DecryptPayload_TamperedCiphertext(t *testing.T) {
	sealed, err := resultValue[[]byte](encryptTransportPayload([]byte("authentic payload"), transportTestSecret))
	if err != nil {
		t.Fatalf("encryptTransportPayload: %v", err)
	}
	sealed[len(sealed)-1] ^= 0xFF // flip a ciphertext byte

	if r := decryptTransportPayload(sealed, transportTestSecret); r.OK {
		t.Fatal("expected AEAD authentication failure on tampered ciphertext")
	}
}

// TestTransport_DecryptPayload_WrongKey rejects a frame sealed under one secret
// when opened with a different secret.
func TestTransport_DecryptPayload_WrongKey(t *testing.T) {
	sealed, err := resultValue[[]byte](encryptTransportPayload([]byte("payload"), transportTestSecret))
	if err != nil {
		t.Fatalf("encryptTransportPayload: %v", err)
	}

	wrong := make([]byte, sharedSecretSize)
	for i := range wrong {
		wrong[i] = byte(0xAA)
	}
	if r := decryptTransportPayload(sealed, wrong); r.OK {
		t.Fatal("expected authentication failure under the wrong key")
	}
}

// TestTransport_idleTimeout_Configured returns the configured idle timeout when
// it is positive.
func TestTransport_idleTimeout_Configured(t *testing.T) {
	nm := testNode(t, "idle-cfg", RoleWorker)
	reg := testRegistry(t)
	cfg := DefaultTransportConfig()
	cfg.IdleTimeout = 7 * time.Second

	tp := NewTransport(nm, reg, cfg)
	if got := tp.idleTimeout(); got != 7*time.Second {
		t.Fatalf("idleTimeout: got %v, want %v", got, 7*time.Second)
	}
}

// TestTransport_idleTimeout_Default falls back to DefaultIdleTimeout when the
// configured value is zero or negative.
func TestTransport_idleTimeout_Default(t *testing.T) {
	nm := testNode(t, "idle-def", RoleWorker)
	reg := testRegistry(t)
	cfg := DefaultTransportConfig()
	cfg.IdleTimeout = 0

	tp := NewTransport(nm, reg, cfg)
	if got := tp.idleTimeout(); got != DefaultIdleTimeout {
		t.Fatalf("idleTimeout: got %v, want %v", got, DefaultIdleTimeout)
	}
}

// TestTransport_EncryptDecryptMessage_Good round-trips a Message through the
// transport encryptMessage/decryptMessage pair.
func TestTransport_EncryptDecryptMessage_Good(t *testing.T) {
	nm := testNode(t, "crypt", RoleWorker)
	reg := testRegistry(t)
	tp := NewTransport(nm, reg, DefaultTransportConfig())

	msg, err := resultValue[*Message](NewMessage(MsgPing, "from", "to", PingPayload{SentAt: 123}))
	if err != nil {
		t.Fatalf("new message: %v", err)
	}

	sealed, err := resultValue[[]byte](tp.encryptMessage(msg, transportTestSecret))
	if err != nil {
		t.Fatalf("encryptMessage: %v", err)
	}

	out, err := resultValue[*Message](tp.decryptMessage(sealed, transportTestSecret))
	if err != nil {
		t.Fatalf("decryptMessage: %v", err)
	}
	if out.Type != MsgPing || out.From != "from" {
		t.Fatalf("decrypted message mismatch: type=%q from=%q", out.Type, out.From)
	}
}

// TestTransport_DecryptMessage_Bad rejects a frame that decrypts but does not
// contain a valid Message.
func TestTransport_DecryptMessage_Bad(t *testing.T) {
	nm := testNode(t, "crypt-bad", RoleWorker)
	reg := testRegistry(t)
	tp := NewTransport(nm, reg, DefaultTransportConfig())

	// Seal non-Message bytes, then decrypt: AEAD opens but message decode fails.
	sealed, err := resultValue[[]byte](encryptTransportPayload([]byte("not-a-message"), transportTestSecret))
	if err != nil {
		t.Fatalf("encryptTransportPayload: %v", err)
	}

	if r := tp.decryptMessage(sealed, transportTestSecret); r.OK {
		t.Fatal("expected message-decode failure after successful decrypt")
	}
}
