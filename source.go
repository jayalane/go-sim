// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

// EventCB is called by a source to generate and send the new
// work
type EventCB func(s *Node) *Call

// SourceConf configures an event source
type SourceConf struct {
	Name     string
	Lambda   float64
	MakeCall EventCB
}

// Source is a source of events
type Source struct {
	Node
	// Later:  (can't say to do due to linting) have concept of
	// customer flow)
}

func (n *Node) generateEvent() {
	ml.La("Generate EVENT!")
	c := n.newEventCb(n)
	c.startTime = Milliseconds(n.loop.GetTime())
	lb := n.loop.GetLB(c.endPoint)
	c.SendCall(lb)
}

// MakeSource turns a source configuration into the source
func MakeSource(sourceConf *SourceConf, l *Loop) *Source {
	source := Source{}
	source.lambda = sourceConf.Lambda
	source.newEventCb = sourceConf.MakeCall

	return &source
}
