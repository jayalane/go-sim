// -*- tab-width:2 -*-

package sim

import (
	"fmt"
	"sync"
)

// Loop is a main driver for the simulation
// call Run() after hooking up all the
// nodes to the SimEntryPoint(s) and
// adding them
type Loop struct {
	time        float64
	muTime      sync.RWMutex
	sources     []*Source
	nodes       []*Node
	lbs         map[string]*LB
	broadcaster *Broadcaster
}

// GetTime returns the current sim time safely
func (l *Loop) GetTime() float64 {
	l.muTime.RLock()
	x := l.time
	l.muTime.RUnlock()
	return x
}

// IncrementTime adds one to the current sim time safely
func (l *Loop) IncrementTime() {
	l.muTime.Lock()
	l.time++
	l.muTime.Unlock()
}

// Run starts the main loop and runs it for length msecs
func (l *Loop) Run(length float64) {
	l.time = 1000
	for i, s := range l.sources {
		fmt.Println("Call start source", i, s)
		s.Run()
	}
	for k, v := range l.lbs {
		fmt.Println("Call start LBs", k, v)
		v.Run()
	}
	for ; l.GetTime() < length+1000.0; l.IncrementTime() {

		for i, s := range l.sources {
			fmt.Println("Call next ms sources", l.time, i, s.n.name)
			s.NextMillisecond()
		}
		for i, n := range l.nodes {
			ml.Ln("Calling next ms nodes", l.time, n.App.Name, i, n.name)
			n.NextMillisecond()
		}
		l.broadcaster.Broadcast() // tell everyone the ms is over
	}
}

// Stats prints out the accumulated stats for the run
// to stop the main loop
func (l *Loop) Stats() {
	ml.La("TBD")
}

// NewLoop initializes and returns a simulation main loop
func NewLoop(name string) *Loop {
	// TBD use name
	loop := Loop{}
	loop.broadcaster = NewBroadcaster()
	return &loop
}

// AddNode adds a node into Loop's internals
func (l *Loop) AddNode(n *Node) {
	l.nodes = append(l.nodes, n)
	n.loop = l
}

// AddSource adds a event generator into Loop's internals
func (l *Loop) AddSource(s *Source) {
	l.sources = append(l.sources, s)
	s.n.loop = l
}

// AddLB adds an LB into the loop
func (l *Loop) AddLB(name string, lb *LB) {
	if l.lbs == nil {
		l.lbs = make(map[string]*LB, 1000)
	}
	l.lbs[name] = lb
	lb.n.loop = l

	return
}

// GetLB adds an LB for the named app
func (l *Loop) GetLB(name string) *LB {
	if l.lbs == nil {
		l.lbs = make(map[string]*LB, 1000)
		return nil
	}
	lb, ok := l.lbs[name]
	if !ok {
		return nil
	}

	return lb
}
