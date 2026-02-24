package node

import "errors"

// Sentinel errors shared across the node package.
var (
	// ErrIdentityNotInitialized is returned when a node operation requires
	// a node identity but none has been generated or loaded.
	ErrIdentityNotInitialized = errors.New("node identity not initialized")

	// ErrMinerManagerNotConfigured is returned when a miner operation is
	// attempted but no MinerManager has been set on the Worker.
	ErrMinerManagerNotConfigured = errors.New("miner manager not configured")
)
