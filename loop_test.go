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
	longTestMsecs  = 10_000
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

	stageConfAry := []StageConf{{LocalWork: UniformCDF(1, 10)}}
	appConf := AppConf{
		Name:     "serverA",
		Size:     5,
		Stages:   stageConfAry,
		ReplyLen: UniformCDF(200, 20000),
	}

	lbConf := LbConf{Name: "serverA", App: &appConf}
	MakeLB(&lbConf, loop)

	sourceConf := SourceConf{
		Name: "external", Lambda: 0.10, // per ms
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

	return !ok
}

// need DNF
// TestLoop2 runs slightly large simulation of rlproxyserv.
func TestLoop2(_ *testing.T) {
	loop := NewLoop()

	proxyAConfAry := []StageConf{
		{
			LocalWork:  UniformCDF(1, 5),
			FilterCall: FilterCallFunc,
			RemoteCalls: []RemoteCall{
				{
					Endpoint: "count-a",
				}, {
					Endpoint: "proxy-b",
					Params:   map[string]string{"DNF": "1"},
				}, {
					Endpoint: "proxy-c",
					Params:   map[string]string{"DNF": "1"},
				},
			},
		},
	}
	proxyAConf := AppConf{
		Name:     "proxy",
		Size:     5,
		Stages:   proxyAConfAry,
		ReplyLen: UniformCDF(200, 500),
	}
	proxyAlbConf := LbConf{Name: "proxy-a", App: &proxyAConf}
	MakeLB(&proxyAlbConf, loop)

	proxyBConfAry := []StageConf{
		{
			LocalWork:  UniformCDF(1, 5),
			FilterCall: FilterCallFunc,
			RemoteCalls: []RemoteCall{
				{
					Endpoint: "proxy-a",
					Params:   map[string]string{"DNF": "1"},
				}, {
					Endpoint: "count-b",
				}, {
					Endpoint: "proxy-c",
					Params:   map[string]string{"DNF": "1"},
				},
			},
		},
	}
	proxyBConf := AppConf{
		Name:     "proxy",
		Size:     5,
		Stages:   proxyBConfAry,
		ReplyLen: UniformCDF(200, 20000),
	}
	proxyBlbConf := LbConf{Name: "proxy-b", App: &proxyBConf}
	MakeLB(&proxyBlbConf, loop)

	proxyCConfAry := []StageConf{
		{
			LocalWork:  UniformCDF(1, 5),
			FilterCall: FilterCallFunc,
			RemoteCalls: []RemoteCall{
				{
					Endpoint: "proxy-a",
					Params:   map[string]string{"DNF": "1"},
				}, {
					Endpoint: "proxy-b",
					Params:   map[string]string{"DNF": "1"},
				}, {
					Endpoint: "count-c",
				},
			},
		},
	}
	proxyCConf := AppConf{
		Name:     "proxy",
		Size:     5,
		Stages:   proxyCConfAry,
		ReplyLen: UniformCDF(100, 200),
	}
	proxyClbConf := LbConf{Name: "proxy-c", App: &proxyCConf}
	MakeLB(&proxyClbConf, loop)

	countConfAry := []StageConf{{LocalWork: UniformCDF(1, 3)}}
	countAConf := AppConf{
		Name:     "count",
		Size:     5,
		Stages:   countConfAry,
		ReplyLen: UniformCDF(100, 200),
	}
	countAlbConf := LbConf{Name: "count-a", App: &countAConf}
	MakeLB(&countAlbConf, loop)

	countBConf := AppConf{
		Name:     "count",
		Size:     5,
		Stages:   countConfAry,
		ReplyLen: UniformCDF(100, 200),
	}
	countBlbConf := LbConf{Name: "count-b", App: &countBConf}
	MakeLB(&countBlbConf, loop)

	countCConf := AppConf{
		Name:     "count",
		Size:     5,
		Stages:   countConfAry,
		ReplyLen: UniformCDF(100, 200),
	}
	countClbConf := LbConf{Name: "count-c", App: &countCConf}
	MakeLB(&countClbConf, loop)

	sourceAConf := SourceConf{
		Name: "ngrl-a", Lambda: longTestLambda, // per ms
		MakeCall: func(s *Source) *Call {
			c := Call{}
			c.ReqID = IncrCallNumber()
			c.TimeoutMs = 90.0
			c.Wakeup = Milliseconds(s.n.loop.GetTime() + 5.0)
			c.Endpoint = "proxy-a"

			return &c
		},
	}

	MakeSource(&sourceAConf, loop)

	sourceBConf := SourceConf{
		Name: "ngrl-b", Lambda: longTestLambda, // per ms
		MakeCall: func(s *Source) *Call {
			c := Call{}
			c.ReqID = IncrCallNumber()
			c.TimeoutMs = 90.0
			c.Wakeup = Milliseconds(s.n.loop.GetTime() + 5.0)
			c.Endpoint = "proxy-b"

			return &c
		},
	}

	MakeSource(&sourceBConf, loop)

	sourceCConf := SourceConf{
		Name: "ngrl-c", Lambda: longTestLambda, // per ms
		MakeCall: func(s *Source) *Call {
			c := Call{}
			c.ReqID = IncrCallNumber()
			c.TimeoutMs = 90.0
			c.Wakeup = Milliseconds(s.n.loop.GetTime() + 5.0)
			c.Endpoint = "proxy-c"

			return &c
		},
	}

	MakeSource(&sourceCConf, loop)

	loop.Run(longTestMsecs)
	loop.Stats()
	count.LogCounters()
}
