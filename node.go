// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

// NodeInterface is the base interface used by sources, LBs, apps, DBs etc.
type NodeInterface interface {
	Run() // starts a goroutine
	NextMillisecond()
	GetMillisecond()
	StatsMillisecond()
	GetCallChannel() chan *Job
	GetReplyChannel() chan *Job
}

// Node is a simulation particle that
// can take in or emit work; it is a Node
type Node struct {
	NodeInterface
	calls     chan *Job
	replies   chan *Job
	events    *PQueue
	resources map[string]float64
	stats     map[string]float64
	App       *AppConf
}
