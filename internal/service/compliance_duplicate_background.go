package service

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"create_sheme/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrBackgroundScanActive = errors.New("background duplicate scan is active for this solution")
)

const (
	bgScanWorkers             = 1
	bgScanBatchSize           = 40
	bgScanSleepBetweenBatches = 100 * time.Millisecond
	bgDirtyPriorityNormal     = 0
	bgDirtyPriorityHigh       = 100
	bgReloadDebounceBatches   = 5
	bgReloadDebounceInterval  = 2 * time.Second
	forcedScanWorkers         = 1
	forcedScanBatchSize       = 40
	forcedScanSleepBetweenBatches = 100 * time.Millisecond
)

type BackgroundScanStatus struct {
	Enabled   bool    `json:"enabled"`
	Mode      string  `json:"mode"`
	Status    string  `json:"status"`
	Processed int     `json:"processed"`
	Total     int     `json:"total"`
	Percent   float64 `json:"percent"`
	Found     int     `json:"found"`
	RuleID    string  `json:"rule_id,omitempty"`
	Error     string  `json:"error_message,omitempty"`
}

type backgroundScannerRuntime struct {
	cancel context.CancelFunc
	mu     sync.Mutex
}

var backgroundScannerRuntimes sync.Map

type tagCountEngine struct {
	counts     map[string]int
	objectTags map[string]map[string]int
	rows       map[string]TagOccurrence
	mu         sync.Mutex
}

func newTagCountEngine() *tagCountEngine {
	return &tagCountEngine{
		counts:     map[string]int{},
		objectTags: map[string]map[string]int{},
		rows:       map[string]TagOccurrence{},
	}
}

func duplicateRowKey(occ TagOccurrence) string {
	return strings.Join([]string{
		occ.Tag, occ.ObjectID, occ.Scope, occ.PropertyID, occ.EntityID, occ.SourceLabel,
	}, "\x00")
}

func occurrenceTagContrib(occs []TagOccurrence) map[string]int {
	out := map[string]int{}
	for _, occ := range occs {
		tag := strings.TrimSpace(occ.Tag)
		if tag == "" {
			continue
		}
		out[tag]++
	}
	return out
}

func (e *tagCountEngine) adjustObject(objectID string, newOccs []TagOccurrence) (affectedTags []string, becameDuplicate []string, lostDuplicate []string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	oldContrib := e.objectTags[objectID]
	if oldContrib == nil {
		oldContrib = map[string]int{}
	}
	newContrib := occurrenceTagContrib(newOccs)
	tagSet := map[string]bool{}
	for tag := range oldContrib {
		tagSet[tag] = true
	}
	for tag := range newContrib {
		tagSet[tag] = true
	}

	for tag := range tagSet {
		oldC := oldContrib[tag]
		newC := newContrib[tag]
		delta := newC - oldC
		if delta == 0 {
			continue
		}
		before := e.counts[tag]
		after := before + delta
		if after < 0 {
			after = 0
		}
		if after == 0 {
			delete(e.counts, tag)
		} else {
			e.counts[tag] = after
		}
		affectedTags = append(affectedTags, tag)
		if before < 2 && after >= 2 {
			becameDuplicate = append(becameDuplicate, tag)
		}
		if before >= 2 && after < 2 {
			lostDuplicate = append(lostDuplicate, tag)
		}
	}

	if len(newContrib) == 0 {
		delete(e.objectTags, objectID)
	} else {
		e.objectTags[objectID] = newContrib
	}

	for key, row := range e.rows {
		if row.ObjectID == objectID {
			delete(e.rows, key)
		}
	}
	for _, occ := range newOccs {
		if e.counts[occ.Tag] >= 2 {
			e.rows[duplicateRowKey(occ)] = occ
		}
	}
		for _, tag := range lostDuplicate {
		for key, row := range e.rows {
			if row.Tag == tag {
				delete(e.rows, key)
			}
		}
	}

	sort.Strings(affectedTags)
	sort.Strings(becameDuplicate)
	sort.Strings(lostDuplicate)
	return affectedTags, becameDuplicate, lostDuplicate
}

func (e *tagCountEngine) duplicateTags() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	var tags []string
	for tag, count := range e.counts {
		if count >= 2 {
			tags = append(tags, tag)
		}
	}
	sort.Strings(tags)
	return tags
}

