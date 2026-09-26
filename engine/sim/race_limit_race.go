// +build race

package sim

// LivenessThreshold is the expected liveness success rate for the 5% loss regime.
// Under the race detector, extreme CPU contention causes goroutine starvation that 
// starves the 250ms timeouts before messages can be read. Since 250ms is the maximum
// allowable timeout before hitting the overall 45-minute test timeout, the liveness 
// ceiling is severely degraded to ~20%.
const LivenessThreshold = 0.15
