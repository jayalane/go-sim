// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

import (
	count "github.com/jayalane/go-counter"
)

// RemoteCall is an endpoint and params.
type RemoteCall struct {
	endpoint string
	params   map[string]string
}

// RemoteCallFuncType is a callback to filter out remote calls based on params.
type RemoteCallFuncType func(endpoint string, params map[string]string) bool

// StageConf is the configuration of a stage of app work.
type StageConf struct {
	LocalWork   ModelCdf
	FilterCall  RemoteCallFuncType
	RemoteCalls []RemoteCall
}

// AppConf is the configuration of an application.
type AppConf struct {
	Name     string
	Size     uint16
	ReplyLen ModelCdf
	Stages   []StageConf
}

// MakeApp takes and lb config and a loop and returns a
// node integrated into that loop. LB config
// is needed so name is per pool not per app.
func MakeApp(lb *LbConf, l *Loop, suffix string) *Node {
	n := Node{}

	n.App = lb.App
	n.name = lb.Name + suffix

	l.AddNode(&n)
	n.Run()

	return &n
}

// MakeCall generates the call from an old call.
func (r *RemoteCall) MakeCall(n *Node, oldC *Call) *Call {
	count.IncrSyncSuffix("remote_call_generated", n.name)

	c := Call{}
	c.reqID = IncrCallNumber()
	c.caller = n
	c.timeoutMs = 90.0
	c.wakeup = Milliseconds(n.loop.GetTime() + 5.0) // nolint:gomnd //TBD
	c.endPoint = r.endpoint

	if oldC.params != nil {
		c.params = oldC.params
	} else {
		c.params = r.params
	}

	return &c
}
