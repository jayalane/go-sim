// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

import (
	"errors"
	"math"
	"math/rand"
	"net/http"
	"sync"

	count "github.com/jayalane/go-counter"
)

// ResourceType represents the three tracked resource types.
type ResourceType int

var (
	errNodeDown        = errors.New("node down")
	errNodeDownMem     = errors.New("node down due to mem")
	errNodeDownNetwork = errors.New("node down due to network")
)

const (
	cpu ResourceType = iota
	memory
	network
)

// ResourceState tracks current utilization for a single resource.
type ResourceState struct {
	Current    float64   // Current utilization (0.0 to 1.0)
	Limit      float64   // Maximum allowed utilization (0.0 to 1.0)
	Historical []float64 // Per-millisecond history for analysis
}

// NodeResources tracks all resource state for a node.
type NodeResources struct {
	mu      sync.RWMutex
	cpu     ResourceState
	memory  ResourceState
	network ResourceState

	// Recovery state
	isDown      bool
	downUntil   Milliseconds // When node becomes available again
	pendingWork []*Call      // Work queued during downtime

	// Configuration
	memoryRecoveryMs Milliseconds // How long memory exhaustion takes to recover
	config           *ResourceConfig
}

// ResourceConfig defines how operations consume resources.
type ResourceConfig struct {
	// Base resource consumption per operation type
	CPUPerLocalWork ModelCdf // CPU consumed per ms of local work
	MemoryPerCall   ModelCdf // Memory consumed per active call
	NetworkPerCall  ModelCdf // Network consumed per call (in+out)
	NetworkPerReply ModelCdf // Network consumed per reply

	// Resource limits (0.0 to 1.0)
	CPULimit     float64
	MemoryLimit  float64
	NetworkLimit float64

	// Recovery settings
	MemoryRecoveryMs Milliseconds // Time to restart after memory exhaustion
	CPUDelayFactor   float64      // Multiplier for task delays when CPU saturated

	// Decay rates (per millisecond)
	CPUDecayRate     float64 // How fast CPU usage decreases
	MemoryDecayRate  float64 // How fast memory usage decreases
	NetworkDecayRate float64 // How fast network usage decreases
}

// DefaultResourceConfig returns sensible default resource configuration.
func DefaultResourceConfig() *ResourceConfig {
	return &ResourceConfig{
		CPUPerLocalWork: UniformCDF(0.1, 0.3),   //nolint:mnd // 10-30% CPU per ms
		MemoryPerCall:   UniformCDF(0.05, 0.15), //nolint:mnd // 5-15% memory per call
		NetworkPerCall:  UniformCDF(0.1, 0.2),   //nolint:mnd // 10-20% network per call
		NetworkPerReply: UniformCDF(0.05, 0.1),  //nolint:mnd // 5-10% network per reply

		CPULimit:     0.95, //nolint:mnd
		MemoryLimit:  0.90, //nolint:mnd
		NetworkLimit: 0.85, //nolint:mnd

		MemoryRecoveryMs: 15000, //nolint:mnd // 15 seconds to restart
		CPUDelayFactor:   2.0,   //nolint:mnd // 2x delay when CPU saturated

		CPUDecayRate:     0.1,  //nolint:mnd // 10% decay per ms
		MemoryDecayRate:  0.02, //nolint:mnd // 2% decay per ms (slower)
		NetworkDecayRate: 0.15, //nolint:mnd // 15% decay per ms (fastest)
	}
}

// initResources initializes resource tracking for a node.
func (n *node) initResources(config *ResourceConfig) {
	if config == nil {
		config = DefaultResourceConfig()
	}

	n.resources = &NodeResources{
		cpu:              ResourceState{Limit: config.CPULimit, Historical: make([]float64, 0)},
		memory:           ResourceState{Limit: config.MemoryLimit, Historical: make([]float64, 0)},
		network:          ResourceState{Limit: config.NetworkLimit, Historical: make([]float64, 0)},
		memoryRecoveryMs: config.MemoryRecoveryMs,
		config:           config,
		pendingWork:      make([]*Call, 0),
	}
}

// consumeResources attempts to consume the specified amount of a resource type.
func (n *node) consumeResources(resType ResourceType, amount float64) error {
	n.resources.mu.Lock()

	if n.resources.isDown {
		ml.La("node ", n.name, " is down until", n.resources.downUntil)
		n.resources.mu.Unlock()

		return errNodeDown
	}

	switch resType {
	case cpu:
		n.resources.cpu.Current = math.Min(1.0, n.resources.cpu.Current+amount)
	case memory:
		n.resources.memory.Current = math.Min(1.0, n.resources.memory.Current+amount)
	case network:
		if n.resources.network.Current+amount > n.resources.network.Limit {
			count.IncrSyncSuffix("node_network_saturated", n.name)
			n.resources.mu.Unlock()

			return errNodeDownNetwork
		}

		n.resources.network.Current = math.Min(1.0, n.resources.network.Current+amount)
	}

	needsOOMKill := n.resources.memory.Current > n.resources.memory.Limit

	var oomErr error

	if needsOOMKill {
		n.resources.isDown = true
		n.resources.downUntil = Milliseconds(n.loop.GetTime()) + n.resources.memoryRecoveryMs
		// Only clear pendingWork (safe under resources.mu)
		n.resources.pendingWork = nil

		count.IncrSyncSuffix("node_memory_exhaustion", n.name)
		ml.La(n.name+": Memory exhausted, restarting in", n.resources.memoryRecoveryMs, "ms")

		oomErr = errNodeDownMem
	}

	n.resources.mu.Unlock()

	// Queue clearing happens at recovery time in updateResources().
	// While the node is down, no new work is processed from queues.

	return oomErr
}

// checkResourceLimits is no longer used; OOM detection is inline in consumeResources.

