// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

import (
	"sync"

	ll "github.com/jayalane/go-lll"
)

var (
	ml     *ll.Lll
	mlOnce sync.Once
)

const (
	bufferSizes      = 1_000_000
	smallChannelSize = 2
	numLBs           = 1_000
	secondsInMin     = 60
)

// Milliseconds is the internal sim time type.
type Milliseconds float64

// Init must be called before any simulation stuff
// it merely inits the logger.
func Init() {
	mlOnce.Do(func() {
		ml = ll.Init("SIM", "none")
	})
}

// InitWithLogger is an init where youc an
// pass in the go-lll logger.
func InitWithLogger(ll *ll.Lll) {
	mlOnce.Do(func() {
		ml = ll
	})
}
