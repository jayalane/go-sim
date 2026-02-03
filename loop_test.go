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

// initTest sets up logging and counters for a test.
func initTest() {
	ll.SetWriter(os.Stdout)
	count.InitCounters()
	count.SetResolution(count.HighRes)

	Init()
}

// TestNetworkSaturationQueuing verifies that high network cost with a low
// network limit causes calls to queue at the sender and eventually deliver
// after network decay.
func TestNetworkSaturationQueuing(t *testing.T) {
	initTest()

	loop := NewLoop()

	// Backend with very tight network limit so calls get queued at sender
	resourceConfig := &ResourceConfig{
		CPUPerLocalWork:     UniformCDF(0.01, 0.02),
		MemoryPerCall:       UniformCDF(0.01, 0.02),
		NetworkPerCall:      UniformCDF(0.4, 0.5), // High network cost per call
		NetworkPerReply:     UniformCDF(0.01, 0.02),
		MemoryPerQueuedCall: UniformCDF(0.01, 0.02),

		CPULimit:     0.95,
		MemoryLimit:  0.95,
		NetworkLimit: 0.30, // Very low network limit → saturates quickly

		MemoryRecoveryMs: 5000,
		CPUDelayFactor:   1.0,
		CPURejectLimit:   0.0, // disabled

		CPUDecayRate:     0.1,
		MemoryDecayRate:  0.05,
		NetworkDecayRate: 0.20, // Moderate decay allows delivery over time
	}

	appConf := AppConf{
		Name:      "netServer",
		Size:      2,
		Stages:    []*StageConf{{LocalWork: UniformCDF(1, 3)}},
		ReplyLen:  UniformCDF(100, 200),
		Resources: resourceConfig,
	}

	lbConf := LbConf{Name: "netServer", App: &appConf}
	MakeLB(&lbConf, loop)

	sourceConf := SourceConf{
		Name: "netSource", Lambda: 50,
		MakeCall: func(s *Source) *Call {
			c := Call{}
			c.ReqID = IncrCallNumber()
			c.TimeoutMs = 500.0 // Long timeout to allow retries
			c.Wakeup = Milliseconds(s.n.loop.GetTime() + 5.0)
			c.Endpoint = "netServer"
			return &c
		},
	}

	MakeSource(&sourceConf, loop)

	loop.Run(100)
	loop.Stats()
	count.LogCounters()

	queued := count.ReadSync("outbound_queued")
	delivered := count.ReadSync("outbound_delivered")

	if queued == 0 {
		t.Error("Expected outbound_queued > 0 (calls should queue at sender)")
	}

	if delivered == 0 {
		t.Error("Expected outbound_delivered > 0 (some calls should deliver after decay)")
	}

	t.Logf("outbound_queued=%d outbound_delivered=%d", queued, delivered)
}

// TestOOMKillAndRecovery verifies that high memory per call with a low memory
// limit triggers OOM kill events and that nodes recover afterward.
func TestOOMKillAndRecovery(t *testing.T) {
	initTest()

	loop := NewLoop()

	resourceConfig := &ResourceConfig{
		CPUPerLocalWork:     UniformCDF(0.01, 0.02),
		MemoryPerCall:       UniformCDF(0.3, 0.5), // High memory per call
		NetworkPerCall:      UniformCDF(0.01, 0.02),
		NetworkPerReply:     UniformCDF(0.01, 0.02),
		MemoryPerQueuedCall: UniformCDF(0.01, 0.02),

		CPULimit:     0.95,
		MemoryLimit:  0.40, // Low memory limit → OOM quickly
		NetworkLimit: 0.95,

		MemoryRecoveryMs: 20, // Fast recovery so we see recovery events in short test
		CPUDelayFactor:   1.0,
		CPURejectLimit:   0.0,

		CPUDecayRate:     0.1,
		MemoryDecayRate:  0.01, // Slow memory decay to trigger OOM
		NetworkDecayRate: 0.15,
	}

	appConf := AppConf{
		Name:      "oomServer",
		Size:      3,
		Stages:    []*StageConf{{LocalWork: UniformCDF(1, 3)}},
		ReplyLen:  UniformCDF(100, 200),
		Resources: resourceConfig,
	}

	lbConf := LbConf{Name: "oomServer", App: &appConf}
	MakeLB(&lbConf, loop)

	sourceConf := SourceConf{
		Name: "oomSource", Lambda: 80,
		MakeCall: func(s *Source) *Call {
			c := Call{}
			c.ReqID = IncrCallNumber()
			c.TimeoutMs = 500.0
			c.Wakeup = Milliseconds(s.n.loop.GetTime() + 5.0)
			c.Endpoint = "oomServer"
			return &c
		},
	}

	MakeSource(&sourceConf, loop)

	loop.Run(150)
	loop.Stats()
	count.LogCounters()

	exhaustion := count.ReadSync("node_memory_exhaustion")
	recovery := count.ReadSync("node_recovery")

	if exhaustion == 0 {
		t.Error("Expected node_memory_exhaustion > 0")
	}

	if recovery == 0 {
		t.Error("Expected node_recovery > 0")
	}

	t.Logf("node_memory_exhaustion=%d node_recovery=%d", exhaustion, recovery)
}

