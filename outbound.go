// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

// OutboundCall represents a call queued at the sender waiting for
// the callee to have capacity.
type OutboundCall struct {
	call       *Call
	callee     *node
	callback   handleReply
	queuedAt   Milliseconds
	retryState *RetryState
}
