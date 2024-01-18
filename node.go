// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

import (
	"container/heap"
	"sync"
	"time"
)

// NodeInterface is the base interface used by sources, LBs, apps, DBs etc.
type NodeInterface interface {
	Run()    // starts a goroutine
	RunApp() // from a task
	GetNode()
	GenerateEvent()
	NextMillisecond()
	GetMillisecond()
	StatsMillisecond()
	GetCallChannel() chan *Job
	GetReplyChannel() chan *Job
	HandleCall(*Call)
	HandleTask(*Task)
}

// Node is a simulation particle that
// can take in or emit work; it is a Node
type Node struct {
	loop      *Loop
	callCh    chan *Call
	repliesCh chan *Result
	msCh      chan bool
	tasksMu   sync.Mutex
	tasks     PQueue
	callsMu   sync.Mutex
	calls     PQueue
	done      chan bool
	name      string
	resources map[string]float64 // for limits later on
	stats     map[string]float64
	App       *AppConf
}

func (n *Node) addCall(j *Call) {
	n.callsMu.Lock()
	defer n.callsMu.Unlock()
	i := &Item{
		value:    j,
		priority: int(j.wakeUp),
	}
	ml.La("Add call", n.name, len(n.calls), j.wakeup)
	heap.Push(&n.calls, i)
}

func (n *Node) addTask(t *Task) {
	n.callsMu.Lock()
	defer n.callsMu.Unlock()
	i := &Item{
		value:    t,
		priority: int(t.wakeUp),
	}
	ml.La("Add task", n.name, len(n.tasks), t.wakeup)
	heap.Push(&n.tasks, i)
}

func (n *Node) callWaiter(j *Job) {
	for {
		select {
		case j2 := <-j.replyCh:
			ml.Ln("Got reply", j2)
		}
	}
}

// HandleCall processes an incoming call
func (n *Node) HandleCall(c *Call) {
	ml.La("Got a call:", c, n.name)
	for _, h := range n.App.Stages {
		ml.La("Build a task for h", *h)
		p := rand.Float64()
		task := Task{
			wakeUp: Milliseconds(n.loop.GetTime() + h.LocalWork[p]), // TBD
		}
		n.addTask(&task)
	}
}

func (n *Node) handleCalls() {
	now := n.loop.GetTime()
	n.callsMu.Lock()
	defer n.callsMu.Unlock()

	for {
		next := n.calls.Peak()
		if next == nil {
			break
		}
		if float64(next.priority) < now {
			item := heap.Pop(&n.calls)
			n.HandleCall(item.(*Item).value.(*Call))
			ml.La("Handled call", item.(*Item).value.(*Call), "len is now", len(n.calls))
			continue
		} else {
			break
		}
	}
}

func (n *Node) runner() {
	ml.La("Starting runner for", n.name)
	if n.name == "" {
		panic("Unnamed node")
	}
	for {
		select {
		case c := <-n.callCh:
			n.addCall(c)
			ml.Ln("Node got call!!!!!!!", n.name, *c)

		case ms := <-n.msCh:
			n.callsMu.Lock()
			ml.Ln("Raw Node got ms", n.name, ms, len(n.calls))
			n.callsMu.Unlock()
			n.handleCalls()

		case <-time.After(60 * time.Second):
			ml.Ls("Node without events for 60 seconds", n.name)

		case <-n.done:
			ml.La("Node shutting down on done", n.name)

			return
		}
	}
}

// GetNode returns the Node struct for this interface
func (n *Node) GetNode() *Node {
	return n
}

// Run starts the goroutine for this node
// it is getting the init time stuff also
func (n *Node) Run() {
	ml.La("Doing Run/Init for", n.name)
	if n.name == "count-lb" {
		ml.La("Got count-lb")
	}

	n.callCh = make(chan *Call, 100) // ?
	n.msCh = n.loop.broadcaster.Subscribe()
	n.calls = make(PQueue, 0)
	n.tasks = make(PQueue, 0)
	heap.Init(&n.calls)
	heap.Init(&n.tasks)

	go n.runner()
}

// NextMillisecond runs all the work due in the next ms
func (n *Node) NextMillisecond() {
	ml.Ls("Node", n.name, "running", n.loop.GetTime())
}

// GenerateEvent does nothing for a base node
func (n *Node) GenerateEvent() {
	ml.Ls("Node", n.name, "has no events to generate")
}
