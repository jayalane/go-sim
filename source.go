// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

// EventCB is called by a source to generate and send the new
// work
type EventCB func(s *Source)

// SourceConf configures an event source
type SourceConf struct {
	Name      string
	Lambda    float64
	MakeEvent EventCB
}

// Source is a source of events
type Source struct {
	NodeInterface
	Node
	// Later:  (can't say to do due to linting) have concept of
	// customer flow)
	nextEvent  Milliseconds
	lambda     float64
	newEventCb EventCB
	downstream *Node // should be an LB
}

// MakeSource turns a source configuration into the source
func MakeSource(sourceConf *SourceConf, l *Loop) *Source {
	source := Source{}
	source.lambda = sourceConf.Lambda
	source.newEventCb = sourceConf.MakeEvent

	return &source
}
