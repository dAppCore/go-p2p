package node

import (
	core "dappco.re/go"
	coreerr "dappco.re/go/log"
)

// ProtocolError represents an error from the remote peer.
type ProtocolError struct {
	Code    int
	Message string
}

func (e *ProtocolError) Error() string {
	return core.Sprintf("remote error (%d): %s", e.Code, e.Message)
}

// ResponseHandler provides helpers for handling protocol responses.
type ResponseHandler struct{}

// ValidateResponse checks if the response is valid and returns a parsed error if it's an error response.
// It checks:
// 1. If response is nil (returns error)
// 2. If response is an error message (returns ProtocolError)
// 3. If response type matches expected (returns error if not)
func (h *ResponseHandler) ValidateResponse(resp *Message, expectedType MessageType) core.Result {
	if resp == nil {
		return core.Fail(coreerr.E("ResponseHandler.ValidateResponse", "nil response", nil))
	}

	// Check for error response
	if resp.Type == MsgError {
		var errPayload ErrorPayload
		if r := resp.ParsePayload(&errPayload); !r.OK {
			return core.Fail(&ProtocolError{Code: ErrCodeUnknown, Message: "unable to parse error response"})
		}
		return core.Fail(&ProtocolError{Code: errPayload.Code, Message: errPayload.Message})
	}

	// Check expected type
	if resp.Type != expectedType {
		return core.Fail(coreerr.E("ResponseHandler.ValidateResponse", "unexpected response type: expected "+string(expectedType)+", got "+string(resp.Type), nil))
	}

	return core.Ok(nil)
}

// ParseResponse validates the response and parses the payload into the target.
// This combines ValidateResponse and ParsePayload into a single call.
func (h *ResponseHandler) ParseResponse(resp *Message, expectedType MessageType, target any) core.Result {
	if r := h.ValidateResponse(resp, expectedType); !r.OK {
		return r
	}

	if target != nil {
		if r := resp.ParsePayload(target); !r.OK {
			err, _ := r.Value.(error)
			return core.Fail(coreerr.E("ResponseHandler.ParseResponse", "failed to parse "+string(expectedType)+" payload", err))
		}
	}

	return core.Ok(nil)
}

// DefaultResponseHandler is the default response handler instance.
var DefaultResponseHandler = &ResponseHandler{}

// ValidateResponse is a convenience function using the default handler.
func ValidateResponse(resp *Message, expectedType MessageType) core.Result {
	return DefaultResponseHandler.ValidateResponse(resp, expectedType)
}

// ParseResponse is a convenience function using the default handler.
func ParseResponse(resp *Message, expectedType MessageType, target any) core.Result {
	return DefaultResponseHandler.ParseResponse(resp, expectedType, target)
}

// IsProtocolError returns true if the error is a ProtocolError.
func IsProtocolError(err error) bool {
	_, ok := err.(*ProtocolError)
	return ok
}

// GetProtocolErrorCode returns the error code if err is a ProtocolError, otherwise returns 0.
func GetProtocolErrorCode(err error) int {
	if pe, ok := err.(*ProtocolError); ok {
		return pe.Code
	}
	return 0
}
