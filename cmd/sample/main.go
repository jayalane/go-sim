// -*- tab-width:2 -*-

// Package main provides a sample data center simulation with realistic
// multi-tier architecture: web frontends, middle-tier services, and databases.
package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // for profiling
	"os"

	count "github.com/jayalane/go-counter"
	ll "github.com/jayalane/go-lll"
	sim "github.com/jayalane/go-sim"
)

// Simulation parameters - easy to tweak for experiments.
const (
	// CPU scaling factor - baseline CPUs for CPU-per-work calculations.
	baselineCPUs = 2.0

	// Duration and load.
	simDurationMs = 5000 // 5 seconds of simulated time
	loginLambda   = 5.0  // requests per second to /login (reduced)
	checkoutLamba = 3.0  // requests per second to /checkout (reduced)
	planLambda    = 5.0  // requests per second to /plan (reduced)
	payLambda     = 2.0  // requests per second to /pay (reduced)

	// Latencies in milliseconds (halved).
	dbReadMinMs  = 0.5
	dbReadMaxMs  = 1.5
	dbWriteMinMs = 9.0
	dbWriteMaxMs = 11.0

	// Pool sizes (number of containers per service).
	defaultPoolSize = 80 // doubled from 40

	// Web tier: 2 CPU containers, can handle ~3 overlapping transactions.
	// With 2 CPUs, each request uses ~33% of container CPU.
	webCPUs           = 2.0 // CPUs per container
	webMaxConcurrent  = 3.0 // max overlapping transactions per container
	webCPULimit       = 1.0 / webMaxConcurrent
	webMemoryLimit    = 0.40
	webNetworkLimit   = 0.95 // high limit
	webLocalWorkMin   = 2.0
	webLocalWorkMax   = 5.0
	webCPUPerWork     = 0.05 * (baselineCPUs / webCPUs) // scales with CPU count
	webMemoryPerCall  = 0.10
	webNetworkPerCall = 0.01 // reduced for high fanout

	// Service tier: 16 CPU containers, can handle ~50 overlapping transactions.
	// With 16 CPUs, each request uses ~2% of container CPU.
	svcCPUs           = 16.0
	svcMaxConcurrent  = 50.0
	svcCPULimit       = 1.0 / svcMaxConcurrent
	svcMemoryLimit    = 0.05
	svcNetworkLimit   = 0.95 // high limit
	svcLocalWorkMin   = 1.0
	svcLocalWorkMax   = 3.0
	svcCPUPerWork     = 0.01 * (baselineCPUs / svcCPUs) // scales with CPU count
	svcMemoryPerCall  = 0.005
	svcNetworkPerCall = 0.001 // reduced for high fanout

	// DB Proxy tier: 16 CPU containers, can handle ~500 overlapping transactions.
	// Mostly I/O bound, very low CPU per request.
	dbProxyCPUs           = 16.0
	dbProxyMaxConcurrent  = 500.0
	dbProxyCPULimit       = 1.0 / dbProxyMaxConcurrent
	dbProxyMemoryLimit    = 0.005
	dbProxyNetworkLimit   = 0.95 // high limit
	dbProxyLocalWorkMin   = 0.5
	dbProxyLocalWorkMax   = 1.0
	dbProxyCPUPerWork     = 0.001 * (baselineCPUs / dbProxyCPUs) // scales with CPU count
	dbProxyMemoryPerCall  = 0.0005
	dbProxyNetworkPerCall = 0.0001 // reduced for high fanout

	// Recovery and decay settings.
	memoryRecoveryMs = 10000 // 10 seconds to restart after OOM
	cpuDelayFactor   = 2.0   // 2x delay when CPU saturated
	cpuDecayRate     = 0.10  // 10% decay per ms
	memoryDecayRate  = 0.02  // 2% decay per ms (slower)
	networkDecayRate = 0.50  // 50% decay per ms (fast for high throughput)

	// Request timeouts.
	defaultTimeoutMs = 2000.0 // 2 seconds

	// CDF variance multipliers.
	lowMultiplier  = 0.8
	highMultiplier = 1.2
	replyLow       = 0.4
	replyHigh      = 0.6

	// Queued call memory costs.
	webQueuedMemMin     = 0.02
	webQueuedMemMax     = 0.04
	svcQueuedMemMin     = 0.005
	svcQueuedMemMax     = 0.01
	dbProxyQueuedMemMin = 0.001
	dbProxyQueuedMemMax = 0.002

	// CPU reject limits.
	webCPURejectLimit     = 0.95
	svcCPURejectLimit     = 0.90
	dbProxyCPURejectLimit = 0.85

	// Database pool size.
	dbPoolSize = 4

	// Reply size ranges.
	dbReadReplyMin  = 100
	dbReadReplyMax  = 1000
	dbWriteReplyMin = 50
	dbWriteReplyMax = 200
	dbProxyReplyMin = 100
	dbProxyReplyMax = 500
	svcReplyMin     = 200
	svcReplyMax     = 2000
	webReplyMin     = 1000
	webReplyMax     = 10000

	// Network delay for wakeup.
	networkDelayMs = 1.0 // reduced from 5.0
)

