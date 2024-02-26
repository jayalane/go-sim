// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

// This file has cdf's and the interfaces to use CDFs

import (
	"gonum.org/v1/gonum/stat/distuv"
)

// ModelCdf is a function that is the Cdf of a Distribution.
// It should be >= 0 for all p.
type ModelCdf func(p float64) float64

// Distribution is an interface for configuring a distribution of an RV.
type Distribution interface {
	getTimeToSleepMs()
	setCdf(f ModelCdf)
}

// UniformCDF returns the CDF of a uniform random variable over [a, b]
// Given a probability p (0 to 1), it returns the corresponding z such that P(Z <= z) = p.
func UniformCDF(a, b float64) ModelCdf {
	return func(p float64) float64 {
		if p < 0 {
			return a
		}

		if p > 1 {
			return b
		}

		return a + p*(b-a) // Linear interpolation between a and b
	}
}

// NormalCDF returns the CDF of a normal random variable with mean μ and standard deviation σ.
func NormalCDF(mu, sigma float64) ModelCdf {
	// Create a normal distribution with mean μ and standard deviation σ
	norm := distuv.Normal{
		Mu:    mu,
		Sigma: sigma,
	}

	return norm.CDF
}

// LogNormalCDF returns the CDF of a Log-Normal distribution with mean mu and standard deviation sigma for the logarithm of the distribution.
func LogNormalCDF(mu, sigma float64) func(x float64) float64 {
	logNorm := distuv.LogNormal{
		Mu:    mu,
		Sigma: sigma,
	}

	return logNorm.CDF
}

// ParetoCDF returns the CDF of a Pareto distribution with scale xm and shape alpha.
func ParetoCDF(xm, alpha float64) func(x float64) float64 {
	pareto := distuv.Pareto{
		Xm:    xm,
		Alpha: alpha,
	}

	return pareto.CDF
}

/*
// cauchyCDF returns the CDF of a Cauchy distribution with location x0 and scale gamma.
func cauchyCDF(x0, gamma float64) func(x float64) float64 {
	cauchy := distuv.Cauchy{
		Mu:    x0,
		Sigma: gamma,
	}

	return func(x float64) float64 {
		return cauchy.CDF(x)
	}
}
*/
