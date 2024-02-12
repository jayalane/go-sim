// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

import (
	"container/heap"
	"math/rand"
	"sync"
	"time"

	count "github.com/jayalane/go-counter"
)

// CallCB allows LB to override node.
type CallCB func(c *Call)

// Node is a simulation particle that
// can take in or emit work; it is a Node.
type Node struct {
	loop             *Loop
	callCh           chan *Call
	msCh             chan *sync.WaitGroup
	tasksMu          sync.Mutex
	tasks            PQueue
	callsMu          sync.Mutex
	calls            PQueue
	callCB           CallCB
	pendingCallMap   map[int]*pendingCall
	pendingCallMapMu sync.RWMutex
	replyCh          chan *Reply
	done             chan bool
	name             string
	//	resources        map[string]float64 // for limits later on
	// stats            map[string]float64
	App *AppConf
}

type pendingCall struct {
	//	reqID int
	reply *Reply
	call  *Call
	f     HandleReply
}

// InitCallMap inits the pending call hash and
// starts a go routine to listen.
func (n *Node) InitCallMap() {
	n.replyCh = make(chan *Reply, bufferSizes)
	n.pendingCallMapMu.Lock()
	n.pendingCallMap = make(map[int]*pendingCall)
	n.pendingCallMapMu.Unlock()

	go func() {
		ml.La(n.name+" starting reply loop", goid())

		for {
			select {
			case response := <-n.replyCh:
				ml.La(n.name+": Got a reply", response)
				n.pendingCallMapMu.RLock()
				val, ok := n.pendingCallMap[response.reqID]
				n.pendingCallMapMu.RUnlock()

				if !ok {
					ml.La(n.name+": Dropping unknown reqid", response.reqID)
					count.IncrSyncSuffix("call_reply_dropping_unknown", n.name)

					continue
				}

				count.IncrSyncSuffix("call_reply_known", n.name)
				n.pendingCallMapMu.Lock()
				delete(n.pendingCallMap, response.reqID) // slight race but short repeats not issue
				n.pendingCallMapMu.Unlock()
				ml.La(n.name + ": about to callback reply")
				val.f(n, response)
				ml.La(n.name+": done handling reply", n.loop.GetTime(), response)
			case <-time.After(secondsInMin * time.Second):
				ml.La(n.name+": one minute with no replies", n.loop.GetTime())
			}
		}
	}()
}

func (n *Node) addCall(j *Call) {
	n.callsMu.Lock()
	defer n.callsMu.Unlock()

	i := &Item{
		value:    j,
		priority: int(j.wakeup),
	}

	ml.La(n.name+": Add call", j.reqID, j.caller.name, len(n.calls), j.wakeup)
	count.IncrSyncSuffix("node_add_call", n.name)
	heap.Push(&n.calls, i)
}

func (n *Node) addTask(t *Task) {
	n.tasksMu.Lock()
	defer n.tasksMu.Unlock()

	i := &Item{
		value:    t,
		priority: int(t.wakeup),
	}

	ml.La(n.name+": Pre Add task", len(n.tasks), t.wakeup, t.call.reqID, t.call.caller.name)
	count.IncrSyncSuffix("node_add_task", n.name)
	heap.Push(&n.tasks, i)
}