func main() {
	// Start pprof server for profiling.
	go func() {
		fmt.Println(http.ListenAndServe(":6060", nil)) //nolint:gosec
	}()

	ll.SetWriter(os.Stdout)
	count.InitCounters()
	count.SetResolution(count.HighRes)

	sim.Init()

	loop := sim.NewLoop()

	// Build the data center from bottom up.
	buildDatabases(loop)
	buildDBProxies(loop)
	buildMiddleTierServices(loop)
	buildWebFrontends(loop)
	buildTrafficSources(loop)

	fmt.Println("=== Data Center Simulation ===")
	fmt.Println("Web Frontends: loginweb, checkoutweb, planweb, payweb (2 containers each)")
	fmt.Println("Services: userdataserv, checkoutserv, walletserv, authserv,")
	fmt.Println("          fulfillmentserv, planningserv, binserv, merchantprefserv (16 containers each)")
	fmt.Println("DB Proxies: one per service (16 containers each)")
	fmt.Println()
	fmt.Printf("Simulating %d ms of traffic...\n", simDurationMs)

	loop.Run(simDurationMs)
	loop.Stats()
	count.LogCounters()

	fmt.Println("\n=== Simulation Complete ===")
}

// webResourceConfig returns resource config for web tier (2 CPU, 3 concurrent).
func webResourceConfig() *sim.ResourceConfig {
	return &sim.ResourceConfig{
		CPUPerLocalWork:     sim.UniformCDF(webCPUPerWork*lowMultiplier, webCPUPerWork*highMultiplier),
		MemoryPerCall:       sim.UniformCDF(webMemoryPerCall*lowMultiplier, webMemoryPerCall*highMultiplier),
		NetworkPerCall:      sim.UniformCDF(webNetworkPerCall*lowMultiplier, webNetworkPerCall*highMultiplier),
		NetworkPerReply:     sim.UniformCDF(webNetworkPerCall*replyLow, webNetworkPerCall*replyHigh),
		MemoryPerQueuedCall: sim.UniformCDF(webQueuedMemMin, webQueuedMemMax),

		CPULimit:     webCPULimit,
		MemoryLimit:  webMemoryLimit,
		NetworkLimit: webNetworkLimit,

		MemoryRecoveryMs: memoryRecoveryMs,
		CPUDelayFactor:   cpuDelayFactor,
		CPURejectLimit:   webCPURejectLimit,

		CPUDecayRate:     cpuDecayRate,
		MemoryDecayRate:  memoryDecayRate,
		NetworkDecayRate: networkDecayRate,
	}
}

// svcResourceConfig returns resource config for service tier (16 CPU, 50 concurrent).
func svcResourceConfig() *sim.ResourceConfig {
	return &sim.ResourceConfig{
		CPUPerLocalWork:     sim.UniformCDF(svcCPUPerWork*lowMultiplier, svcCPUPerWork*highMultiplier),
		MemoryPerCall:       sim.UniformCDF(svcMemoryPerCall*lowMultiplier, svcMemoryPerCall*highMultiplier),
		NetworkPerCall:      sim.UniformCDF(svcNetworkPerCall*lowMultiplier, svcNetworkPerCall*highMultiplier),
		NetworkPerReply:     sim.UniformCDF(svcNetworkPerCall*replyLow, svcNetworkPerCall*replyHigh),
		MemoryPerQueuedCall: sim.UniformCDF(svcQueuedMemMin, svcQueuedMemMax),

		CPULimit:     svcCPULimit,
		MemoryLimit:  svcMemoryLimit,
		NetworkLimit: svcNetworkLimit,

		MemoryRecoveryMs: memoryRecoveryMs,
		CPUDelayFactor:   cpuDelayFactor,
		CPURejectLimit:   svcCPURejectLimit,

		CPUDecayRate:     cpuDecayRate,
		MemoryDecayRate:  memoryDecayRate,
		NetworkDecayRate: networkDecayRate,
	}
}

