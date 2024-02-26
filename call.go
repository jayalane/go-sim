// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

import (
	"sync"

	count "github.com/jayalane/go-counter"
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
	caller *node
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
func (c *Call) sendCall(callee *node, f handleReply) {
	reqID := IncrCallNumber()
	c.ReqID = reqID
	c.caller.pendingCallMapMu.Lock()
	c.caller.pendingCallMap[c.ReqID] = &pendingCall{reply: nil, call: c, f: f}
	c.caller.pendingCallMapMu.Unlock()
	select {
	case callee.callCh <- c:
		count.IncrSyncSuffix("call_ch_sent", "call")
	default:
		panic("call channel full")
	}
}