// TestRetryExhaustion sets up a two-tier system (frontend -> backend) where
// the backend has very tight network. Frontend RemoteCalls use a RetryPolicy
// with MaxRetries=2. Verifies no deadlock or panic.
func TestRetryExhaustion(t *testing.T) {
	initTest()

	loop := NewLoop()

	// Backend with very tight network
	backendResources := &ResourceConfig{
		CPUPerLocalWork:     UniformCDF(0.01, 0.02),
		MemoryPerCall:       UniformCDF(0.01, 0.02),
		NetworkPerCall:      UniformCDF(0.5, 0.6), // Very high network per call
		NetworkPerReply:     UniformCDF(0.01, 0.02),
		MemoryPerQueuedCall: UniformCDF(0.01, 0.02),

		CPULimit:     0.95,
		MemoryLimit:  0.95,
		NetworkLimit: 0.20, // Very tight network

		MemoryRecoveryMs: 5000,
		CPUDelayFactor:   1.0,
		CPURejectLimit:   0.0,

		CPUDecayRate:     0.1,
		MemoryDecayRate:  0.05,
		NetworkDecayRate: 0.10, // Slow decay so retries exhaust
	}

	backendConf := AppConf{
		Name:      "backend",
		Size:      2,
		Stages:    []*StageConf{{LocalWork: UniformCDF(1, 3)}},
		ReplyLen:  UniformCDF(100, 200),
		Resources: backendResources,
	}

	backendLB := LbConf{Name: "backend", App: &backendConf}
	MakeLB(&backendLB, loop)

	// Frontend that calls backend with retry policy (MaxRetries=2)
	retryPolicy := &RetryPolicy{
		MaxRetries:    2,
		InitialDelay:  5,
		BackoffFactor: 2.0,
		MaxDelay:      50,
		Jitter:        0.1,
	}

	frontendResources := &ResourceConfig{
		CPUPerLocalWork:     UniformCDF(0.01, 0.02),
		MemoryPerCall:       UniformCDF(0.01, 0.02),
		NetworkPerCall:      UniformCDF(0.01, 0.02),
		NetworkPerReply:     UniformCDF(0.01, 0.02),
		MemoryPerQueuedCall: UniformCDF(0.01, 0.02),

		CPULimit:     0.95,
		MemoryLimit:  0.95,
		NetworkLimit: 0.95,

		MemoryRecoveryMs: 5000,
		CPUDelayFactor:   1.0,
		CPURejectLimit:   0.0,

		CPUDecayRate:     0.1,
		MemoryDecayRate:  0.05,
		NetworkDecayRate: 0.15,
	}

	frontendConf := AppConf{
		Name: "frontend",
		Size: 2,
		Stages: []*StageConf{{
			LocalWork: UniformCDF(1, 2),
			RemoteCalls: []*RemoteCall{{
				Endpoint: "backend",
				Retry:    retryPolicy,
			}},
		}},
		ReplyLen:  UniformCDF(100, 200),
		Resources: frontendResources,
	}

	frontendLB := LbConf{Name: "frontend", App: &frontendConf}
	MakeLB(&frontendLB, loop)

	sourceConf := SourceConf{
		Name: "retrySource", Lambda: 40,
		MakeCall: func(s *Source) *Call {
			c := Call{}
			c.ReqID = IncrCallNumber()
			c.TimeoutMs = 200.0
			c.Wakeup = Milliseconds(s.n.loop.GetTime() + 5.0)
			c.Endpoint = "frontend"
			return &c
		},
	}

	MakeSource(&sourceConf, loop)

	// Run the simulation - primary goal is no deadlock/panic
	loop.Run(100)
	loop.Stats()
	count.LogCounters()

	t.Log("TestRetryExhaustion completed without deadlock or panic")
}

