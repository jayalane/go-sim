// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

import (
	"fmt"
	"math/rand"

	count "github.com/jayalane/go-counter"
)

const (
	msInSec = 1000.0
)

// EventCB is called by a source to generate and send the new
// work.
type EventCB func(s *Source) *Call

// SourceConf configures an event source.
type SourceConf struct {
	Name     string
	Lambda   float64
	MakeCall EventCB
}

// Source is a source of events.
type Source struct {
	n node
	// Later:  (can't say to do due to linting) have concept of
	// customer flow)
	// state      int
	nextEvent  Milliseconds
	lambda     float64
	newEventCb EventCB // only for sources
}

// GetTime returns the loop time.
func (s *Source) GetTime() float64 {
	return s.n.loop.GetTime()
}

// Run starts the goroutine for this node.
func (s *Source) Run() {
	ml.La("Doing Run/Init for source ", s.n.name)
	s.n.run()
}

// GenerateEvent for a source generates load.
func (s *Source) GenerateEvent() {
	count.IncrSuffix("source_generated", "source")

	c := s.newEventCb(s)
	c.caller = &s.n
	c.StartTime = Milliseconds(s.n.loop.GetTime())
	lb := s.n.loop.GetLB(c.Endpoint + "-lb")

	ml.La("Generate EVENT!", s.n.name, s.n.loop.GetTime(), c.ReqID, lb.n.name)

	c.sendCall(&lb.n,
		func(
			n *node,
			r *Reply,
		) {
			ml.La("Finished EVENT!", s.n.name, s.n.loop.GetTime(), n.name, r)
			count.IncrSuffix("source_generated_finished", "source")
			count.MarkDistributionSuffix(s.n.name, (s.n.loop.GetTime()-float64(c.StartTime))/msInSec,
				"source")
		},
	)
}

// HandleCall for a source does nothing.
func (s *Source) HandleCall() {
	panic("Source got a task?" + s.n.name + fmt.Sprintf("%f", s.n.loop.GetTime()))
}

// NextMillisecond runs all the work due in the last ms for a source.
func (s *Source) NextMillisecond() {
	numThisMs := float64(0)

	ml.La(s.n.name+": source running next ms", s.n.loop.GetTime())

	if s.nextEvent <= 0 {
		timeToSleep := rand.ExpFloat64()/s.lambda + s.n.loop.GetTime()
		s.nextEvent += Milliseconds(timeToSleep)

		ml.La("Source", s.n.name, "sleeping for", timeToSleep, "ms")

		numThisMs++
	}

	for Milliseconds(s.n.loop.GetTime()) > s.nextEvent {
		// make the call
		s.GenerateEvent()

		numThisMs++

		timeToSleep := rand.ExpFloat64() / s.lambda
		s.nextEvent = Milliseconds(timeToSleep) + s.nextEvent
		ml.La(s.n.name+": Source sleeping for", timeToSleep, "ms", s.n.loop.GetTime())
	}
	count.MarkDistributionSuffix("eventsPerMs-"+s.n.name, numThisMs, "source")
}

// MakeSource turns a source configuration into the source.
func MakeSource(sourceConf *SourceConf, l *Loop) *Source {
	source := Source{}
	source.n.name = sourceConf.Name
	source.lambda = sourceConf.Lambda
	source.newEventCb = sourceConf.MakeCall
	l.AddSource(&source)

	return &source
}
