// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

import (
	"strconv"

	count "github.com/jayalane/go-counter"
)

const (
	lbSuffix = "-lb"
)

// LbConf is the configuration of an application.
type LbConf struct {
	Name string
	App  *AppConf
}

// LB is a load balancer.
type LB struct {
	n            Node
	appInstances []*Node
	lastSent     int
}

// GetNode returns the Node struct for this interface.
func (lb *LB) GetNode() *Node {
	return &lb.n
}

// Run starts the goroutine for this node.
func (lb *LB) Run() {
	lb.n.Run()
}

// NextMillisecond runs all the work due in the next ms.
func (lb *LB) NextMillisecond() {
	lb.n.NextMillisecond()
}

// GenerateEvent does nothing for an LB.
func (lb *LB) GenerateEvent() {
	lb.n.GenerateEvent()
}

// HandleCall just round robin forwards for an LB.
func (lb *LB) HandleCall(c *Call) {
	ml.La(lb.n.name+": LB got an Incoming call:", c, c.ReqID, c.caller.name)

	lb.lastSent++
	poolSize := len(lb.appInstances)

	ml.La(lb.n.name+": sending call", c.ReqID, "to", lb.appInstances[lb.lastSent%poolSize].name)

	newCall := lb.MakeCall(&lb.n, c, lb.appInstances[lb.lastSent%poolSize])
	newCall.Params = c.Params
	newCall.caller = &lb.n

	count.IncrSuffix("lb_call_send", lb.n.name)

	newCall.SendCall(lb.appInstances[lb.lastSent%poolSize],
		func(n *Node, r *Reply) {
			count.Incr("lb_call_get_reply")
			ml.La(n.name+": LB got a reply", *r, *c)
			r.reqID = c.ReqID
			c.caller.replyCh <- r
		},
	)
}

// MakeCall generates the call from an old call.
func (lb *LB) MakeCall(n *Node, oldC *Call, destN *Node) *Call {
	c := Call{}

	count.IncrSyncSuffix("remote_call_generated", n.name)

	if oldC != nil {
		c.ReqID = oldC.ReqID
	}

	c.caller = n
	c.TimeoutMs = 90.0
	c.Wakeup = Milliseconds(n.loop.GetTime() + 5.0) //nolint:gomnd // TBD
	c.Endpoint = destN.name
	c.Params = oldC.Params

	return &c
}

// MakeLB takes an LB config and a loop and returns an
// LB integrated into that loop.
func MakeLB(lbConf *LbConf, l *Loop) *LB {
	lb := LB{}
	lb.n.App = lbConf.App
	lb.n.name = lbConf.Name + lbSuffix
	lb.n.callCB = lb.HandleCall
	lb.appInstances = make([]*Node, lbConf.App.Size)

	for i := uint16(0); i < lbConf.App.Size; i++ {
		n := MakeApp(lbConf, l, "-"+strconv.FormatUint(uint64(i), 10))
		lb.appInstances[i] = n
	}

	l.AddLB(lb.n.name, &lb) // this name is the lookup for the app
	l.AddNode(&lb.n)        // this name is the lookup for the app

	return &lb
}
