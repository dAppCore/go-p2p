package node

import (
	"crypto/ed25519"
	"testing"

	core "dappco.re/go"
)

// TestEnvelope_DecodeEnvelope_Good decodes a fully-populated signed envelope.
func TestEnvelope_DecodeEnvelope_Good(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	body := []byte("payload-body")
	env := Envelope{PeerPubkey: pub, Body: body, Signature: ed25519.Sign(priv, body)}

	data, err := resultValue[[]byte](MarshalJSON(env))
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	res := decodeEnvelope(data)
	decoded, err := resultValue[decodeEnvelopeResult](res)
	if err != nil {
		t.Fatalf("decodeEnvelope: %v", err)
	}
	if !decoded.OK {
		t.Fatal("expected envelope to decode as a real envelope")
	}
	if !core.DeepEqual(decoded.Envelope.Body, body) {
		t.Fatalf("body: got %q, want %q", decoded.Envelope.Body, body)
	}
}

// TestEnvelope_DecodeEnvelope_Bad rejects bytes that are not valid JSON.
func TestEnvelope_DecodeEnvelope_Bad(t *testing.T) {
	res := decodeEnvelope([]byte("{not json"))
	if res.OK {
		t.Fatalf("expected decode failure, got %#v", res.Value)
	}
}

// TestEnvelope_DecodeEnvelope_Ugly treats a JSON object with none of the
// envelope fields as a non-envelope (OK=false sentinel), not an error.
func TestEnvelope_DecodeEnvelope_Ugly(t *testing.T) {
	res := decodeEnvelope([]byte(`{"id":"abc","type":"ping"}`))
	decoded, err := resultValue[decodeEnvelopeResult](res)
	if err != nil {
		t.Fatalf("decodeEnvelope: %v", err)
	}
	if decoded.OK {
		t.Fatal("a non-envelope JSON object should decode with OK=false")
	}
}

// TestEnvelope_UnwrapEnvelope_PlainMessage passes through a plain (non-envelope)
// message body unchanged.
func TestEnvelope_UnwrapEnvelope_PlainMessage(t *testing.T) {
	plain := []byte(`{"id":"abc","type":"ping"}`)
	out, err := resultValue[[]byte](unwrapEnvelope(plain))
	if err != nil {
		t.Fatalf("unwrapEnvelope: %v", err)
	}
	if !core.DeepEqual(out, plain) {
		t.Fatalf("plain message should pass through: got %q", out)
	}
}

// TestEnvelope_UnwrapEnvelope_EmptyBody rejects an envelope whose body is empty.
func TestEnvelope_UnwrapEnvelope_EmptyBody(t *testing.T) {
	env := Envelope{PeerPubkey: make([]byte, ed25519.PublicKeySize), Body: nil, Signature: make([]byte, ed25519.SignatureSize)}
	data, err := resultValue[[]byte](MarshalJSON(env))
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	res := unwrapEnvelope(data)
	if res.OK {
		t.Fatalf("expected empty-body rejection, got %#v", res.Value)
	}
	if !core.Is(resultErr(res), ErrEnvelopeBodyEmpty) {
		t.Fatalf("error: got %v, want ErrEnvelopeBodyEmpty", resultErr(res))
	}
}

// TestEnvelope_UnwrapEnvelope_BadSignature rejects an envelope whose signature
// does not verify against the body.
func TestEnvelope_UnwrapEnvelope_BadSignature(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	env := Envelope{PeerPubkey: pub, Body: []byte("body"), Signature: make([]byte, ed25519.SignatureSize)}
	data, err := resultValue[[]byte](MarshalJSON(env))
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	res := unwrapEnvelope(data)
	if res.OK {
		t.Fatalf("expected signature rejection, got %#v", res.Value)
	}
	if !core.Is(resultErr(res), ErrEnvelopeSignatureInvalid) {
		t.Fatalf("error: got %v, want ErrEnvelopeSignatureInvalid", resultErr(res))
	}
}

// TestEnvelope_DecodeReceivedMessage_Good unwraps a signed envelope around a
// real Message and parses it back out.
func TestEnvelope_DecodeReceivedMessage_Good(t *testing.T) {
	msg, err := resultValue[*Message](NewMessage(MsgPing, "from", "to", nil))
	if err != nil {
		t.Fatalf("new message: %v", err)
	}
	msgData, err := resultValue[[]byte](MarshalJSON(msg))
	if err != nil {
		t.Fatalf("marshal message: %v", err)
	}

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	env := Envelope{PeerPubkey: pub, Body: msgData, Signature: ed25519.Sign(priv, msgData)}
	envData, err := resultValue[[]byte](MarshalJSON(env))
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	out, err := resultValue[*Message](decodeReceivedMessage(envData))
	if err != nil {
		t.Fatalf("decodeReceivedMessage: %v", err)
	}
	if out.Type != MsgPing {
		t.Fatalf("type: got %q, want %q", out.Type, MsgPing)
	}
}

// TestEnvelope_DecodeReceivedMessage_Bad fails when the unwrapped body is not a
// valid Message.
func TestEnvelope_DecodeReceivedMessage_Bad(t *testing.T) {
	res := decodeReceivedMessage([]byte("not-json-at-all"))
	if res.OK {
		t.Fatalf("expected decode failure, got %#v", res.Value)
	}
}