// dbProxyResourceConfig returns resource config for db proxy tier (16 CPU, 500 concurrent).
func dbProxyResourceConfig() *sim.ResourceConfig {
	return &sim.ResourceConfig{
		CPUPerLocalWork:     sim.UniformCDF(dbProxyCPUPerWork*lowMultiplier, dbProxyCPUPerWork*highMultiplier),
		MemoryPerCall:       sim.UniformCDF(dbProxyMemoryPerCall*lowMultiplier, dbProxyMemoryPerCall*highMultiplier),
		NetworkPerCall:      sim.UniformCDF(dbProxyNetworkPerCall*lowMultiplier, dbProxyNetworkPerCall*highMultiplier),
		NetworkPerReply:     sim.UniformCDF(dbProxyNetworkPerCall*replyLow, dbProxyNetworkPerCall*replyHigh),
		MemoryPerQueuedCall: sim.UniformCDF(dbProxyQueuedMemMin, dbProxyQueuedMemMax),

		CPULimit:     dbProxyCPULimit,
		MemoryLimit:  dbProxyMemoryLimit,
		NetworkLimit: dbProxyNetworkLimit,

		MemoryRecoveryMs: memoryRecoveryMs,
		CPUDelayFactor:   cpuDelayFactor,
		CPURejectLimit:   dbProxyCPURejectLimit,

		CPUDecayRate:     cpuDecayRate,
		MemoryDecayRate:  memoryDecayRate,
		NetworkDecayRate: networkDecayRate,
	}
}

// Database names - each service has its own DB.
var databases = []string{
	"db-userdata",
	"db-checkout",
	"db-wallet",
	"db-auth",
	"db-fulfillment",
	"db-planning",
	"db-bin",
	"db-merchantpref",
}

// buildDatabases creates the database backends.
// These are simple endpoints that just add latency (read: 1-3ms, write: 20ms).
func buildDatabases(loop *sim.Loop) {
	for _, dbName := range databases {
		// Read endpoint.
		readApp := &sim.AppConf{
			Name:     dbName + "-read",
			Size:     dbPoolSize,
			Stages:   []*sim.StageConf{{LocalWork: sim.UniformCDF(dbReadMinMs, dbReadMaxMs)}},
			ReplyLen: sim.UniformCDF(dbReadReplyMin, dbReadReplyMax),
		}
		sim.MakeLB(&sim.LbConf{Name: dbName + "-read", App: readApp}, loop)

		// Write endpoint.
		writeApp := &sim.AppConf{
			Name:     dbName + "-write",
			Size:     dbPoolSize,
			Stages:   []*sim.StageConf{{LocalWork: sim.UniformCDF(dbWriteMinMs, dbWriteMaxMs)}},
			ReplyLen: sim.UniformCDF(dbWriteReplyMin, dbWriteReplyMax),
		}
		sim.MakeLB(&sim.LbConf{Name: dbName + "-write", App: writeApp}, loop)
	}
}

// DB proxy services - each talks to its own DB.
var dbProxies = []struct {
	name   string
	dbName string
}{
	{"dbproxy-userdata", "db-userdata"},
	{"dbproxy-checkout", "db-checkout"},
	{"dbproxy-wallet", "db-wallet"},
	{"dbproxy-auth", "db-auth"},
	{"dbproxy-fulfillment", "db-fulfillment"},
	{"dbproxy-planning", "db-planning"},
	{"dbproxy-bin", "db-bin"},
	{"dbproxy-merchantpref", "db-merchantpref"},
}

// buildDBProxies creates the database proxy services.
func buildDBProxies(loop *sim.Loop) {
	for _, proxy := range dbProxies {
		app := &sim.AppConf{
			Name:      proxy.name,
			Size:      defaultPoolSize,
			Resources: dbProxyResourceConfig(),
			ReplyLen:  sim.UniformCDF(dbProxyReplyMin, dbProxyReplyMax),
			Stages: []*sim.StageConf{
				{
					LocalWork: sim.UniformCDF(dbProxyLocalWorkMin, dbProxyLocalWorkMax),
					RemoteCalls: []*sim.RemoteCall{
						{Endpoint: proxy.dbName + "-read"},
					},
				},
			},
		}
		sim.MakeLB(&sim.LbConf{Name: proxy.name, App: app}, loop)
	}
}

