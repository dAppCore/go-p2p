// SPDX-License-Identifier: EUPL-1.2

// Package api exposes the go-p2p library as a core/api provider.
package api

import (
	"dappco.re/go/api"
	p2pnode "dappco.re/go/p2p/node"
	"github.com/gin-gonic/gin"
)

// P2PProvider mounts the P2P HTTP surface for non-Go consumers.
type P2PProvider struct {
	registry  *p2pnode.PeerRegistry
	transport *p2pnode.Transport
}

// NewProvider creates a P2P provider backed by the existing go-p2p library
// primitives. Pass nil for registry or transport when that capability is not
// available; dependent handlers will return a documented service error.
func NewProvider(registry *p2pnode.PeerRegistry, transport *p2pnode.Transport) *P2PProvider {
	return &P2PProvider{
		registry:  registry,
		transport: transport,
	}
}

// Name implements api.RouteGroup.
func (p *P2PProvider) Name() string { return "p2p" }

// BasePath implements api.RouteGroup.
func (p *P2PProvider) BasePath() string { return "/v1/p2p" }

// RegisterRoutes implements api.RouteGroup.
func (p *P2PProvider) RegisterRoutes(rg *gin.RouterGroup) {
	if p == nil || rg == nil {
		return
	}
	rg.POST("/upload", p.uploadEncryptedBlob)
	rg.GET("/sync/:topic", p.syncTopicEvents)
	rg.POST("/peer/:id/announce", p.announcePeer)
	rg.GET("/health", p.health)
	rg.GET("/peers", p.listPeers)
}

// Describe implements api.DescribableGroup for OpenAPI generation when mounted
// by core/api.
func (p *P2PProvider) Describe() []api.RouteDescription {
	return []api.RouteDescription{
		{
			Method:      "POST",
			Path:        "/upload",
			Summary:     "Upload an encrypted blob to the swarm",
			Description: "Accepts an encrypted blob upload request. The route currently returns 501 until the go-p2p encrypted swarm upload primitive lands.",
			Tags:        []string{"p2p"},
			RequestBody: map[string]any{
				"type":     "object",
				"required": []string{"encryptedBlob"},
				"properties": map[string]any{
					"topic":         map[string]any{"type": "string"},
					"encryptedBlob": map[string]any{"type": "string", "description": "Caller-encrypted blob payload."},
				},
			},
			Response: notImplementedSchema(),
		},
		{
			Method:      "GET",
			Path:        "/sync/:topic",
			Summary:     "Sync topic events",
			Description: "Returns 501 until the go-p2p library exposes an HTTP-friendly topic event sync primitive.",
			Tags:        []string{"p2p"},
			Response:    notImplementedSchema(),
		},
		{
			Method:      "POST",
			Path:        "/peer/:id/announce",
			Summary:     "Announce a peer",
			Description: "Adds or updates a peer in the local peer registry.",
			Tags:        []string{"p2p"},
			RequestBody: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":        map[string]any{"type": "string"},
					"name":      map[string]any{"type": "string"},
					"publicKey": map[string]any{"type": "string"},
					"address":   map[string]any{"type": "string"},
					"role":      map[string]any{"type": "string"},
				},
			},
			Response: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ok":     map[string]any{"type": "boolean"},
					"action": map[string]any{"type": "string"},
					"peer":   map[string]any{"type": "object"},
				},
			},
		},
		{
			Method:      "GET",
			Path:        "/health",
			Summary:     "Get P2P health",
			Description: "Reports peer count, connected peer count, and swarm state.",
			Tags:        []string{"p2p"},
			Response: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ok":             map[string]any{"type": "boolean"},
					"peerCount":      map[string]any{"type": "integer"},
					"connectedPeers": map[string]any{"type": "integer"},
					"swarmState":     map[string]any{"type": "string"},
					"error":          map[string]any{"type": "string"},
					"code":           map[string]any{"type": "string"},
				},
			},
		},
		{
			Method:      "GET",
			Path:        "/peers",
			Summary:     "List peers",
			Description: "Returns the local peer roster from the go-p2p peer registry.",
			Tags:        []string{"p2p"},
			Response: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"count": map[string]any{"type": "integer"},
					"peers": map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
				},
			},
		},
	}
}

// Channels declares the event namespace this provider will use once topic sync
// is backed by a streaming core/api mount.
func (p *P2PProvider) Channels() []string {
	return []string{"p2p"}
}

func notImplementedSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"error":     map[string]any{"type": "string"},
			"code":      map[string]any{"type": "string"},
			"operation": map[string]any{"type": "string"},
			"todo":      map[string]any{"type": "string"},
		},
	}
}

// Registration note: core/api consumers should import dappco.re/go/p2p/pkg/api
// and mount api.NewProvider(registry, transport) from the core/api Engine.
