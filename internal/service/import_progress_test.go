package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestImportPhasePercent(t *testing.T) {
	tests := []struct {
		phase            string
		processed, total int
		want             int
	}{
		{ImportPhaseUnpack, 0, 100, 0},
		{ImportPhaseUnpack, 50, 100, 2},
		{ImportPhaseUnpack, 100, 100, 5},
		{ImportPhaseParse, 0, 100, 5},
		{ImportPhaseParse, 50, 100, 22},
		{ImportPhaseParse, 100, 100, 40},
		{ImportPhasePersist, 0, 10, 40},
		{ImportPhasePersist, 5, 10, 60},
		{ImportPhasePersist, 10, 10, 80},
		{ImportPhaseIndex, 0, 4, 80},
		{ImportPhaseIndex, 2, 4, 90},
		{ImportPhaseIndex, 4, 4, 100},
		{ImportPhaseParse, 0, 0, 5},
	}
	for _, tc := range tests {
		got := importPhasePercent(tc.phase, tc.processed, tc.total)
		if got != tc.want {
			t.Fatalf("importPhasePercent(%q, %d, %d) = %d, want %d",
				tc.phase, tc.processed, tc.total, got, tc.want)
		}
	}
}

func TestImportProgressMonotonicAndETA(t *testing.T) {
	svc := NewSolutionService(nil, 0)
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	svc.beginImportProgress(id, ImportPhaseParse)

	snap, ok := svc.ImportProgress(id)
	if !ok {
		t.Fatal("expected progress")
	}
	if snap.Phase != ImportPhaseParse || snap.Percent != 5 {
		t.Fatalf("raw xml start snap=%+v", snap)
	}

	svc.reportImportProgress(id, ImportPhaseParse, 50, 100, true)
	snap, ok = svc.ImportProgress(id)
	if !ok {
		t.Fatal("expected progress")
	}
	if snap.Phase != ImportPhaseParse || snap.Percent != 22 {
		t.Fatalf("snap=%+v", snap)
	}
	if snap.ETASeconds != 0 {
		t.Fatalf("eta should be 0 early, got %d", snap.ETASeconds)
	}

	svc.reportImportProgress(id, ImportPhaseParse, 10, 100, true)
	snap, _ = svc.ImportProgress(id)
	if snap.Percent != 22 {
		t.Fatalf("percent must be monotonic, got %d", snap.Percent)
	}

	svc.progressMu.Lock()
	svc.progress[id].startedAt = time.Now().Add(-5 * time.Second)
	svc.progressMu.Unlock()

	svc.reportImportProgress(id, ImportPhasePersist, 5, 10, true)
	snap, _ = svc.ImportProgress(id)
	if snap.Percent != 60 {
		t.Fatalf("percent=%d", snap.Percent)
	}
	if snap.ETASeconds <= 0 {
		t.Fatalf("expected eta > 0, got %d", snap.ETASeconds)
	}

	svc.clearImportProgress(id)
	if _, ok := svc.ImportProgress(id); ok {
		t.Fatal("progress should be cleared")
	}
}

func TestBeginImportProgressXMLStartsAtParse(t *testing.T) {
	svc := NewSolutionService(nil, 0)
	id := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	svc.beginImportProgress(id, ImportPhaseParse)
	snap, ok := svc.ImportProgress(id)
	if !ok {
		t.Fatal("expected progress")
	}
	if snap.Phase != ImportPhaseParse {
		t.Fatalf("phase=%q, want parse", snap.Phase)
	}
	if snap.Percent != 5 {
		t.Fatalf("percent=%d, want 5", snap.Percent)
	}
}

func TestBeginImportProgressZIPStartsAtUnpack(t *testing.T) {
	svc := NewSolutionService(nil, 0)
	id := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	svc.beginImportProgress(id, ImportPhaseUnpack)
	snap, ok := svc.ImportProgress(id)
	if !ok {
		t.Fatal("expected progress")
	}
	if snap.Phase != ImportPhaseUnpack {
		t.Fatalf("phase=%q, want unpack", snap.Phase)
	}
	if snap.Percent != 0 {
		t.Fatalf("percent=%d, want 0", snap.Percent)
	}
	svc.reportImportProgress(id, ImportPhaseUnpack, 50, 100, true)
	snap, _ = svc.ImportProgress(id)
	if snap.Phase != ImportPhaseUnpack || snap.Percent != 2 {
		t.Fatalf("unpack mid snap=%+v", snap)
	}
}
