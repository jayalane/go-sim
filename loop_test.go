// This example demonstrates a priority queue built using the heap interface.
package sim

import (
	"fmt"
	"testing"
)

// TestLoop runs a small simulation
func TestLoop(t *testing.T) {
	loop := Loop{}

	stageConfAry := []StageConf{{LocalWork: uniformCDF(1, 10)}}
	appConf := AppConf{Name: "count", Size: 5, Stages: stageConfAry}

	lbConf := LbConf{Name: "count", App: &appConf}
	lb := MakeLB(&lbConf, &loop)

	sourceConf := SourceConf{
		Name: "ngrl", Lambda: 0.05,
		MakeEvent: func(s *Source) {
			j := Job{}
			j.sender = s.getReplyCh()
			j.app = l.GetLB("count")
			j.StartTime = Now
			j.TimeoutMs = 100.0
		},
	}

	src := MakeSource(sourceConf, &loop)

	stop := loop.Run(10000) // msecs
	loop.Stats()
}
