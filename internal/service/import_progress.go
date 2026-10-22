package service

import (
	"log/slog"
	"time"

	"github.com/google/uuid"
)

const (
	ImportPhaseUnpack  = "unpack"
	ImportPhaseParse   = "parse"
	ImportPhasePersist = "persist"
	ImportPhaseIndex   = "index"

	importProgressThrottle = 300 * time.Millisecond
	importParseByteStep    = int64(1 << 20)
	importETAMinPercent    = 8
	importETAMinElapsed    = 3 * time.Second
)

type ImportProgressSnapshot struct {
	Phase      string
	Percent    int
	Processed  int
	Total      int
	StartedAt  time.Time
	ETASeconds int
}

type importProgressState struct {
	phase             string
	percent           int
	processed         int
	total             int
	startedAt         time.Time
	lastEmit          time.Time
	lastReportedBytes int64
	etaSeconds        int
}

func (s *SolutionService) beginImportProgress(id uuid.UUID, phase string) {
	start := time.Now()
	defer func() {
		slog.Info("beginImportProgress done",
			"solution_id", id,
			"phase", phase,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}()
	slog.Debug("beginImportProgress start", "solution_id", id, "phase", phase)

	if phase == "" {
		phase = ImportPhaseUnpack
	}
	pct := importPhasePercent(phase, 0, 0)

	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	if s.progress == nil {
		s.progress = map[uuid.UUID]*importProgressState{}
	}
	s.progress[id] = &importProgressState{
		phase:     phase,
		percent:   pct,
		startedAt: time.Now(),
	}
}

func (s *SolutionService) clearImportProgress(id uuid.UUID) {
	start := time.Now()
	defer func() {
		slog.Info("clearImportProgress done",
			"solution_id", id,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}()
	slog.Debug("clearImportProgress start", "solution_id", id)

	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	delete(s.progress, id)
}

func (s *SolutionService) ImportProgress(id uuid.UUID) (ImportProgressSnapshot, bool) {
	s.progressMu.RLock()
	defer s.progressMu.RUnlock()
	st, ok := s.progress[id]
	if !ok {
		return ImportProgressSnapshot{}, false
	}
	return ImportProgressSnapshot{
		Phase:      st.phase,
		Percent:    st.percent,
		Processed:  st.processed,
		Total:      st.total,
		StartedAt:  st.startedAt,
		ETASeconds: st.etaSeconds,
	}, true
}

func (s *SolutionService) reportImportProgress(id uuid.UUID, phase string, processed, total int, force bool) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	st, ok := s.progress[id]
	if !ok {
		return
	}

	pct := importPhasePercent(phase, processed, total)
	if pct < st.percent {
		pct = st.percent
	}

	now := time.Now()
	shouldEmit := force
	if !shouldEmit && now.Sub(st.lastEmit) >= importProgressThrottle {
		shouldEmit = true
	}
	if !shouldEmit && phase == ImportPhaseParse && int64(processed)-st.lastReportedBytes >= importParseByteStep {
		shouldEmit = true
	}
	if !shouldEmit {
		return
	}

	st.phase = phase
	st.percent = pct
	st.processed = processed
	st.total = total
	st.lastEmit = now
	st.lastReportedBytes = int64(processed)

	eta := 0
	elapsed := now.Sub(st.startedAt)
	if pct >= importETAMinPercent && elapsed >= importETAMinElapsed {
		eta = int(float64(elapsed) / float64(pct) * float64(100-pct) / float64(time.Second))
		if eta < 1 {
			eta = 1
		}
	}
	st.etaSeconds = eta
}

func importPhasePercent(phase string, processed, total int) int {
	base, span := 0, 0
	switch phase {
	case ImportPhaseUnpack:
		base, span = 0, 5
	case ImportPhaseParse:
		base, span = 5, 35
	case ImportPhasePersist:
		base, span = 40, 40
	case ImportPhaseIndex:
		base, span = 80, 20
	default:
		return 0
	}
	if total <= 0 {
		return base
	}
	if processed < 0 {
		processed = 0
	}
	if processed > total {
		processed = total
	}
	return base + (span*processed)/total
}
