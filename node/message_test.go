package node

import (
	"testing"
	"time"
)

func TestNewMessage(t *testing.T) {
	t.Run("BasicMessage", func(t *testing.T) {
		msg, err := resultValue[*Message](NewMessage(MsgPing, "sender-id", "receiver-id", nil))
		if err != nil {
			t.Fatalf("failed to create message: %v", err)
		}

		if msg.Type != MsgPing {
			t.Errorf("expected type MsgPing, got %s", msg.Type)
		}

		if msg.From != "sender-id" {
			t.Errorf("expected from 'sender-id', got '%s'", msg.From)
		}

		if msg.To != "receiver-id" {
			t.Errorf("expected to 'receiver-id', got '%s'", msg.To)
		}

		if msg.ID == "" {
			t.Error("message ID should not be empty")
		}

		if msg.Timestamp.IsZero() {
			t.Error("timestamp should be set")
		}
	})

	t.Run("MessageWithPayload", func(t *testing.T) {
		payload := PingPayload{
			SentAt: time.Now().UnixMilli(),
		}

		msg, err := resultValue[*Message](NewMessage(MsgPing, "sender", "receiver", payload))
		if err != nil {
			t.Fatalf("failed to create message: %v", err)
		}

		if msg.Payload == nil {
			t.Error("payload should not be nil")
		}

		var parsed PingPayload
		err = resultErr(msg.ParsePayload(&parsed))
		if err != nil {
			t.Fatalf("failed to parse payload: %v", err)
		}

		if parsed.SentAt != payload.SentAt {
			t.Errorf("expected SentAt %d, got %d", payload.SentAt, parsed.SentAt)
		}
	})
}

func TestMessage_IsProtocolVersionSupported_Good(t *testing.T) {
	if !IsProtocolVersionSupported(ProtocolVersion) {
		t.Fatalf("version %q should be supported", ProtocolVersion)
	}
	if !IsProtocolVersionSupported(MinProtocolVersion) {
		t.Fatalf("version %q should be supported", MinProtocolVersion)
	}
}

func TestMessage_IsProtocolVersionSupported_Bad(t *testing.T) {
	if IsProtocolVersionSupported("2.0") {
		t.Fatal("unexpected support for 2.0")
	}
	if len(SupportedProtocolVersions) == 0 {
		t.Fatal("supported versions should not be empty")
	}
}

func TestMessage_IsProtocolVersionSupported_Ugly(t *testing.T) {
	if IsProtocolVersionSupported("") {
		t.Fatal("empty version should not be supported")
	}
	if IsProtocolVersionSupported(" 1.0 ") {
		t.Fatal("version matching should not trim input")
	}
}

func TestMessage_NewMessage_Good(t *testing.T) {
	msg, err := resultValue[*Message](NewMessage(MsgPing, "from", "to", PingPayload{SentAt: 1}))
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	if msg.Type != MsgPing || len(msg.Payload) == 0 {
		t.Fatalf("message: %#v", msg)
	}
}

func TestMessage_NewMessage_Bad(t *testing.T) {
	msg, err := resultValue[*Message](NewMessage(MsgPing, "from", "to", func() {}))
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if msg != nil {
		t.Fatalf("message: got %#v, want nil", msg)
	}
}

func TestMessage_NewMessage_Ugly(t *testing.T) {
	msg, err := resultValue[*Message](NewMessage("", "", "", nil))
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	if msg.ID == "" || msg.Payload != nil {
		t.Fatalf("message: %#v", msg)
	}
}

