// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

import (
	"container/heap"
	"time"
)

// NodeInterface is the base interface used by sources, LBs, apps, DBs etc.
type NodeInterface interface {
	Run() // starts a goroutine
	NextMillisecond()
	GetMillisecond()
	StatsMillisecond()
	GetCallChannel() chan *Job
	GetReplyChannel() chan *Job
}

// Node is a simulation particle that
// can take in or emit work; it is a Node
type Node struct {
	loop       *Loop
	callCh     chan *Call
	repliesCh  chan *Call
	msCh       chan bool
	events     *PQueue
	done       chan bool
	resources  map[string]float64
	stats      map[string]float64
	App        *AppConf
	nextEvent  Milliseconds
	lambda     float64
	newEventCb EventCB // only for sources
}

func (n *Node) addEvent(j *Call) {
	i := &Item{
		value:    j,
		priority: int(j.wakeUp),
	}
	heap.Push(n.events, i)
}

func (n *Node) callWaiter(j *Job) {
	for {
		select {
		case j2 := <-j.replyCh:
			ml.Ln("Got reply", j2)
		}
	}
}

func (n *Node) runner() {
	for {
		select {
		case c := <-n.callCh:
			n.addEvent(c)
			ml.Ln("Node got call", *n, *c)

		case ms := <-n.msCh:
			ml.Ln("Node got ms", *n, ms)
			if n.newEventCb != nil {
				ml.Ln("Node got ms and cb", *n, ms, n.loop.GetTime())
				n.generateEvent()
			}

		case <-time.After(60 * time.Second):
			ml.Ls("Node without events for 60 seconds", *n)

		case <-n.done:
			ml.La("Node shutting down on done", *n)

			return
		}
	}
}

// Run starts the goroutine for this node
func (n *Node) Run() {
	n.msCh = n.loop.broadcaster.Subscribe()
	go n.runner()
}

// NextMillisecond runs all the work due in the next ms
func (n *Node) NextMillisecond() {
	ml.Ls("Node", *n, "running", n.loop.GetTime())
}
