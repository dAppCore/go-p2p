package node

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	coreerr "forge.lthn.ai/core/go-log"

	"forge.lthn.ai/core/go-p2p/logging"
)

// Controller manages remote peer operations from a controller node.
type Controller struct {
	node      *NodeManager
	peers     *PeerRegistry
	transport *Transport
	mu        sync.RWMutex

	// Pending requests awaiting responses
	pending map[string]chan *Message // message ID -> response channel
}

// NewController creates a new Controller instance.
func NewController(node *NodeManager, peers *PeerRegistry, transport *Transport) *Controller {
	c := &Controller{
		node:      node,
		peers:     peers,
		transport: transport,
		pending:   make(map[string]chan *Message),
	}

	// Register message handler for responses
	transport.OnMessage(c.handleResponse)

	return c
}

// handleResponse processes incoming messages that are responses to our requests.
func (c *Controller) handleResponse(conn *PeerConnection, msg *Message) {
	if msg.ReplyTo == "" {
		return // Not a response, let worker handle it
	}

	c.mu.Lock()
	ch, exists := c.pending[msg.ReplyTo]
	if exists {
		delete(c.pending, msg.ReplyTo)
	}
	c.mu.Unlock()

	if exists && ch != nil {
		select {
		case ch <- msg:
		default:
			// Channel full or closed
		}
	}
}

// sendRequest sends a message and waits for a response.
func (c *Controller) sendRequest(peerID string, msg *Message, timeout time.Duration) (*Message, error) {
	actualPeerID := peerID

	// Auto-connect if not already connected
	if c.transport.GetConnection(peerID) == nil {
		peer := c.peers.GetPeer(peerID)
		if peer == nil {
			return nil, coreerr.E("Controller.sendRequest", "peer not found: "+peerID, nil)
		}
		conn, err := c.transport.Connect(peer)
		if err != nil {
			return nil, coreerr.E("Controller.sendRequest", "failed to connect to peer", err)
		}
		// Use the real peer ID after handshake (it may have changed)
		actualPeerID = conn.Peer.ID
		// Update the message destination
		msg.To = actualPeerID
	}

	// Create response channel
	respCh := make(chan *Message, 1)

	c.mu.Lock()
	c.pending[msg.ID] = respCh
	c.mu.Unlock()

	// Clean up on exit - ensure channel is closed and removed from map
	defer func() {
		c.mu.Lock()
		delete(c.pending, msg.ID)
		c.mu.Unlock()
		close(respCh) // Close channel to allow garbage collection
	}()

	// Send the message
	if err := c.transport.Send(actualPeerID, msg); err != nil {
		return nil, coreerr.E("Controller.sendRequest", "failed to send message", err)
	}

	// Wait for response
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case resp := <-respCh:
		return resp, nil
	case <-ctx.Done():
		return nil, coreerr.E("Controller.sendRequest", "request timeout", nil)
	}
}

// GetRemoteStats requests miner statistics from a remote peer.
func (c *Controller) GetRemoteStats(peerID string) (*StatsPayload, error) {
	identity := c.node.GetIdentity()
	if identity == nil {
		return nil, ErrIdentityNotInitialized
	}

	msg, err := NewMessage(MsgGetStats, identity.ID, peerID, nil)
	if err != nil {
		return nil, coreerr.E("Controller.GetRemoteStats", "failed to create message", err)
	}

	resp, err := c.sendRequest(peerID, msg, 10*time.Second)
	if err != nil {
		return nil, err
	}

	var stats StatsPayload
	if err := ParseResponse(resp, MsgStats, &stats); err != nil {
		return nil, err
	}

	return &stats, nil
}

// StartRemoteMiner requests a remote peer to start a miner with a given profile.
func (c *Controller) StartRemoteMiner(peerID, minerType, profileID string, configOverride json.RawMessage) error {
	identity := c.node.GetIdentity()
	if identity == nil {
		return ErrIdentityNotInitialized
	}

	if minerType == "" {
		return coreerr.E("Controller.StartRemoteMiner", "miner type is required", nil)
	}

	payload := StartMinerPayload{
		MinerType: minerType,
		ProfileID: profileID,
		Config:    configOverride,
	}

	msg, err := NewMessage(MsgStartMiner, identity.ID, peerID, payload)
	if err != nil {
		return coreerr.E("Controller.StartRemoteMiner", "failed to create message", err)
	}

	resp, err := c.sendRequest(peerID, msg, 30*time.Second)
	if err != nil {
		return err
	}

	var ack MinerAckPayload
	if err := ParseResponse(resp, MsgMinerAck, &ack); err != nil {
		return err
	}

	if !ack.Success {
		return coreerr.E("Controller.StartRemoteMiner", "miner start failed: "+ack.Error, nil)
	}

	return nil
}

