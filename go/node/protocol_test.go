package node

import (
	core "dappco.re/go"
	"testing"
)

func TestResponseHandler_ValidateResponse(t *testing.T) {
	handler := &ResponseHandler{}

	t.Run("NilResponse", func(t *testing.T) {
		err := resultErr(handler.ValidateResponse(nil, MsgStats))
		if err == nil {
			t.Error("Expected error for nil response")
		}
	})

	t.Run("ErrorResponse", func(t *testing.T) {
		errMsg, _ := resultValue[*Message](NewErrorMessage("sender", "receiver", ErrCodeOperationFailed, "operation failed", ""))
		err := resultErr(handler.ValidateResponse(errMsg, MsgStats))
		if err == nil {
			t.Fatal("Expected error for error response")
		}

		if !IsProtocolError(err) {
			t.Errorf("Expected ProtocolError, got %T", err)
		}

		if GetProtocolErrorCode(err) != ErrCodeOperationFailed {
			t.Errorf("Expected code %d, got %d", ErrCodeOperationFailed, GetProtocolErrorCode(err))
		}
	})

	t.Run("WrongType", func(t *testing.T) {
		msg, _ := resultValue[*Message](NewMessage(MsgPong, "sender", "receiver", nil))
		err := resultErr(handler.ValidateResponse(msg, MsgStats))
		if err == nil {
			t.Error("Expected error for wrong type")
		}
		if IsProtocolError(err) {
			t.Error("Should not be a ProtocolError for type mismatch")
		}
	})

	t.Run("ValidResponse", func(t *testing.T) {
		msg, _ := resultValue[*Message](NewMessage(MsgStats, "sender", "receiver", StatsPayload{NodeID: "test"}))
		err := resultErr(handler.ValidateResponse(msg, MsgStats))
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})
}

func TestProtocol_ProtocolError_Error_Good(t *testing.T) {
	err := &ProtocolError{Code: 4, Message: "failed"}
	if err.Error() != "remote error (4): failed" {
		t.Fatalf("error: got %q", err.Error())
	}
	if !IsProtocolError(err) {
		t.Fatal("expected protocol error")
	}
}

func TestProtocol_ProtocolError_Error_Bad(t *testing.T) {
	err := &ProtocolError{}
	if err.Error() != "remote error (0): " {
		t.Fatalf("error: got %q", err.Error())
	}
	if GetProtocolErrorCode(err) != 0 {
		t.Fatal("expected zero code")
	}
}

func TestProtocol_ProtocolError_Error_Ugly(t *testing.T) {
	err := &ProtocolError{Code: -1, Message: "x"}
	if err.Error() != "remote error (-1): x" {
		t.Fatalf("error: got %q", err.Error())
	}
	if GetProtocolErrorCode(err) != -1 {
		t.Fatal("expected negative code")
	}
}

func TestProtocol_ResponseHandler_ValidateResponse_Good(t *testing.T) {
	handler := &ResponseHandler{}
	msg, err := resultValue[*Message](NewMessage(MsgStats, "from", "to", StatsPayload{}))
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	if err := resultErr(handler.ValidateResponse(msg, MsgStats)); err != nil {
		t.Fatalf("ValidateResponse: %v", err)
	}
}

func TestProtocol_ResponseHandler_ValidateResponse_Bad(t *testing.T) {
	handler := &ResponseHandler{}
	err := resultErr(handler.ValidateResponse(nil, MsgStats))
	if err == nil {
		t.Fatal("expected nil response error")
	}
	if IsProtocolError(err) {
		t.Fatal("nil response should not be protocol error")
	}
}

func TestProtocol_ResponseHandler_ValidateResponse_Ugly(t *testing.T) {
	handler := &ResponseHandler{}
	msg, _ := resultValue[*Message](NewErrorMessage("from", "to", ErrCodeAuthFailed, "denied", ""))
	err := resultErr(handler.ValidateResponse(msg, MsgStats))
	if !IsProtocolError(err) {
		t.Fatalf("expected protocol error, got %T", err)
	}
	if GetProtocolErrorCode(err) != ErrCodeAuthFailed {
		t.Fatalf("code: got %d", GetProtocolErrorCode(err))
	}
}

func TestProtocol_ResponseHandler_ParseResponse_Good(t *testing.T) {
	handler := &ResponseHandler{}
	msg, _ := resultValue[*Message](NewMessage(MsgStats, "from", "to", StatsPayload{NodeID: "node"}))
	var payload StatsPayload
	err := resultErr(handler.ParseResponse(msg, MsgStats, &payload))
	if err != nil || payload.NodeID != "node" {
		t.Fatalf("payload: %#v err=%v", payload, err)
	}
}

