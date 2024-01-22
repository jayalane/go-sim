// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

// this file is for mapping app conf to tasks and then doing them
import (
	"container/heap"
)

type closure func()

// Task is a structure to track work for an app locally
type Task struct {
	wakeup    Milliseconds
	startTime Milliseconds
	endPoint  string
	timeoutMs float64
	replyCh   chan *Result
	later     closure
	nextTask  *Task
}

func (n *Node) handleTasks() {
	now := n.loop.GetTime()
	n.tasksMu.Lock()
	defer n.tasksMu.Unlock()

	for {
		next := n.tasks.Peak()
		if next == nil {
			break
		}
		if float64(next.priority) < now {
			item := heap.Pop(&n.tasks)
			n.HandleTask(item.(*Item).value.(*Task))
			ml.La("Handled call", item.(*Item).value.(*Task), "len is now",
				len(n.tasks))
			continue
		} else {
			break
		}
	}
}

// HandleTask for node reads the app config and generates the work
func (n *Node) HandleTask(t *Task) {
	ml.La("Got a task to do", *t)
}
