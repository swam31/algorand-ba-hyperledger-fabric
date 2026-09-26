package main

import (
	"fmt"
	"testing"
	"github.com/swayamchavan/algorand-fabric/engine/sim"
)

func TestFast(t *testing.T) {
	sim.TestSim_AllHonestFast(t)
}
