// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

// StageConf is the configuration of a stage of app work
type StageConf struct {
	LocalWork   ModelCdf
	RemoteCalls []string
}

// Stage is a piece of an application run
type Stage struct {
	FirstLocalWork Distribution
	DepsToCall     []*Node
}

// AppConf is the configuration of an application
type AppConf struct {
	Name     string
	Size     uint16
	ReplyLen ModelCdf
	Stages   []StageConf
}

// MakeApp takes and app config and a loop and returns a
// node integrated into that loop
func MakeApp(a *AppConf, l *Loop, suffix string) *Node {
	n := Node{}

	n.App = a
	n.name = a.Name + suffix

	l.AddNode(&n)
	n.Run()
	return &n
}
