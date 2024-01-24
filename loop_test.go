// This example demonstrates a priority queue built using the heap interface.
package sim

import (
	//	"os"
	"testing"

	count "github.com/jayalane/go-counter"
	ll "github.com/jayalane/go-lll"
)

// TestLoop1 runs a small simulation
func TestLoop1(t *testing.T) {
	//	ll.SetWriter(os.Stdout)
	count.InitCounters()

	Init()
	ll.SetLevel(ml, "never")

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
		Name: "ngrl", Lambda: 10, // per ms
		MakeCall: func(s *Source) *Call {
			c := Call{}
			c.replyCh = make(chan *Reply, 2)
			c.timeoutMs = 90.0
			c.wakeup = Milliseconds(s.n.loop.GetTime() + 5.0) // really
			c.endPoint = "count"
			return &c
		},
	}

	src := MakeSource(&sourceConf, loop)
	loop.AddSource(src)

	loop.Run(1000) // msecs
	loop.Stats()
	count.LogCounters()
}

// need DNF
// TestLoop2 runs slightly large simulation
/* func TestLoop1(t *testing.T) {

	loop := NewLoop("default2")

	proxyConfAry := []StageConf{{LocalWork: uniformCDF(1, 5),
		RemoteCalls: []string{"count-a", "proxy-b", "proxy-c"}},
	}
	proxyConf := AppConf{
		Name:     "proxy-a",
		Size:     5,
		Stages:   proxyConfAry,
		ReplyLen: uniformCDF(200, 20000),
	}
	proxylbConf := LbConf{Name: "proxy-a", App: &appConf}
	MakeLB(&lbConf, loop)


	proxyConfAry := []StageConf{{LocalWork: uniformCDF(1, 5),
		RemoteCalls: []string{"count-a", "proxy-b", "proxy-c"}},
	}
	proxyConf := AppConf{
		Name:     "proxy-a",
		Size:     5,
		Stages:   proxyConfAry,
		ReplyLen: uniformCDF(200, 20000),
	}
	proxylbConf := LbConf{Name: "proxy-a", App: &appConf}
	MakeLB(&lbConf, loop)


	proxyConfAry := []StageConf{{LocalWork: uniformCDF(1, 5),
		RemoteCalls: []string{"proxy-a", "count-b", "count-cy-c"}},
	}
	proxyConf := AppConf{
		Name:     "proxy-a",
		Size:     5,
		Stages:   proxyConfAry,
		ReplyLen: uniformCDF(200, 20000),
	}
	proxylbConf := LbConf{Name: "proxy-a", App: &appConf}
	MakeLB(&lbConf, loop)

	countConfAry := []StageConf{{LocalWork: uniformCDF(1, 10)}}
	appConf := AppConf{
		Name:     "count",
		Size:     5,
		Stages:   stageConfAry,
		ReplyLen: uniformCDF(200, 20000),
	}

	lbConf := LbConf{Name: "count", App: &appConf}
	MakeLB(&lbConf, loop)

	sourceConf := SourceConf{
		Name: "ngrl", Lambda: 10,  // per ms
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

	loop.Run(1000) // msecs
	loop.Stats()
	count.LogCounters()
}
*/
