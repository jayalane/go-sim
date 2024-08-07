// -*- tab-width:2 -*-

package sim

import (
	"fmt"
	"runtime"
	"sync"
)

// Loop is a main driver for the simulation
// call Run() after hooking up all the
// nodes to the SimEntryPoint(s) and
// adding them.
type Loop struct {
	time        float64
	muTime      sync.RWMutex
	sources     []*Source
	nodes       []*node
	lbs         map[string]*LB
	broadcaster *Broadcaster
}

// GetTime returns the current sim time safely.
func (l *Loop) GetTime() float64 {
	l.muTime.RLock()
	x := l.time
	l.muTime.RUnlock()

	return x
}

// IncrementTime adds one to the current sim time safely.
func (l *Loop) IncrementTime() {
	l.muTime.Lock()
	l.time++
	l.muTime.Unlock()
}

// Run starts the main loop and runs it for length msecs.
func (l *Loop) Run(length float64) {
	l.time = 1000 // instead of 1 or 0 - just to make them stand out.

	for i, s := range l.sources {
		fmt.Println("Call start source", i, s)
		s.Run()
	}

	for k, v := range l.lbs {
		fmt.Println("Call start LBs", k, v)
		v.Run()
	}

	for ; l.GetTime() < length+1000.0; l.IncrementTime() {
		var wg sync.WaitGroup

		ml.La("Main loop looping", l.GetTime(), "***********************************************", runtime.NumGoroutine(), goid())

		for i, s := range l.sources {
			fmt.Println("Call next ms sources", l.GetTime(), i, s.n.name)
			wg.Add(1) // Add 1 for the first task

			go func() {
				defer wg.Done()
				s.NextMillisecond()
			}()
		}

		for i, n := range l.nodes {
			ml.La(n.name+": Calling next ms", l.time, "app", n.App.Name, "order", i)
			wg.Add(1) // Add 1 for the first task.

			go func() {
				defer wg.Done()
				n.nextMillisecond()
			}()
		}

		l.broadcaster.Broadcast(&wg) // tell everyone the ms is over.
		wg.Wait()
	}

	for _, n := range l.nodes {
		n.done <- true
	}

	ml.La("Exiting main loop", "***********************************************")
}

// Stats prints out the accumulated stats for the run
// to stop the main loop.
func (l *Loop) Stats() {
	ml.La("TBD")
}

// NewLoop initializes and returns a simulation main loop.
func NewLoop() *Loop {
	// TBD use name
	loop := Loop{}
	loop.broadcaster = NewBroadcaster()

	return &loop
}

// addNode adds a node into Loop's internals.
func (l *Loop) addNode(n *node) {
	l.nodes = append(l.nodes, n)
	n.loop = l
}

// AddSource adds a event generator into Loop's internals.
func (l *Loop) AddSource(s *Source) {
	l.sources = append(l.sources, s)
	s.n.loop = l
}

// AddLB adds an LB into the loop.
func (l *Loop) AddLB(name string, lb *LB) {
	if l.lbs == nil {
		l.lbs = make(map[string]*LB, numLBs)
	}

	l.lbs[name] = lb
	lb.n.loop = l
}

// GetLB adds an LB for the named app.
func (l *Loop) GetLB(name string) *LB {
	if l.lbs == nil {
		l.lbs = make(map[string]*LB, numLBs)

		return nil
	}

	lb, ok := l.lbs[name]
	if !ok {
		return nil
	}

	return lb
}
