// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

import (
	"sync"
)

// HandleReply type is a callback to process the reply from a call.
type handleReply func(*node, *Reply)

// Call is a structure to track a remote call.
type Call struct {
	Wakeup    Milliseconds
	StartTime Milliseconds
	Endpoint  string
	TimeoutMs float64
	ReqID     int
	//	length     uint64
	//	id1        uint64
	// id2        uint64
	Params map[string]string
	// connection *Connection
	caller      *node
	cpuCost     ModelCdf // Per-call CPU cost CDF (nil = use node default)
	memoryCost  ModelCdf // Per-call memory cost CDF (nil = use node default)
	networkCost ModelCdf // Per-call network cost CDF (nil = use node default)
}

var (
	callNumber      = 22222
	callNumberMutex sync.RWMutex
)

// IncrCallNumber sets the global req ID.
func IncrCallNumber() int {
	callNumberMutex.Lock()
	callNumber++
	a := callNumber
	callNumberMutex.Unlock()

	return a
}

// sendCall sends the call to the callee node channel.
// On failure to deliver, the call is queued at the sender for retry.
func (c *Call) sendCall(callee *node, f handleReply) {
	reqID := IncrCallNumber()
	c.ReqID = reqID
	c.caller.pendingCallMapMu.Lock()
	c.caller.pendingCallMap[c.ReqID] = &pendingCall{reply: nil, call: c, f: f}
	c.caller.pendingCallMapMu.Unlock()

	if !callee.tryAcceptCall(c) {
		oc := &OutboundCall{
			call:     c,
			callee:   callee,
			callback: f,
			queuedAt: Milliseconds(c.caller.loop.GetTime()),
		}
		c.caller.queueOutbound(oc)
	}
}

// sendCallWithRetry sends the call with an existing retry state.
func (c *Call) sendCallWithRetry(callee *node, f handleReply, rs *RetryState) {
	reqID := IncrCallNumber()
	c.ReqID = reqID
	c.caller.pendingCallMapMu.Lock()
	c.caller.pendingCallMap[c.ReqID] = &pendingCall{reply: nil, call: c, f: f}
	c.caller.pendingCallMapMu.Unlock()

	if !callee.tryAcceptCall(c) {
		oc := &OutboundCall{
			call:       c,
			callee:     callee,
			callback:   f,
			queuedAt:   Milliseconds(c.caller.loop.GetTime()),
			retryState: rs,
		}
		c.caller.queueOutbound(oc)
	}
}
