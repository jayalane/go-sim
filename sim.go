// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

import (
	"math/rand"
)

// Milliseconds is the internal sim time type
type Milliseconds float64

// Now is the current global sim time
var Now Milliseconds