// TestPerCallCosts sets up two backends (heavy + light) with different per-call
// CPUCost/MemoryCost/NetworkCost CDFs. Verifies the system runs without
// deadlock or panic.
func TestPerCallCosts(t *testing.T) {
	initTest()

	loop := NewLoop()

	// Heavy backend
	heavyResources := &ResourceConfig{
		CPUPerLocalWork:     UniformCDF(0.05, 0.1),
		MemoryPerCall:       UniformCDF(0.05, 0.1),
		NetworkPerCall:      UniformCDF(0.05, 0.1),
		NetworkPerReply:     UniformCDF(0.02, 0.05),
		MemoryPerQueuedCall: UniformCDF(0.01, 0.02),

		CPULimit:     0.90,
		MemoryLimit:  0.90,
		NetworkLimit: 0.90,

		MemoryRecoveryMs: 50,
		CPUDelayFactor:   1.5,
		CPURejectLimit:   0.0,

		CPUDecayRate:     0.1,
		MemoryDecayRate:  0.05,
		NetworkDecayRate: 0.15,
	}

	heavyConf := AppConf{
		Name:      "heavyBackend",
		Size:      2,
		Stages:    []*StageConf{{LocalWork: UniformCDF(2, 5)}},
		ReplyLen:  UniformCDF(100, 200),
		Resources: heavyResources,
	}

	heavyLB := LbConf{Name: "heavyBackend", App: &heavyConf}
	MakeLB(&heavyLB, loop)

	// Light backend
	lightResources := &ResourceConfig{
		CPUPerLocalWork:     UniformCDF(0.01, 0.02),
		MemoryPerCall:       UniformCDF(0.01, 0.02),
		NetworkPerCall:      UniformCDF(0.01, 0.02),
		NetworkPerReply:     UniformCDF(0.01, 0.02),
		MemoryPerQueuedCall: UniformCDF(0.01, 0.02),

		CPULimit:     0.95,
		MemoryLimit:  0.95,
		NetworkLimit: 0.95,

		MemoryRecoveryMs: 50,
		CPUDelayFactor:   1.0,
		CPURejectLimit:   0.0,

		CPUDecayRate:     0.1,
		MemoryDecayRate:  0.05,
		NetworkDecayRate: 0.15,
	}

	lightConf := AppConf{
		Name:      "lightBackend",
		Size:      2,
		Stages:    []*StageConf{{LocalWork: UniformCDF(1, 2)}},
		ReplyLen:  UniformCDF(100, 200),
		Resources: lightResources,
	}

	lightLB := LbConf{Name: "lightBackend", App: &lightConf}
	MakeLB(&lightLB, loop)

	// Frontend that fans out to both backends with per-call costs
	frontendResources := &ResourceConfig{
		CPUPerLocalWork:     UniformCDF(0.01, 0.02),
		MemoryPerCall:       UniformCDF(0.01, 0.02),
		NetworkPerCall:      UniformCDF(0.01, 0.02),
		NetworkPerReply:     UniformCDF(0.01, 0.02),
		MemoryPerQueuedCall: UniformCDF(0.01, 0.02),

		CPULimit:     0.95,
		MemoryLimit:  0.95,
		NetworkLimit: 0.95,

		MemoryRecoveryMs: 5000,
		CPUDelayFactor:   1.0,
		CPURejectLimit:   0.0,

		CPUDecayRate:     0.1,
		MemoryDecayRate:  0.05,
		NetworkDecayRate: 0.15,
	}

	frontendConf := AppConf{
		Name: "pcFrontend",
		Size: 2,
		Stages: []*StageConf{{
			LocalWork: UniformCDF(1, 2),
			RemoteCalls: []*RemoteCall{
				{
					Endpoint:    "heavyBackend",
					CPUCost:     UniformCDF(0.2, 0.4),
					MemoryCost:  UniformCDF(0.15, 0.3),
					NetworkCost: UniformCDF(0.1, 0.2),
				},
				{
					Endpoint:    "lightBackend",
					CPUCost:     UniformCDF(0.01, 0.05),
					MemoryCost:  UniformCDF(0.01, 0.05),
					NetworkCost: UniformCDF(0.01, 0.05),
				},
			},
		}},
		ReplyLen:  UniformCDF(100, 200),
		Resources: frontendResources,
	}

	frontendLB := LbConf{Name: "pcFrontend", App: &frontendConf}
	MakeLB(&frontendLB, loop)

	sourceConf := SourceConf{
		Name: "pcSource", Lambda: 30,
		MakeCall: func(s *Source) *Call {
			c := Call{}
			c.ReqID = IncrCallNumber()
			c.TimeoutMs = 200.0
			c.Wakeup = Milliseconds(s.n.loop.GetTime() + 5.0)
			c.Endpoint = "pcFrontend"
			return &c
		},
	}

	MakeSource(&sourceConf, loop)

	loop.Run(100)
	loop.Stats()
	count.LogCounters()

	t.Log("TestPerCallCosts completed without deadlock or panic")
}

