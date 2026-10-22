package service

import "testing"

func TestMaxParallelWorkersFromCPU(t *testing.T) {
	tests := []struct {
		numCPU int
		want   int
	}{
		{numCPU: 0, want: 1},
		{numCPU: 1, want: 1},
		{numCPU: 2, want: 1},
		{numCPU: 3, want: 1},
		{numCPU: 4, want: 2},
		{numCPU: 5, want: 3},
		{numCPU: 8, want: 4},
		{numCPU: 10, want: 6},
		{numCPU: 16, want: 9},
		{numCPU: 100, want: 60},
	}
	for _, tc := range tests {
		got := maxParallelWorkersFromCPU(tc.numCPU)
		if got != tc.want {
			t.Fatalf("maxParallelWorkersFromCPU(%d) = %d, want %d", tc.numCPU, got, tc.want)
		}
	}
}

func TestParallelWorkerCount(t *testing.T) {
	tests := []struct {
		numCPU   int
		jobCount int
		want     int
	}{
		{numCPU: 10, jobCount: 100, want: 6},
		{numCPU: 10, jobCount: 3, want: 3},
		{numCPU: 10, jobCount: 0, want: 1},
		{numCPU: 1, jobCount: 50, want: 1},
		{numCPU: 8, jobCount: 2, want: 2},
	}
	for _, tc := range tests {
		got := parallelWorkerCount(tc.numCPU, tc.jobCount)
		if got != tc.want {
			t.Fatalf("parallelWorkerCount(%d, %d) = %d, want %d", tc.numCPU, tc.jobCount, got, tc.want)
		}
	}
}