func TestProtocol_ResponseHandler_ParseResponse_Bad(t *testing.T) {
	handler := &ResponseHandler{}
	msg, _ := resultValue[*Message](NewMessage(MsgPong, "from", "to", nil))
	err := resultErr(handler.ParseResponse(msg, MsgStats, &StatsPayload{}))
	if err == nil {
		t.Fatal("expected wrong type error")
	}
	if IsProtocolError(err) {
		t.Fatal("wrong type should not be protocol error")
	}
}

func TestProtocol_ResponseHandler_ParseResponse_Ugly(t *testing.T) {
	handler := &ResponseHandler{}
	msg, _ := resultValue[*Message](NewMessage(MsgStats, "from", "to", StatsPayload{NodeID: "node"}))
	err := resultErr(handler.ParseResponse(msg, MsgStats, nil))
	if err != nil {
		t.Fatalf("ParseResponse nil target: %v", err)
	}
}

// TestProtocol_ResponseHandler_ParseResponse_PayloadParseFail covers the
// branch where the response type matches but its payload cannot unmarshal into
// the target.
func TestProtocol_ResponseHandler_ParseResponse_PayloadParseFail(t *testing.T) {
	handler := &ResponseHandler{}
	// Correct type, but payload is a JSON string that cannot decode into a struct.
	msg := &Message{Type: MsgStats, From: "from", To: "to", Payload: RawMessage(`"not-a-stats-object"`)}
	err := resultErr(handler.ParseResponse(msg, MsgStats, &StatsPayload{}))
	if err == nil {
		t.Fatal("expected payload-parse failure")
	}
	if IsProtocolError(err) {
		t.Fatal("payload parse failure should not be a protocol error")
	}
}

func TestProtocol_ValidateResponse_Good(t *testing.T) {
	msg, err := resultValue[*Message](NewMessage(MsgPong, "from", "to", PongPayload{}))
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	if err := resultErr(ValidateResponse(msg, MsgPong)); err != nil {
		t.Fatalf("ValidateResponse: %v", err)
	}
}

func TestProtocol_ValidateResponse_Bad(t *testing.T) {
	err := resultErr(ValidateResponse(nil, MsgPong))
	if err == nil {
		t.Fatal("expected nil response error")
	}
	if IsProtocolError(err) {
		t.Fatal("nil response should not be protocol error")
	}
}

func TestProtocol_ValidateResponse_Ugly(t *testing.T) {
	msg, _ := resultValue[*Message](NewErrorMessage("from", "to", ErrCodeTimeout, "timeout", ""))
	err := resultErr(ValidateResponse(msg, MsgPong))
	if !IsProtocolError(err) {
		t.Fatalf("expected protocol error, got %T", err)
	}
	if GetProtocolErrorCode(err) != ErrCodeTimeout {
		t.Fatalf("code: got %d", GetProtocolErrorCode(err))
	}
}

func TestProtocol_ParseResponse_Good(t *testing.T) {
	msg, _ := resultValue[*Message](NewMessage(MsgPong, "from", "to", PongPayload{SentAt: 5}))
	var payload PongPayload
	err := resultErr(ParseResponse(msg, MsgPong, &payload))
	if err != nil || payload.SentAt != 5 {
		t.Fatalf("payload: %#v err=%v", payload, err)
	}
}

func TestProtocol_ParseResponse_Bad(t *testing.T) {
	msg, _ := resultValue[*Message](NewMessage(MsgPing, "from", "to", nil))
	err := resultErr(ParseResponse(msg, MsgPong, &PongPayload{}))
	if err == nil {
		t.Fatal("expected wrong type error")
	}
	if IsProtocolError(err) {
		t.Fatal("wrong type should not be protocol error")
	}
}

func TestProtocol_ParseResponse_Ugly(t *testing.T) {
	msg, _ := resultValue[*Message](NewMessage(MsgPong, "from", "to", nil))
	err := resultErr(ParseResponse(msg, MsgPong, nil))
	if err != nil {
		t.Fatalf("ParseResponse nil target: %v", err)
	}
}

func TestProtocol_IsProtocolError_Good(t *testing.T) {
	err := &ProtocolError{Code: 1, Message: "missing"}
	if !IsProtocolError(err) {
		t.Fatal("expected protocol error")
	}
	if core.Sprint(err) == "" {
		t.Fatal("expected error text")
	}
}

func TestProtocol_IsProtocolError_Bad(t *testing.T) {
	err := core.Errorf("plain")
	if IsProtocolError(err) {
		t.Fatal("plain error should not be protocol error")
	}
	if err.Error() != "plain" {
		t.Fatal("plain error changed")
	}
}

func TestProtocol_IsProtocolError_Ugly(t *testing.T) {
	if IsProtocolError(nil) {
		t.Fatal("nil should not be protocol error")
	}
	if GetProtocolErrorCode(nil) != 0 {
		t.Fatal("nil code should be zero")
	}
}

