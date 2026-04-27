// SPDX-License-Identifier: EUPL-1.2

package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	coreapi "dappco.re/go/api"
	coreprovider "dappco.re/go/api/pkg/provider"
	p2pnode "dappco.re/go/p2p/node"
	"github.com/gin-gonic/gin"
)

var (
	_ coreprovider.Provider    = (*P2PProvider)(nil)
	_ coreapi.DescribableGroup = (*P2PProvider)(nil)
)

func TestNewProvider_Good(t *testing.T) {
	registry := newProviderTestRegistry(t)
	provider := NewProvider(registry, nil)
	if provider == nil {
		t.Fatal("expected provider")
	}

	var frameworkProvider coreprovider.Provider = provider
	if frameworkProvider.Name() != "p2p" {
		t.Fatalf("provider name: got %q, want %q", frameworkProvider.Name(), "p2p")
	}
	if frameworkProvider.BasePath() != "/v1/p2p" {
		t.Fatalf("provider base path: got %q, want %q", frameworkProvider.BasePath(), "/v1/p2p")
	}

	descriptions := provider.Describe()
	if len(descriptions) != 5 {
		t.Fatalf("route description count: got %d, want 5", len(descriptions))
	}

	router := newProviderTestRouter(provider)
	recorder := performProviderRequest(router, http.MethodGet, "/v1/p2p/health", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("health route status: got %d, want %d with body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}

func TestNewProvider_Bad(t *testing.T) {
	provider := NewProvider(nil, nil)
	router := newProviderTestRouter(provider)

	recorder := performProviderRequest(router, http.MethodGet, "/v1/p2p/health", "")
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("health route status: got %d, want %d with body %s", recorder.Code, http.StatusServiceUnavailable, recorder.Body.String())
	}
}

func TestNewProvider_Ugly(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("nil provider route registration panicked: %v", recovered)
		}
	}()

	var provider *P2PProvider
	provider.RegisterRoutes(nil)
	if provider.Name() != "p2p" {
		t.Fatalf("nil provider name: got %q, want %q", provider.Name(), "p2p")
	}
}

func newProviderTestRegistry(t *testing.T) *p2pnode.PeerRegistry {
	t.Helper()
	registry, err := p2pnode.NewPeerRegistryWithPath(filepath.Join(t.TempDir(), "peers.json"))
	if err != nil {
		t.Fatalf("create peer registry: %v", err)
	}
	t.Cleanup(func() {
		if err := registry.Close(); err != nil {
			t.Fatalf("close peer registry: %v", err)
		}
	})
	return registry
}

func newProviderTestRouter(provider *P2PProvider) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	if provider != nil {
		provider.RegisterRoutes(router.Group(provider.BasePath()))
	}
	return router
}

func performProviderRequest(router *gin.Engine, method string, path string, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}
