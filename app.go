// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

import (
	count "github.com/jayalane/go-counter"
)

const (
	networkDelayConst = 5.0 // later better method here
)

// RemoteCall is an endpoint and params.
type RemoteCall struct {
	Endpoint string
	Params   map[string]string
}

// RemoteCallFuncType is a callback to filter out remote calls based on params.
type RemoteCallFuncType func(endpoint string, params map[string]string) bool

// StageConf is the configuration of a stage of app work.
type StageConf struct {
	LocalWork   ModelCdf
	FilterCall  RemoteCallFuncType
	RemoteCalls []*RemoteCall
}

// AppConf is the configuration of an application.
type AppConf struct {
	Name      string
	Size      uint16
	ReplyLen  ModelCdf
	Stages    []*StageConf
	Resources *ResourceConfig // Optional resource configuration
}

// MakeApp takes and lb config and a loop
// integrates the new node into that loop. LB config
// is needed so name is per pool not per app.
func MakeApp(lb *LbConf, l *Loop, suffix string) {
	_ = makeApp(lb, l, suffix)
}

// makeApp takes and lb config and a loop and returns a
// node integrated into that loop.
func makeApp(lb *LbConf, l *Loop, suffix string) *node {
	n := node{}

	n.App = lb.App
	n.name = lb.Name + suffix

	// Initialize resources with app configuration
	n.initResources(lb.App.Resources)

	l.addNode(&n)
	n.run()

	return &n
}

// MakeCall generates the call from an old call.
func (r *RemoteCall) MakeCall(n *node, oldC *Call) *Call {
	count.IncrSyncSuffix("remote_call_generated", n.name)

	c := Call{}
	c.ReqID = IncrCallNumber()
	c.caller = n
	c.TimeoutMs = 90.0
	c.Wakeup = Milliseconds(n.loop.GetTime() + networkDelayConst) // nolint:gomnd //TBD
	c.Endpoint = r.Endpoint

	if oldC.Params != nil {
		c.Params = oldC.Params
	} else {
		c.Params = r.Params
	}

	return &c
}
