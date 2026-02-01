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

const (
	oneHundred = 100
)

// CallCB allows LB to override node.
type CallCB func(c *Call)

// node is a simulation particle that
// can take in or emit work; it is a node.
type node struct {
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
	resources        *NodeResources // Resource utilization tracking
	outboundQueue    []*OutboundCall
	outboundMu       sync.Mutex
	App              *AppConf
}

type pendingCall struct {
	//	reqID int
	reply *Reply
	call  *Call
	f     handleReply
}

// InitCallMap inits the pending call hash and
// starts a go routine to listen.
func (n *node) initCallMap() {
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

func (n *node) addCall(j *Call) {
	n.callsMu.Lock()
	defer n.callsMu.Unlock()

	i := &Item{
		value:    j,
		priority: int(j.Wakeup),
	}

	ml.La(n.name+": Add call", j.ReqID, j.caller.name, len(n.calls), j.Wakeup)
	count.IncrSyncSuffix("node_add_call", n.name)
	heap.Push(&n.calls, i)
}

func (n *node) addTask(t *Task) {
	n.tasksMu.Lock()
	defer n.tasksMu.Unlock()

	i := &Item{
		value:    t,
		priority: int(t.wakeup),
	}

	ml.La(n.name+": Pre Add task", len(n.tasks), t.wakeup, t.call.ReqID, t.call.caller.name)
	count.IncrSyncSuffix("node_add_task", n.name)
	heap.Push(&n.tasks, i)
}

// tryAcceptCall atomically checks callee network capacity and isDown.
// If the callee has room, it consumes network and pushes onto callCh.
// Returns true if the call was accepted, false otherwise.
func (n *node) tryAcceptCall(c *Call) bool {
	if n.resources != nil {
		n.resources.mu.RLock()
		isDown := n.resources.isDown
		n.resources.mu.RUnlock()

		if isDown {
			return false
		}

		if err := n.consumeNetworkForCall(); err != nil {
			return false
		}
	}

	select {
	case n.callCh <- c:
		count.IncrSyncSuffix("call_ch_sent", "call")

		return true
	default:
		return false
	}
}

// queueOutbound adds an outbound call to the sender's queue,
// consuming sender memory for the queued call.
func (n *node) queueOutbound(oc *OutboundCall) {
	n.outboundMu.Lock()
	n.outboundQueue = append(n.outboundQueue, oc)
	n.outboundMu.Unlock()

	// Consume sender memory for queued call
	if n.resources != nil {
		if err := n.consumeMemoryForCall(); err != nil {
			ml.La(n.name+": Memory error queuing outbound call:", err.Error())
		}
	}

	count.IncrSyncSuffix("outbound_queued", n.name)
	ml.La(n.name+": Queued outbound call", oc.call.ReqID, "to", oc.callee.name)
}

// drainOutbound attempts to deliver queued outbound calls each tick.
func (n *node) drainOutbound() {
	n.outboundMu.Lock()

	if len(n.outboundQueue) == 0 {
		n.outboundMu.Unlock()

		return
	}

	// Copy queue and release lock so tryAcceptCall doesn't deadlock
	queue := n.outboundQueue
	n.outboundQueue = nil
	n.outboundMu.Unlock()

	now := Milliseconds(n.loop.GetTime())
	var remaining []*OutboundCall

	for _, oc := range queue {
		// Check retry backoff delay
		if oc.retryState != nil && now < oc.retryState.nextRetryAt {
			remaining = append(remaining, oc)

			continue
		}

		// Check timeout
		if oc.call.TimeoutMs > 0 && float64(now-oc.queuedAt) > oc.call.TimeoutMs {
			// Timed out - send 503
			n.sendErrorReply(oc.call, "Outbound call timed out")
			count.IncrSyncSuffix("outbound_timeout", n.name)
			ml.La(n.name+": Outbound call timed out", oc.call.ReqID)

			continue
		}

		// Attempt delivery
		if oc.callee.tryAcceptCall(oc.call) {
			count.IncrSyncSuffix("outbound_delivered", n.name)
			ml.La(n.name+": Delivered outbound call", oc.call.ReqID)

			continue
		}

		// Delivery failed - handle retry
		if oc.retryState == nil {
			oc.retryState = &RetryState{
				policy:  DefaultRetryPolicy(),
				attempt: 0,
			}
		}

		oc.retryState.attempt++

		if oc.retryState.attempt > oc.retryState.policy.MaxRetries {
			// Retry exhaustion - send 503
			n.sendErrorReply(oc.call, "Outbound call retry exhausted")
			count.IncrSyncSuffix("outbound_retry_exhausted", n.name)
			ml.La(n.name+": Outbound call retry exhausted", oc.call.ReqID)

			continue
		}

		delay := oc.retryState.policy.DelayForAttempt(oc.retryState.attempt)
		oc.retryState.nextRetryAt = now + delay
		remaining = append(remaining, oc)
		count.IncrSyncSuffix("outbound_retry", n.name)
	}

	if len(remaining) > 0 {
		n.outboundMu.Lock()
		n.outboundQueue = append(n.outboundQueue, remaining...)
		n.outboundMu.Unlock()
	}
}

// buildRemoteCallsFunc creates a closure for future work.
func (n *node) buildRemoteCallsFunc(c *Call, h *StageConf) func() {
	return func() {
		count.IncrSyncSuffix("node_task_run", n.name)
		ml.La(n.name+": Running closure for task", h, c.Params, c.ReqID, c.caller.name)

		for _, rc := range h.RemoteCalls {
			if h.FilterCall != nil {
				ml.La(n.name+": checking filter rule", c.Params, c.ReqID, c.caller.name)

				doCall := h.FilterCall(rc.Endpoint, c.Params)
				if !doCall {
					ml.La(n.name+": skipping", rc.Endpoint, "due to filter", c.ReqID, c.caller.name)
					count.IncrSyncSuffix("node_task_remote_call_filter",
						n.name)

					continue
				}
			}

			ml.La(n.name+": Fanning out to", rc.Endpoint, rc, c.ReqID, c.caller.name)
			count.IncrSyncSuffix("node_make_remote_call", n.name)
			count.IncrSyncSuffix("node_make_remote_call_"+rc.Endpoint, n.name)
			newCall := rc.MakeCall(n, c)
			newCall.StartTime = Milliseconds(n.loop.GetTime())
			lb := n.loop.GetLB(rc.Endpoint + "-lb")

			replyHandler := func(
				n *node,
				r *Reply,
			) {
				ml.La(n.name+": Got a reply", *r)
			}

			if rc.Retry != nil {
				rs := &RetryState{policy: rc.Retry}
				newCall.sendCallWithRetry(&lb.n, replyHandler, rs)
			} else {
				newCall.sendCall(&lb.n, replyHandler)
			}
		}
	}
}

// handleCall processes an incoming call.
func (n *node) handleCall(c *Call) {
	ml.La(n.name+": Got an incoming call:", c, n.name, c.ReqID)

	// Check if node is down - send error reply instead of queuing
	if n.resources != nil && !n.IsAvailable() {
		n.sendErrorReply(c, "Node is down")
		ml.La(n.name + ": Node down, sending error reply")

		return
	}

	// Consume memory for new call (network already consumed in tryAcceptCall)
	if n.resources != nil {
		if err := n.consumeMemoryForCall(); err != nil {
			ml.La(n.name+": Memory resource error:", err.Error())

			return
		}
	}

	tasks := make([]Task, len(n.App.Stages))

	for i, h := range n.App.Stages {
		ml.La(n.name+": Build a task for h", h, c.ReqID, c.caller.name, i)
		count.IncrSyncSuffix("node_task_make", n.name)
		count.IncrSyncSuffix("node_task_make_"+c.caller.name, n.name)

		p := rand.Float64() //nolint:gosec

		tasks[i] = Task{
			wakeup: Milliseconds(n.loop.GetTime() + h.LocalWork(p)), // TBD
			call:   c,
			reqID:  c.ReqID,
		}

		tasks[i].later = n.buildRemoteCallsFunc(c, h)
	}

	for i := range len(tasks) - 1 {
		tasks[i].nextTask = &tasks[i+1]
	}

	for i := range tasks {
		ml.La(n.name+": adding task", tasks[i].wakeup, tasks[i].later, len(n.tasks))
		n.addTask(&tasks[i])
	}
}

func (n *node) handleCalls() {
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
				ml.La(n.name+": Got call for LB", call.Params, call.ReqID, call.caller.name)
				// poorly implemented bug riddled CLOS
				n.callCB(call)
			} else {
				ml.La(n.name+": got call for node", call.Params, call.ReqID, call.caller.name)
				n.handleCall(call)
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

func (n *node) runner() {
	ml.La(n.name+": Starting runner goid", goid())

	if n.name == "" {
		panic("Unnamed node")
	}

	for {
		select {
		case c := <-n.callCh:
			ml.La(n.name+": Node got call ", c.ReqID, c.caller.name)
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

// Run starts the goroutine for this node
// it is getting the init time stuff also.
func (n *node) run() {
	ml.La(n.name, ": Doing Run/Init", goid())

	n.initCallMap()

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
func (n *node) nextMillisecond() {
	ml.La(n.name+":  running", n.loop.GetTime())

	// Drain outbound queue each tick
	n.drainOutbound()

	// Update resource utilization
	if n.resources != nil {
		n.updateResources()

		// Record metrics (need to lock for reading)
		n.resources.mu.RLock()
		cpuCurrent := n.resources.cpu.Current * oneHundred         // Convert to percentage (0-100)
		memoryCurrent := n.resources.memory.Current * oneHundred   // Convert to percentage (0-100)
		networkCurrent := n.resources.network.Current * oneHundred // Convert to percentage (0-100)
		n.resources.mu.RUnlock()

		count.MarkDistributionSuffix("cpu_utilization", cpuCurrent, n.name)
		count.MarkDistributionSuffix("memory_utilization", memoryCurrent, n.name)
		count.MarkDistributionSuffix("network_utilization", networkCurrent, n.name)
	}
}

// generateEvent does nothing for a base node.
func (n *node) generateEvent() {
	ml.La("Node", n.name, "has no events to generate")
}
