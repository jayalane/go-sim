// -*- tab-width:2 -*-

package sim

import (
	"fmt"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // for pprof
	"os"
	"strings"
	"testing"

	count "github.com/jayalane/go-counter"
	ll "github.com/jayalane/go-lll"
)

const (
	longTestMsecs  = 200
	longTestLambda = 140
)

// TestLoop1 runs a small simulation.
func TestLoop1(_ *testing.T) {
	// start the profiler
	go func() {
		fmt.Println(http.ListenAndServe(":6060", nil))
	}()

	ll.SetWriter(os.Stdout)
	count.InitCounters()
	count.SetResolution(count.HighRes)

	Init()

	loop := NewLoop()

	stageConfAry := []*StageConf{{LocalWork: UniformCDF(1, 10)}}

	// Configure resource tracking
	resourceConfig := &ResourceConfig{
		CPUPerLocalWork: UniformCDF(0.2, 0.4),   // Higher CPU usage for demo
		MemoryPerCall:   UniformCDF(0.1, 0.2),   // More memory per call
		NetworkPerCall:  UniformCDF(0.15, 0.25), // Network per call
		NetworkPerReply: UniformCDF(0.08, 0.12), // Network per reply

		CPULimit:     0.80, // 80% CPU limit
		MemoryLimit:  0.85, // 85% memory limit
		NetworkLimit: 0.90, // 90% network limit

		MemoryRecoveryMs: 5000, // 5 second recovery for demo
		CPUDelayFactor:   1.5,  // 1.5x delay when CPU saturated

		CPUDecayRate:     0.15, // Faster decay for demo
		MemoryDecayRate:  0.05, // Slower memory decay
		NetworkDecayRate: 0.20, // Fast network decay
	}

	appConf := AppConf{
		Name:      "serverA",
		Size:      5,
		Stages:    stageConfAry,
		ReplyLen:  UniformCDF(200, 20000),
		Resources: resourceConfig,
	}

	lbConf := LbConf{Name: "serverA", App: &appConf}
	MakeLB(&lbConf, loop)

	sourceConf := SourceConf{
		Name: "external", Lambda: longTestLambda, // Increased load to demonstrate resource limits
		MakeCall: func(s *Source) *Call {
			c := Call{}
			c.ReqID = IncrCallNumber()
			c.TimeoutMs = 90.0
			c.Wakeup = Milliseconds(s.n.loop.GetTime() + 5.0)
			c.Endpoint = "serverA"

			return &c
		},
	}

	MakeSource(&sourceConf, loop)

	fmt.Println("Running simulation with resource tracking...")
	loop.Run(longTestMsecs) // Longer run to see resource effects
	loop.Stats()
	count.LogCounters()
	fmt.Println("Resource tracking simulation complete.")
}

func FilterCallFunc(
	endpoint string,
	params map[string]string,
) bool {
	if strings.Contains(endpoint, "count") {
		return true
	}

	_, ok := params["DNF"]

	return !ok
}