// HandleCall processes an incoming call.
func (n *Node) HandleCall(c *Call) {
	ml.La(n.name+": Got an incoming call:", c, n.name, c.reqID)
	tasks := make([]Task, len(n.App.Stages))

	for i, h := range n.App.Stages {
		ml.La(n.name+": Build a task for h", h, c.reqID, c.caller.name, i)
		count.IncrSyncSuffix("node_task_make", n.name)
		count.IncrSyncSuffix("node_task_make_"+c.caller.name, n.name)

		p := rand.Float64() //nolint:gosec

		tasks[i] = Task{
			wakeup: Milliseconds(n.loop.GetTime() + h.LocalWork(p)), // TBD
			call:   c,
			reqID:  c.reqID,
		}

		tasks[i].later = func() {
			count.IncrSyncSuffix("node_task_run", n.name)
			ml.La(n.name+": Running closure for task", h, c.params, c.reqID, c.caller.name)

			for _, rc := range h.RemoteCalls {
				rc := rc

				if h.FilterCall != nil {
					ml.La(n.name+": checking filter rule", c.params, c.reqID, c.caller.name)

					doCall := h.FilterCall(rc.endpoint, c.params)
					if !doCall {
						ml.La(n.name+": skipping", rc.endpoint, "due to filter", c.reqID, c.caller.name)
						count.IncrSyncSuffix("node_task_remote_call_filter",
							n.name)

						continue
					}
				}

				ml.La(n.name+": Fanning out to", rc.endpoint, rc, c.reqID, c.caller.name)
				count.IncrSyncSuffix("node_make_remote_call", n.name)
				count.IncrSyncSuffix("node_make_remote_call_"+rc.endpoint, n.name)
				newCall := rc.MakeCall(n, c)
				lb := n.loop.GetLB(rc.endpoint + "-lb")

				newCall.SendCall(&lb.n,
					func(
						n *Node,
						r *Reply,
					) {
						ml.La(n.name+": Got a reply", *r)
						count.IncrSyncSuffix("node_task_get_reply", n.name)
						count.IncrSyncSuffix("node_task_get_reply_"+rc.endpoint, n.name)
						n.replyCh <- r
					},
				)
			}
		}
	}

	for i := 0; i < len(tasks)-1; i++ {
		tasks[i].nextTask = &tasks[i+1]
	}

	for i := 0; i < len(tasks); i++ {
		ml.La(n.name+": adding task", tasks[i].wakeup, tasks[i].later, len(n.tasks))
		n.addTask(&tasks[i])
	}
}

func (n *Node) handleCalls() {
	now := n.loop.GetTime()
	n.callsMu.Lock()
	defer n.callsMu.Unlock()

	for {
		next := n.calls.Peak()
		if next == nil {
			if len(n.calls) > 0 {
				panic("wtf")
			}

			ml.La(n.name + ": no calls")

			break
		}

		if float64(next.priority) < now {
			item := heap.Pop(&n.calls)
			call, ok := item.(*Item).value.(*Call)

			if !ok {
				panic("Got non-call from call pqueue")
			}

			if n.callCB != nil {
				ml.La(n.name+": Got call for LB", call.params, call.reqID, call.caller.name)
				// poorly implemented bug riddled CLOS
				n.callCB(call)
			} else {
				ml.La(n.name+": got call for node", call.params, call.reqID, call.caller.name)
				n.HandleCall(call)
			}

			ml.La(n.name+": Handled call",
				call,
				"len is now", len(n.calls))

			continue
		}

		ml.La(n.name+": call too young", next.priority, "len is now",
			len(n.calls))

		break
	}
}

func (n *Node) runner() {
	ml.La(n.name+": Starting runner goid", goid())

	if n.name == "" {
		panic("Unnamed node")
	}

	for {
		select {
		case c := <-n.callCh:
			ml.La(n.name+": Node got call ", c.reqID, c.caller.name)
			n.addCall(c)

		case msWg := <-n.msCh:
			n.callsMu.Lock()
			ml.La(n.name+": Raw Node got ms", len(n.calls), len(n.tasks))
			n.callsMu.Unlock()
			n.handleCalls()
			n.handleTasks()
			ml.La(n.name + ": Ending msWG")
			msWg.Done()

		case <-time.After(secondsInMin * time.Second):
			ml.La(n.name + "Node without events for 60 seconds")

		case <-n.done:
			ml.La(n.name + ":Node shutting down on done")

			return
		}
	}
}

// GetNode returns the Node struct for this interface.
func (n *Node) GetNode() *Node {
	return n
}

// Run starts the goroutine for this node
// it is getting the init time stuff also.
func (n *Node) Run() {
	ml.La(n.name, ": Doing Run/Init", goid())

	n.InitCallMap()

	n.callCh = make(chan *Call, bufferSizes) // ?
	n.done = make(chan bool, smallChannelSize)
	n.msCh = n.loop.broadcaster.Subscribe()
	n.calls = make(PQueue, 0)
	n.tasks = make(PQueue, 0)
	heap.Init(&n.calls)
	heap.Init(&n.tasks)

	go n.runner()
}

// NextMillisecond runs all the work due in the next ms.
func (n *Node) NextMillisecond() {
	ml.La(n.name+":  running", n.loop.GetTime())
}

// GenerateEvent does nothing for a base node.
func (n *Node) GenerateEvent() {
	ml.La("Node", n.name, "has no events to generate")
}