func (e *tagCountEngine) duplicateRows() []TagOccurrence {
	e.mu.Lock()
	defer e.mu.Unlock()
	rows := make([]TagOccurrence, 0, len(e.rows))
	for _, row := range e.rows {
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Tag != rows[j].Tag {
			return rows[i].Tag < rows[j].Tag
		}
		if rows[i].ObjectID != rows[j].ObjectID {
			return rows[i].ObjectID < rows[j].ObjectID
		}
		return rows[i].SourceLabel < rows[j].SourceLabel
	})
	return rows
}

func (e *tagCountEngine) countFor(tag string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.counts[tag]
}

func (e *tagCountEngine) isDuplicateTag(tag string) bool {
	return e.countFor(tag) >= 2
}

type dirtyQueueItem struct {
	ObjectID string
	Priority int
	Reason   string
}

type dirtyQueue struct {
	items []dirtyQueueItem
	mu    sync.Mutex
}

func (q *dirtyQueue) enqueue(objectID string, priority int, reason string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i := range q.items {
		if q.items[i].ObjectID == objectID {
			if priority > q.items[i].Priority {
				q.items[i].Priority = priority
				q.items[i].Reason = reason
			}
			return
		}
	}
	q.items = append(q.items, dirtyQueueItem{ObjectID: objectID, Priority: priority, Reason: reason})
}

func (q *dirtyQueue) popHighest() (dirtyQueueItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return dirtyQueueItem{}, false
	}
	best := 0
	for i := 1; i < len(q.items); i++ {
		if q.items[i].Priority > q.items[best].Priority {
			best = i
		}
	}
	item := q.items[best]
	q.items = append(q.items[:best], q.items[best+1:]...)
	return item, true
}

func (q *dirtyQueue) len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

type backgroundScanPlan struct {
	Mode       string
	Cursor     int
	Processed  int
	Total      int
	FullQueue  []string
	DirtyQueue *dirtyQueue
	Workers    int
	BatchSize  int
}

func newBackgroundScanPlan(objectIDs []string) *backgroundScanPlan {
	return &backgroundScanPlan{
		Mode:       "full",
		Cursor:     0,
		Processed:  0,
		Total:      len(objectIDs),
		FullQueue:  append([]string(nil), objectIDs...),
		DirtyQueue: &dirtyQueue{},
		Workers:    bgScanWorkers,
		BatchSize:  bgScanBatchSize,
	}
}

func resumeBackgroundScanPlan(objectIDs []string, mode string, cursor, processed int) *backgroundScanPlan {
	plan := newBackgroundScanPlan(objectIDs)
	switch mode {
	case "incremental":
		plan.Mode = "incremental"
		plan.Cursor = len(objectIDs)
		plan.Processed = processed
		if plan.Processed < plan.Total {
			plan.Processed = plan.Total
		}
	case "full":
		if cursor < 0 {
			cursor = 0
		}
		if cursor > len(objectIDs) {
			cursor = len(objectIDs)
		}
		plan.Cursor = cursor
		plan.Processed = processed
		if plan.Processed < cursor {
			plan.Processed = cursor
		}
		if plan.Cursor >= len(objectIDs) {
			plan.Mode = "incremental"
		}
	}
	return plan
}

func (p *backgroundScanPlan) nextBatch() (objectIDs []string, fromPriority bool, done bool) {
	if p.DirtyQueue != nil {
		var batch []string
		for len(batch) < p.BatchSize {
			item, ok := p.DirtyQueue.popHighest()
			if !ok {
				break
			}
			batch = append(batch, item.ObjectID)
		}
		if len(batch) > 0 {
			return batch, true, false
		}
	}
	if p.Mode == "incremental" {
		return nil, false, p.DirtyQueue == nil || p.DirtyQueue.len() == 0
	}
	if p.Cursor >= len(p.FullQueue) {
		p.Mode = "incremental"
		return nil, false, true
	}
	end := p.Cursor + p.BatchSize
	if end > len(p.FullQueue) {
		end = len(p.FullQueue)
	}
	batch := append([]string(nil), p.FullQueue[p.Cursor:end]...)
	p.Cursor = end
	p.Processed = p.Cursor
	if p.Cursor >= len(p.FullQueue) {
		p.Mode = "incremental"
	}
	return batch, false, false
}

func (s *SolutionService) isBackgroundScanEnabled(ctx context.Context, solutionID uuid.UUID) bool {
	sc, err := s.store.GetComplianceDuplicateScanner(ctx, solutionID)
	if err != nil {
		return false
	}
	return sc.Enabled
}