// StopRemoteMiner requests a remote peer to stop a miner.
func (c *Controller) StopRemoteMiner(peerID, minerName string) error {
	identity := c.node.GetIdentity()
	if identity == nil {
		return ErrIdentityNotInitialized
	}

	payload := StopMinerPayload{
		MinerName: minerName,
	}

	msg, err := NewMessage(MsgStopMiner, identity.ID, peerID, payload)
	if err != nil {
		return coreerr.E("Controller.StopRemoteMiner", "failed to create message", err)
	}

	resp, err := c.sendRequest(peerID, msg, 30*time.Second)
	if err != nil {
		return err
	}

	var ack MinerAckPayload
	if err := ParseResponse(resp, MsgMinerAck, &ack); err != nil {
		return err
	}

	if !ack.Success {
		return coreerr.E("Controller.StopRemoteMiner", "miner stop failed: "+ack.Error, nil)
	}

	return nil
}

// GetRemoteLogs requests console logs from a remote miner.
func (c *Controller) GetRemoteLogs(peerID, minerName string, lines int) ([]string, error) {
	identity := c.node.GetIdentity()
	if identity == nil {
		return nil, ErrIdentityNotInitialized
	}

	payload := GetLogsPayload{
		MinerName: minerName,
		Lines:     lines,
	}

	msg, err := NewMessage(MsgGetLogs, identity.ID, peerID, payload)
	if err != nil {
		return nil, coreerr.E("Controller.GetRemoteLogs", "failed to create message", err)
	}

	resp, err := c.sendRequest(peerID, msg, 10*time.Second)
	if err != nil {
		return nil, err
	}

	var logs LogsPayload
	if err := ParseResponse(resp, MsgLogs, &logs); err != nil {
		return nil, err
	}

	return logs.Lines, nil
}

// GetAllStats fetches stats from all connected peers.
func (c *Controller) GetAllStats() map[string]*StatsPayload {
	results := make(map[string]*StatsPayload)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for peer := range c.peers.ConnectedPeers() {
		wg.Add(1)
		go func(p *Peer) {
			defer wg.Done()
			stats, err := c.GetRemoteStats(p.ID)
			if err != nil {
				logging.Debug("failed to get stats from peer", logging.Fields{
					"peer_id": p.ID,
					"peer":    p.Name,
					"error":   err.Error(),
				})
				return // Skip failed peers
			}
			mu.Lock()
			results[p.ID] = stats
			mu.Unlock()
		}(peer)
	}

	wg.Wait()
	return results
}

// PingPeer sends a ping to a peer and updates metrics.
func (c *Controller) PingPeer(peerID string) (float64, error) {
	identity := c.node.GetIdentity()
	if identity == nil {
		return 0, ErrIdentityNotInitialized
	}
	sentAt := time.Now()

	payload := PingPayload{
		SentAt: sentAt.UnixMilli(),
	}

	msg, err := NewMessage(MsgPing, identity.ID, peerID, payload)
	if err != nil {
		return 0, coreerr.E("Controller.PingPeer", "failed to create message", err)
	}

	resp, err := c.sendRequest(peerID, msg, 5*time.Second)
	if err != nil {
		return 0, err
	}

	if err := ValidateResponse(resp, MsgPong); err != nil {
		return 0, err
	}

	// Calculate round-trip time
	rtt := time.Since(sentAt).Seconds() * 1000 // Convert to ms

	// Update peer metrics
	peer := c.peers.GetPeer(peerID)
	if peer != nil {
		c.peers.UpdateMetrics(peerID, rtt, peer.GeoKM, peer.Hops)
	}

	return rtt, nil
}

// ConnectToPeer establishes a connection to a peer.
func (c *Controller) ConnectToPeer(peerID string) error {
	peer := c.peers.GetPeer(peerID)
	if peer == nil {
		return coreerr.E("Controller.ConnectToPeer", "peer not found: "+peerID, nil)
	}

	_, err := c.transport.Connect(peer)
	return err
}

// DisconnectFromPeer closes connection to a peer.
func (c *Controller) DisconnectFromPeer(peerID string) error {
	conn := c.transport.GetConnection(peerID)
	if conn == nil {
		return coreerr.E("Controller.DisconnectFromPeer", "peer not connected: "+peerID, nil)
	}

	return conn.Close()
}
