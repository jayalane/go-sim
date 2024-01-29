// -*- tab-width:2 -*-

package sim

import (
	"bytes"
	"errors"
	"runtime"
	"strconv"
)

var (
	goroutinePrefix = []byte("goroutine ")
	errBadStack     = errors.New("invalid runtime.Stack output")
)

// This is terrible, slow, and should never be used.
func goid() int {
	buf := make([]byte, 32)
	n := runtime.Stack(buf, false)
	buf = buf[:n]
	// goroutine 1 [running]: ...

	buf, ok := bytes.CutPrefix(buf, goroutinePrefix)
	if !ok {
		return -1
	}

	i := bytes.IndexByte(buf, ' ')
	if i < 0 {
		return -1
	}

	n, err := strconv.Atoi(string(buf[:i]))
	if err != nil {
		return -1
	}

	return n
}
