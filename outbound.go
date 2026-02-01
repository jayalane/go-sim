// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

// OutboundCall represents a call queued for sending, with retry state.
type OutboundCall struct {
	call       *Call
	callee     *node
	callback   handleReply
	queuedAt   Milliseconds
	retryState *RetryState
}
