// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

// this file is for mapping app conf to tasks and then doing them.
import (
	"container/heap"
	"math/rand"

	count "github.com/jayalane/go-counter"
)

type closure func()

// Task is a structure to track work for an app locally.
type Task struct {
	wakeup Milliseconds
	// startTime Milliseconds
	reqID    int
	call     *Call
	later    closure
	nextTask *Task
}

func (n *node) handleTasks() {
	now := n.loop.GetTime()

	ml.La(n.name+": handle tasks, time is ", n.loop.GetTime())

	for {
		n.tasksMu.Lock()

		next := n.tasks.Peak()
		if next == nil {
			if len(n.tasks) > 0 {
				panic("wtf")
			}

			ml.La(n.name + ": no tasks")
			n.tasksMu.Unlock()

			break
		}

		if float64(next.priority) <= now {
			item := heap.Pop(&n.tasks)
			n.tasksMu.Unlock()

			task, ok := item.(*Item).value.(*Task)
			if !ok {
				panic("task pqueue had non task")
			}

			n.HandleTask(task)
			ml.La(n.name+": Handled task", task,
				"reqid", task.call.ReqID, "from", task.call.caller.name)

			continue
		}
		// like an else here thanks lint
		ml.La(n.name+": Task too young", next.priority)
		n.tasksMu.Unlock()

		break
	}
}

// handleTask for node reads the app config and generates the work.
func (n *node) HandleTask(t *Task) {
	ml.La(n.name+": Got a task to do", *t, t.call.ReqID, t.call.caller.name)

	// Check if node is available
	if n.resources != nil && !n.IsAvailable() {
		// Queue task for later processing
		n.resources.mu.Lock()
		n.resources.pendingWork = append(n.resources.pendingWork, t.call)
		n.resources.mu.Unlock()
		ml.La(n.name + ": Node down, queuing task for later")

		return
	}

	// Consume CPU for local work
	if n.resources != nil {
		if n.handleTaskCPU(t) {
			return
		}
	}

	if t.later != nil {
		t.later()
		ml.La(n.name + ": ran closure")
	} else {
		ml.La(n.name + ": No closure to run")
	}

	if t.nextTask != nil {
		ml.La(n.name + ": another task to do")
	} else {
		ml.La(n.name+": last task, send result", "reqid", t.call.ReqID, t.call.caller.name)

		// Consume network resources for reply
		if n.resources != nil {
			if err := n.consumeNetworkForReply(); err != nil {
				ml.La(n.name+": Network resource error sending reply:", err.Error())

				return
			}
		}

		r := Reply{}
		p := rand.Float64() //nolint:gosec
		r.reqID = t.reqID
		r.length = uint64(n.App.ReplyLen(p))
		r.status = 0
		r.call = t.call
		t.call.caller.replyCh <- &r
	}
}

// handleTaskCPU consumes CPU and checks reject/delay limits.
// Returns true if the task was handled (rejected or re-queued).
func (n *node) handleTaskCPU(t *Task) bool {
	n.consumeCPUForLocalWork()

	// CPURejectLimit check - if CPU above reject threshold, send 503
	if n.resources.config.CPURejectLimit > 0 {
		n.resources.mu.RLock()
		cpuCurrent := n.resources.cpu.Current
		n.resources.mu.RUnlock()

		if cpuCurrent >= n.resources.config.CPURejectLimit {
			count.IncrSyncSuffix("node_cpu_reject", n.name)
			ml.La(n.name+": CPU above reject limit, sending 503", cpuCurrent)
			n.sendErrorReply(t.call, "CPU reject limit exceeded")

			return true
		}
	}

	// Check if CPU delay should be applied
	delay := n.calculateCPUDelay()
	if delay > 0 {
		ml.La(n.name+": CPU saturated, delaying task by", delay, "ms")
		t.wakeup += Milliseconds(delay)
		n.addTask(t) // Re-queue with delay

		// Consume memory for queued work (CPU saturation creates OOM pressure)
		if err := n.consumeMemoryForQueuedCall(); err != nil {
			ml.La(n.name+": Memory error for queued work:", err.Error())
		}

		return true
	}

	return false
}
