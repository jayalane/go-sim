// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

// Job is work generated by a call to an app instance
// in the app
type Job struct {
	wakeUp       Milliseconds
	startTime    Milliseconds
	length       uint64
	id1          uint64
	id2          uint64
	TimeoutMs    float64
	pendingCalls map[string]*Call
	replyCh      chan *Result
}
