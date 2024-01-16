// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

// Connection is a TCP connection (pooled or not)
type Connection struct {
	maxSize     uint64
	currSize    uint64
	ssl         bool
	multiplexed bool
	owned       *Call // only if !multiplexed
}
