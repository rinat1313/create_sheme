package service

import "runtime"

const parallelWorkerCPUPercent = 60

func maxParallelWorkersFromCPU(numCPU int) int {
	if numCPU < 1 {
		numCPU = 1
	}
	workers := numCPU * parallelWorkerCPUPercent / 100
	if workers < 1 {
		return 1
	}
	return workers
}

func parallelWorkerCount(numCPU, jobCount int) int {
	if jobCount < 1 {
		return 1
	}
	workers := maxParallelWorkersFromCPU(numCPU)
	if workers > jobCount {
		return jobCount
	}
	return workers
}

func parallelWorkersForJobs(jobCount int) int {
	return parallelWorkerCount(runtime.NumCPU(), jobCount)
}
