package service

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"create_sheme/internal/cache"
	"create_sheme/internal/config"
	"create_sheme/internal/domain"
	"create_sheme/internal/importfile"
	"create_sheme/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrImportBusy = errors.New("import already in progress")
var ErrSaveBusy = errors.New("save already in progress")
var ErrSchemeXMLIDRequired = errors.New("scheme_xml_id required")

type SolutionService struct {
	store          *repository.Store
	cache          cache.Cache
	cacheCfg       config.Cache
	importSem      chan struct{}
	maxXMLBytes    int64
	progressMu     sync.RWMutex
	progress       map[uuid.UUID]*importProgressState
	saveProgressMu sync.RWMutex
	saveProgress   map[uuid.UUID]*saveProgressState
}

func NewSolutionService(store *repository.Store, maxXMLBytes int64) *SolutionService {
	return NewSolutionServiceWithCache(store, maxXMLBytes, cache.NopCache{}, defaultCacheConfig())
}

func NewSolutionServiceWithCache(store *repository.Store, maxXMLBytes int64, c cache.Cache, cacheCfg config.Cache) *SolutionService {
	if maxXMLBytes < 1 {
		maxXMLBytes = 1 << 30
	}
	if c == nil {
		c = cache.NopCache{}
	}
	if cacheCfg.MaxValueBytes < 1 {
		cacheCfg.MaxValueBytes = 2097152
	}
	if cacheCfg.SchemeTTL <= 0 {
		cacheCfg.SchemeTTL = 15 * time.Minute
	}
	if cacheCfg.SchemeObjectsTTL <= 0 {
		cacheCfg.SchemeObjectsTTL = 15 * time.Minute
	}
	if cacheCfg.ComplianceTTL <= 0 {
		cacheCfg.ComplianceTTL = 5 * time.Minute
	}
	return &SolutionService{
		store:        store,
		cache:        c,
		cacheCfg:     cacheCfg,
		importSem:    make(chan struct{}, 1),
		maxXMLBytes:  maxXMLBytes,
		progress:     map[uuid.UUID]*importProgressState{},
		saveProgress: map[uuid.UUID]*saveProgressState{},
	}
}

type TreeNode struct {
	ID       string     `json:"id"`
	XMLID    string     `json:"xml_id"`
	Type     string     `json:"type"`
	Name     string     `json:"name"`
	Tag      string     `json:"tag"`
	Meta     any        `json:"meta,omitempty"`
	Children []TreeNode `json:"children,omitempty"`
}

type TreeOptions struct {
	Depth        string
	ProjectXMLID string
}

func (s *SolutionService) BuildTree(ctx context.Context, solutionID uuid.UUID, opts TreeOptions) ([]TreeNode, error) {
	start := time.Now()
	defer func() {
		slog.Info("BuildTree done", "solution_id", solutionID, "depth", opts.Depth, "project_xml_id", opts.ProjectXMLID, "duration_ms", time.Since(start).Milliseconds())
	}()

	if opts.ProjectXMLID != "" {
		return s.buildProjectSchemes(ctx, solutionID, opts.ProjectXMLID)
	}

	nodes, err := s.store.ListNodes(ctx, solutionID)
	if err != nil {
		return nil, err
	}
	byParent := map[string][]repository.NodeRow{}
	var roots []repository.NodeRow
	for _, n := range nodes {
		if n.ParentID == nil {
			roots = append(roots, n)
			continue
		}
		pid := n.ParentID.String()
		byParent[pid] = append(byParent[pid], n)
	}

	skeleton := opts.Depth == "skeleton"

	var build func(repository.NodeRow) TreeNode
	build = func(n repository.NodeRow) TreeNode {
		tn := TreeNode{
			ID: n.ID.String(), XMLID: n.XMLID, Type: n.NodeType, Name: n.Name, Tag: n.Tag,
		}
		if len(n.Meta) > 0 && string(n.Meta) != "{}" && string(n.Meta) != "null" {
			var meta any
			_ = json.Unmarshal(n.Meta, &meta)
			tn.Meta = meta
		}
		if skeleton && n.NodeType == "project" {
			return tn
		}
		for _, child := range byParent[n.ID.String()] {
			if skeleton && child.NodeType == "graphic_scheme" {
				continue
			}
			tn.Children = append(tn.Children, build(child))
		}
		return tn
	}
	var result []TreeNode
	for _, r := range roots {
		result = append(result, build(r))
	}
	return result, nil
}

func (s *SolutionService) buildProjectSchemes(ctx context.Context, solutionID uuid.UUID, projectXMLID string) ([]TreeNode, error) {
	rows, err := s.store.ListSchemeNodesByProject(ctx, solutionID, projectXMLID)
	if err != nil {
		return nil, err
	}
	var result []TreeNode
	for _, n := range rows {
		tn := TreeNode{
			ID: n.ID.String(), XMLID: n.XMLID, Type: n.NodeType, Name: n.Name, Tag: n.Tag,
		}
		if len(n.Meta) > 0 && string(n.Meta) != "{}" && string(n.Meta) != "null" {
			var meta any
			_ = json.Unmarshal(n.Meta, &meta)
			tn.Meta = meta
		}
		result = append(result, tn)
	}
	return result, nil
}

