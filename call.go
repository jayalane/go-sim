// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

// HandleReply type is a callback to process the reply from a call
type HandleReply func(*Node, *Reply)

// Call is a structure to track a remote call
type Call struct {
	wakeup     Milliseconds
	startTime  Milliseconds
	endPoint   string
	timeoutMs  float64
	length     uint64
	id1        uint64
	id2        uint64
	connection *Connection
	replyCh    chan *Reply
}

// SendCall sends the call to the node
func (c *Call) SendCall(n *Node, f HandleReply) {
	n.callCh <- c // blocking as is
	go func() {
		ml.La(n.name+" waiting for reply", c.wakeup)
		response := <-c.replyCh
		f(n, response)
		ml.La(n.name+" done waiting for reply", n.loop.GetTime())
	}()
}

// Fanout will send a call to an endpoint
func (c *Call) Fanout(endpoint string, n *Node) {
	target := n.loop.GetLB(endpoint + "-lb")
	ml.La("Sending call", c, "to", endpoint)
	c.SendCall(&target.n,
		func(
			n *Node,
			r *Reply,
		) {
			ml.Ln("Got a reply", n.name, *r)
		},
	)
}
