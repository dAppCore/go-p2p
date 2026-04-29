// SPDX-License-Identifier: EUPL-1.2

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "dappco.re/go"
	p2pnode "dappco.re/go/p2p/node"
)

func TestUploadEncryptedBlob_Good(t *testing.T) {
	router := newProviderTestRouter(NewProvider(newProviderTestRegistry(t), nil))

	recorder := performProviderRequest(router, http.MethodPost, "/v1/p2p/upload", `{"topic":"agency.sync","encryptedBlob":"Y2lwaGVydGV4dA=="}`)
	body := decodeJSONBody(t, recorder)

	if recorder.Code != http.StatusNotImplemented {
		t.Fatalf("status: got %d, want %d with body %s", recorder.Code, http.StatusNotImplemented, recorder.Body.String())
	}
	if body["code"] != codeNotImplemented {
		t.Fatalf("code: got %#v, want %q", body["code"], codeNotImplemented)
	}
	if body["operation"] != "upload" {
		t.Fatalf("operation: got %#v, want upload", body["operation"])
	}
	if body["todo"] == "" {
		t.Fatal("expected todo field")
	}
}

func TestUploadEncryptedBlob_Bad(t *testing.T) {
	router := newProviderTestRouter(NewProvider(newProviderTestRegistry(t), nil))

	recorder := performProviderRequest(router, http.MethodPost, "/v1/p2p/upload", `{`)
	body := decodeJSONBody(t, recorder)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d with body %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if body["code"] != codeInvalidRequest {
		t.Fatalf("code: got %#v, want %q", body["code"], codeInvalidRequest)
	}
}

func TestSyncTopicEvents_Good(t *testing.T) {
	router := newProviderTestRouter(NewProvider(newProviderTestRegistry(t), nil))

	recorder := performProviderRequest(router, http.MethodGet, "/v1/p2p/sync/agency.sync", "")
	body := decodeJSONBody(t, recorder)

	if recorder.Code != http.StatusNotImplemented {
		t.Fatalf("status: got %d, want %d with body %s", recorder.Code, http.StatusNotImplemented, recorder.Body.String())
	}
	if body["code"] != codeNotImplemented {
		t.Fatalf("code: got %#v, want %q", body["code"], codeNotImplemented)
	}
	if body["topic"] != "agency.sync" {
		t.Fatalf("topic: got %#v, want agency.sync", body["topic"])
	}
}

func TestSyncTopicEvents_Bad(t *testing.T) {
	router := newProviderTestRouter(NewProvider(newProviderTestRegistry(t), nil))

	recorder := performProviderRequest(router, http.MethodGet, "/v1/p2p/sync/%20", "")
	body := decodeJSONBody(t, recorder)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d with body %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if body["code"] != codeInvalidTopic {
		t.Fatalf("code: got %#v, want %q", body["code"], codeInvalidTopic)
	}
}

func TestAnnouncePeer_Good(t *testing.T) {
	registry := newProviderTestRegistry(t)
	router := newProviderTestRouter(NewProvider(registry, nil))

	recorder := performProviderRequest(router, http.MethodPost, "/v1/p2p/peer/peer-1/announce", `{"name":"Agency One","publicKey":"pub","address":"127.0.0.1:9091","role":"worker"}`)
	body := decodeJSONBody(t, recorder)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d with body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if body["ok"] != true {
		t.Fatalf("ok: got %#v, want true", body["ok"])
	}
	if body["action"] != "announced" {
		t.Fatalf("action: got %#v, want announced", body["action"])
	}
	if peer := registry.GetPeer("peer-1"); peer == nil {
		t.Fatal("expected peer in registry")
	}
}

func TestAnnouncePeer_Bad(t *testing.T) {
	router := newProviderTestRouter(NewProvider(newProviderTestRegistry(t), nil))

	recorder := performProviderRequest(router, http.MethodPost, "/v1/p2p/peer/peer-1/announce", `{"id":"peer-2","name":"Agency One"}`)
	body := decodeJSONBody(t, recorder)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d with body %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if body["code"] != codePeerMismatch {
		t.Fatalf("code: got %#v, want %q", body["code"], codePeerMismatch)
	}
}

func TestHealth_Good(t *testing.T) {
	registry := newProviderTestRegistry(t)
	if err := registry.AddPeer(&p2pnode.Peer{ID: "peer-1", Name: "Agency One"}); err != nil {
		t.Fatalf("add peer: %v", err)
	}
	registry.SetConnected("peer-1", true)
	router := newProviderTestRouter(NewProvider(registry, nil))

	recorder := performProviderRequest(router, http.MethodGet, "/v1/p2p/health", "")
	body := decodeJSONBody(t, recorder)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d with body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if body["ok"] != true {
		t.Fatalf("ok: got %#v, want true", body["ok"])
	}
	if body["peerCount"] != float64(1) {
		t.Fatalf("peerCount: got %#v, want 1", body["peerCount"])
	}
	if body["connectedPeers"] != float64(1) {
		t.Fatalf("connectedPeers: got %#v, want 1", body["connectedPeers"])
	}
	if body["swarmState"] != "connected" {
		t.Fatalf("swarmState: got %#v, want connected", body["swarmState"])
	}
}

func TestHealth_Bad(t *testing.T) {
	router := newProviderTestRouter(NewProvider(nil, nil))

	recorder := performProviderRequest(router, http.MethodGet, "/v1/p2p/health", "")
	body := decodeJSONBody(t, recorder)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want %d with body %s", recorder.Code, http.StatusServiceUnavailable, recorder.Body.String())
	}
	if body["swarmState"] != "unconfigured" {
		t.Fatalf("swarmState: got %#v, want unconfigured", body["swarmState"])
	}
	if body["code"] != codeProviderUnconfigured {
		t.Fatalf("code: got %#v, want %q", body["code"], codeProviderUnconfigured)
	}
}

func TestListPeers_Good(t *testing.T) {
	registry := newProviderTestRegistry(t)
	if err := registry.AddPeer(&p2pnode.Peer{ID: "peer-1", Name: "Agency One"}); err != nil {
		t.Fatalf("add peer one: %v", err)
	}
	if err := registry.AddPeer(&p2pnode.Peer{ID: "peer-2", Name: "Agency Two"}); err != nil {
		t.Fatalf("add peer two: %v", err)
	}
	router := newProviderTestRouter(NewProvider(registry, nil))

	recorder := performProviderRequest(router, http.MethodGet, "/v1/p2p/peers", "")
	body := decodeJSONBody(t, recorder)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d with body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if body["count"] != float64(2) {
		t.Fatalf("count: got %#v, want 2", body["count"])
	}
	peers, ok := body["peers"].([]any)
	if !ok {
		t.Fatalf("peers: got %#v, want array", body["peers"])
	}
	if len(peers) != 2 {
		t.Fatalf("peers length: got %d, want 2", len(peers))
	}
}

func TestListPeers_Bad(t *testing.T) {
	router := newProviderTestRouter(NewProvider(nil, nil))

	recorder := performProviderRequest(router, http.MethodGet, "/v1/p2p/peers", "")
	body := decodeJSONBody(t, recorder)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want %d with body %s", recorder.Code, http.StatusServiceUnavailable, recorder.Body.String())
	}
	if body["code"] != codeProviderUnconfigured {
		t.Fatalf("code: got %#v, want %q", body["code"], codeProviderUnconfigured)
	}
}

func decodeJSONBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if r := core.JSONUnmarshal(recorder.Body.Bytes(), &body); !r.OK {
		t.Fatalf("decode response body: %v", r.Value)
	}
	return body
}
