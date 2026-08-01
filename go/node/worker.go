package node

import (
	"encoding/base64"
	"time"

	core "dappco.re/go"
	coreerr "dappco.re/go/log"

	"dappco.re/go/p2p/logging"
	"github.com/adrg/xdg" // Note: intrinsic - XDG data directory resolution; c.Fs() does not expose XDG paths.
)

// MinerManager interface for the mining package integration.
// This allows the node package to interact with mining.Manager without import cycles.
type MinerManager interface {
	StartMiner(minerType string, config any) (MinerInstance, error)
	StopMiner(name string) error
	ListMiners() []MinerInstance
	GetMiner(name string) (MinerInstance, error)
}

// MinerInstance represents a running miner for stats collection.
type MinerInstance interface {
	GetName() string
	GetType() string
	GetStats() (any, error)
	GetConsoleHistory(lines int) []string
}

type historicalMinerInstance interface {
	GetConsoleHistorySince(lines int, since time.Time) []string
}

// ProfileManager interface for profile operations.
type ProfileManager interface {
	GetProfile(id string) (any, error)
	SaveProfile(profile any) error
}

// Worker handles incoming messages on a worker node.
type Worker struct {
	node           *NodeManager
	transport      *Transport
	minerManager   MinerManager
	profileManager ProfileManager
	startTime      time.Time
	DataDir        string // Base directory for deployments (defaults to xdg.DataHome)
}

// NewWorker creates a new Worker instance.
func NewWorker(node *NodeManager, transport *Transport) *Worker {
	return &Worker{
		node:      node,
		transport: transport,
		startTime: time.Now(),
		DataDir:   xdg.DataHome,
	}
}

// SetMinerManager sets the miner manager for handling miner operations.
func (w *Worker) SetMinerManager(manager MinerManager) {
	w.minerManager = manager
}

// SetProfileManager sets the profile manager for handling profile operations.
func (w *Worker) SetProfileManager(manager ProfileManager) {
	w.profileManager = manager
}

// HandleMessage processes incoming messages and returns a response.
func (w *Worker) HandleMessage(conn *PeerConnection, msg *Message) {
	var response *Message
	var result core.Result

	switch msg.Type {
	case MsgPing:
		result = w.handlePing(msg)
	case MsgGetStats:
		result = w.handleGetStats(msg)
	case MsgStartMiner:
		result = w.handleStartMiner(msg)
	case MsgStopMiner:
		result = w.handleStopMiner(msg)
	case MsgGetLogs:
		result = w.handleGetLogs(msg)
	case MsgDeploy:
		result = w.handleDeploy(conn, msg)
	default:
		// Unknown message type - ignore or send error
		return
	}

	if !result.OK {
		// Send error response
		identity := w.node.GetIdentity()
		if identity != nil {
			errMsg := NewErrorMessage(
				identity.ID,
				msg.From,
				ErrCodeOperationFailed,
				result.Error(),
				msg.ID,
			)
			if errMsg.OK {
				conn.Send(errMsg.Value.(*Message))
			}
		}
		return
	}

	if result.Value != nil {
		response, _ = result.Value.(*Message)
	}
	if response != nil {
		logging.Debug("sending response", logging.Fields{"type": response.Type, "to": msg.From})
		if r := conn.Send(response); !r.OK {
			logging.Error("failed to send response", logging.Fields{"error": r.Error()})
		} else {
			logging.Debug("response sent successfully")
		}
	}
}

// handlePing responds to ping requests.
func (w *Worker) handlePing(msg *Message) core.Result {
	var ping PingPayload
	if r := msg.ParsePayload(&ping); !r.OK {
		err, _ := r.Value.(error)
		return core.Fail(coreerr.E("Worker.handlePing", "invalid ping payload", err))
	}

	pong := PongPayload{
		SentAt:     ping.SentAt,
		ReceivedAt: time.Now().UnixMilli(),
	}

	return msg.Reply(MsgPong, pong)
}

// handleGetStats responds with current miner statistics.
func (w *Worker) handleGetStats(msg *Message) core.Result {
	identity := w.node.GetIdentity()
	if identity == nil {
		return core.Fail(ErrIdentityNotInitialized)
	}

	stats := StatsPayload{
		NodeID:   identity.ID,
		NodeName: identity.Name,
		Miners:   []MinerStatsItem{},
		Uptime:   int64(time.Since(w.startTime).Seconds()),
	}

	if w.minerManager != nil {
		miners := w.minerManager.ListMiners()
		for _, miner := range miners {
			minerStats, err := miner.GetStats()
			if err != nil {
				continue
			}

			// Convert to MinerStatsItem - this is a simplified conversion
			// The actual implementation would need to match the mining package's stats structure
			item := convertMinerStats(miner, minerStats)
			stats.Miners = append(stats.Miners, item)
		}
	}

	return msg.Reply(MsgStats, stats)
}

