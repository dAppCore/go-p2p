// SPDX-License-Identifier: EUPL-1.2

package api

import (
	"maps"
	"net/http"
	"time"

	core "dappco.re/go"
	p2pnode "dappco.re/go/p2p/node"
	"github.com/gin-gonic/gin"
)

const (
	codeInvalidRequest       = "invalid_request"
	codeInvalidTopic         = "invalid_topic"
	codeProviderUnconfigured = "provider_unconfigured"
	codePeerMismatch         = "peer_id_mismatch"
	codePeerRejected         = "peer_rejected"
	codeNotImplemented       = "not_implemented"
)

type uploadRequest struct {
	Topic         string `json:"topic,omitempty"`
	EncryptedBlob string `json:"encryptedBlob"`
}

type healthResponse struct {
	OK             bool   `json:"ok"`
	PeerCount      int    `json:"peerCount"`
	ConnectedPeers int    `json:"connectedPeers"`
	SwarmState     string `json:"swarmState"`
	Error          string `json:"error,omitempty"`
	Code           string `json:"code,omitempty"`
}

type peersResponse struct {
	Count int             `json:"count"`
	Peers []*p2pnode.Peer `json:"peers"`
}

type announceResponse struct {
	OK     bool          `json:"ok"`
	Action string        `json:"action"`
	Peer   *p2pnode.Peer `json:"peer"`
}

func (p *P2PProvider) uploadEncryptedBlob(c *gin.Context) {
	if c == nil {
		return
	}

	var req uploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, codeInvalidRequest, "upload request body must be valid JSON")
		return
	}
	if core.Trim(req.EncryptedBlob) == "" {
		writeError(c, http.StatusBadRequest, codeInvalidRequest, "encryptedBlob is required")
		return
	}

	// TODO(go-p2p library work): delegate to the encrypted swarm blob upload
	// primitive once the out-of-scope P2P protocol work lands.
	writeNotImplemented(c, "upload", "go-p2p library work: encrypted swarm blob upload primitive")
}

func (p *P2PProvider) syncTopicEvents(c *gin.Context) {
	if c == nil {
		return
	}

	topic := core.Trim(c.Param("topic"))
	if topic == "" {
		writeError(c, http.StatusBadRequest, codeInvalidTopic, "topic is required")
		return
	}

	// TODO(go-p2p library work): delegate to an HTTP-friendly topic event sync
	// primitive once the out-of-scope P2P protocol work lands.
	writeNotImplemented(c, "sync", "go-p2p library work: topic event sync primitive", gin.H{"topic": topic})
}

func (p *P2PProvider) announcePeer(c *gin.Context) {
	if c == nil {
		return
	}
	if p == nil || p.registry == nil {
		writeError(c, http.StatusServiceUnavailable, codeProviderUnconfigured, "peer registry is not configured")
		return
	}

	id := core.Trim(c.Param("id"))
	if id == "" {
		writeError(c, http.StatusBadRequest, codeInvalidRequest, "peer id is required")
		return
	}

	var peer p2pnode.Peer
	if err := c.ShouldBindJSON(&peer); err != nil {
		writeError(c, http.StatusBadRequest, codeInvalidRequest, "peer announcement body must be valid JSON")
		return
	}
	if peer.ID != "" && peer.ID != id {
		writeError(c, http.StatusBadRequest, codePeerMismatch, "peer id in path and body must match")
		return
	}

	peer.ID = id
	if peer.LastSeen.IsZero() {
		peer.LastSeen = time.Now()
	}

	action := "announced"
	if existing := p.registry.GetPeer(id); existing != nil {
		action = "updated"
		if peer.AddedAt.IsZero() {
			peer.AddedAt = existing.AddedAt
		}
		if peer.Score == 0 {
			peer.Score = existing.Score
		}
		if r := p.registry.UpdatePeer(&peer); !r.OK {
			writeError(c, http.StatusBadRequest, codePeerRejected, r.Error())
			return
		}
		c.JSON(http.StatusOK, announceResponse{OK: true, Action: action, Peer: &peer})
		return
	}

	if r := p.registry.AddPeer(&peer); !r.OK {
		writeError(c, http.StatusBadRequest, codePeerRejected, r.Error())
		return
	}
	c.JSON(http.StatusOK, announceResponse{OK: true, Action: action, Peer: &peer})
}

func (p *P2PProvider) health(c *gin.Context) {
	if c == nil {
		return
	}
	if p == nil || p.registry == nil {
		c.JSON(http.StatusServiceUnavailable, healthResponse{
			OK:         false,
			SwarmState: "unconfigured",
			Error:      "peer registry is not configured",
			Code:       codeProviderUnconfigured,
		})
		return
	}

	peerCount := p.registry.Count()
	connectedPeers := len(p.registry.GetConnectedPeers())
	if p.transport != nil {
		connectedPeers = p.transport.ConnectedPeers()
	}

	c.JSON(http.StatusOK, healthResponse{
		OK:             true,
		PeerCount:      peerCount,
		ConnectedPeers: connectedPeers,
		SwarmState:     swarmState(peerCount, connectedPeers),
	})
}

func (p *P2PProvider) listPeers(c *gin.Context) {
	if c == nil {
		return
	}
	if p == nil || p.registry == nil {
		writeError(c, http.StatusServiceUnavailable, codeProviderUnconfigured, "peer registry is not configured")
		return
	}

	peers := p.registry.ListPeers()
	c.JSON(http.StatusOK, peersResponse{
		Count: len(peers),
		Peers: peers,
	})
}

func swarmState(peerCount int, connectedPeers int) string {
	if peerCount == 0 {
		return "empty"
	}
	if connectedPeers > 0 {
		return "connected"
	}
	return "idle"
}

func writeError(c *gin.Context, status int, code string, message string) {
	if c == nil {
		return
	}
	c.JSON(status, gin.H{
		"error": message,
		"code":  code,
	})
}

func writeNotImplemented(c *gin.Context, operation string, todo string, fields ...gin.H) {
	if c == nil {
		return
	}
	body := gin.H{
		"error":     "not implemented",
		"code":      codeNotImplemented,
		"operation": operation,
		"todo":      todo,
	}
	for _, fieldSet := range fields {
		maps.Copy(body, fieldSet)
	}
	c.JSON(http.StatusNotImplemented, body)
}
