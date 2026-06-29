// SPDX-License-Identifier: EUPL-1.2

package api

import (
	"net/http"
	"testing"

	core "dappco.re/go"
	p2pnode "dappco.re/go/p2p/node"
	"github.com/gin-gonic/gin"
)

// TestAnnouncePeer_Update re-announces an existing peer, exercising the
// GetPeer/UpdatePeer "updated" branch and the AddedAt/Score carry-over.
func TestAnnouncePeer_Update(t *testing.T) {
	registry := newProviderTestRegistry(t)
	if err := apiResultErr(registry.AddPeer(&p2pnode.Peer{ID: "peer-1", Name: "Original", Score: 73})); err != nil {
		t.Fatalf("seed peer: %v", err)
	}
	router := newProviderTestRouter(NewProvider(registry, nil))

	recorder := performProviderRequest(router, http.MethodPost, "/v1/p2p/peer/peer-1/announce", `{"name":"Updated"}`)
	body := decodeJSONBody(t, recorder)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d with body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if body["action"] != "updated" {
		t.Fatalf("action: got %#v, want updated", body["action"])
	}
	peer := registry.GetPeer("peer-1")
	if peer == nil {
		t.Fatal("expected peer to remain in registry")
	}
	if peer.Score != 73 {
		t.Fatalf("score should carry over from existing peer: got %v, want 73", peer.Score)
	}
	if peer.Name != "Updated" {
		t.Fatalf("name: got %q, want Updated", peer.Name)
	}
}

// TestAnnouncePeer_MissingRegistry returns a service error when the provider
// has no peer registry configured.
func TestAnnouncePeer_MissingRegistry(t *testing.T) {
	router := newProviderTestRouter(NewProvider(nil, nil))

	recorder := performProviderRequest(router, http.MethodPost, "/v1/p2p/peer/peer-1/announce", `{"name":"Agency"}`)
	body := decodeJSONBody(t, recorder)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want %d with body %s", recorder.Code, http.StatusServiceUnavailable, recorder.Body.String())
	}
	if body["code"] != codeProviderUnconfigured {
		t.Fatalf("code: got %#v, want %q", body["code"], codeProviderUnconfigured)
	}
}

// TestAnnouncePeer_InvalidBody rejects a malformed JSON announcement body.
func TestAnnouncePeer_InvalidBody(t *testing.T) {
	router := newProviderTestRouter(NewProvider(newProviderTestRegistry(t), nil))

	recorder := performProviderRequest(router, http.MethodPost, "/v1/p2p/peer/peer-1/announce", `{`)
	body := decodeJSONBody(t, recorder)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d with body %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if body["code"] != codeInvalidRequest {
		t.Fatalf("code: got %#v, want %q", body["code"], codeInvalidRequest)
	}
}

// TestAnnouncePeer_RejectedNewPeer surfaces a registry AddPeer rejection (here,
// an invalid peer name) as codePeerRejected.
func TestAnnouncePeer_RejectedNewPeer(t *testing.T) {
	router := newProviderTestRouter(NewProvider(newProviderTestRegistry(t), nil))

	recorder := performProviderRequest(router, http.MethodPost, "/v1/p2p/peer/peer-1/announce", `{"name":"bad@@name!!"}`)
	body := decodeJSONBody(t, recorder)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d with body %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if body["code"] != codePeerRejected {
		t.Fatalf("code: got %#v, want %q", body["code"], codePeerRejected)
	}
}

// TestUploadEncryptedBlob_EmptyBlob rejects a syntactically-valid request that
// omits the required encryptedBlob field.
func TestUploadEncryptedBlob_EmptyBlob(t *testing.T) {
	router := newProviderTestRouter(NewProvider(newProviderTestRegistry(t), nil))

	recorder := performProviderRequest(router, http.MethodPost, "/v1/p2p/upload", `{"topic":"agency.sync","encryptedBlob":"   "}`)
	body := decodeJSONBody(t, recorder)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d with body %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if body["code"] != codeInvalidRequest {
		t.Fatalf("code: got %#v, want %q", body["code"], codeInvalidRequest)
	}
}

// TestHealth_IdleSwarm reports "idle" when peers are known but none connected.
func TestHealth_IdleSwarm(t *testing.T) {
	registry := newProviderTestRegistry(t)
	if err := apiResultErr(registry.AddPeer(&p2pnode.Peer{ID: "peer-1", Name: "Agency One"})); err != nil {
		t.Fatalf("add peer: %v", err)
	}
	router := newProviderTestRouter(NewProvider(registry, nil))

	recorder := performProviderRequest(router, http.MethodGet, "/v1/p2p/health", "")
	body := decodeJSONBody(t, recorder)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d with body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if body["swarmState"] != "idle" {
		t.Fatalf("swarmState: got %#v, want idle", body["swarmState"])
	}
}

// TestHealth_EmptySwarm reports "empty" when the registry has no peers.
func TestHealth_EmptySwarm(t *testing.T) {
	router := newProviderTestRouter(NewProvider(newProviderTestRegistry(t), nil))

	recorder := performProviderRequest(router, http.MethodGet, "/v1/p2p/health", "")
	body := decodeJSONBody(t, recorder)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d with body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if body["swarmState"] != "empty" {
		t.Fatalf("swarmState: got %#v, want empty", body["swarmState"])
	}
}

// TestHealth_TransportConnectedCount uses the transport's connected-peer count
// in preference to the registry's when a transport is configured.
func TestHealth_TransportConnectedCount(t *testing.T) {
	registry := newProviderTestRegistry(t)
	if err := apiResultErr(registry.AddPeer(&p2pnode.Peer{ID: "peer-1", Name: "Agency One"})); err != nil {
		t.Fatalf("add peer: %v", err)
	}
	transport := p2pnode.NewTransport(nil, registry, p2pnode.DefaultTransportConfig())
	router := newProviderTestRouter(NewProvider(registry, transport))

	recorder := performProviderRequest(router, http.MethodGet, "/v1/p2p/health", "")
	body := decodeJSONBody(t, recorder)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d with body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	// A freshly-created transport has no connections.
	if body["connectedPeers"] != float64(0) {
		t.Fatalf("connectedPeers: got %#v, want 0", body["connectedPeers"])
	}
}

// TestWriteError_NilContext is a no-op guard that must not panic.
func TestWriteError_NilContext(t *testing.T) {
	writeError(nil, http.StatusBadRequest, codeInvalidRequest, "msg")
}

// TestWriteNotImplemented_NilContext is a no-op guard that must not panic.
func TestWriteNotImplemented_NilContext(t *testing.T) {
	writeNotImplemented(nil, "op", "todo")
}

// TestHandlers_NilContext confirms each handler tolerates a nil gin context.
func TestHandlers_NilContext(t *testing.T) {
	p := NewProvider(newProviderTestRegistry(t), nil)
	p.uploadEncryptedBlob(nil)
	p.syncTopicEvents(nil)
	p.announcePeer(nil)
	p.health(nil)
	p.listPeers(nil)
}

// ExampleNewProvider_routeCount shows the route-mount entry point yields the
// five P2P HTTP routes.
func ExampleNewProvider_routeCount() {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewProvider(nil, nil).RegisterRoutes(router.Group("/v1/p2p"))
	core.Println(len(router.Routes()))
	// Output: 5
}
