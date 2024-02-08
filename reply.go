// -*- tab-width:2 -*-

package sim

// Reply is a structure to track a remote call result.
type Reply struct {
	reqID  int
	length uint64
	status uint64
	call   *Call
}
