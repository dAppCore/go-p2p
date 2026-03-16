package node

import coreerr "forge.lthn.ai/core/go-log"

// Sentinel errors shared across the node package.
var (
	// ErrIdentityNotInitialized is returned when a node operation requires
	// a node identity but none has been generated or loaded.
	ErrIdentityNotInitialized = coreerr.E("node", "node identity not initialized", nil)

	// ErrMinerManagerNotConfigured is returned when a miner operation is
	// attempted but no MinerManager has been set on the Worker.
	ErrMinerManagerNotConfigured = coreerr.E("node", "miner manager not configured", nil)
)
