// This example demonstrates a priority queue built using the heap interface.
package sim

import (
	"os"
	"testing"

	ll "github.com/jayalane/go-lll"
)

// TestLoop runs a small simulation
func TestLoop(t *testing.T) {
	ll.SetWriter(os.Stdout)
	Init()
	// ll.SetWriter(os.Stdout) // tbd look up which is proper

	loop := NewLoop("default")

	stageConfAry := []StageConf{{LocalWork: uniformCDF(1, 10)}}
	appConf := AppConf{Name: "count", Size: 5, Stages: stageConfAry}

	lbConf := LbConf{Name: "count", App: &appConf}
	MakeLB(&lbConf, loop)

	sourceConf := SourceConf{
		Name: "ngrl", Lambda: 0.05,
		MakeCall: func(s *Source) *Call {
			c := Call{}
			c.replyCh = make(chan *Result, 2)
			c.timeoutMs = 90.0
			c.wakeUp = Milliseconds(s.n.loop.GetTime() + 5.0)
			c.endPoint = "count"
			return &c
		},
	}

	src := MakeSource(&sourceConf, loop)
	loop.AddSource(src)

	loop.Run(100) // msecs
	loop.Stats()
}
