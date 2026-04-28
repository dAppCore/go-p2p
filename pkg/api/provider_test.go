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

func TestProvider_NewProvider_Good(t *testing.T) {
	registry := newProviderTestRegistry(t)
	provider := NewProvider(registry, nil)
	if provider == nil {
		t.Fatal("expected provider")
	}
	if provider.registry != registry {
		t.Fatal("registry not retained")
	}
}

func TestProvider_NewProvider_Bad(t *testing.T) {
	provider := NewProvider(nil, nil)
	if provider == nil {
		t.Fatal("expected provider")
	}
	if provider.registry != nil {
		t.Fatal("expected nil registry")
	}
}

func TestProvider_NewProvider_Ugly(t *testing.T) {
	registry := newProviderTestRegistry(t)
	transport := p2pnode.NewTransport(nil, registry, p2pnode.DefaultTransportConfig())
	provider := NewProvider(registry, transport)
	if provider.transport != transport {
		t.Fatal("transport not retained")
	}
}

func TestProvider_P2PProvider_Name_Good(t *testing.T) {
	provider := NewProvider(nil, nil)
	if provider.Name() != "p2p" {
		t.Fatalf("name: got %q", provider.Name())
	}
	if provider.Name() == "" {
		t.Fatal("expected non-empty name")
	}
}

func TestProvider_P2PProvider_Name_Bad(t *testing.T) {
	var provider *P2PProvider
	got := provider.Name()
	if got != "p2p" {
		t.Fatalf("name: got %q", got)
	}
}

func TestProvider_P2PProvider_Name_Ugly(t *testing.T) {
	provider := NewProvider(newProviderTestRegistry(t), nil)
	got := provider.Name()
	if strings.TrimSpace(got) != got {
		t.Fatalf("name has whitespace: %q", got)
	}
}

func TestProvider_P2PProvider_BasePath_Good(t *testing.T) {
	provider := NewProvider(nil, nil)
	if provider.BasePath() != "/v1/p2p" {
		t.Fatalf("base path: got %q", provider.BasePath())
	}
	if provider.BasePath()[0] != '/' {
		t.Fatal("base path should be absolute")
	}
}

func TestProvider_P2PProvider_BasePath_Bad(t *testing.T) {
	var provider *P2PProvider
	got := provider.BasePath()
	if got != "/v1/p2p" {
		t.Fatalf("base path: got %q", got)
	}
}

func TestProvider_P2PProvider_BasePath_Ugly(t *testing.T) {
	provider := NewProvider(newProviderTestRegistry(t), nil)
	got := provider.BasePath()
	if strings.Contains(got, "//") {
		t.Fatalf("base path contains duplicate slash: %q", got)
	}
}

func TestProvider_P2PProvider_RegisterRoutes_Good(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/v1/p2p")
	NewProvider(newProviderTestRegistry(t), nil).RegisterRoutes(group)
	routes := router.Routes()
	if len(routes) != 5 {
		t.Fatalf("route count: got %d", len(routes))
	}
}

func TestProvider_P2PProvider_RegisterRoutes_Bad(t *testing.T) {
	provider := NewProvider(nil, nil)
	provider.RegisterRoutes(nil)
	if provider.Name() != "p2p" {
		t.Fatal("provider changed after nil route group")
	}
}

func TestProvider_P2PProvider_RegisterRoutes_Ugly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("")
	var provider *P2PProvider
	provider.RegisterRoutes(group)
	if len(router.Routes()) != 0 {
		t.Fatalf("route count: got %d", len(router.Routes()))
	}
}

func TestProvider_P2PProvider_Describe_Good(t *testing.T) {
	provider := NewProvider(nil, nil)
	descriptions := provider.Describe()
	if len(descriptions) != 5 {
		t.Fatalf("description count: got %d", len(descriptions))
	}
}

func TestProvider_P2PProvider_Describe_Bad(t *testing.T) {
	var provider *P2PProvider
	descriptions := provider.Describe()
	if len(descriptions) != 5 {
		t.Fatalf("description count: got %d", len(descriptions))
	}
}

func TestProvider_P2PProvider_Describe_Ugly(t *testing.T) {
	provider := NewProvider(newProviderTestRegistry(t), nil)
	descriptions := provider.Describe()
	if descriptions[0].Path == "" {
		t.Fatal("expected route path")
	}
}

func TestProvider_P2PProvider_Channels_Good(t *testing.T) {
	provider := NewProvider(nil, nil)
	channels := provider.Channels()
	if len(channels) != 1 {
		t.Fatalf("channels: got %#v", channels)
	}
}

func TestProvider_P2PProvider_Channels_Bad(t *testing.T) {
	var provider *P2PProvider
	channels := provider.Channels()
	if len(channels) != 1 {
		t.Fatalf("channels: got %#v", channels)
	}
}

func TestProvider_P2PProvider_Channels_Ugly(t *testing.T) {
	provider := NewProvider(newProviderTestRegistry(t), nil)
	channels := provider.Channels()
	channels[0] = "mutated"
	if provider.Channels()[0] != "p2p" {
		t.Fatal("channels should be recreated")
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
