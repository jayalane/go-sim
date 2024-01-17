// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

import (
	"container/heap"
	"math/rand"
)

// EventCB is called by a source to generate and send the new
// work
type EventCB func(s *Source) *Call

// SourceConf configures an event source
type SourceConf struct {
	Name     string
	Lambda   float64
	MakeCall EventCB
}

const (
	none = iota
	sleeping
)

// Source is a source of events
type Source struct {
	n Node
	// Later:  (can't say to do due to linting) have concept of
	// customer flow)
	state      int
	nextEvent  Milliseconds
	lambda     float64
	newEventCb EventCB // only for sources
}

// GetNode returns the embedded Node
func (s *Source) GetNode() *Node {
	return &s.n
}

// Run starts the goroutine for this node
func (s *Source) Run() {
	ml.La("Doing Run/Init for source ", s.n.name)
	s.n.msCh = s.n.loop.broadcaster.Subscribe()
	s.n.callCh = make(chan *Call, 100) // ?
	s.n.tasks = make(PQueue, 0)
	heap.Init(&s.n.tasks)
	go s.n.runner()
}

// GenerateEvent for a source generates load
func (s *Source) GenerateEvent() {
	ml.La("Generate EVENT!", s.n.name, s.n.loop.GetTime())
	c := s.newEventCb(s)
	c.startTime = Milliseconds(s.n.loop.GetTime())
	lb := s.n.loop.GetLB(c.endPoint + "-lb")
	c.SendCall(lb)
}

// HandleTask for a source does nothing
func (s *Source) HandleTask() {
	ml.La("Source got a task?", s.n.name, s.n.loop.GetTime())
}

// NextMillisecond runs all the work due in the last ms for a source
func (s *Source) NextMillisecond() {
	ml.Ls("Source", s.n.name, "running", s.n.loop.GetTime())

	if s.state == sleeping {
		if Milliseconds(s.n.loop.GetTime()) > s.nextEvent {
			// make the call
			s.GenerateEvent()
			s.state = none
		}

		return
	}

	s.state = sleeping

	timeToSleep := rand.ExpFloat64() / s.lambda

	s.nextEvent = Milliseconds(timeToSleep + s.n.loop.GetTime())

	ml.Ls("Source", s.n.name, "sleeping for", timeToSleep, "ms")
}

// MakeSource turns a source configuration into the source
func MakeSource(sourceConf *SourceConf, l *Loop) *Source {
	source := Source{}
	source.n.name = sourceConf.Name
	source.lambda = sourceConf.Lambda
	source.newEventCb = sourceConf.MakeCall

	return &source
}
