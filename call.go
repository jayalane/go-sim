// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

import (
	"sync"

	count "github.com/jayalane/go-counter"
)

// HandleReply type is a callback to process the reply from a call.
type HandleReply func(*Node, *Reply)

// Call is a structure to track a remote call.
type Call struct {
	wakeup    Milliseconds
	startTime Milliseconds
	endPoint  string
	timeoutMs float64
	reqID     int
	//	length     uint64
	//	id1        uint64
	// id2        uint64
	params map[string]string
	// connection *Connection
	caller *Node
}

var (
	callNumber      = 0
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

// SendCall sends the call to the callee node channel.
func (c *Call) SendCall(callee *Node, f HandleReply) {
	reqID := IncrCallNumber()
	c.reqID = reqID

	callee.pendingCallMapMu.Lock()
	callee.pendingCallMap[c.reqID] = &pendingCall{reply: nil, call: c, f: f}
	callee.pendingCallMapMu.Unlock()
	select {
	case callee.callCh <- c:
		count.Incr("call_ch_sent")
	default:
		panic("call channel full")
	}
}
