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

	loop := NewLoop("default")

	stageConfAry := []StageConf{{LocalWork: uniformCDF(1, 10)}}
	appConf := AppConf{
		Name:     "count",
		Size:     5,
		Stages:   stageConfAry,
		ReplyLen: uniformCDF(200, 20000),
	}

	lbConf := LbConf{Name: "count", App: &appConf}
	MakeLB(&lbConf, loop)

	sourceConf := SourceConf{
		Name: "ngrl", Lambda: 0.5,
		MakeCall: func(s *Source) *Call {
			c := Call{}
			c.replyCh = make(chan *Reply, 2)
			c.timeoutMs = 90.0
			c.wakeup = Milliseconds(s.n.loop.GetTime() + 5.0)
			c.endPoint = "count"
			return &c
		},
	}

	src := MakeSource(&sourceConf, loop)
	loop.AddSource(src)

	loop.Run(100) // msecs
	loop.Stats()
}
