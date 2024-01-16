// -*- tab-width:2 -*-

package sim

import (
	"fmt"
)

// Loop is a main driver for the simulation
// call Run() after hooking up all the
// nodes to the SimEntryPoint(s) and
// adding them
type Loop struct {
	time        float64
	sources     []*Source
	nodes       []*Node
	lbs         map[string]*LB
	broadcaster *Broadcaster
}

// Run starts the main loop as a go routine and returns a channel
// to stop the main loop
func (l *Loop) Run(length float64) {
	l.broadcaster = NewBroadcaster()

	l.time = 1000
	for i, s := range l.sources {
		fmt.Println("Call run sources", l.time, i, s)
		s.Run()
	}
	for ; l.time < length+1000.0; l.time = l.time + 1.0 {
		Now = Milliseconds(l.time)
		for i, s := range l.sources {
			fmt.Println("Call next ms sources", l.time, i, s)
			s.NextMillisecond()
		}
		for i, n := range l.nodes {
			n.NextMillisecond()
			ml.Ln("Calling node for time", l.time, n.App.Name, i, n)
		}
		l.broadcaster.Broadcast() // tell everyone the ms is over
	}
}

// Stats prints out the accumulated stats for the run
// to stop the main loop
func (l *Loop) Stats() {
	ml.La("TBD")
}

// AddNode adds a node into Loop's internals
func (l *Loop) AddNode(n *Node) {
	l.nodes = append(l.nodes, n)
}

// AddSource adds a event generator into Loop's internals
func (l *Loop) AddSource(s *Source) {
	l.sources = append(l.sources, s)
}

// AddLB adds an LB into the loop
func (l *Loop) AddLB(name string, lb *LB) {
	if l.lbs == nil {
		l.lbs = make(map[string]*LB, 1000)
	}
	l.lbs[name] = lb

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
