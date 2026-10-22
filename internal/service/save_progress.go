package service

import (
	"log/slog"
	"time"

	"github.com/google/uuid"
)

const (
	SavePhaseLoad       = "load"
	SavePhaseApply      = "apply"
	SavePhaseMarshal    = "marshal"
	SavePhasePersistXML = "persist_xml"
	SavePhaseMetadata   = "metadata"
	SavePhaseIndex      = "index"
	SavePhaseSchemes    = "schemes"
	SavePhaseFinalize   = "finalize"

	saveProgressThrottle = 300 * time.Millisecond
	saveETAMinPercent    = 8
	saveETAMinElapsed    = 3 * time.Second
)

type SaveProgressSnapshot struct {
	Phase      string
	Percent    int
	Processed  int
	Total      int
	StartedAt  time.Time
	ETASeconds int
}

type saveProgressState struct {
	phase      string
	percent    int
	processed  int
	total      int
	startedAt  time.Time
	lastEmit   time.Time
	etaSeconds int
}

func (s *SolutionService) beginSaveProgress(id uuid.UUID) bool {
	start := time.Now()
	defer func() {
		slog.Info("beginSaveProgress done",
			"solution_id", id,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}()
	slog.Debug("beginSaveProgress start", "solution_id", id)

	s.saveProgressMu.Lock()
	defer s.saveProgressMu.Unlock()
	if s.saveProgress == nil {
		s.saveProgress = map[uuid.UUID]*saveProgressState{}
	}
	if _, ok := s.saveProgress[id]; ok {
		return false
	}
	s.saveProgress[id] = &saveProgressState{
		phase:     SavePhaseLoad,
		percent:   savePhasePercent(SavePhaseLoad, 0, 0),
		startedAt: time.Now(),
	}
	return true
}

func (s *SolutionService) clearSaveProgress(id uuid.UUID) {
	start := time.Now()
	defer func() {
		slog.Info("clearSaveProgress done",
			"solution_id", id,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}()
	slog.Debug("clearSaveProgress start", "solution_id", id)

	s.saveProgressMu.Lock()
	defer s.saveProgressMu.Unlock()
	delete(s.saveProgress, id)
}

func (s *SolutionService) SaveProgress(id uuid.UUID) (SaveProgressSnapshot, bool) {
	s.saveProgressMu.RLock()
	defer s.saveProgressMu.RUnlock()
	st, ok := s.saveProgress[id]
	if !ok {
		return SaveProgressSnapshot{}, false
	}
	return SaveProgressSnapshot{
		Phase:      st.phase,
		Percent:    st.percent,
		Processed:  st.processed,
		Total:      st.total,
		StartedAt:  st.startedAt,
		ETASeconds: st.etaSeconds,
	}, true
}

func (s *SolutionService) reportSaveProgress(id uuid.UUID, phase string, processed, total int, force bool) {
	s.saveProgressMu.Lock()
	defer s.saveProgressMu.Unlock()
	st, ok := s.saveProgress[id]
	if !ok {
		return
	}

	pct := savePhasePercent(phase, processed, total)
	if pct < st.percent {
		pct = st.percent
	}

	now := time.Now()
	shouldEmit := force
	if !shouldEmit && now.Sub(st.lastEmit) >= saveProgressThrottle {
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

	eta := 0
	elapsed := now.Sub(st.startedAt)
	if pct >= saveETAMinPercent && elapsed >= saveETAMinElapsed {
		eta = int(float64(elapsed) / float64(pct) * float64(100-pct) / float64(time.Second))
		if eta < 1 {
			eta = 1
		}
	}
	st.etaSeconds = eta
}

func savePhasePercent(phase string, processed, total int) int {
	base, span := 0, 0
	switch phase {
	case SavePhaseLoad:
		base, span = 0, 15
	case SavePhaseApply:
		base, span = 15, 10
	case SavePhaseMarshal:
		base, span = 25, 15
	case SavePhasePersistXML:
		base, span = 40, 15
	case SavePhaseMetadata:
		base, span = 55, 5
	case SavePhaseIndex:
		base, span = 60, 25
	case SavePhaseSchemes:
		base, span = 85, 10
	case SavePhaseFinalize:
		base, span = 95, 5
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
