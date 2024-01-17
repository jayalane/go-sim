// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

import (
	ll "github.com/jayalane/go-lll"
)

var ml *ll.Lll

// Milliseconds is the internal sim time type
type Milliseconds float64

// Init must be called before any simulation stuff
// it merely inits the logger
func Init() {
	ml = ll.Init("SIM", "network")
}

// InitWithLogger is an init where youc an
// pass in the go-lll logger.
func InitWithLogger(ll *ll.Lll) {
	ml = ll
}
