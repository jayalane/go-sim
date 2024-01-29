// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

import (
	"fmt"

	count "github.com/jayalane/go-counter"
)

// LbConf is the configuration of an application
type LbConf struct {
	Name string
	App  *AppConf
}

// LB is a load balancer
type LB struct {
	n            Node
	appInstances []*Node
	lastSent     int
}

// GetNode returns the Node struct for this interface
func (lb *LB) GetNode() *Node {
	return &lb.n
}

// Run starts the goroutine for this node
func (lb *LB) Run() {
	lb.n.Run()
}

// NextMillisecond runs all the work due in the next ms
func (lb *LB) NextMillisecond() {
	lb.n.NextMillisecond()
}

// GenerateEvent does nothing for a base node
func (lb *LB) GenerateEvent() {
	lb.n.GenerateEvent()
}

// HandleCall does nothing for a base node
func (lb *LB) HandleCall(c *Call) {
	ml.La(lb.n.name+": LB got an Incoming call:", c)
	lb.lastSent++
	poolSize := len(lb.appInstances)
	ml.La(lb.n.name+": sending call to", lb.lastSent%poolSize)
	newCall := lb.MakeCall(&lb.n, c, lb.appInstances[lb.lastSent%poolSize])
	newCall.params = c.params
	count.IncrSuffix("lb_call_send", lb.n.name)
	// the next line is able to reuse the call because
	// it is just an LB
	newCall.SendCall(lb.appInstances[lb.lastSent%poolSize],
		func(n *Node, r *Reply) {
			count.Incr("lb_call_get_reply")
			ml.La(n.name+": LB got a reply", *r, *c)
			lb.n.replyCh <- r
		},
	)
}

// MakeCall generates the call from an old call
func (lb *LB) MakeCall(n *Node, oldC *Call, destN *Node) *Call {
	c := Call{}
	if oldC != nil {
		c.reqID = oldC.reqID
	}
	c.caller = n
	c.timeoutMs = 90.0
	c.wakeup = Milliseconds(n.loop.GetTime() + 5.0) // TBD
	c.endPoint = destN.name
	c.params = oldC.params
	return &c
}

// MakeLB takes an LB config and a loop and returns an
// LB integrated into that loop
func MakeLB(lbConf *LbConf, l *Loop) *LB {
	lb := LB{}
	lb.n.App = lbConf.App
	lb.n.name = lb.n.App.Name + "-lb"
	lb.n.callCB = lb.HandleCall
	lb.appInstances = make([]*Node, lbConf.App.Size)

	for i := uint16(0); i < lbConf.App.Size; i++ {
		n := MakeApp(lbConf.App, l, "-"+fmt.Sprintf("%d", i))
		lb.appInstances[i] = n
	}

	l.AddLB(lb.n.name, &lb) // this name is the lookup for the app
	l.AddNode(&lb.n)        // this name is the lookup for the app

	return &lb
}
