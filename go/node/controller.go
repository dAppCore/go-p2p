package node

import (
	"context"
	"sync"
	"time"

	core "dappco.re/go"
	coreerr "dappco.re/go/log"

	"dappco.re/go/p2p/logging"
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
func (c *Controller) sendRequest(peerID string, msg *Message, timeout time.Duration) core.Result {
	actualPeerID := peerID

	// Auto-connect if not already connected
	if c.transport.GetConnection(peerID) == nil {
		peer := c.peers.GetPeer(peerID)
		if peer == nil {
			return core.Fail(coreerr.E("Controller.sendRequest", "peer not found: "+peerID, nil))
		}
		connResult := c.transport.Connect(peer)
		if !connResult.OK {
			err, _ := connResult.Value.(error)
			return core.Fail(coreerr.E("Controller.sendRequest", "failed to connect to peer", err))
		}
		conn := connResult.Value.(*PeerConnection)
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
	if r := c.transport.Send(actualPeerID, msg); !r.OK {
		err, _ := r.Value.(error)
		return core.Fail(coreerr.E("Controller.sendRequest", "failed to send message", err))
	}

	// Wait for response
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case resp := <-respCh:
		return core.Ok(resp)
	case <-ctx.Done():
		return core.Fail(coreerr.E("Controller.sendRequest", "request timeout", nil))
	}
}

// GetRemoteStats requests miner statistics from a remote peer.
func (c *Controller) GetRemoteStats(peerID string) core.Result {
	identity := c.node.GetIdentity()
	if identity == nil {
		return core.Fail(ErrIdentityNotInitialized)
	}

	msgResult := NewMessage(MsgGetStats, identity.ID, peerID, nil)
	if !msgResult.OK {
		err, _ := msgResult.Value.(error)
		return core.Fail(coreerr.E("Controller.GetRemoteStats", "failed to create message", err))
	}
	msg := msgResult.Value.(*Message)

	respResult := c.sendRequest(peerID, msg, 10*time.Second)
	if !respResult.OK {
		return respResult
	}
	resp := respResult.Value.(*Message)

	var stats StatsPayload
	if r := ParseResponse(resp, MsgStats, &stats); !r.OK {
		return r
	}

	return core.Ok(&stats)
}

// StartRemoteMiner requests a remote peer to start a miner with a given profile.
func (c *Controller) StartRemoteMiner(peerID, minerType, profileID string, configOverride RawMessage) core.Result {
	identity := c.node.GetIdentity()
	if identity == nil {
		return core.Fail(ErrIdentityNotInitialized)
	}

	if minerType == "" {
		return core.Fail(coreerr.E("Controller.StartRemoteMiner", "miner type is required", nil))
	}

	payload := StartMinerPayload{
		MinerType: minerType,
		ProfileID: profileID,
		Config:    configOverride,
	}

	msgResult := NewMessage(MsgStartMiner, identity.ID, peerID, payload)
	if !msgResult.OK {
		err, _ := msgResult.Value.(error)
		return core.Fail(coreerr.E("Controller.StartRemoteMiner", "failed to create message", err))
	}
	msg := msgResult.Value.(*Message)

	respResult := c.sendRequest(peerID, msg, 30*time.Second)
	if !respResult.OK {
		return respResult
	}
	resp := respResult.Value.(*Message)

	var ack MinerAckPayload
	if r := ParseResponse(resp, MsgMinerAck, &ack); !r.OK {
		return r
	}

	if !ack.Success {
		return core.Fail(coreerr.E("Controller.StartRemoteMiner", "miner start failed: "+ack.Error, nil))
	}

	return core.Ok(nil)
}

// StopRemoteMiner requests a remote peer to stop a miner.
func (c *Controller) StopRemoteMiner(peerID, minerName string) core.Result {
	identity := c.node.GetIdentity()
	if identity == nil {
		return core.Fail(ErrIdentityNotInitialized)
	}

	payload := StopMinerPayload{
		MinerName: minerName,
	}

	msgResult := NewMessage(MsgStopMiner, identity.ID, peerID, payload)
	if !msgResult.OK {
		err, _ := msgResult.Value.(error)
		return core.Fail(coreerr.E("Controller.StopRemoteMiner", "failed to create message", err))
	}
	msg := msgResult.Value.(*Message)

	respResult := c.sendRequest(peerID, msg, 30*time.Second)
	if !respResult.OK {
		return respResult
	}
	resp := respResult.Value.(*Message)

	var ack MinerAckPayload
	if r := ParseResponse(resp, MsgMinerAck, &ack); !r.OK {
		return r
	}

	if !ack.Success {
		return core.Fail(coreerr.E("Controller.StopRemoteMiner", "miner stop failed: "+ack.Error, nil))
	}

	return core.Ok(nil)
}

// GetRemoteLogs requests console logs from a remote miner.
func (c *Controller) GetRemoteLogs(peerID, minerName string, lines int) core.Result {
	return c.GetRemoteLogsSince(peerID, minerName, lines, time.Time{})
}

// GetRemoteLogsSince requests console logs from a remote miner after a point in time.
func (c *Controller) GetRemoteLogsSince(peerID, minerName string, lines int, since time.Time) core.Result {
	identity := c.node.GetIdentity()
	if identity == nil {
		return core.Fail(ErrIdentityNotInitialized)
	}

	payload := GetLogsPayload{
		MinerName: minerName,
		Lines:     lines,
	}
	if !since.IsZero() {
		payload.Since = since.UnixMilli()
	}

	msgResult := NewMessage(MsgGetLogs, identity.ID, peerID, payload)
	if !msgResult.OK {
		err, _ := msgResult.Value.(error)
		return core.Fail(coreerr.E("Controller.GetRemoteLogsSince", "failed to create message", err))
	}
	msg := msgResult.Value.(*Message)

	respResult := c.sendRequest(peerID, msg, 10*time.Second)
	if !respResult.OK {
		return respResult
	}
	resp := respResult.Value.(*Message)

	var logs LogsPayload
	if r := ParseResponse(resp, MsgLogs, &logs); !r.OK {
		return r
	}

	return core.Ok(logs.Lines)
}

// GetAllStats fetches stats from all connected peers.
func (c *Controller) GetAllStats() map[string]*StatsPayload {
	results := make(map[string]*StatsPayload)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for peer := range c.peers.ConnectedPeers() {
		wg.Go(func() {
			statsResult := c.GetRemoteStats(peer.ID)
			if !statsResult.OK {
				logging.Debug("failed to get stats from peer", logging.Fields{
					"peer_id": peer.ID,
					"peer":    peer.Name,
					"error":   statsResult.Error(),
				})
				return // Skip failed peers
			}
			stats := statsResult.Value.(*StatsPayload)
			mu.Lock()
			results[peer.ID] = stats
			mu.Unlock()
		})
	}

	wg.Wait()
	return results
}

// PingPeer sends a ping to a peer and updates metrics.
func (c *Controller) PingPeer(peerID string) core.Result {
	identity := c.node.GetIdentity()
	if identity == nil {
		return core.Fail(ErrIdentityNotInitialized)
	}
	sentAt := time.Now()

	payload := PingPayload{
		SentAt: sentAt.UnixMilli(),
	}

	msgResult := NewMessage(MsgPing, identity.ID, peerID, payload)
	if !msgResult.OK {
		err, _ := msgResult.Value.(error)
		return core.Fail(coreerr.E("Controller.PingPeer", "failed to create message", err))
	}
	msg := msgResult.Value.(*Message)

	respResult := c.sendRequest(peerID, msg, 5*time.Second)
	if !respResult.OK {
		return respResult
	}
	resp := respResult.Value.(*Message)

	if r := ValidateResponse(resp, MsgPong); !r.OK {
		return r
	}

	// Calculate round-trip time
	rtt := time.Since(sentAt).Seconds() * 1000 // Convert to ms

	// Update peer metrics
	peer := c.peers.GetPeer(peerID)
	if peer != nil {
		c.peers.UpdateMetrics(peerID, rtt, peer.GeoKM, peer.Hops)
	}

	return core.Ok(rtt)
}

// ConnectToPeer establishes a connection to a peer.
func (c *Controller) ConnectToPeer(peerID string) core.Result {
	peer := c.peers.GetPeer(peerID)
	if peer == nil {
		return core.Fail(coreerr.E("Controller.ConnectToPeer", "peer not found: "+peerID, nil))
	}

	return c.transport.Connect(peer)
}

// DisconnectFromPeer closes connection to a peer.
func (c *Controller) DisconnectFromPeer(peerID string) core.Result {
	conn := c.transport.GetConnection(peerID)
	if conn == nil {
		return core.Fail(coreerr.E("Controller.DisconnectFromPeer", "peer not connected: "+peerID, nil))
	}

	return conn.Close()
}