func (s *SolutionService) ListSolutionTemplates(ctx context.Context, solutionID uuid.UUID) ([]repository.SolutionTemplateRow, error) {
	start := time.Now()
	defer func() {
		slog.Info("ListSolutionTemplates done", "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds())
	}()

	templates, err := s.store.ListSolutionTemplates(ctx, solutionID)
	if err != nil {
		return nil, err
	}
	if len(templates) > 0 {
		return templates, nil
	}

	schemes, err := s.store.ListSchemes(ctx, solutionID)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, sch := range schemes {
		var scheme domain.GraphicScheme
		if err := json.Unmarshal(sch.Payload, &scheme); err != nil {
			continue
		}
		for _, obj := range scheme.GraphicObject {
			tmpl := obj.TechnologyTemplate()
			if tmpl != "" {
				counts[tmpl]++
			}
		}
	}
	for tmpl, count := range counts {
		_ = s.store.UpsertSolutionTemplate(ctx, solutionID, tmpl, count)
	}
	return s.store.ListSolutionTemplates(ctx, solutionID)
}

type ImportResult struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

func (s *SolutionService) TryAcquireImportSlot() bool {
	select {
	case s.importSem <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *SolutionService) ReleaseImportSlot() {
	select {
	case <-s.importSem:
	default:
	}
}

func (s *SolutionService) BeginImport(ctx context.Context, nameHint string) (result ImportResult, err error) {
	start := time.Now()
	userID := UserIDFromContext(ctx)
	defer func() {
		slog.Info("BeginImport done",
			"duration_ms", time.Since(start).Milliseconds(),
			"user_id", userID,
			"solution_id", result.ID,
			"err", err,
		)
	}()
	slog.Debug("BeginImport start", "user_id", userID, "name_hint", nameHint)

	id := uuid.New()
	row := repository.SolutionRow{
		ID:           id,
		Name:         nameHint,
		Data:         json.RawMessage(`{}`),
		ImportStatus: "importing",
		ImportError:  "",
		IsDirty:      false,
	}
	if err = s.store.CreateSolution(ctx, row); err != nil {
		return ImportResult{}, err
	}
	if err = s.store.LinkSolutionToUser(ctx, userID, id, "owner"); err != nil {
		_ = s.store.DeleteSolution(ctx, id)
		return ImportResult{}, err
	}
	return ImportResult{ID: id.String(), Name: nameHint, Status: "importing"}, nil
}

func (s *SolutionService) FinishImportFromFile(ctx context.Context, solutionID uuid.UUID, path string) (err error) {
	start := time.Now()
	var xmlBytes int64
	var format string
	defer func() {
		if err != nil {
			s.clearImportProgress(solutionID)
		}
		slog.Info("FinishImportFromFile done",
			"duration_ms", time.Since(start).Milliseconds(),
			"solution_id", solutionID,
			"format", format,
			"xml_bytes", xmlBytes,
			"err", err,
		)
	}()
	slog.Debug("FinishImportFromFile start", "solution_id", solutionID, "path", path)

	info, statErr := os.Stat(path)
	if statErr != nil {
		err = statErr
		return err
	}
	slog.Info("import bytes", "solution_id", solutionID, "bytes", info.Size())

	sniffed, sniffErr := importfile.SniffFormat(path)
	if sniffErr != nil {
		err = sniffErr
		return err
	}
	format = sniffed
	if format == importfile.FormatXML {
		s.beginImportProgress(solutionID, ImportPhaseParse)
	} else {
		s.beginImportProgress(solutionID, ImportPhaseUnpack)
	}

	data, resolvedFormat, resolveErr := importfile.ResolveImportXML(path, importfile.Limits{
		MaxXMLBytes: s.maxXMLBytes,
	}, func(processed, total int64) {
		s.reportImportProgress(solutionID, ImportPhaseUnpack, int(processed), int(total), false)
	})
	format = resolvedFormat
	if resolveErr != nil {
		err = resolveErr
		return err
	}
	xmlBytes = int64(len(data))
	if format == importfile.FormatZIP {
		s.reportImportProgress(solutionID, ImportPhaseUnpack, int(xmlBytes), int(xmlBytes), true)
	}
	return s.finishImportData(ctx, solutionID, data)
}

func (s *SolutionService) MarkImportFailed(ctx context.Context, solutionID uuid.UUID, msg string) (err error) {
	start := time.Now()
	defer func() {
		s.clearImportProgress(solutionID)
		slog.Info("MarkImportFailed done",
			"duration_ms", time.Since(start).Milliseconds(),
			"solution_id", solutionID,
			"err", err,
		)
	}()
	slog.Debug("MarkImportFailed start", "solution_id", solutionID, "message", msg)
	_ = s.store.ClearSolutionContent(ctx, solutionID)
	err = s.store.UpdateImportStatus(ctx, solutionID, "failed", msg)
	return err
}

func (s *SolutionService) finishImportData(ctx context.Context, id uuid.UUID, data []byte) (err error) {
	start := time.Now()
	defer func() {
		if err != nil {
			s.clearImportProgress(id)
		}
		slog.Info("finishImportData done",
			"duration_ms", time.Since(start).Milliseconds(),
			"solution_id", id,
			"bytes", len(data),
			"err", err,
		)
	}()

	if _, ok := s.ImportProgress(id); !ok {
		s.beginImportProgress(id, ImportPhaseParse)
	}

	totalBytes := len(data)
	s.reportImportProgress(id, ImportPhaseParse, 0, totalBytes, true)

	slog.Info("import parse_start", "solution_id", id, "bytes", totalBytes)
	parseStart := time.Now()
	var sol domain.Solution
	cr := &countingReader{
		r:     bytes.NewReader(data),
		total: int64(totalBytes),
		onProgress: func(processed, total int64) {
			s.reportImportProgress(id, ImportPhaseParse, int(processed), int(total), false)
		},
	}
	if err = xml.NewDecoder(cr).Decode(&sol); err != nil {
		slog.Info("import parse_fail",
			"solution_id", id,
			"duration_ms", time.Since(parseStart).Milliseconds(),
			"bytes", totalBytes,
			"err", err,
		)
		return fmt.Errorf("parse xml: %w", err)
	}
	s.reportImportProgress(id, ImportPhaseParse, totalBytes, totalBytes, true)
	slog.Info("import parse_done",
		"solution_id", id,
		"duration_ms", time.Since(parseStart).Milliseconds(),
		"bytes", totalBytes,
		"name", sol.Name,
	)

	schemeTotal := 0
	for _, project := range sol.Projects.Project {
		schemeTotal += len(project.GraphicScheme)
	}
	if schemeTotal < 1 {
		schemeTotal = 1
	}
	s.reportImportProgress(id, ImportPhasePersist, 0, schemeTotal, true)

	if err = s.store.ClearSolutionContent(ctx, id); err != nil {
		return err
	}
	if err = s.store.UpdateSolutionImportMeta(ctx, id, sol.Name, sol.Company, sol.Description, sol.Id); err != nil {
		return err
	}

	rootNodeID := uuid.New()
	if err = s.store.InsertNode(ctx, repository.NodeRow{
		ID: rootNodeID, SolutionID: id, ParentID: nil, NodeType: "solution",
		XMLID: sol.Id, Name: sol.Name, Tag: sol.Tag, SortIndex: 0, Meta: mustJSON(nil),
	}); err != nil {
		return err
	}

	projectsID := uuid.New()
	if err = s.store.InsertNode(ctx, repository.NodeRow{
		ID: projectsID, SolutionID: id, ParentID: &rootNodeID, NodeType: "folder",
		XMLID: "projects", Name: "Projects", SortIndex: 0, Meta: mustJSON(nil),
	}); err != nil {
		return err
	}

	sortIdx := 0
	schemeProcessed := 0
	templateCounts := map[string]int{}
	for pi := range sol.Projects.Project {
		project := &sol.Projects.Project[pi]
		projectID := uuid.New()
		meta := mustJSON(map[string]string{"description": project.Description})
		if err = s.store.InsertNode(ctx, repository.NodeRow{
			ID: projectID, SolutionID: id, ParentID: &projectsID, NodeType: "project",
			XMLID: project.Id, Name: project.Name, Tag: project.Tag, SortIndex: sortIdx, Meta: meta,
		}); err != nil {
			return err
		}
		sortIdx++

		for si := range project.GraphicScheme {
			scheme := &project.GraphicScheme[si]
			for _, obj := range scheme.GraphicObject {
				tmpl := obj.TechnologyTemplate()
				if tmpl != "" {
					templateCounts[tmpl]++
				}
			}
			schemeID := uuid.New()
			payload, marshalErr := json.Marshal(scheme)
			if marshalErr != nil {
				err = marshalErr
				return err
			}
			if err = checkSchemePayloadSize(scheme.Id, payload); err != nil {
				return err
			}
			if err = s.store.InsertScheme(ctx, repository.SchemeRow{
				ID: schemeID, SolutionID: id, XMLID: scheme.Id, ProjectXMLID: project.Id,
				Name: scheme.Name, Template: scheme.Template, Payload: payload,
			}); err != nil {
				return err
			}
			schMeta := mustJSON(map[string]string{"template": scheme.Template, "project_id": project.Id})
			if err = s.store.InsertNode(ctx, repository.NodeRow{
				ID: uuid.New(), SolutionID: id, ParentID: &projectID, NodeType: "graphic_scheme",
				XMLID: scheme.Id, Name: scheme.Name, Tag: scheme.Tag, SortIndex: si, Meta: schMeta,
			}); err != nil {
				return err
			}
			schemeProcessed++
			s.reportImportProgress(id, ImportPhasePersist, schemeProcessed, schemeTotal, false)
		}
	}
	if schemeProcessed == 0 {
		s.reportImportProgress(id, ImportPhasePersist, schemeTotal, schemeTotal, true)
	} else {
		s.reportImportProgress(id, ImportPhasePersist, schemeProcessed, schemeTotal, true)
	}

	solData, marshalErr := marshalLeanSolution(sol)
	if marshalErr != nil {
		err = fmt.Errorf("marshal solution data: %w", marshalErr)
		return err
	}
	if err = s.store.UpdateSolutionData(ctx, id, solData, false); err != nil {
		return err
	}

	for tmpl, count := range templateCounts {
		if err = s.store.UpsertSolutionTemplate(ctx, id, tmpl, count); err != nil {
			return err
		}
	}

	indexTotal := 3
	s.reportImportProgress(id, ImportPhaseIndex, 0, indexTotal, true)

	if err = s.persistSolutionMetadata(ctx, id, SolutionMetadata{
		Automations: sol.Automations.Automation,
		Commands:    sol.ControlCommands.ControlCommand,
	}); err != nil {
		return err
	}
	s.reportImportProgress(id, ImportPhaseIndex, 1, indexTotal, true)

	if err = s.buildAndPersistSolutionIndexes(ctx, id, sol); err != nil {
		return err
	}
	s.reportImportProgress(id, ImportPhaseIndex, 2, indexTotal, true)
	sol.StripSchemeGraphicObjects()

	workspacesID := uuid.New()
	if len(sol.Workspaces.Workspace) > 0 {
		if err = s.store.InsertNode(ctx, repository.NodeRow{
			ID: workspacesID, SolutionID: id, ParentID: &rootNodeID, NodeType: "folder",
			XMLID: "workspaces", Name: "Workspaces", SortIndex: 1, Meta: mustJSON(nil),
		}); err != nil {
			return err
		}
		for wi, ws := range sol.Workspaces.Workspace {
			wsID := uuid.New()
			if err = s.store.InsertNode(ctx, repository.NodeRow{
				ID: wsID, SolutionID: id, ParentID: &workspacesID, NodeType: "workspace",
				XMLID: ws.Id, Name: ws.Name, Tag: ws.Tag, SortIndex: wi, Meta: mustJSON(nil),
			}); err != nil {
				return err
			}
			for ri, ref := range ws.ObjectReferences.ObjectReference {
				if err = s.store.InsertNode(ctx, repository.NodeRow{
					ID: uuid.New(), SolutionID: id, ParentID: &wsID, NodeType: "object_reference",
					XMLID: ref.Id, Name: ref.Name, Tag: ref.Tag, SortIndex: ri,
					Meta: mustJSON(map[string]string{"target": ref.Target}),
				}); err != nil {
					return err
				}
			}
		}
	}
	s.reportImportProgress(id, ImportPhaseIndex, indexTotal, indexTotal, true)

	if err = s.store.UpdateImportStatus(ctx, id, "ready", ""); err != nil {
		return err
	}
	s.clearImportProgress(id)
	s.BumpSolutionGen(ctx, id)
	return nil
}

type countingReader struct {
	r          io.Reader
	n          int64
	total      int64
	onProgress func(processed, total int64)
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		c.n += int64(n)
		if c.onProgress != nil {
			c.onProgress(c.n, c.total)
		}
	}
	return n, err
}

func (s *SolutionService) ImportXML(ctx context.Context, data []byte) (result ImportResult, err error) {
	start := time.Now()
	userID := UserIDFromContext(ctx)
	defer func() {
		slog.Info("ImportXML done", "duration_ms", time.Since(start).Milliseconds(), "user_id", userID, "err", err)
	}()

	result, err = s.BeginImport(ctx, "")
	if err != nil {
		return ImportResult{}, err
	}
	id, parseErr := uuid.Parse(result.ID)
	if parseErr != nil {
		return ImportResult{}, parseErr
	}
	if err = s.finishImportData(ctx, id, data); err != nil {
		_ = s.MarkImportFailed(ctx, id, err.Error())
		return ImportResult{}, err
	}
	meta, metaErr := s.store.GetSolutionMeta(ctx, id)
	if metaErr == nil {
		result.Name = meta.Name
	}
	result.Status = "ready"
	return result, nil
}

func mustJSON(v any) json.RawMessage {
	if v == nil {
		return json.RawMessage(`{}`)
	}
	b, err := json.Marshal(v)
	if err != nil || len(b) == 0 {
		return json.RawMessage(`{}`)
	}
	return b
}

func (s *SolutionService) GetScheme(ctx context.Context, solutionID uuid.UUID, schemeXMLID string, sessionID *uuid.UUID) (json.RawMessage, error) {
	start := time.Now()
	defer func() {
		slog.Info("GetScheme done", "scheme_xml_id", schemeXMLID, "duration_ms", time.Since(start).Milliseconds())
	}()

	baseJSON, err := s.getSchemeBaseJSON(ctx, solutionID, schemeXMLID)
	if err != nil {
		return nil, err
	}

	if sessionID == nil {
		return baseJSON, nil
	}

	var scheme domain.GraphicScheme
	if err := json.Unmarshal(baseJSON, &scheme); err != nil {
		return nil, err
	}
	changes, err := s.store.ListDraftChanges(ctx, *sessionID)
	if err == nil {
		applyChangesToScheme(&scheme, changes)
	}
	return json.Marshal(scheme)
}

func (s *SolutionService) getSchemeBaseJSON(ctx context.Context, solutionID uuid.UUID, schemeXMLID string) (json.RawMessage, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Debug("getSchemeBaseJSON done", "solution_id", solutionID, "scheme_xml_id", schemeXMLID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()

	gen := s.getSolutionGen(ctx, solutionID)
	key := cache.SchemeKey(solutionID, gen, schemeXMLID)
	if baseJSON, hit := s.cacheGet(ctx, key, "scheme", solutionID); hit {
		return baseJSON, nil
	}
	baseJSON, err := s.loadSchemeBaseJSON(ctx, solutionID, schemeXMLID)
	if err != nil {
		return nil, err
	}
	s.cacheSet(ctx, key, baseJSON, s.cacheCfg.SchemeTTL, "scheme", solutionID)
	return baseJSON, nil
}

func (s *SolutionService) loadSchemeBaseJSON(ctx context.Context, solutionID uuid.UUID, schemeXMLID string) (json.RawMessage, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Debug("loadSchemeBaseJSON done", "solution_id", solutionID, "scheme_xml_id", schemeXMLID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()

	sch, err := s.store.GetScheme(ctx, solutionID, schemeXMLID)
	if err != nil {
		return nil, err
	}
	var scheme domain.GraphicScheme
	if err = json.Unmarshal(sch.Payload, &scheme); err != nil {
		return nil, err
	}
	working, wErr := s.store.ListWorkingChangesForScheme(ctx, solutionID, schemeXMLID)
	if wErr == nil {
		applyChangesToScheme(&scheme, working)
	}
	var result json.RawMessage
	result, err = json.Marshal(scheme)
	return result, err
}

func applyChangesToScheme(scheme *domain.GraphicScheme, changes []repository.ChangeRow) {
	for _, ch := range changes {
		if ch.SchemeXMLID != scheme.Id {
			continue
		}
		if ch.EntityType == "graphic_object" {
			for i := range scheme.GraphicObject {
				if scheme.GraphicObject[i].Id == ch.EntityXMLID {
					_ = json.Unmarshal(ch.Patch, &scheme.GraphicObject[i])
				}
			}
		}
	}
}

type ObjectDetails struct {
	GraphicObject    domain.GraphicObject     `json:"graphic_object"`
	TechnologyObject []domain.TechnologyObject `json:"technology_objects"`
	Automations      []domain.Automation      `json:"automations"`
	Commands         []domain.ControlCommand  `json:"commands"`
}

func (s *SolutionService) GetObjectDetails(ctx context.Context, solutionID uuid.UUID, graphicObjectID string, sessionID *uuid.UUID) (ObjectDetails, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("GetObjectDetails done", "object_id", graphicObjectID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	slog.Debug("GetObjectDetails start", "solution_id", solutionID, "object_id", graphicObjectID)

	if err = s.ensureSolutionIndexes(ctx, solutionID); err != nil {
		return ObjectDetails{}, err
	}

	meta, err := s.loadSolutionMetadata(ctx, solutionID)
	if err != nil {
		return ObjectDetails{}, err
	}
	metaChanges, _ := s.store.ListMetadataWorkingChanges(ctx, solutionID)
	applyMetadataChanges(&meta, metaChanges)
	if sessionID != nil {
		drafts, draftErr := s.store.ListDraftChanges(ctx, *sessionID)
		if draftErr == nil {
			applyMetadataChanges(&meta, drafts)
		}
	}

	row, err := s.store.GetSolutionObject(ctx, solutionID, graphicObjectID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return s.getObjectDetailsLegacyScan(ctx, solutionID, graphicObjectID, meta, sessionID)
		}
		return ObjectDetails{}, err
	}

	sch, err := s.store.GetScheme(ctx, solutionID, row.SchemeXMLID)
	if err != nil {
		return ObjectDetails{}, err
	}
	var scheme domain.GraphicScheme
	if err = json.Unmarshal(sch.Payload, &scheme); err != nil {
		return ObjectDetails{}, err
	}
	working, _ := s.store.ListWorkingChangesForScheme(ctx, solutionID, row.SchemeXMLID)
	applyChangesToScheme(&scheme, working)
	if sessionID != nil {
		drafts, draftErr := s.store.ListDraftChanges(ctx, *sessionID)
		if draftErr == nil {
			applyChangesToScheme(&scheme, drafts)
		}
	}

	var obj domain.GraphicObject
	found := false
	for _, o := range scheme.GraphicObject {
		if o.Id == graphicObjectID {
			obj = o
			found = true
			break
		}
	}
	if !found {
		err = fmt.Errorf("object not found: %s", graphicObjectID)
		return ObjectDetails{}, err
	}

	keys := objectTargetKeys(obj)
	refs, _ := s.store.ListCommandTargetsByKeys(ctx, solutionID, keys)
	return buildObjectDetailsFromMetadataAndTargets(meta, obj, refs), nil
}

func (s *SolutionService) getObjectDetailsLegacyScan(ctx context.Context, solutionID uuid.UUID, graphicObjectID string, meta SolutionMetadata, sessionID *uuid.UUID) (ObjectDetails, error) {
	schemes, err := s.store.ListSchemes(ctx, solutionID)
	if err != nil {
		return ObjectDetails{}, err
	}
	for _, sch := range schemes {
		var scheme domain.GraphicScheme
		if err := json.Unmarshal(sch.Payload, &scheme); err != nil {
			continue
		}
		working, _ := s.store.ListWorkingChangesForScheme(ctx, solutionID, sch.XMLID)
		applyChangesToScheme(&scheme, working)
		if sessionID != nil {
			drafts, draftErr := s.store.ListDraftChanges(ctx, *sessionID)
			if draftErr == nil {
				applyChangesToScheme(&scheme, drafts)
			}
		}
		for _, obj := range scheme.GraphicObject {
			if obj.Id == graphicObjectID {
				return buildObjectDetailsFromMetadata(meta, obj), nil
			}
		}
	}
	return ObjectDetails{}, fmt.Errorf("object not found: %s", graphicObjectID)
}

func (s *SolutionService) GetSchemeObjectsDetails(ctx context.Context, solutionID uuid.UUID, schemeXMLID string, sessionID *uuid.UUID) ([]ObjectDetails, error) {
	start := time.Now()
	defer func() {
		slog.Info("GetSchemeObjectsDetails done", "solution_id", solutionID, "scheme_xml_id", schemeXMLID, "duration_ms", time.Since(start).Milliseconds())
	}()

	if sessionID == nil {
		gen := s.getSolutionGen(ctx, solutionID)
		key := cache.SchemeObjectsKey(solutionID, gen, schemeXMLID)
		if raw, hit := s.cacheGet(ctx, key, "scheme_objects", solutionID); hit {
			var cached []ObjectDetails
			if err := json.Unmarshal(raw, &cached); err == nil {
				return cached, nil
			}
		}
		result, err := s.buildSchemeObjectsDetails(ctx, solutionID, schemeXMLID, nil)
		if err != nil {
			return nil, err
		}
		if raw, err := json.Marshal(result); err == nil {
			s.cacheSet(ctx, key, raw, s.cacheCfg.SchemeObjectsTTL, "scheme_objects", solutionID)
		}
		return result, nil
	}

	return s.buildSchemeObjectsDetails(ctx, solutionID, schemeXMLID, sessionID)
}

func (s *SolutionService) buildSchemeObjectsDetails(ctx context.Context, solutionID uuid.UUID, schemeXMLID string, sessionID *uuid.UUID) ([]ObjectDetails, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Debug("buildSchemeObjectsDetails done", "solution_id", solutionID, "scheme_xml_id", schemeXMLID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()

	if err = s.ensureSolutionIndexes(ctx, solutionID); err != nil {
		return nil, err
	}

	meta, err := s.loadSolutionMetadata(ctx, solutionID)
	if err != nil {
		return nil, err
	}
	metaChanges, _ := s.store.ListMetadataWorkingChanges(ctx, solutionID)
	applyMetadataChanges(&meta, metaChanges)

	baseJSON, loadErr := s.getSchemeBaseJSON(ctx, solutionID, schemeXMLID)
	if loadErr != nil {
		err = loadErr
		return nil, err
	}
	var scheme domain.GraphicScheme
	if err = json.Unmarshal(baseJSON, &scheme); err != nil {
		return nil, err
	}

	if sessionID != nil {
		drafts, _ := s.store.ListDraftChanges(ctx, *sessionID)
		applyChangesToScheme(&scheme, drafts)
	}

	indexRows, _ := s.store.ListSolutionObjectsByScheme(ctx, solutionID, schemeXMLID)
	indexByID := map[string]repository.SolutionObjectRow{}
	for _, row := range indexRows {
		indexByID[row.GraphicObjectID] = row
	}

	result := make([]ObjectDetails, 0, len(scheme.GraphicObject))
	for _, obj := range scheme.GraphicObject {
		keys := objectTargetKeys(obj)
		var refs []repository.CommandTargetRef
		if _, ok := indexByID[obj.Id]; ok {
			refs, _ = s.store.ListCommandTargetsByKeys(ctx, solutionID, keys)
		} else {
			refs = nil
		}
		result = append(result, buildObjectDetailsFromMetadataAndTargets(meta, obj, refs))
	}
	return result, nil
}

func matchesTarget(target string, obj domain.GraphicObject) bool {
	if target == "" {
		return false
	}
	for _, key := range objectTargetKeys(obj) {
		if key == target {
			return true
		}
	}
	return false
}

func objectTargetKeys(obj domain.GraphicObject) []string {
	var keys []string
	if obj.Id != "" {
		keys = append(keys, obj.Id)
	}
	if obj.Tag != "" {
		keys = append(keys, obj.Tag)
	}
	for _, to := range obj.TechnologyObject {
		if to.Id != "" {
			keys = append(keys, to.Id)
		}
		if to.Tag != "" {
			keys = append(keys, to.Tag)
		}
		if to.ExternalCode != "" {
			keys = append(keys, to.ExternalCode)
		}
	}
	return keys
}

func buildTargetKeysForTemplate(sol domain.Solution, objectTemplate string) map[string]struct{} {
	keys := map[string]struct{}{}
	for _, project := range sol.Projects.Project {
		for _, scheme := range project.GraphicScheme {
			for _, obj := range scheme.GraphicObject {
				if obj.TechnologyTemplate() != objectTemplate {
					continue
				}
				for _, key := range objectTargetKeys(obj) {
					keys[key] = struct{}{}
				}
			}
		}
	}
	return keys
}

func (s *SolutionService) loadSolution(ctx context.Context, solutionID uuid.UUID) (domain.Solution, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Debug("loadSolution done", "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	slog.Debug("loadSolution start", "solution_id", solutionID)

	raw, err := s.store.GetSolutionData(ctx, solutionID)
	if err != nil {
		return domain.Solution{}, err
	}
	var sol domain.Solution
	if err = json.Unmarshal(raw, &sol); err != nil {
		return domain.Solution{}, err
	}
	schemes, listErr := s.store.ListSchemes(ctx, solutionID)
	if listErr != nil {
		err = listErr
		return domain.Solution{}, err
	}
	if err = hydrateSolutionSchemes(&sol, schemes); err != nil {
		return domain.Solution{}, err
	}
	return sol, nil
}

func (s *SolutionService) CreateSession(ctx context.Context, solutionID uuid.UUID) (uuid.UUID, error) {
	id := uuid.New()
	return id, s.store.CreateSession(ctx, id, solutionID)
}

func (s *SolutionService) OpenScheme(ctx context.Context, sessionID uuid.UUID, schemeXMLID string) (err error) {
	start := time.Now()
	var solutionID uuid.UUID
	defer func() {
		slog.Info("OpenScheme done",
			"session_id", sessionID,
			"solution_id", solutionID,
			"scheme_xml_id", schemeXMLID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("OpenScheme start", "session_id", sessionID, "scheme_xml_id", schemeXMLID)

	solutionID, _, err = s.store.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if err = s.store.FlushDraftToWorking(ctx, solutionID, sessionID); err != nil {
		return err
	}
	s.BumpSolutionGen(ctx, solutionID)
	if err = s.store.ClearDraftChangesExceptScheme(ctx, sessionID, schemeXMLID); err != nil {
		return err
	}
	if err = s.store.SetActiveScheme(ctx, sessionID, schemeXMLID); err != nil {
		return err
	}
	return nil
}

func (s *SolutionService) flushAllSessionDrafts(ctx context.Context, solutionID uuid.UUID) (err error) {
	start := time.Now()
	defer func() {
		slog.Info("flushAllSessionDrafts done",
			"solution_id", solutionID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("flushAllSessionDrafts start", "solution_id", solutionID)

	sessionIDs, err := s.store.ListSessionIDs(ctx, solutionID)
	if err != nil {
		return err
	}
	for _, sid := range sessionIDs {
		if flushErr := s.store.FlushDraftToWorking(ctx, solutionID, sid); flushErr != nil {
			err = flushErr
			return err
		}
	}
	return nil
}

func (s *SolutionService) CloseScheme(ctx context.Context, sessionID uuid.UUID) error {
	start := time.Now()
	var err error
	var solutionID uuid.UUID
	defer func() {
		slog.Info("CloseScheme done", "session_id", sessionID, "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	solutionID, _, err = s.store.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if err = s.store.FlushDraftToWorking(ctx, solutionID, sessionID); err != nil {
		return err
	}
	if err = s.store.SetActiveScheme(ctx, sessionID, ""); err != nil {
		return err
	}
	s.BumpSolutionGen(ctx, solutionID)
	return nil
}

func (s *SolutionService) PatchObject(ctx context.Context, sessionID uuid.UUID, schemeXMLID, entityType, entityXMLID string, patch json.RawMessage) error {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("PatchObject done", "session_id", sessionID, "entity_type", entityType, "entity_xml_id", entityXMLID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	if schemeXMLID == "" {
		err = ErrSchemeXMLIDRequired
		return err
	}
	err = s.store.UpsertDraftChange(ctx, uuid.New(), sessionID, schemeXMLID, entityType, entityXMLID, patch)
	if err != nil {
		return err
	}
	solutionID, _, serr := s.store.GetSession(ctx, sessionID)
	if serr == nil {
		objectID := entityXMLID
		if entityType != "graphic_object" {
			if sol, lerr := s.loadSolutionWithChanges(ctx, solutionID, &sessionID); lerr == nil {
				objectID = s.resolveObjectIDFromEntity(sol, entityType, entityXMLID)
			}
		}
		s.enqueueDirtyAfterTagChange(ctx, solutionID, objectID, "draft_patch")
	}
	return nil
}

func (s *SolutionService) Save(ctx context.Context, solutionID uuid.UUID) (err error) {
	start := time.Now()
	defer func() {
		slog.Info("Save done",
			"solution_id", solutionID,
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("Save start", "solution_id", solutionID)

	if !s.beginSaveProgress(solutionID) {
		return ErrSaveBusy
	}
	defer s.clearSaveProgress(solutionID)

	s.reportSaveProgress(solutionID, SavePhaseLoad, 0, 1, true)
	if err = s.flushAllSessionDrafts(ctx, solutionID); err != nil {
		return err
	}
	sol, err := s.loadSolution(ctx, solutionID)
	if err != nil {
		return err
	}
	changes, err := s.store.ListWorkingChanges(ctx, solutionID)
	if err != nil {
		return err
	}
	s.reportSaveProgress(solutionID, SavePhaseLoad, 1, 1, true)

	changeTotal := len(changes)
	s.reportSaveProgress(solutionID, SavePhaseApply, 0, changeTotal, true)
	for i, ch := range changes {
		applyOneChange(&sol, ch)
		s.reportSaveProgress(solutionID, SavePhaseApply, i+1, changeTotal, false)
	}
	if changeTotal > 0 {
		s.reportSaveProgress(solutionID, SavePhaseApply, changeTotal, changeTotal, true)
	}

	s.reportSaveProgress(solutionID, SavePhaseMetadata, 0, 1, true)
	if err = s.persistSolutionMetadata(ctx, solutionID, SolutionMetadata{
		Automations: sol.Automations.Automation,
		Commands:    sol.ControlCommands.ControlCommand,
	}); err != nil {
		return err
	}
	s.reportSaveProgress(solutionID, SavePhaseMetadata, 1, 1, true)

	s.reportSaveProgress(solutionID, SavePhaseIndex, 0, 1, true)
	if err = s.buildAndPersistSolutionIndexes(ctx, solutionID, sol); err != nil {
		return err
	}
	s.reportSaveProgress(solutionID, SavePhaseIndex, 1, 1, true)

	affectedSchemeIDs := map[string]struct{}{}
	for _, ch := range changes {
		if ch.EntityType == "graphic_object" && ch.SchemeXMLID != "" {
			affectedSchemeIDs[ch.SchemeXMLID] = struct{}{}
		}
	}
	schemeTotal := len(affectedSchemeIDs)
	s.reportSaveProgress(solutionID, SavePhaseSchemes, 0, schemeTotal, true)
	schemeDone := 0
	for pi := range sol.Projects.Project {
		for si := range sol.Projects.Project[pi].GraphicScheme {
			scheme := sol.Projects.Project[pi].GraphicScheme[si]
			if _, ok := affectedSchemeIDs[scheme.Id]; !ok {
				continue
			}
			updated, marshalErr := json.Marshal(scheme)
			if marshalErr != nil {
				err = marshalErr
				return err
			}
			if err = checkSchemePayloadSize(scheme.Id, updated); err != nil {
				return err
			}
			if err = s.store.UpdateSchemePayload(ctx, solutionID, scheme.Id, updated); err != nil {
				return err
			}
			schemeDone++
			s.reportSaveProgress(solutionID, SavePhaseSchemes, schemeDone, schemeTotal, false)
		}
	}
	if schemeTotal > 0 {
		s.reportSaveProgress(solutionID, SavePhaseSchemes, schemeTotal, schemeTotal, true)
	}

	s.reportSaveProgress(solutionID, SavePhaseMarshal, 0, 1, true)
	out, err := marshalLeanSolution(sol)
	if err != nil {
		return err
	}
	s.reportSaveProgress(solutionID, SavePhaseMarshal, 1, 1, true)

	s.reportSaveProgress(solutionID, SavePhasePersistXML, 0, 1, true)
	if err = s.store.UpdateSolutionData(ctx, solutionID, out, false); err != nil {
		return err
	}
	s.reportSaveProgress(solutionID, SavePhasePersistXML, 1, 1, true)

	s.reportSaveProgress(solutionID, SavePhaseFinalize, 0, 1, true)
	if err = s.store.MergeWorkingIntoChangedObjects(ctx, solutionID); err != nil {
		return err
	}
	if err = s.store.ClearWorkingChanges(ctx, solutionID); err != nil {
		return err
	}
	s.BumpSolutionGen(ctx, solutionID)
	s.reportSaveProgress(solutionID, SavePhaseFinalize, 1, 1, true)
	return nil
}

func applyAllChanges(sol *domain.Solution, changes []repository.ChangeRow) {
	for _, ch := range changes {
		applyOneChange(sol, ch)
	}
}

func applyOneChange(sol *domain.Solution, ch repository.ChangeRow) {
	switch ch.EntityType {
	case "control_command":
		applyControlCommandPatch(sol, ch)
	case "automation":
		applyAutomationPatch(sol, ch)
	default:
		if ch.EntityType != "graphic_object" {
			return
		}
		applyGraphicObjectPatch(sol, ch)
	}
}

func applyControlCommandPatch(sol *domain.Solution, ch repository.ChangeRow) {
	var cmd domain.ControlCommand
	if err := json.Unmarshal(ch.Patch, &cmd); err != nil {
		return
	}
	for i := range sol.ControlCommands.ControlCommand {
		if sol.ControlCommands.ControlCommand[i].Id == ch.EntityXMLID {
			sol.ControlCommands.ControlCommand[i] = cmd
		}
	}
}

func applyAutomationPatch(sol *domain.Solution, ch repository.ChangeRow) {
	var auto domain.Automation
	if err := json.Unmarshal(ch.Patch, &auto); err != nil {
		return
	}
	for i := range sol.Automations.Automation {
		if sol.Automations.Automation[i].Id == ch.EntityXMLID {
			sol.Automations.Automation[i] = auto
		}
	}
}

func applyGraphicObjectPatch(sol *domain.Solution, ch repository.ChangeRow) {
	var obj domain.GraphicObject
	if err := json.Unmarshal(ch.Patch, &obj); err != nil {
		return
	}
	for pi := range sol.Projects.Project {
		for si := range sol.Projects.Project[pi].GraphicScheme {
			if sol.Projects.Project[pi].GraphicScheme[si].Id != ch.SchemeXMLID {
				continue
			}
			for oi := range sol.Projects.Project[pi].GraphicScheme[si].GraphicObject {
				if sol.Projects.Project[pi].GraphicScheme[si].GraphicObject[oi].Id == ch.EntityXMLID {
					sol.Projects.Project[pi].GraphicScheme[si].GraphicObject[oi] = obj
				}
			}
		}
	}
}

func (s *SolutionService) Export(ctx context.Context, solutionID uuid.UUID) ([]byte, string, error) {
	meta, err := s.store.GetSolutionMeta(ctx, solutionID)
	if err != nil {
		return nil, "", err
	}
	sol, err := s.loadSolution(ctx, solutionID)
	if err != nil {
		return nil, "", err
	}
	changes, err := s.store.ListWorkingChanges(ctx, solutionID)
	if err != nil {
		return nil, "", err
	}
	applyAllChanges(&sol, changes)
	out, err := xml.Marshal(sol)
	if err != nil {
		return nil, "", err
	}
	full := append([]byte(xml.Header), out...)
	name := meta.Name
	if name == "" {
		name = "solution"
	}
	return full, name + ".xml", nil
}

func (s *SolutionService) ListSolutions(ctx context.Context) ([]repository.SolutionSummary, error) {
	start := time.Now()
	defer func() {
		slog.Info("ListSolutions done", "duration_ms", time.Since(start).Milliseconds())
	}()
	return s.store.ListSolutionsSummary(ctx)
}

func (s *SolutionService) DeleteSolution(ctx context.Context, solutionID uuid.UUID) error {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("DeleteSolution done", "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	err = s.store.DeleteSolution(ctx, solutionID)
	if err == nil {
		s.BumpSolutionGen(ctx, solutionID)
	}
	return err
}

func (s *SolutionService) DeleteAllSolutions(ctx context.Context) (int, error) {
	start := time.Now()
	var err error
	defer func() {
		slog.Info("DeleteAllSolutions done", "duration_ms", time.Since(start).Milliseconds(), "err", err)
	}()
	items, err := s.store.ListSolutionsSummary(ctx)
	if err != nil {
		return 0, err
	}
	count := len(items)
	err = s.store.DeleteAllSolutions(ctx)
	if err == nil {
		for _, item := range items {
			s.BumpSolutionGen(ctx, item.ID)
		}
	}
	return count, err
}

func (s *SolutionService) GetSolutionMeta(ctx context.Context, solutionID uuid.UUID) (repository.SolutionSummary, error) {
	start := time.Now()
	defer func() {
		slog.Info("GetSolutionMeta done", "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds())
	}()
	return s.store.GetSolutionMeta(ctx, solutionID)
}

func (s *SolutionService) ListDisplaySettings(ctx context.Context) ([]repository.DisplaySetting, error) {
	return s.store.ListDisplaySettings(ctx, UserIDFromContext(ctx))
}

func (s *SolutionService) UpdateDisplaySetting(ctx context.Context, d repository.DisplaySetting) error {
	return s.store.UpsertDisplaySetting(ctx, UserIDFromContext(ctx), d)
}
