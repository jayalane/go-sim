// -*- tab-width:2 -*-
package sim

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestDistribution(t *testing.T) {
	for _, f := range []LatencyCdf{
		uniformCDF(1, 5),
		normalCDF(3, 1),
		logNormalCDF(2, 1),
		paretoCDF(5, 1),
	} {
		for x := 0; x < 1000; x++ {
			p := rand.Float64()
			fmt.Printf("p is %f RV is %f\n", p, f(p))
		}
		fmt.Println("Next RV type")
	}
}
