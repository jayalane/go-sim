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
	calls     chan *Job
	replies   chan *Job
	events    *PQueue
	done      chan bool
	resources map[string]float64
	stats     map[string]float64
	App       *AppConf
}

func (n *Node) addEvent(j *Job) {
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
		case e := <-n.calls:
			n.addEvent(e)
			ml.Ln("Node got event", *n, *e)

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
	go n.runner()
}

// NextMillisecond runs all the work due in the next ms
func (n *Node) NextMillisecond() {
	ml.Ls("Node", n, "running", Now)
}