// TestCPUCascadeToOOM sets up high CPU per work, low CPU limit, high delay
// factor, and CPURejectLimit=0.99. Long local work causes CPU delays which
// re-queue tasks, consuming memory for queued calls, potentially cascading
// to OOM.
func TestCPUCascadeToOOM(t *testing.T) {
	initTest()

	loop := NewLoop()

	resourceConfig := &ResourceConfig{
		CPUPerLocalWork:     UniformCDF(0.10, 0.15), // Each task adds moderate CPU
		MemoryPerCall:       UniformCDF(0.001, 0.002),
		NetworkPerCall:      UniformCDF(0.001, 0.002),
		NetworkPerReply:     UniformCDF(0.001, 0.002),
		MemoryPerQueuedCall: UniformCDF(0.05, 0.1), // Memory grows from queued work

		CPULimit:     0.20,  // Low: 2+ tasks/ms pushes above limit
		MemoryLimit:  0.80,  // Moderate memory limit
		NetworkLimit: 0.99,

		MemoryRecoveryMs: 20,  // Fast recovery
		CPUDelayFactor:   3.0, // High delay factor
		CPURejectLimit:   0.99,

		CPUDecayRate:     0.15, // Decays faster than single task, but multi-task ms spikes above limit
		MemoryDecayRate:  0.01, // Slow memory decay → queued work causes pressure
		NetworkDecayRate: 0.15,
	}

	appConf := AppConf{
		Name:      "cpuServer",
		Size:      2,
		Stages:    []*StageConf{{LocalWork: UniformCDF(1, 2)}},
		ReplyLen:  UniformCDF(100, 200),
		Resources: resourceConfig,
	}

	lbConf := LbConf{Name: "cpuServer", App: &appConf}
	MakeLB(&lbConf, loop)

	sourceConf := SourceConf{
		Name: "cpuSource", Lambda: 200, // Very high load to ensure multiple tasks/ms
		MakeCall: func(s *Source) *Call {
			c := Call{}
			c.ReqID = IncrCallNumber()
			c.TimeoutMs = 500.0
			c.Wakeup = Milliseconds(s.n.loop.GetTime() + 5.0)
			c.Endpoint = "cpuServer"
			return &c
		},
	}

	MakeSource(&sourceConf, loop)

	loop.Run(50) // Short run to avoid accumulation hang
	loop.Stats()
	count.LogCounters()

	cpuDelay := count.ReadSync("node_cpu_delay")
	if cpuDelay == 0 {
		t.Error("Expected node_cpu_delay > 0 (CPU should be saturated)")
	}

	// OOM may or may not occur depending on timing, but check that the
	// cascade path exists
	memExhaustion := count.ReadSync("node_memory_exhaustion")
	t.Logf("node_cpu_delay=%d node_memory_exhaustion=%d", cpuDelay, memExhaustion)
}
