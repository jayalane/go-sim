// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

// Call is a structure to track a remote call
type Call struct {
	wakeUp     Milliseconds
	startTime  Milliseconds
	endPoint   string
	timeoutMs  float64
	connection *Connection
	replyCh    chan *Result
}

// SendCall sends the call to the LB
func (c *Call) SendCall(lb *LB) {
	lb.n.callCh <- c // blocking as is
}