// convertMinerStats converts miner stats to the protocol format.
func convertMinerStats(miner MinerInstance, rawStats any) MinerStatsItem {
	item := MinerStatsItem{
		Name:         miner.GetName(),
		Type:         miner.GetType(),
		IsRunning:    true,
		SharedCount:  0,
		InvalidCount: 0,
	}

	// Try to extract common fields from the stats
	if statsMap, ok := rawStats.(map[string]any); ok {
		if hashrate, ok := statsMap["hashrate"].(float64); ok {
			item.Hashrate = hashrate
		}
		if shares, ok := statsMap["shares"].(int); ok {
			item.SharedCount = shares
			item.Shares = shares
		}
		if shares, ok := statsMap["sharedCount"].(int); ok {
			item.SharedCount = shares
			item.Shares = shares
		}
		if rejected, ok := statsMap["rejected"].(int); ok {
			item.InvalidCount = rejected
			item.Rejected = rejected
		}
		if invalid, ok := statsMap["invalid"].(int); ok {
			item.InvalidCount = invalid
			item.Rejected = invalid
		}
		if uptime, ok := statsMap["uptime"].(int); ok {
			item.Uptime = int64(uptime)
		}
		if uptime, ok := statsMap["uptime"].(int64); ok {
			item.Uptime = uptime
		}
		if pool, ok := statsMap["pool"].(string); ok {
			item.Pool = pool
		}
		if algorithm, ok := statsMap["algorithm"].(string); ok {
			item.Algorithm = algorithm
		}
		if temp, ok := statsMap["temp"].(float64); ok {
			item.Temperature = temp
		}
		if temp, ok := statsMap["temperature"].(float64); ok {
			item.Temperature = temp
		}
		if running, ok := statsMap["running"].(bool); ok {
			item.IsRunning = running
		}
	}

	return item
}

// handleStartMiner starts a miner with the given profile.
func (w *Worker) handleStartMiner(msg *Message) core.Result {
	if w.minerManager == nil {
		return core.Fail(ErrMinerManagerNotConfigured)
	}

	var payload StartMinerPayload
	if r := msg.ParsePayload(&payload); !r.OK {
		err, _ := r.Value.(error)
		return core.Fail(coreerr.E("Worker.handleStartMiner", "invalid start miner payload", err))
	}

	// Validate miner type is provided
	if payload.MinerType == "" {
		return core.Fail(coreerr.E("Worker.handleStartMiner", "miner type is required", nil))
	}

	// Get the config from the profile or use the override
	var config any
	if payload.Config != nil {
		config = payload.Config
	} else if w.profileManager != nil {
		profile, err := w.profileManager.GetProfile(payload.ProfileID)
		if err != nil {
			return core.Fail(coreerr.E("Worker.handleStartMiner", "profile not found: "+payload.ProfileID, nil))
		}
		config = profile
	} else {
		return core.Fail(coreerr.E("Worker.handleStartMiner", "no config provided and no profile manager configured", nil))
	}

	// Start the miner
	miner, err := w.minerManager.StartMiner(payload.MinerType, config)
	if err != nil {
		ack := MinerAckPayload{
			Success: false,
			Error:   err.Error(),
		}
		return msg.Reply(MsgMinerAck, ack)
	}

	ack := MinerAckPayload{
		Success:   true,
		Name:      miner.GetName(),
		MinerName: miner.GetName(),
	}
	return msg.Reply(MsgMinerAck, ack)
}

// handleStopMiner stops a running miner.
func (w *Worker) handleStopMiner(msg *Message) core.Result {
	if w.minerManager == nil {
		return core.Fail(ErrMinerManagerNotConfigured)
	}

	var payload StopMinerPayload
	if r := msg.ParsePayload(&payload); !r.OK {
		err, _ := r.Value.(error)
		return core.Fail(coreerr.E("Worker.handleStopMiner", "invalid stop miner payload", err))
	}

	err := w.minerManager.StopMiner(payload.MinerName)
	ack := MinerAckPayload{
		Success:   err == nil,
		Name:      payload.MinerName,
		MinerName: payload.MinerName,
	}
	if err != nil {
		ack.Error = err.Error()
	}

	return msg.Reply(MsgMinerAck, ack)
}

// handleGetLogs returns console logs from a miner.
func (w *Worker) handleGetLogs(msg *Message) core.Result {
	if w.minerManager == nil {
		return core.Fail(ErrMinerManagerNotConfigured)
	}

	var payload GetLogsPayload
	if r := msg.ParsePayload(&payload); !r.OK {
		err, _ := r.Value.(error)
		return core.Fail(coreerr.E("Worker.handleGetLogs", "invalid get logs payload", err))
	}

	// Validate and limit the Lines parameter to prevent resource exhaustion
	const maxLogLines = 10000
	if payload.Lines <= 0 || payload.Lines > maxLogLines {
		payload.Lines = maxLogLines
	}

	miner, err := w.minerManager.GetMiner(payload.MinerName)
	if err != nil {
		return core.Fail(coreerr.E("Worker.handleGetLogs", "miner not found: "+payload.MinerName, nil))
	}

	var since time.Time
	if payload.Since > 0 {
		since = time.UnixMilli(payload.Since)
	}

	lines := getMinerConsoleHistory(miner, payload.Lines, since)

	logs := LogsPayload{
		MinerName: payload.MinerName,
		Lines:     lines,
		HasMore:   len(lines) >= payload.Lines,
	}

	return msg.Reply(MsgLogs, logs)
}