func TestMessage_RawMessage_MarshalRawJSON_Good(t *testing.T) {
	raw := RawMessage(`{"ok":true}`)
	data, err := resultValue[[]byte](raw.MarshalRawJSON())
	if err != nil {
		t.Fatalf("MarshalRawJSON: %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Fatalf("data: got %s", data)
	}
}

func TestMessage_RawMessage_MarshalRawJSON_Bad(t *testing.T) {
	var raw RawMessage
	data, err := resultValue[[]byte](raw.MarshalRawJSON())
	if err != nil {
		t.Fatalf("MarshalRawJSON nil: %v", err)
	}
	if string(data) != "null" {
		t.Fatalf("data: got %s", data)
	}
}

func TestMessage_RawMessage_MarshalRawJSON_Ugly(t *testing.T) {
	raw := RawMessage(`[]`)
	data, err := resultValue[[]byte](raw.MarshalRawJSON())
	if err != nil {
		t.Fatalf("MarshalRawJSON array: %v", err)
	}
	if string(data) != `[]` {
		t.Fatalf("data: got %s", data)
	}
}

func TestMessage_RawMessage_UnmarshalRawJSON_Good(t *testing.T) {
	var raw RawMessage
	if err := resultErr(raw.UnmarshalRawJSON([]byte(`{"ok":true}`))); err != nil {
		t.Fatalf("UnmarshalRawJSON: %v", err)
	}
	if string(raw) != `{"ok":true}` {
		t.Fatalf("raw: got %s", raw)
	}
}

func TestMessage_RawMessage_UnmarshalRawJSON_Bad(t *testing.T) {
	var raw *RawMessage
	if err := resultErr(raw.UnmarshalRawJSON([]byte(`{"ok":true}`))); err == nil {
		t.Fatal("expected UnmarshalRawJSON nil pointer error")
	}
}

func TestMessage_RawMessage_UnmarshalRawJSON_Ugly(t *testing.T) {
	raw := RawMessage(`{"old":true}`)
	if err := resultErr(raw.UnmarshalRawJSON([]byte(`null`))); err != nil {
		t.Fatalf("UnmarshalRawJSON null: %v", err)
	}
	if string(raw) != `null` {
		t.Fatalf("raw: got %s", raw)
	}
}

func TestMessage_Message_Reply_Good(t *testing.T) {
	msg, err := resultValue[*Message](NewMessage(MsgPing, "from", "to", nil))
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	reply, err := resultValue[*Message](msg.Reply(MsgPong, PongPayload{SentAt: 1}))
	if err != nil || reply.ReplyTo != msg.ID {
		t.Fatalf("reply: %#v err=%v", reply, err)
	}
}

func TestMessage_Message_Reply_Bad(t *testing.T) {
	msg, err := resultValue[*Message](NewMessage(MsgPing, "from", "to", nil))
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	reply, err := resultValue[*Message](msg.Reply(MsgPong, func() {}))
	if err == nil || reply != nil {
		t.Fatalf("reply: %#v err=%v", reply, err)
	}
}

func TestMessage_Message_Reply_Ugly(t *testing.T) {
	msg := &Message{ID: "id-1"}
	reply, err := resultValue[*Message](msg.Reply("", nil))
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if reply.From != msg.To || reply.To != msg.From {
		t.Fatalf("reply routing: %#v", reply)
	}
}

func TestMessage_Message_ParsePayload_Good(t *testing.T) {
	msg, err := resultValue[*Message](NewMessage(MsgPing, "from", "to", PingPayload{SentAt: 123}))
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	var payload PingPayload
	if err := resultErr(msg.ParsePayload(&payload)); err != nil || payload.SentAt != 123 {
		t.Fatalf("payload: %#v err=%v", payload, err)
	}
}

func TestMessage_Message_ParsePayload_Bad(t *testing.T) {
	msg := &Message{Payload: RawMessage(`{`)}
	var payload PingPayload
	err := resultErr(msg.ParsePayload(&payload))
	if err == nil {
		t.Fatal("expected parse error")
	}
	if payload.SentAt != 0 {
		t.Fatalf("payload: %#v", payload)
	}
}

func TestMessage_Message_ParsePayload_Ugly(t *testing.T) {
	msg := &Message{}
	var payload PingPayload
	err := resultErr(msg.ParsePayload(&payload))
	if err != nil {
		t.Fatalf("ParsePayload nil: %v", err)
	}
	if payload.SentAt != 0 {
		t.Fatalf("payload: %#v", payload)
	}
}

func TestMessage_NewErrorMessage_Good(t *testing.T) {
	msg, err := resultValue[*Message](NewErrorMessage("from", "to", ErrCodeOperationFailed, "failed", "reply"))
	if err != nil {
		t.Fatalf("NewErrorMessage: %v", err)
	}
	if msg.Type != MsgError || msg.ReplyTo != "reply" {
		t.Fatalf("message: %#v", msg)
	}
}

func TestMessage_NewErrorMessage_Bad(t *testing.T) {
	msg, err := resultValue[*Message](NewErrorMessage("", "", ErrCodeUnknown, "", ""))
	if err != nil {
		t.Fatalf("NewErrorMessage: %v", err)
	}
	if msg.From != "" || msg.To != "" {
		t.Fatalf("message: %#v", msg)
	}
}

func TestMessage_NewErrorMessage_Ugly(t *testing.T) {
	msg, err := resultValue[*Message](NewErrorMessage("from", "to", -1, "custom", "id"))
	if err != nil {
		t.Fatalf("NewErrorMessage: %v", err)
	}
	var payload ErrorPayload
	if err := resultErr(msg.ParsePayload(&payload)); err != nil || payload.Code != -1 {
		t.Fatalf("payload: %#v err=%v", payload, err)
	}
}

func TestMessageReply(t *testing.T) {
	original, _ := resultValue[*Message](NewMessage(MsgPing, "sender", "receiver", PingPayload{SentAt: 12345}))

	reply, err := resultValue[*Message](original.Reply(MsgPong, PongPayload{
		SentAt:     12345,
		ReceivedAt: 12350,
	}))

	if err != nil {
		t.Fatalf("failed to create reply: %v", err)
	}

	if reply.ReplyTo != original.ID {
		t.Errorf("reply should reference original message ID")
	}

	if reply.From != original.To {
		t.Error("reply From should be original To")
	}

	if reply.To != original.From {
		t.Error("reply To should be original From")
	}

	if reply.Type != MsgPong {
		t.Errorf("expected type MsgPong, got %s", reply.Type)
	}
}

func TestParsePayload(t *testing.T) {
	t.Run("ValidPayload", func(t *testing.T) {
		payload := StartMinerPayload{
			MinerType: "xmrig",
			ProfileID: "test-profile",
		}

		msg, _ := resultValue[*Message](NewMessage(MsgStartMiner, "ctrl", "worker", payload))

		var parsed StartMinerPayload
		err := resultErr(msg.ParsePayload(&parsed))
		if err != nil {
			t.Fatalf("failed to parse payload: %v", err)
		}

		if parsed.ProfileID != "test-profile" {
			t.Errorf("expected ProfileID 'test-profile', got '%s'", parsed.ProfileID)
		}
	})

	t.Run("NilPayload", func(t *testing.T) {
		msg, _ := resultValue[*Message](NewMessage(MsgGetStats, "ctrl", "worker", nil))

		var parsed StatsPayload
		err := resultErr(msg.ParsePayload(&parsed))
		if err != nil {
			t.Errorf("parsing nil payload should not error: %v", err)
		}
	})

	t.Run("ComplexPayload", func(t *testing.T) {
		stats := StatsPayload{
			NodeID:   "node-123",
			NodeName: "Test Node",
			Miners: []MinerStatsItem{
				{
					Name:      "xmrig-1",
					Type:      "xmrig",
					Hashrate:  1234.56,
					Shares:    100,
					Rejected:  2,
					Uptime:    3600,
					Pool:      "pool.example.com:3333",
					Algorithm: "RandomX",
				},
			},
			Uptime: 86400,
		}

		msg, _ := resultValue[*Message](NewMessage(MsgStats, "worker", "ctrl", stats))

		var parsed StatsPayload
		err := resultErr(msg.ParsePayload(&parsed))
		if err != nil {
			t.Fatalf("failed to parse stats payload: %v", err)
		}

		if parsed.NodeID != "node-123" {
			t.Errorf("expected NodeID 'node-123', got '%s'", parsed.NodeID)
		}

		if len(parsed.Miners) != 1 {
			t.Fatalf("expected 1 miner, got %d", len(parsed.Miners))
		}

		if parsed.Miners[0].Hashrate != 1234.56 {
			t.Errorf("expected hashrate 1234.56, got %f", parsed.Miners[0].Hashrate)
		}
	})
}

func TestNewErrorMessage(t *testing.T) {
	errMsg, err := resultValue[*Message](NewErrorMessage("sender", "receiver", ErrCodeOperationFailed, "something went wrong", "original-msg-id"))
	if err != nil {
		t.Fatalf("failed to create error message: %v", err)
	}

	if errMsg.Type != MsgError {
		t.Errorf("expected type MsgError, got %s", errMsg.Type)
	}

	if errMsg.ReplyTo != "original-msg-id" {
		t.Errorf("expected ReplyTo 'original-msg-id', got '%s'", errMsg.ReplyTo)
	}

	var errPayload ErrorPayload
	err = resultErr(errMsg.ParsePayload(&errPayload))
	if err != nil {
		t.Fatalf("failed to parse error payload: %v", err)
	}

	if errPayload.Code != ErrCodeOperationFailed {
		t.Errorf("expected code %d, got %d", ErrCodeOperationFailed, errPayload.Code)
	}

	if errPayload.Message != "something went wrong" {
		t.Errorf("expected message 'something went wrong', got '%s'", errPayload.Message)
	}
}

func TestMessageSerialization(t *testing.T) {
	original, _ := resultValue[*Message](NewMessage(MsgStartMiner, "ctrl", "worker", StartMinerPayload{
		MinerType: "xmrig",
		ProfileID: "my-profile",
	}))

	// Serialize
	data, err := testJSONMarshal(original)
	if err != nil {
		t.Fatalf("failed to serialize message: %v", err)
	}

	// Deserialize
	var restored Message
	err = testJSONUnmarshal(data, &restored)
	if err != nil {
		t.Fatalf("failed to deserialize message: %v", err)
	}

	if restored.ID != original.ID {
		t.Error("ID mismatch after serialization")
	}

	if restored.Type != original.Type {
		t.Error("Type mismatch after serialization")
	}

	if restored.From != original.From {
		t.Error("From mismatch after serialization")
	}

	var payload StartMinerPayload
	err = resultErr(restored.ParsePayload(&payload))
	if err != nil {
		t.Fatalf("failed to parse restored payload: %v", err)
	}

	if payload.ProfileID != "my-profile" {
		t.Errorf("expected ProfileID 'my-profile', got '%s'", payload.ProfileID)
	}
}

func TestMessageTypes(t *testing.T) {
	types := []MessageType{
		MsgHandshake,
		MsgHandshakeAck,
		MsgPing,
		MsgPong,
		MsgDisconnect,
		MsgGetStats,
		MsgStats,
		MsgStartMiner,
		MsgStopMiner,
		MsgMinerAck,
		MsgDeploy,
		MsgDeployAck,
		MsgGetLogs,
		MsgLogs,
		MsgError,
	}

	for _, msgType := range types {
		t.Run(string(msgType), func(t *testing.T) {
			msg, err := resultValue[*Message](NewMessage(msgType, "from", "to", nil))
			if err != nil {
				t.Fatalf("failed to create message of type %s: %v", msgType, err)
			}

			if msg.Type != msgType {
				t.Errorf("expected type %s, got %s", msgType, msg.Type)
			}
		})
	}
}

func TestErrorCodes(t *testing.T) {
	codes := map[int]string{
		ErrCodeUnknown:         "Unknown",
		ErrCodeNotFound:        "NotFound",
		ErrCodeAlreadyRunning:  "AlreadyRunning",
		ErrCodeNotRunning:      "NotRunning",
		ErrCodeOperationFailed: "OperationFailed",
		ErrCodeInvalidConfig:   "InvalidConfig",
		ErrCodeAuthFailed:      "AuthFailed",
	}

	for code, name := range codes {
		t.Run(name, func(t *testing.T) {
			if code < 0 || code > 6 {
				t.Errorf("error code %d should be in 0-6 range", code)
			}
		})
	}
}

func TestNewMessage_NilPayload(t *testing.T) {
	msg, err := resultValue[*Message](NewMessage(MsgPing, "from", "to", nil))
	if err != nil {
		t.Fatalf("NewMessage with nil payload should succeed: %v", err)
	}
	if msg.Payload != nil {
		t.Error("payload should be nil for nil input")
	}
}

func TestMessage_ParsePayload_Nil(t *testing.T) {
	msg := &Message{Payload: nil}
	var target PingPayload
	err := resultErr(msg.ParsePayload(&target))
	if err != nil {
		t.Errorf("ParsePayload with nil payload should succeed: %v", err)
	}
}

func TestNewErrorMessage_Success(t *testing.T) {
	msg, err := resultValue[*Message](NewErrorMessage("from", "to", ErrCodeOperationFailed, "something went wrong", "reply-123"))
	if err != nil {
		t.Fatalf("NewErrorMessage failed: %v", err)
	}
	if msg.Type != MsgError {
		t.Errorf("expected type %s, got %s", MsgError, msg.Type)
	}
	if msg.ReplyTo != "reply-123" {
		t.Errorf("expected ReplyTo 'reply-123', got '%s'", msg.ReplyTo)
	}

	var payload ErrorPayload
	_ = resultErr(msg.ParsePayload(&payload))
	if payload.Code != ErrCodeOperationFailed {
		t.Errorf("expected code %d, got %d", ErrCodeOperationFailed, payload.Code)
	}
	if payload.Message != "something went wrong" {
		t.Errorf("expected message 'something went wrong', got '%s'", payload.Message)
	}
}
