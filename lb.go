// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

import (
	"math/rand"
)

// LbConf is the configuration of an application
type LbConf struct {
	Name string
	App  *AppConf
}

// LB is a load balancer
type LB struct {
	NodeInterface
	Node
	appInstances []*Node
}

// MakeLB takes an LB config and a loop and returns an
// LB integrated into that loop
func MakeLB(lbConf *LbConf, l *Loop) *LB {
	lb := LB{}
	lb.App = lbConf.App

	lb.appInstances = make([]*Node, lbConf.App.Size)

	for i := uint16(0); i < lbConf.App.Size; i++ {
		n := MakeApp(lbConf.App, l)
		l.AddNode(n)
		lb.appInstances = append(lb.appInstances, n)
	}

	l.AddLB(lbConf.App.Name, &lb)

	return &lb
}
