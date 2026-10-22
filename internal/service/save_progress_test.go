package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSavePhasePercent(t *testing.T) {
	tests := []struct {
		phase            string
		processed, total int
		want             int
	}{
		{SavePhaseLoad, 0, 0, 0},
		{SavePhaseLoad, 0, 1, 0},
		{SavePhaseLoad, 1, 1, 15},
		{SavePhaseApply, 0, 10, 15},
		{SavePhaseApply, 5, 10, 20},
		{SavePhaseApply, 10, 10, 25},
		{SavePhaseMarshal, 0, 1, 25},
		{SavePhaseMarshal, 1, 1, 40},
		{SavePhasePersistXML, 0, 1, 40},
		{SavePhasePersistXML, 1, 1, 55},
		{SavePhaseMetadata, 0, 1, 55},
		{SavePhaseMetadata, 1, 1, 60},
		{SavePhaseIndex, 0, 1, 60},
		{SavePhaseIndex, 1, 1, 85},
		{SavePhaseSchemes, 0, 4, 85},
		{SavePhaseSchemes, 2, 4, 90},
		{SavePhaseSchemes, 4, 4, 95},
		{SavePhaseFinalize, 0, 1, 95},
		{SavePhaseFinalize, 1, 1, 100},
	}
	for _, tc := range tests {
		got := savePhasePercent(tc.phase, tc.processed, tc.total)
		if got != tc.want {
			t.Fatalf("savePhasePercent(%q, %d, %d) = %d, want %d",
				tc.phase, tc.processed, tc.total, got, tc.want)
		}
	}
}

func TestSaveProgressMonotonicAndETA(t *testing.T) {
	svc := NewSolutionService(nil, 0)
	id := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	if !svc.beginSaveProgress(id) {
		t.Fatal("beginSaveProgress should succeed")
	}
	if svc.beginSaveProgress(id) {
		t.Fatal("concurrent beginSaveProgress should fail")
	}

	snap, ok := svc.SaveProgress(id)
	if !ok {
		t.Fatal("expected progress")
	}
	if snap.Phase != SavePhaseLoad || snap.Percent != 0 {
		t.Fatalf("start snap=%+v", snap)
	}

	svc.reportSaveProgress(id, SavePhaseApply, 5, 10, true)
	snap, ok = svc.SaveProgress(id)
	if !ok {
		t.Fatal("expected progress")
	}
	if snap.Phase != SavePhaseApply || snap.Percent != 20 {
		t.Fatalf("snap=%+v", snap)
	}
	if snap.ETASeconds != 0 {
		t.Fatalf("eta should be 0 early, got %d", snap.ETASeconds)
	}

	svc.reportSaveProgress(id, SavePhaseApply, 1, 10, true)
	snap, _ = svc.SaveProgress(id)
	if snap.Percent != 20 {
		t.Fatalf("percent must be monotonic, got %d", snap.Percent)
	}

	svc.saveProgressMu.Lock()
	svc.saveProgress[id].startedAt = time.Now().Add(-5 * time.Second)
	svc.saveProgressMu.Unlock()

	svc.reportSaveProgress(id, SavePhaseIndex, 1, 1, true)
	snap, _ = svc.SaveProgress(id)
	if snap.Percent != 85 {
		t.Fatalf("percent=%d", snap.Percent)
	}
	if snap.ETASeconds <= 0 {
		t.Fatalf("expected eta > 0, got %d", snap.ETASeconds)
	}

	svc.clearSaveProgress(id)
	if _, ok := svc.SaveProgress(id); ok {
		t.Fatal("progress should be cleared")
	}
}