// Middle tier services with their DB proxy dependencies.
var middleTierServices = []struct {
	name      string
	dbProxies []string // Which DB proxies this service calls.
}{
	{"userdataserv", []string{"dbproxy-userdata"}},
	{"checkoutserv", []string{"dbproxy-checkout", "dbproxy-wallet"}},
	{"walletserv", []string{"dbproxy-wallet"}},
	{"authserv", []string{"dbproxy-auth", "dbproxy-userdata"}},
	{"fulfillmentserv", []string{"dbproxy-fulfillment", "dbproxy-checkout"}},
	{"planningserv", []string{"dbproxy-planning"}},
	{"binserv", []string{"dbproxy-bin"}},
	{"merchantprefserv", []string{"dbproxy-merchantpref"}},
}

// buildMiddleTierServices creates the middle tier services.
func buildMiddleTierServices(loop *sim.Loop) {
	for _, svc := range middleTierServices {
		remoteCalls := make([]*sim.RemoteCall, len(svc.dbProxies))
		for i, proxy := range svc.dbProxies {
			remoteCalls[i] = &sim.RemoteCall{Endpoint: proxy}
		}

		app := &sim.AppConf{
			Name:      svc.name,
			Size:      defaultPoolSize,
			Resources: svcResourceConfig(),
			ReplyLen:  sim.UniformCDF(svcReplyMin, svcReplyMax),
			Stages: []*sim.StageConf{
				{
					LocalWork:   sim.UniformCDF(svcLocalWorkMin, svcLocalWorkMax),
					RemoteCalls: remoteCalls,
				},
			},
		}
		sim.MakeLB(&sim.LbConf{Name: svc.name, App: app}, loop)
	}
}

// Web frontends and their service dependencies.
var webFrontends = []struct {
	name     string
	services []string
}{
	{"loginweb", []string{"authserv", "userdataserv"}},
	{"checkoutweb", []string{"checkoutserv", "walletserv", "merchantprefserv"}},
	{"planweb", []string{"planningserv", "userdataserv"}},
	{"payweb", []string{"walletserv", "binserv", "fulfillmentserv", "merchantprefserv"}},
}

// buildWebFrontends creates the web frontend services.
func buildWebFrontends(loop *sim.Loop) {
	for _, web := range webFrontends {
		remoteCalls := make([]*sim.RemoteCall, len(web.services))
		for i, svc := range web.services {
			remoteCalls[i] = &sim.RemoteCall{Endpoint: svc}
		}

		app := &sim.AppConf{
			Name:      web.name,
			Size:      defaultPoolSize,
			Resources: webResourceConfig(),
			ReplyLen:  sim.UniformCDF(webReplyMin, webReplyMax),
			Stages: []*sim.StageConf{
				{
					LocalWork:   sim.UniformCDF(webLocalWorkMin, webLocalWorkMax),
					RemoteCalls: remoteCalls,
				},
			},
		}
		sim.MakeLB(&sim.LbConf{Name: web.name, App: app}, loop)
	}
}

// Traffic sources - external traffic to the 4 URLs.
var trafficSources = []struct {
	name     string
	endpoint string
	lambda   float64
}{
	{"login-traffic", "loginweb", loginLambda},
	{"checkout-traffic", "checkoutweb", checkoutLamba},
	{"plan-traffic", "planweb", planLambda},
	{"pay-traffic", "payweb", payLambda},
}

// buildTrafficSources creates the external traffic generators.
func buildTrafficSources(loop *sim.Loop) {
	for _, src := range trafficSources {
		endpoint := src.endpoint // Capture for closure.
		sourceConf := sim.SourceConf{
			Name:   src.name,
			Lambda: src.lambda,
			MakeCall: func(s *sim.Source) *sim.Call {
				c := sim.Call{}
				c.ReqID = sim.IncrCallNumber()
				c.TimeoutMs = defaultTimeoutMs
				c.Wakeup = sim.Milliseconds(s.GetTime() + networkDelayMs)
				c.Endpoint = endpoint

				return &c
			},
		}
		sim.MakeSource(&sourceConf, loop)
	}
}