func (s *SolutionService) SetBackgroundDuplicateScan(ctx context.Context, solutionID, ruleID uuid.UUID, sessionID *uuid.UUID, enabled bool) error {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("SetBackgroundDuplicateScan done",
			"solution_id", solutionID,
			"enabled", enabled,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("SetBackgroundDuplicateScan start", "solution_id", solutionID, "enabled", enabled)

	if enabled {
		if _, err = s.findDuplicateTagRule(ctx, ruleID); err != nil {
			return err
		}
		if s.isTagEditRunning(ctx, solutionID) {
			err = ErrTagEditJobActive
			return err
		}
		if _, loaded := duplicateScanJobLocks.Load(solutionID.String()); loaded {
			err = ErrDuplicateScanActive
			return err
		}
		active, aerr := s.store.HasActiveComplianceDuplicateJob(ctx, solutionID)
		if aerr != nil {
			err = aerr
			return err
		}
		if active {
			err = ErrDuplicateScanActive
			return err
		}
		rid := ruleID
		if err = s.store.SetComplianceDuplicateScannerEnabled(ctx, solutionID, true, &rid, sessionID, "full", "running"); err != nil {
			return err
		}
		s.startBackgroundScanner(solutionID)
		return nil
	}

	s.stopBackgroundScanner(solutionID)
	if err = s.store.SetComplianceDuplicateScannerEnabled(ctx, solutionID, false, nil, sessionID, "idle", "idle"); err != nil {
		return err
	}
	return nil
}

func (s *SolutionService) stopBackgroundScanner(solutionID uuid.UUID) {
	if v, ok := backgroundScannerRuntimes.LoadAndDelete(solutionID.String()); ok {
		rt := v.(*backgroundScannerRuntime)
		rt.mu.Lock()
		if rt.cancel != nil {
			rt.cancel()
		}
		rt.mu.Unlock()
	}
}

func (s *SolutionService) startBackgroundScanner(solutionID uuid.UUID) {
	s.stopBackgroundScanner(solutionID)
	ctx, cancel := context.WithCancel(context.Background())
	rt := &backgroundScannerRuntime{cancel: cancel}
	backgroundScannerRuntimes.Store(solutionID.String(), rt)
	go s.runBackgroundScannerLoop(ctx, solutionID)
}

func (s *SolutionService) ResumeEnabledBackgroundScanners(ctx context.Context) error {
	start := time.Now()
	var err error
	var resumed int
	defer func() {
		slog.Info("ResumeEnabledBackgroundScanners done",
			"resumed", resumed,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Info("ResumeEnabledBackgroundScanners start")

	scanners, err := s.store.ListEnabledComplianceDuplicateScanners(ctx)
	if err != nil {
		return err
	}
	for _, sc := range scanners {
		slog.Info("ResumeEnabledBackgroundScanners resume",
			"solution_id", sc.SolutionID,
			"mode", sc.Mode,
			"cursor_offset", sc.CursorOffset,
			"processed", sc.Processed,
			"status", sc.Status,
		)
		s.startBackgroundScanner(sc.SolutionID)
		resumed++
	}
	return nil
}

func (s *SolutionService) restoreTagCountEngineFromStore(ctx context.Context, solutionID uuid.UUID, sol domain.Solution, refByID map[string]tagEditObjectRef, schemeNames map[string]string, excluded map[string]bool, excludedTagPatterns []duplicateTagPattern) (*tagCountEngine, map[string][]TagOccurrence, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Debug("restoreTagCountEngineFromStore done",
			"solution_id", solutionID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("restoreTagCountEngineFromStore start", "solution_id", solutionID)

	engine := newTagCountEngine()
	counts, err := s.store.ListComplianceTagCounts(ctx, solutionID)
	if err != nil {
		return nil, nil, err
	}
	engine.counts = counts

	objectTags, err := s.store.ListAllComplianceTagObjectTags(ctx, solutionID)
	if err != nil {
		return nil, nil, err
	}
	engine.objectTags = objectTags

	rows, err := s.store.ListComplianceDuplicateRows(ctx, solutionID)
	if err != nil {
		return nil, nil, err
	}
	for _, row := range rows {
		occ := tagOccurrenceFromDuplicateRow(row)
		if occ.TechnologyObjectID == "" {
			if ref, ok := refByID[occ.ObjectID]; ok {
				var to *domain.TechnologyObject
				if len(ref.obj.TechnologyObject) > 0 {
					to = &ref.obj.TechnologyObject[0]
				}
				occ.TechnologyObjectID = technologyObjectDisplayID(to)
			}
		}
		engine.rows[duplicateRowKey(occ)] = occ
	}

	occByObject := map[string][]TagOccurrence{}
	for objectID := range objectTags {
		ref, ok := refByID[objectID]
		if !ok {
			continue
		}
		schemeName := schemeNames[ref.schemeXMLID]
		occByObject[objectID] = collectTagOccurrencesForObject(sol, ref.schemeXMLID, schemeName, ref.obj, excluded, excludedTagPatterns)
	}
	return engine, occByObject, nil
}

func (s *SolutionService) runBackgroundScannerLoop(ctx context.Context, solutionID uuid.UUID) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("runBackgroundScannerLoop done",
			"solution_id", solutionID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
		backgroundScannerRuntimes.Delete(solutionID.String())
	}()
	slog.Info("runBackgroundScannerLoop start", "solution_id", solutionID)

	sc, err := s.store.GetComplianceDuplicateScanner(ctx, solutionID)
	if err != nil || !sc.Enabled || sc.RuleID == nil {
		return
	}

	rule, err := s.findDuplicateTagRule(ctx, *sc.RuleID)
	if err != nil {
		_ = s.store.UpdateComplianceDuplicateScannerProgress(ctx, solutionID, "idle", 0, 0, 0, "error", err.Error())
		return
	}

	sol, err := s.loadSolutionWithChanges(ctx, solutionID, sc.SessionID)
	if err != nil {
		_ = s.store.UpdateComplianceDuplicateScannerProgress(ctx, solutionID, "idle", 0, 0, 0, "error", err.Error())
		return
	}

	refs := s.collectAllGraphicObjectRefs(sol)
	schemeNames := s.buildSchemeNames(ctx, solutionID, refs)
	excluded := parseDuplicateExcludedScopes(rule)
	excludedTagPatterns := parseDuplicateExcludedTagPatterns(rule)

	objectIDs := make([]string, 0, len(refs))
	refByID := map[string]tagEditObjectRef{}
	for _, ref := range refs {
		objectIDs = append(objectIDs, ref.obj.Id)
		refByID[ref.obj.Id] = ref
	}

	freshStart := sc.Mode == "full" && sc.CursorOffset == 0 && sc.Processed == 0
	var plan *backgroundScanPlan
	var engine *tagCountEngine
	occByObject := map[string][]TagOccurrence{}
	var lastReloadAt time.Time
	dirtyBatchesSinceReload := 0
	loadedGen := s.getSolutionGen(ctx, solutionID)

	if freshStart {
		if err = s.store.ClearComplianceTagCounts(ctx, solutionID); err != nil {
			return
		}
		if err = s.store.ClearComplianceTagObjectTags(ctx, solutionID); err != nil {
			return
		}
		if err = s.store.ClearComplianceDuplicateRows(ctx, solutionID); err != nil {
			return
		}
		plan = newBackgroundScanPlan(objectIDs)
		engine = newTagCountEngine()
		_ = s.store.UpdateComplianceDuplicateScannerProgress(ctx, solutionID, "full", 0, 0, plan.Total, "running", "")
	} else {
		mode := sc.Mode
		if mode != "full" && mode != "incremental" {
			mode = "full"
		}
		plan = resumeBackgroundScanPlan(objectIDs, mode, sc.CursorOffset, sc.Processed)
		engine, occByObject, err = s.restoreTagCountEngineFromStore(ctx, solutionID, sol, refByID, schemeNames, excluded, excludedTagPatterns)
		if err != nil {
			_ = s.store.UpdateComplianceDuplicateScannerProgress(ctx, solutionID, plan.Mode, plan.Cursor, plan.Processed, plan.Total, "error", err.Error())
			return
		}
		status := "running"
		if plan.Mode == "incremental" && (sc.Status == "completed" || sc.Status == "idle") {
			status = "completed"
		}
		_ = s.store.UpdateComplianceDuplicateScannerProgress(ctx, solutionID, plan.Mode, plan.Cursor, plan.Processed, plan.Total, status, "")
		slog.Info("runBackgroundScannerLoop resumed",
			"solution_id", solutionID,
			"mode", plan.Mode,
			"cursor", plan.Cursor,
			"processed", plan.Processed,
			"total", plan.Total,
		)
	}

	for {
		select {
		case <-ctx.Done():
			_ = s.store.UpdateComplianceDuplicateScannerProgress(ctx, solutionID, plan.Mode, plan.Cursor, plan.Processed, plan.Total, "idle", "")
			return
		default:
		}

		scNow, serr := s.store.GetComplianceDuplicateScanner(ctx, solutionID)
		if serr != nil || !scNow.Enabled {
			_ = s.store.UpdateComplianceDuplicateScannerProgress(ctx, solutionID, "idle", plan.Cursor, plan.Processed, plan.Total, "idle", "")
			return
		}

		if dirty, derr := s.store.PopComplianceDuplicateDirty(ctx, solutionID, plan.BatchSize); derr == nil {
			for _, d := range dirty {
				plan.DirtyQueue.enqueue(d.ObjectID, d.Priority, d.Reason)
			}
		}

		batch, fromPriority, fullDone := plan.nextBatch()
		if len(batch) == 0 {
			if plan.Mode == "incremental" {
				_ = s.store.UpdateComplianceDuplicateScannerProgress(ctx, solutionID, "incremental", plan.Cursor, plan.Processed, plan.Total, "completed", "")
				select {
				case <-ctx.Done():
					return
				case <-time.After(bgScanSleepBetweenBatches):
				}
				continue
			}
			if fullDone {
				plan.Mode = "incremental"
				_ = s.store.UpdateComplianceDuplicateScannerProgress(ctx, solutionID, "incremental", plan.Cursor, plan.Processed, plan.Total, "completed", "")
				continue
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(bgScanSleepBetweenBatches):
			}
			continue
		}

		if fromPriority || plan.Mode == "incremental" {
			dirtyBatchesSinceReload++
			currentGen := s.getSolutionGen(ctx, solutionID)
			needReload := lastReloadAt.IsZero() ||
				currentGen != loadedGen ||
				dirtyBatchesSinceReload >= bgReloadDebounceBatches ||
				time.Since(lastReloadAt) >= bgReloadDebounceInterval
			if needReload {
				sol, err = s.loadSolutionWithChanges(ctx, solutionID, sc.SessionID)
				if err != nil {
					_ = s.store.UpdateComplianceDuplicateScannerProgress(ctx, solutionID, plan.Mode, plan.Cursor, plan.Processed, plan.Total, "error", err.Error())
					return
				}
				refs = s.collectAllGraphicObjectRefs(sol)
				schemeNames = s.buildSchemeNames(ctx, solutionID, refs)
				refByID = map[string]tagEditObjectRef{}
				for _, ref := range refs {
					refByID[ref.obj.Id] = ref
				}
				lastReloadAt = time.Now()
				dirtyBatchesSinceReload = 0
				loadedGen = currentGen
			}
		}

		for _, objectID := range batch {
			select {
			case <-ctx.Done():
				return
			default:
			}
			ref, ok := refByID[objectID]
			var newOccs []TagOccurrence
			if ok {
				schemeName := schemeNames[ref.schemeXMLID]
				newOccs = collectTagOccurrencesForObject(sol, ref.schemeXMLID, schemeName, ref.obj, excluded, excludedTagPatterns)
			}
			oldOccs := occByObject[objectID]
			_, became, lost := engine.adjustObject(objectID, newOccs)
			occByObject[objectID] = newOccs

			if err = s.persistObjectTagAdjust(ctx, solutionID, objectID, engine, oldOccs, newOccs, became, lost, occByObject); err != nil {
				slog.Warn("persistObjectTagAdjust failed", "solution_id", solutionID, "object_id", objectID, "err", err)
			}
		}

		mode := plan.Mode
		status := "running"
		if mode == "incremental" && plan.DirtyQueue.len() == 0 {
			status = "completed"
		}
		_ = s.store.UpdateComplianceDuplicateScannerProgress(ctx, solutionID, mode, plan.Cursor, plan.Processed, plan.Total, status, "")

		select {
		case <-ctx.Done():
			return
		case <-time.After(bgScanSleepBetweenBatches):
		}
	}
}

func (s *SolutionService) persistObjectTagAdjust(
	ctx context.Context,
	solutionID uuid.UUID,
	objectID string,
	engine *tagCountEngine,
	oldOccs, newOccs []TagOccurrence,
	became, lost []string,
	occByObject map[string][]TagOccurrence,
) error {
	contrib := occurrenceTagContrib(newOccs)
	if err := s.store.ReplaceComplianceTagObjectTags(ctx, solutionID, objectID, contrib); err != nil {
		return err
	}

	affected := map[string]bool{}
	for tag := range occurrenceTagContrib(oldOccs) {
		affected[tag] = true
	}
	for tag := range contrib {
		affected[tag] = true
	}
	for _, tag := range became {
		affected[tag] = true
	}
	for _, tag := range lost {
		affected[tag] = true
	}

	for tag := range affected {
		count := engine.countFor(tag)
		if err := s.store.UpsertComplianceTagCount(ctx, solutionID, tag, count); err != nil {
			return err
		}
		if count < 2 {
			if err := s.store.DeleteComplianceDuplicateRowsForTag(ctx, solutionID, tag); err != nil {
				return err
			}
			continue
		}
		if err := s.store.DeleteComplianceDuplicateRowsForTag(ctx, solutionID, tag); err != nil {
			return err
		}
		for _, occs := range occByObject {
			for _, occ := range occs {
				if occ.Tag != tag {
					continue
				}
				row := complianceDuplicateRowFromOccurrence(solutionID, occ)
				if err := s.store.UpsertComplianceDuplicateRow(ctx, row); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *SolutionService) GetBackgroundDuplicateScanStatus(ctx context.Context, solutionID uuid.UUID) (BackgroundScanStatus, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("GetBackgroundDuplicateScanStatus done",
			"solution_id", solutionID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()

	sc, err := s.store.GetComplianceDuplicateScanner(ctx, solutionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = nil
			return BackgroundScanStatus{
				Enabled: false,
				Mode:    "idle",
				Status:  "idle",
			}, nil
		}
		return BackgroundScanStatus{}, err
	}

	found := 0
	if tags, terr := s.store.ListDuplicateTags(ctx, solutionID); terr == nil {
		found = len(tags)
	}

	percent := 0.0
	if sc.Total > 0 {
		percent = float64(sc.Processed) / float64(sc.Total) * 100
		if percent > 100 {
			percent = 100
		}
	}

	ruleID := ""
	if sc.RuleID != nil {
		ruleID = sc.RuleID.String()
	}

	return BackgroundScanStatus{
		Enabled:   sc.Enabled,
		Mode:      sc.Mode,
		Status:    sc.Status,
		Processed: sc.Processed,
		Total:     sc.Total,
		Percent:   percent,
		Found:     found,
		RuleID:    ruleID,
		Error:     sc.ErrorMessage,
	}, nil
}

func (s *SolutionService) GetBackgroundDuplicateScanResults(ctx context.Context, solutionID uuid.UUID) (DuplicateScanResult, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("GetBackgroundDuplicateScanResults done",
			"solution_id", solutionID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("GetBackgroundDuplicateScanResults start", "solution_id", solutionID)

	sc, err := s.store.GetComplianceDuplicateScanner(ctx, solutionID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return DuplicateScanResult{}, err
	}
	err = nil

	scannerRuleID := uuid.Nil
	if sc.RuleID != nil {
		scannerRuleID = *sc.RuleID
	}
	ruleID, rerr := s.resolveDuplicateResultsRuleID(ctx, solutionID, scannerRuleID)
	if rerr != nil || ruleID == uuid.Nil {
		dbRows, lerr := s.store.ListComplianceDuplicateRows(ctx, solutionID)
		if lerr != nil {
			err = lerr
			return DuplicateScanResult{}, err
		}
		duplicateTags, terr := s.store.ListDuplicateTags(ctx, solutionID)
		if terr != nil {
			err = terr
			return DuplicateScanResult{}, err
		}
		rows := make([]TagOccurrence, 0, len(dbRows))
		violations := map[string]bool{}
		for _, r := range dbRows {
			rows = append(rows, tagOccurrenceFromDuplicateRow(r))
			violations[r.ObjectID] = true
		}
		return DuplicateScanResult{
			ViolationsByObject: violations,
			DuplicateTags:      duplicateTags,
			Nodes:              []ComplianceNodeHint{},
			Rows:               rows,
		}, nil
	}

	return s.buildDuplicateScanResultFromStore(ctx, solutionID, ruleID)
}

func (s *SolutionService) triggerBackgroundFullRebuild(ctx context.Context, solutionID uuid.UUID) {
	start := time.Now()
	defer func() {
		slog.Info("triggerBackgroundFullRebuild done",
			"solution_id", solutionID,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}()
	slog.Info("triggerBackgroundFullRebuild start", "solution_id", solutionID)

	if !s.isBackgroundScanEnabled(ctx, solutionID) {
		return
	}
	sc, err := s.store.GetComplianceDuplicateScanner(ctx, solutionID)
	if err != nil || !sc.Enabled || sc.RuleID == nil {
		return
	}
	_ = s.store.ClearComplianceDuplicateDirty(ctx, solutionID)
	_ = s.store.ClearComplianceTagCounts(ctx, solutionID)
	_ = s.store.ClearComplianceTagObjectTags(ctx, solutionID)
	_ = s.store.ClearComplianceDuplicateRows(ctx, solutionID)
	_ = s.store.UpdateComplianceDuplicateScannerProgress(ctx, solutionID, "full", 0, 0, sc.Total, "running", "")
	s.startBackgroundScanner(solutionID)
}

func (s *SolutionService) EnqueueDuplicateDirtyObject(ctx context.Context, solutionID uuid.UUID, objectID string, priority int, reason string) {
	start := time.Now()
	defer func() {
		slog.Debug("EnqueueDuplicateDirtyObject done",
			"solution_id", solutionID,
			"object_id", objectID,
			"priority", priority,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}()
	if objectID == "" || !s.isBackgroundScanEnabled(ctx, solutionID) {
		return
	}
	if priority < bgDirtyPriorityNormal {
		priority = bgDirtyPriorityNormal
	}

	isDupRelated := false
	if tags, err := s.store.ListComplianceTagObjectTags(ctx, solutionID, objectID); err == nil {
		for tag := range tags {
			if ok, _ := s.store.IsDuplicateTag(ctx, solutionID, tag); ok {
				isDupRelated = true
				break
			}
		}
	}
	if !isDupRelated {
		if ok, err := s.store.ObjectHasComplianceDuplicateRows(ctx, solutionID, objectID); err == nil && ok {
			isDupRelated = true
		}
	}
	if isDupRelated && priority < bgDirtyPriorityHigh {
		priority = bgDirtyPriorityHigh
	}

	_ = s.store.EnqueueComplianceDuplicateDirty(ctx, solutionID, objectID, priority, reason)
}

func (s *SolutionService) enqueueDirtyAfterTagChange(ctx context.Context, solutionID uuid.UUID, objectID string, reason string) {
	s.EnqueueDuplicateDirtyObject(ctx, solutionID, objectID, bgDirtyPriorityNormal, reason)
}

func (s *SolutionService) resolveObjectIDFromEntity(sol domain.Solution, entityType, entityXMLID string) string {
	switch entityType {
	case "graphic_object":
		return entityXMLID
	case "control_command":
		for _, cmd := range sol.ControlCommands.ControlCommand {
			if cmd.Id != entityXMLID {
				continue
			}
			for _, project := range sol.Projects.Project {
				for _, scheme := range project.GraphicScheme {
					for _, obj := range scheme.GraphicObject {
						if matchesTarget(cmd.Target, obj) {
							return obj.Id
						}
					}
				}
			}
		}
	case "automation":
		for _, auto := range sol.Automations.Automation {
			if auto.Id != entityXMLID {
				continue
			}
			for _, project := range sol.Projects.Project {
				for _, scheme := range project.GraphicScheme {
					for _, obj := range scheme.GraphicObject {
						if matchesTarget(auto.Target, obj) {
							return obj.Id
						}
					}
				}
			}
		}
	}
	return entityXMLID
}

func bgScanThrottleConfig() (workers, batchSize int, sleep time.Duration) {
	return bgScanWorkers, bgScanBatchSize, bgScanSleepBetweenBatches
}

func forcedScanThrottleConfig() (workers, batchSize int, sleep time.Duration) {
	return forcedScanWorkers, forcedScanBatchSize, forcedScanSleepBetweenBatches
}

func ensureBackgroundExclusive(solutionID uuid.UUID, backgroundEnabled bool) error {
	if backgroundEnabled {
		return ErrBackgroundScanActive
	}
	key := solutionID.String()
	if _, loaded := duplicateScanJobLocks.Load(key); loaded {
		return ErrDuplicateScanActive
	}
	return nil
}
