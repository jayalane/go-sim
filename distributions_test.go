// -*- tab-width:2 -*-
package sim

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestDistribution(_ *testing.T) {
	for _, f := range []ModelCdf{
		UniformCDF(1, 5),
		NormalCDF(3, 1),
		LogNormalCDF(2, 1),
		ParetoCDF(5, 1),
	} {
		for x := 0; x < 1000; x++ {
			p := rand.Float64() //nolint:gosec
			fmt.Printf("p is %f RV is %f\n", p, f(p))
		}
		fmt.Println("Next RV type")
	}
}
