// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

// this file is for mapping app conf to tasks and then doing them.
import (
	"container/heap"
	"math/rand"
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
	n.tasksMu.Lock()
	defer n.tasksMu.Unlock()

	ml.La(n.name+": handle tasks, time is ", n.loop.GetTime())

	for {
		next := n.tasks.Peak()
		if next == nil {
			if len(n.tasks) > 0 {
				panic("wtf")
			}

			ml.La(n.name + ": no tasks")

			break
		}

		if float64(next.priority) <= now {
			item := heap.Pop(&n.tasks)

			task, ok := item.(*Item).value.(*Task)
			if !ok {
				panic("task pqueue had non task")
			}

			n.HandleTask(task)
			ml.La(n.name+": Handled task", task, "len is now",
				len(n.tasks), "reqid", task.call.ReqID, "from", task.call.caller.name)

			continue
		}
		// like an else here thanks lint
		ml.La(n.name+": Task too young", next.priority, "len is now",
			len(n.tasks))

		break
	}
}

// handleTask for node reads the app config and generates the work.
func (n *node) HandleTask(t *Task) {
	ml.La(n.name+": Got a task to do", *t, t.call.ReqID, t.call.caller.name)

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

		r := Reply{}
		p := rand.Float64() //nolint:gosec
		r.reqID = t.reqID
		r.length = uint64(n.App.ReplyLen(p))
		r.status = 0
		r.call = t.call
		t.call.caller.replyCh <- &r
	}
}
