package node

import (
	"crypto/ed25519"
	"testing"

	core "dappco.re/go"
)

func TestEnvelope_Envelope_VerifySignature_Good(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	env := Envelope{PeerPubkey: pub, Body: []byte("message")}
	env.Signature = ed25519.Sign(priv, env.Body)
	if err := env.VerifySignature(); err != nil {
		t.Fatalf("VerifySignature: %v", err)
	}
}

func TestEnvelope_Envelope_VerifySignature_Bad(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	env := Envelope{PeerPubkey: pub, Body: []byte("message"), Signature: make([]byte, ed25519.SignatureSize)}
	err = env.VerifySignature()
	if !core.Is(err, ErrEnvelopeSignatureInvalid) {
		t.Fatalf("error: got %v", err)
	}
}

func TestEnvelope_Envelope_VerifySignature_Ugly(t *testing.T) {
	env := Envelope{Body: []byte("unsigned")}
	err := env.VerifySignature()
	if err != nil {
		t.Fatalf("unsigned envelope should verify: %v", err)
	}
	if len(env.Signature) != 0 {
		t.Fatal("signature should remain empty")
	}
}
