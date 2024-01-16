// This example demonstrates a priority queue built using the heap interface.
package sim

import (
	"testing"
)

// TestLoop runs a small simulation
func TestLoop(t *testing.T) {

	Init()
	
	loop := Loop{}

	stageConfAry := []StageConf{{LocalWork: uniformCDF(1, 10)}}
	appConf := AppConf{Name: "count", Size: 5, Stages: stageConfAry}

	lbConf := LbConf{Name: "count", App: &appConf}
	lb := MakeLB(&lbConf, &loop)

	loop.AddLB("count", lb)

	sourceConf := SourceConf{
		Name: "ngrl", Lambda: 0.05,
		MakeCall: func(s *Source) *Call {
			c := Call{}
			c.replyCh = make(chan *Result, 2)
			c.timeoutMs = 90.0
			c.endPoint = "count"
			return &c
		},
	}

	src := MakeSource(&sourceConf, &loop)
	loop.AddSource(src)

	loop.Run(10000) // msecs
	loop.Stats()
}