func TestProtocol_GetProtocolErrorCode_Good(t *testing.T) {
	code := GetProtocolErrorCode(&ProtocolError{Code: 7, Message: "x"})
	if code != 7 {
		t.Fatalf("code: got %d", code)
	}
	if code == 0 {
		t.Fatal("expected non-zero code")
	}
}

func TestProtocol_GetProtocolErrorCode_Bad(t *testing.T) {
	code := GetProtocolErrorCode(core.Errorf("plain"))
	if code != 0 {
		t.Fatalf("code: got %d", code)
	}
	if IsProtocolError(core.Errorf("plain")) {
		t.Fatal("plain error should not be protocol error")
	}
}

func TestProtocol_GetProtocolErrorCode_Ugly(t *testing.T) {
	code := GetProtocolErrorCode(nil)
	if code != 0 {
		t.Fatalf("code: got %d", code)
	}
	if IsProtocolError(nil) {
		t.Fatal("nil should not be protocol error")
	}
}

func TestResponseHandler_ParseResponse(t *testing.T) {
	handler := &ResponseHandler{}

	t.Run("ParseStats", func(t *testing.T) {
		payload := StatsPayload{
			NodeID:   "node-123",
			NodeName: "Test Node",
			Uptime:   3600,
		}
		msg, _ := resultValue[*Message](NewMessage(MsgStats, "sender", "receiver", payload))

		var parsed StatsPayload
		err := resultErr(handler.ParseResponse(msg, MsgStats, &parsed))
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if parsed.NodeID != "node-123" {
			t.Errorf("Expected NodeID 'node-123', got '%s'", parsed.NodeID)
		}
		if parsed.Uptime != 3600 {
			t.Errorf("Expected Uptime 3600, got %d", parsed.Uptime)
		}
	})

	t.Run("ParseMinerAck", func(t *testing.T) {
		payload := MinerAckPayload{
			Success:   true,
			MinerName: "xmrig-1",
		}
		msg, _ := resultValue[*Message](NewMessage(MsgMinerAck, "sender", "receiver", payload))

		var parsed MinerAckPayload
		err := resultErr(handler.ParseResponse(msg, MsgMinerAck, &parsed))
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if !parsed.Success {
			t.Error("Expected Success to be true")
		}
		if parsed.MinerName != "xmrig-1" {
			t.Errorf("Expected MinerName 'xmrig-1', got '%s'", parsed.MinerName)
		}
	})

	t.Run("ErrorResponse", func(t *testing.T) {
		errMsg, _ := resultValue[*Message](NewErrorMessage("sender", "receiver", ErrCodeNotFound, "not found", ""))

		var parsed StatsPayload
		err := resultErr(handler.ParseResponse(errMsg, MsgStats, &parsed))
		if err == nil {
			t.Error("Expected error for error response")
		}
		if !IsProtocolError(err) {
			t.Errorf("Expected ProtocolError, got %T", err)
		}
	})

	t.Run("NilTarget", func(t *testing.T) {
		msg, _ := resultValue[*Message](NewMessage(MsgPong, "sender", "receiver", nil))
		err := resultErr(handler.ParseResponse(msg, MsgPong, nil))
		if err != nil {
			t.Errorf("Unexpected error with nil target: %v", err)
		}
	})
}

func TestProtocolError(t *testing.T) {
	err := &ProtocolError{Code: 1001, Message: "test error"}

	if err.Error() != "remote error (1001): test error" {
		t.Errorf("Unexpected error message: %s", err.Error())
	}

	if !IsProtocolError(err) {
		t.Error("IsProtocolError should return true")
	}

	if GetProtocolErrorCode(err) != 1001 {
		t.Errorf("Expected code 1001, got %d", GetProtocolErrorCode(err))
	}
}

func TestConvenienceFunctions(t *testing.T) {
	msg, _ := resultValue[*Message](NewMessage(MsgStats, "sender", "receiver", StatsPayload{NodeID: "test"}))

	// Test ValidateResponse
	if err := resultErr(ValidateResponse(msg, MsgStats)); err != nil {
		t.Errorf("ValidateResponse failed: %v", err)
	}

	// Test ParseResponse
	var parsed StatsPayload
	if err := resultErr(ParseResponse(msg, MsgStats, &parsed)); err != nil {
		t.Errorf("ParseResponse failed: %v", err)
	}
	if parsed.NodeID != "test" {
		t.Errorf("Expected NodeID 'test', got '%s'", parsed.NodeID)
	}
}

func TestGetProtocolErrorCode_NonProtocolError(t *testing.T) {
	err := core.Errorf("regular error")
	if GetProtocolErrorCode(err) != 0 {
		t.Error("Expected 0 for non-ProtocolError")
	}
}
