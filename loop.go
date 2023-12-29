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
	time          uint64
	eventHandlers []*TimeProcessor
	sources       []*EntryPoint
}

// Run starts the main loop as a go routine and returns a channel
// to stop the main loop
func (l *Loop) Run() chan bool {
	stopCh := make(chan bool, 2)
	go func(stopCh chan bool) {
		l.time = 1000
		for i, s := range l.sources {
			fmt.Println("Calling sources for time", l.time, i, s)
		}
	}(stopCh)
	return stopCh
}