// updateResources updates resource utilization each millisecond.
func (n *node) updateResources() {
	n.resources.mu.Lock()

	currentTime := Milliseconds(n.loop.GetTime())
	needsRecovery := false

	// Check if node should come back online
	if n.resources.isDown && currentTime >= n.resources.downUntil {
		n.resources.isDown = false
		n.resources.cpu.Current = 0
		n.resources.memory.Current = 0
		n.resources.network.Current = 0
		needsRecovery = true

		count.IncrSyncSuffix("node_recovery", n.name)
		ml.La(n.name + ": Node recovered from memory exhaustion")
	}

	if !n.resources.isDown {
		// Apply decay to all resources
		n.resources.cpu.Current = math.Max(0, n.resources.cpu.Current-n.resources.config.CPUDecayRate)
		n.resources.memory.Current = math.Max(0, n.resources.memory.Current-n.resources.config.MemoryDecayRate)
		n.resources.network.Current = math.Max(0, n.resources.network.Current-n.resources.config.NetworkDecayRate)
	}

	// Record historical data
	n.resources.cpu.Historical = append(n.resources.cpu.Historical, n.resources.cpu.Current)
	n.resources.memory.Historical = append(n.resources.memory.Historical, n.resources.memory.Current)
	n.resources.network.Historical = append(n.resources.network.Historical, n.resources.network.Current)

	n.resources.mu.Unlock()

	// At recovery time, clear stale queues and restore pending work
	// outside of resources.mu to avoid deadlock
	if needsRecovery {
		n.clearQueues()
		n.restorePendingWork()
	}
}

// calculateCPUDelay returns delay in milliseconds if CPU is over limit.
func (n *node) calculateCPUDelay() float64 {
	n.resources.mu.RLock()
	defer n.resources.mu.RUnlock()

	if n.resources.cpu.Current > n.resources.cpu.Limit {
		overage := n.resources.cpu.Current - n.resources.cpu.Limit
		delay := overage * n.resources.config.CPUDelayFactor
		count.IncrSyncSuffix("node_cpu_delay", n.name)

		return delay
	}

	return 0
}

// clearQueues clears task, call, and outbound queues during OOM.
// Must be called outside of resources.mu to avoid deadlock.
func (n *node) clearQueues() {
	n.tasksMu.Lock()
	n.tasks = make(PQueue, 0)
	n.tasksMu.Unlock()

	n.callsMu.Lock()
	n.calls = make(PQueue, 0)
	n.callsMu.Unlock()

	n.outboundMu.Lock()
	n.outboundQueue = nil
	n.outboundMu.Unlock()

	ml.La(n.name + ": Cleared all queues due to memory exhaustion")
}

// restorePendingWork restores work that was queued during downtime.
// Must be called outside of resources.mu to avoid deadlock with addCall.
func (n *node) restorePendingWork() {
	n.resources.mu.Lock()
	pending := n.resources.pendingWork
	n.resources.pendingWork = nil
	n.resources.mu.Unlock()

	for _, call := range pending {
		n.addCall(call)
		ml.La(n.name+": Restored pending call", call.ReqID)
	}
}

// sendErrorReply sends an error response for rejected calls.
func (n *node) sendErrorReply(c *Call, message string) {
	r := Reply{
		reqID:  c.ReqID,
		status: http.StatusServiceUnavailable,
		call:   c,
	}

	if c.caller != nil {
		c.caller.replyCh <- &r
		ml.La(n.name+": Sent error reply", message, "for call", c.ReqID)
	}
}

// GetResourceUtilization returns current resource utilization.
func (n *node) GetResourceUtilization() map[string]float64 {
	n.resources.mu.RLock()
	defer n.resources.mu.RUnlock()

	return map[string]float64{
		"cpu":     n.resources.cpu.Current,
		"memory":  n.resources.memory.Current,
		"network": n.resources.network.Current,
	}
}

// GetResourceHistory returns resource history for analysis.
func (n *node) GetResourceHistory() map[string][]float64 {
	n.resources.mu.RLock()
	defer n.resources.mu.RUnlock()

	return map[string][]float64{
		"cpu":     append([]float64(nil), n.resources.cpu.Historical...),
		"memory":  append([]float64(nil), n.resources.memory.Historical...),
		"network": append([]float64(nil), n.resources.network.Historical...),
	}
}

// IsAvailable checks if node is operational.
func (n *node) IsAvailable() bool {
	n.resources.mu.RLock()
	defer n.resources.mu.RUnlock()

	return !n.resources.isDown
}

// consumeCPUForLocalWork consumes CPU resources for local work processing.
func (n *node) consumeCPUForLocalWork() {
	p := rand.Float64() //nolint:gosec
	cpuCost := n.resources.config.CPUPerLocalWork(p)

	if err := n.consumeResources(cpu, cpuCost); err != nil {
		ml.La(n.name+": CPU resource error:", err.Error())
	}
}

// consumeMemoryForCall consumes memory resources for handling a call.
func (n *node) consumeMemoryForCall() error {
	p := rand.Float64() //nolint:gosec
	memoryCost := n.resources.config.MemoryPerCall(p)

	return n.consumeResources(memory, memoryCost)
}

// consumeNetworkForCall consumes network resources for incoming calls.
func (n *node) consumeNetworkForCall() error {
	p := rand.Float64() //nolint:gosec
	networkCost := n.resources.config.NetworkPerCall(p)

	return n.consumeResources(network, networkCost)
}

// consumeNetworkForReply consumes network resources for outgoing replies.
func (n *node) consumeNetworkForReply() error {
	p := rand.Float64() //nolint:gosec
	networkCost := n.resources.config.NetworkPerReply(p)

	return n.consumeResources(network, networkCost)
}
