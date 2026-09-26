// +build !race

package sim

// LivenessThreshold is the expected liveness success rate for the 5% loss regime.
// Without the race detector, the protocol plateaus at ~48% success at n=4.
const LivenessThreshold = 0.40
