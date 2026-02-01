// -*- tab-width:2 -*-

// Package sim provides a library to specify a distributed system
// discrete event simulation and then run it to generate statistics
package sim

import (
	"math"
	"math/rand"
)

// RetryPolicy configures retry behavior for outbound calls.
type RetryPolicy struct {
	MaxRetries    int
	InitialDelay  Milliseconds
	BackoffFactor float64
	MaxDelay      Milliseconds
	Jitter        float64 // 0.0 to 1.0, fraction of delay to randomize
}

// RetryState tracks retry progress for a single outbound call.
type RetryState struct {
	policy      *RetryPolicy
	attempt     int
	nextRetryAt Milliseconds
}

// DefaultRetryPolicy returns a sensible default retry configuration.
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:    3,    //nolint:mnd
		InitialDelay:  100,  //nolint:mnd // 100ms
		BackoffFactor: 2.0,  //nolint:mnd
		MaxDelay:      5000, //nolint:mnd // 5 seconds
		Jitter:        0.2,  //nolint:mnd // 20% jitter
	}
}

// DelayForAttempt returns the delay in Milliseconds for the given attempt,
// using exponential backoff with jitter.
func (p *RetryPolicy) DelayForAttempt(attempt int) Milliseconds {
	delay := float64(p.InitialDelay) * math.Pow(p.BackoffFactor, float64(attempt))

	if Milliseconds(delay) > p.MaxDelay {
		delay = float64(p.MaxDelay)
	}

	// Apply jitter
	if p.Jitter > 0 {
		jitterAmount := delay * p.Jitter
		delay += (rand.Float64()*2 - 1) * jitterAmount //nolint:gosec
	}

	return Milliseconds(delay)
}
