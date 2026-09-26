package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func runSweep() {
	timeouts := []string{"50", "200", "1000"}
	for _, to := range timeouts {
		// Update FastParams
		script := fmt.Sprintf(`sed -i '' 's/p.LambdaStep = .* \*/p.LambdaStep = %s \*/g' engine/sim/sim_test.go; sed -i '' 's/p.LambdaBlock = .* \*/p.LambdaBlock = %s \*/g' engine/sim/sim_test.go`, to, to)
		exec.Command("sh", "-c", script).Run()
		
		fmt.Printf("Running with timeout %sms...\n", to)
		cmd := exec.Command("go", "test", "./engine/sim", "-run", "TestAdversarialSoak", "-count=1", "-v")
		out, _ := cmd.CombinedOutput()
		
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Regime (5% loss):") {
				fmt.Printf("  %sms: %s\n", to, strings.TrimSpace(line))
			}
		}
	}
}

func main() {
	runSweep()
}
