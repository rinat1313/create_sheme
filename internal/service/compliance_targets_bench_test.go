package service

import (
	"encoding/xml"
	"os"
	"testing"

	"create_sheme/internal/domain"
)

func loadSolutionXMLForBench(b *testing.B) domain.Solution {
	b.Helper()
	data, err := os.ReadFile("../../Solution.xml")
	if err != nil {
		b.Skip("Solution.xml not available")
	}
	var sol domain.Solution
	if err := xml.Unmarshal(data, &sol); err != nil {
		b.Fatalf("unmarshal: %v", err)
	}
	return sol
}

func BenchmarkListComplianceTargetsFromSolution_Valve(b *testing.B) {
	sol := loadSolutionXMLForBench(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := listComplianceTargetsFromSolution(sol, "Valve", "command")
		if err != nil {
			b.Fatal(err)
		}
	}
}
