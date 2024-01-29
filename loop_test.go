// -*- tab-width:2 -*-

package sim

import (
	"fmt"
	"net/http"
	_ "net/http/pprof" // for pprof
	// "os"
	"strings"
	"testing"

	count "github.com/jayalane/go-counter"
	// ll "github.com/jayalane/go-lll"
)

// TestLoop1 runs a small simulation
func TestLoop1(t *testing.T) {
	// start the profiler
	go func() {
		fmt.Println(http.ListenAndServe(":6060", nil))
	}()

	//	ll.SetWriter(os.Stdout)
	count.InitCounters()

	Init()

	loop := NewLoop("default")

	stageConfAry := []StageConf{{LocalWork: uniformCDF(1, 10)}}
	appConf := AppConf{
		Name:     "serverA",
		Size:     5,
		Stages:   stageConfAry,
		ReplyLen: uniformCDF(200, 20000),
	}

	lbConf := LbConf{Name: "serverA", App: &appConf}
	MakeLB(&lbConf, loop)

	sourceConf := SourceConf{
		Name: "ngrl", Lambda: 0.010, // per ms
		MakeCall: func(s *Source) *Call {
			c := Call{}
			c.reqID = IncrCallNumber()
			c.timeoutMs = 90.0
			c.wakeup = Milliseconds(s.n.loop.GetTime() + 5.0)
			c.endPoint = "serverA"
			return &c
		},
	}

	src := MakeSource(&sourceConf, loop)
	loop.AddSource(src)

	loop.Run(100) // msecs
	loop.Stats()
	count.LogCounters()
}

func FilterCallFunc(
	endpoint string,
	params map[string]string,
) bool {
	if strings.Contains(endpoint, "count") {
		return true
	}
	_, ok := params["DNF"]
	if ok {
		return false
	}
	return true
}

// need DNF
// TestLoop2 runs slightly large simulation of rlproxyserv
func TestLoop2(t *testing.T) {
	loop := NewLoop("default2")

	proxyAConfAry := []StageConf{
		{
			LocalWork: uniformCDF(1, 5),
			RemoteCalls: []RemoteCall{
				{
					endpoint: "count-a",
				}, {
					endpoint: "proxy-b",
					params:   map[string]string{"DNF": "1"},
				}, {
					endpoint: "proxy-c",
					params:   map[string]string{"DNF": "1"},
				},
			},
		},
	}
	proxyAConf := AppConf{
		Name:     "proxy-a",
		Size:     5,
		Stages:   proxyAConfAry,
		ReplyLen: uniformCDF(200, 500),
	}
	proxyAlbConf := LbConf{Name: "proxy-a", App: &proxyAConf}
	MakeLB(&proxyAlbConf, loop)

	proxyBConfAry := []StageConf{
		{
			LocalWork: uniformCDF(1, 5),
			RemoteCalls: []RemoteCall{
				{
					endpoint: "proxy-a",
					params:   map[string]string{"DNF": "1"},
				}, {
					endpoint: "count-b",
				}, {
					endpoint: "count-c",
					params:   map[string]string{"DNF": "1"},
				},
			},
		},
	}
	proxyBConf := AppConf{
		Name:     "proxy-b",
		Size:     5,
		Stages:   proxyBConfAry,
		ReplyLen: uniformCDF(200, 20000),
	}
	proxyBlbConf := LbConf{Name: "proxy-b", App: &proxyBConf}
	MakeLB(&proxyBlbConf, loop)

	proxyCConfAry := []StageConf{
		{
			LocalWork:  uniformCDF(1, 5),
			FilterCall: FilterCallFunc,
			RemoteCalls: []RemoteCall{
				{
					endpoint: "proxy-a",
					params:   map[string]string{"DNF": "1"},
				}, {
					endpoint: "proxy-b",
					params:   map[string]string{"DNF": "1"},
				}, {
					endpoint: "count-c",
				},
			},
		},
	}
	proxyCConf := AppConf{
		Name:     "proxy-c",
		Size:     5,
		Stages:   proxyCConfAry,
		ReplyLen: uniformCDF(100, 200),
	}
	proxyClbConf := LbConf{Name: "proxy-c", App: &proxyCConf}
	MakeLB(&proxyClbConf, loop)

	countConfAry := []StageConf{{LocalWork: uniformCDF(1, 3)}}
	countAConf := AppConf{
		Name:     "count-a",
		Size:     5,
		Stages:   countConfAry,
		ReplyLen: uniformCDF(100, 200),
	}
	countAlbConf := LbConf{Name: "count-a", App: &countAConf}
	MakeLB(&countAlbConf, loop)

	countBConf := AppConf{
		Name:     "count-b",
		Size:     5,
		Stages:   countConfAry,
		ReplyLen: uniformCDF(100, 200),
	}
	countBlbConf := LbConf{Name: "count-b", App: &countBConf}
	MakeLB(&countBlbConf, loop)

	countCConf := AppConf{
		Name:     "count-c",
		Size:     5,
		Stages:   countConfAry,
		ReplyLen: uniformCDF(100, 200),
	}
	countClbConf := LbConf{Name: "count-c", App: &countCConf}
	MakeLB(&countClbConf, loop)

	sourceAConf := SourceConf{
		Name: "ngrl-a", Lambda: 0.1, // per ms
		MakeCall: func(s *Source) *Call {
			c := Call{}
			c.reqID = IncrCallNumber()
			c.timeoutMs = 90.0
			c.wakeup = Milliseconds(s.n.loop.GetTime() + 5.0)
			c.endPoint = "proxy-a"
			return &c
		},
	}

	srcA := MakeSource(&sourceAConf, loop)
	loop.AddSource(srcA)

	sourceBConf := SourceConf{
		Name: "ngrl-b", Lambda: 0.10, // per ms
		MakeCall: func(s *Source) *Call {
			c := Call{}
			c.reqID = IncrCallNumber()
			c.timeoutMs = 90.0
			c.wakeup = Milliseconds(s.n.loop.GetTime() + 5.0)
			c.endPoint = "proxy-b"
			return &c
		},
	}

	srcB := MakeSource(&sourceBConf, loop)
	loop.AddSource(srcB)

	sourceCConf := SourceConf{
		Name: "ngrl-c", Lambda: 0.10, // per ms
		MakeCall: func(s *Source) *Call {
			c := Call{}
			c.reqID = IncrCallNumber()
			c.timeoutMs = 90.0
			c.wakeup = Milliseconds(s.n.loop.GetTime() + 5.0)
			c.endPoint = "proxy-c"
			return &c
		},
	}

	srcC := MakeSource(&sourceCConf, loop)
	loop.AddSource(srcC)

	loop.Run(10000) // msecs
	loop.Stats()
	count.LogCounters()
}