func getMinerConsoleHistory(miner MinerInstance, lines int, since time.Time) []string {
	if since.IsZero() {
		return miner.GetConsoleHistory(lines)
	}

	if hist, ok := miner.(historicalMinerInstance); ok {
		return hist.GetConsoleHistorySince(lines, since)
	}

	return miner.GetConsoleHistory(lines)
}

// handleDeploy handles deployment of profiles or miner bundles.
func (w *Worker) handleDeploy(conn *PeerConnection, msg *Message) core.Result {
	var payload DeployPayload
	if r := msg.ParsePayload(&payload); !r.OK {
		err, _ := r.Value.(error)
		return core.Fail(coreerr.E("Worker.handleDeploy", "invalid deploy payload", err))
	}

	// Reconstruct Bundle object from payload, preferring the structured bundle
	// when present but retaining legacy fields for backward compatibility.
	bundle := payload.Bundle
	if bundle == nil {
		bundle = &Bundle{
			Type:     BundleType(payload.BundleType),
			Name:     payload.Name,
			Data:     payload.Data,
			Checksum: payload.Checksum,
		}
	} else {
		if bundle.Name == "" {
			bundle.Name = payload.Name
		}
		if bundle.Type == "" {
			bundle.Type = BundleType(payload.BundleType)
		}
	}

	// Use shared secret as password (base64 encoded)
	password := ""
	if conn != nil && len(conn.SharedSecret) > 0 {
		password = base64.StdEncoding.EncodeToString(conn.SharedSecret)
	}

	switch bundle.Type {
	case BundleProfile:
		if w.profileManager == nil {
			return core.Fail(coreerr.E("Worker.handleDeploy", "profile manager not configured", nil))
		}

		// Decrypt and extract profile data
		profileDataResult := ExtractProfileBundle(bundle, password)
		if !profileDataResult.OK {
			err, _ := profileDataResult.Value.(error)
			return core.Fail(coreerr.E("Worker.handleDeploy", "failed to extract profile bundle", err))
		}
		profileData := profileDataResult.Value.([]byte)

		// Unmarshal into any to pass to ProfileManager
		var profile any
		if r := core.JSONUnmarshal(profileData, &profile); !r.OK {
			err, _ := r.Value.(error)
			return core.Fail(coreerr.E("Worker.handleDeploy", "invalid profile data JSON", err))
		}

		if err := w.profileManager.SaveProfile(profile); err != nil {
			ack := DeployAckPayload{
				Success: false,
				Name:    payload.Name,
				Error:   err.Error(),
			}
			return msg.Reply(MsgDeployAck, ack)
		}

		ack := DeployAckPayload{
			Success: true,
			Name:    payload.Name,
		}
		return msg.Reply(MsgDeployAck, ack)

	case BundleMiner, BundleFull:
		// Determine installation directory
		// We use w.DataDir/lethean-desktop/miners/<bundle_name>
		minersDir := core.PathJoin(w.DataDir, "lethean-desktop", "miners")
		installDir := core.PathJoin(minersDir, payload.Name)

		logging.Info("deploying miner bundle", logging.Fields{
			"name":         payload.Name,
			"install_path": installDir,
			"type":         payload.BundleType,
		})

		// Extract miner bundle
		extractedResult := ExtractMinerBundle(bundle, password, installDir)
		if !extractedResult.OK {
			err, _ := extractedResult.Value.(error)
			return core.Fail(coreerr.E("Worker.handleDeploy", "failed to extract miner bundle", err))
		}
		extracted := extractedResult.Value.(extractedMinerBundleResult)
		minerPath := extracted.Path
		profileData := extracted.Profile

		// If the bundle contained a profile config, save it
		if len(profileData) > 0 && w.profileManager != nil {
			var profile any
			if r := core.JSONUnmarshal(profileData, &profile); !r.OK {
				err, _ := r.Value.(error)
				logging.Warn("failed to parse profile from miner bundle", logging.Fields{"error": err})
			} else {
				if err := w.profileManager.SaveProfile(profile); err != nil {
					logging.Warn("failed to save profile from miner bundle", logging.Fields{"error": err})
				}
			}
		}

		// Success response
		ack := DeployAckPayload{
			Success: true,
			Name:    payload.Name,
		}

		// Log the installation
		logging.Info("miner bundle installed successfully", logging.Fields{
			"name":       payload.Name,
			"miner_path": minerPath,
		})

		return msg.Reply(MsgDeployAck, ack)

	default:
		return core.Fail(coreerr.E("Worker.handleDeploy", "unknown bundle type: "+payload.BundleType, nil))
	}
}

// RegisterWithTransport registers the worker's message handler with the transport.
func (w *Worker) RegisterWithTransport() {
	w.transport.OnMessage(w.HandleMessage)
}
