package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"create_sheme/internal/repository"
	"create_sheme/internal/service"

	"github.com/google/uuid"
)

type API struct {
	svc            *service.SolutionService
	users          *service.UserService
	importMaxBytes int64
}

func NewAPI(svc *service.SolutionService, users *service.UserService, importMaxBytes int64) *API {
	return &API{svc: svc, users: users, importMaxBytes: importMaxBytes}
}

func attachImportProgressFields(out map[string]any, svc *service.SolutionService, id uuid.UUID, status string) {
	if status != "importing" {
		return
	}
	prog, ok := svc.ImportProgress(id)
	if !ok {
		return
	}
	out["import_phase"] = prog.Phase
	out["import_percent"] = prog.Percent
	out["import_processed"] = prog.Processed
	out["import_total"] = prog.Total
	if !prog.StartedAt.IsZero() {
		out["import_started_at"] = prog.StartedAt.UTC().Format(time.RFC3339)
	}
	if prog.ETASeconds > 0 {
		out["import_eta_seconds"] = prog.ETASeconds
	}
}

func attachSaveProgressFields(out map[string]any, svc *service.SolutionService, id uuid.UUID) {
	prog, ok := svc.SaveProgress(id)
	if !ok {
		return
	}
	out["save_status"] = "saving"
	out["save_phase"] = prog.Phase
	out["save_percent"] = prog.Percent
	out["save_processed"] = prog.Processed
	out["save_total"] = prog.Total
	if !prog.StartedAt.IsZero() {
		out["save_started_at"] = prog.StartedAt.UTC().Format(time.RFC3339)
	}
	if prog.ETASeconds > 0 {
		out["save_eta_seconds"] = prog.ETASeconds
	}
}

func (a *API) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/users", a.handleUsers)
	mux.HandleFunc("/api/v1/me/", a.handleMe)
	mux.HandleFunc("/api/v1/solutions", a.handleSolutions)
	mux.HandleFunc("/api/v1/solutions/", a.handleSolutionSub)
	mux.HandleFunc("/api/v1/display-settings", a.handleDisplaySettings)
	mux.HandleFunc("/api/v1/display-settings/", a.handleDisplaySettingItem)
	mux.HandleFunc("/api/v1/compliance-rules", a.handleComplianceRules)
	mux.HandleFunc("/api/v1/compliance-rules/", a.handleComplianceRuleItem)
	mux.HandleFunc("/api/v1/tag-edit-rules", a.handleTagEditRules)
	mux.HandleFunc("/api/v1/tag-edit-rules/", a.handleTagEditRuleItem)
}

func (a *API) handleSolutions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.listSolutions(w, r)
	case http.MethodPost:
		a.importSolution(w, r)
	case http.MethodDelete:
		a.deleteAllSolutions(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) listSolutions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		slog.Info("listSolutions done", "duration_ms", time.Since(start).Milliseconds())
	}()
	items, err := a.svc.ListSolutionsForUser(r.Context(), service.UserIDFromContext(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var out []map[string]any
	for _, it := range items {
		row := map[string]any{
			"id": it.ID.String(), "name": it.Name, "company": it.Company,
			"is_dirty": it.IsDirty, "import_status": it.ImportStatus,
			"created_at": it.CreatedAt.Format(time.RFC3339),
		}
		if it.ImportError != "" {
			row["import_error"] = it.ImportError
		}
		attachImportProgressFields(row, a.svc, it.ID, it.ImportStatus)
		out = append(out, row)
	}
	writeJSON(w, out)
}

func (a *API) importSolution(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var bytesRead int64
	var solutionID string
	defer func() {
		slog.Info("importSolution done",
			"duration_ms", time.Since(start).Milliseconds(),
			"bytes_read", bytesRead,
			"solution_id", solutionID,
		)
	}()

	slog.Info("import accept", "remote", r.RemoteAddr, "content_length", r.ContentLength)

	if !a.svc.TryAcquireImportSlot() {
		writeError(w, http.StatusConflict, "import already in progress")
		return
	}
	acquired := true
	release := func() {
		if acquired {
			a.svc.ReleaseImportSlot()
			acquired = false
		}
	}

	spoolPath, size, nameHint, err := a.spoolImportFile(w, r)
	if err != nil {
		release()
		if errors.Is(err, errImportTooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "file exceeds maximum upload size")
			return
		}
		if errors.Is(err, errImportFileMissing) {
			writeError(w, http.StatusBadRequest, "file required")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	bytesRead = size
	slog.Info("import spool", "path", spoolPath, "bytes", size, "duration_ms", time.Since(start).Milliseconds())

	result, err := a.svc.BeginImport(r.Context(), nameHint)
	if err != nil {
		_ = os.Remove(spoolPath)
		release()
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	solutionID = result.ID
	parsedID, err := uuid.Parse(result.ID)
	if err != nil {
		_ = os.Remove(spoolPath)
		release()
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	userID := service.UserIDFromContext(r.Context())
	go a.runBackgroundImport(userID, parsedID, spoolPath)

	acquired = false
	writeJSONStatus(w, http.StatusAccepted, result)
}

func (a *API) runBackgroundImport(userID, solutionID uuid.UUID, spoolPath string) {
	start := time.Now()
	defer func() {
		a.svc.ReleaseImportSlot()
		if err := os.Remove(spoolPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("import temp remove failed", "path", spoolPath, "err", err)
		}
		slog.Info("runBackgroundImport done",
			"duration_ms", time.Since(start).Milliseconds(),
			"solution_id", solutionID,
		)
	}()

	ctx := service.ContextWithUserID(context.Background(), userID)
	slog.Info("runBackgroundImport start", "solution_id", solutionID, "path", spoolPath)
	if err := a.svc.FinishImportFromFile(ctx, solutionID, spoolPath); err != nil {
		slog.Error("import background failed", "solution_id", solutionID, "err", err)
		_ = a.svc.MarkImportFailed(ctx, solutionID, err.Error())
	}
}

var (
	errImportTooLarge    = errors.New("import file too large")
	errImportFileMissing = errors.New("import file missing")
)

func (a *API) spoolImportFile(w http.ResponseWriter, r *http.Request) (path string, size int64, nameHint string, err error) {
	start := time.Now()
	defer func() {
		slog.Info("spoolImportFile done",
			"duration_ms", time.Since(start).Milliseconds(),
			"bytes", size,
			"err", err,
		)
	}()
	slog.Debug("spoolImportFile start")

	maxBytes := a.importMaxBytes
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+1<<20)

	mr, err := r.MultipartReader()
	if err != nil {
		if isRequestBodyTooLarge(err) {
			return "", 0, "", errImportTooLarge
		}
		return "", 0, "", err
	}

	tmp, err := os.CreateTemp("", "solution-import-*")
	if err != nil {
		return "", 0, "", err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	var found bool
	for {
		part, nextErr := mr.NextPart()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			if isRequestBodyTooLarge(nextErr) {
				return "", 0, "", errImportTooLarge
			}
			return "", 0, "", nextErr
		}
		if part.FormName() != "file" {
			_, _ = io.Copy(io.Discard, part)
			_ = part.Close()
			continue
		}
		nameHint = strings.TrimSuffix(filepath.Base(part.FileName()), filepath.Ext(part.FileName()))
		limited := io.LimitReader(part, maxBytes+1)
		written, copyErr := io.Copy(tmp, limited)
		_ = part.Close()
		if copyErr != nil {
			if isRequestBodyTooLarge(copyErr) {
				return "", 0, "", errImportTooLarge
			}
			return "", 0, "", copyErr
		}
		if written > maxBytes {
			return "", 0, "", errImportTooLarge
		}
		size = written
		found = true
		break
	}
	if !found || size == 0 {
		return "", 0, "", errImportFileMissing
	}
	if err = tmp.Close(); err != nil {
		return "", 0, "", err
	}
	cleanup = false
	return tmpPath, size, nameHint, nil
}

func isRequestBodyTooLarge(err error) bool {
	return err != nil && strings.Contains(err.Error(), "request body too large")
}

func (a *API) handleSolutionSub(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/solutions/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	solutionID, err := uuid.Parse(parts[0])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid solution id")
		return
	}
	if len(parts) > 1 {
		if err := a.svc.EnsureUserAccess(r.Context(), service.UserIDFromContext(r.Context()), solutionID); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
	} else if r.Method == http.MethodGet || r.Method == http.MethodDelete {
		if err := a.svc.EnsureUserAccess(r.Context(), service.UserIDFromContext(r.Context()), solutionID); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
	}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			a.getSolutionMeta(w, r, solutionID)
			return
		case http.MethodDelete:
			a.deleteSolution(w, r, solutionID)
			return
		}
		http.NotFound(w, r)
		return
	}
	switch parts[1] {
	case "tree":
		a.getTree(w, r, solutionID)
	case "templates", "types":
		if len(parts) >= 4 && parts[3] == "schema" {
			a.getTypeSchema(w, r, solutionID, parts[2])
			return
		}
		if len(parts) == 2 {
			a.getSolutionTemplates(w, r, solutionID)
			return
		}
		http.NotFound(w, r)
	case "schemes":
		if len(parts) >= 3 {
			if len(parts) >= 4 && parts[3] == "object-details" {
				a.getSchemeObjectDetails(w, r, solutionID, parts[2])
				return
			}
			a.getScheme(w, r, solutionID, parts[2])
		}
	case "objects":
		if len(parts) >= 4 && parts[3] == "commands" {
			a.getObjectCommands(w, r, solutionID, parts[2])
			return
		}
		if len(parts) >= 3 {
			a.getObject(w, r, solutionID, parts[2])
		}
	case "sessions":
		a.handleSessions(w, r, solutionID, parts[2:])
	case "save":
		a.saveSolution(w, r, solutionID)
	case "export":
		a.exportSolution(w, r, solutionID)
	case "export-objects":
		a.exportObjects(w, r, solutionID)
	case "template-parameters":
		a.getTemplateParameters(w, r, solutionID)
	case "template-characteristics":
		a.getTemplateCharacteristics(w, r, solutionID)
	case "compliance-targets":
		a.getComplianceTargets(w, r, solutionID)
	case "compliance-target-parameters":
		a.getComplianceTargetParameters(w, r, solutionID)
	case "changed-objects-count":
		a.getChangedObjectsCount(w, r, solutionID)
	case "changed-objects":
		a.handleChangedObjects(w, r, solutionID, parts[2:])
	case "compliance-report":
		a.getComplianceReport(w, r, solutionID)
	case "search":
		a.searchSolution(w, r, solutionID)
	case "tag-edit":
		a.handleTagEdit(w, r, solutionID, parts[2:])
	case "compliance-duplicate-scan":
		a.handleComplianceDuplicateScan(w, r, solutionID, parts[2:])
	default:
		http.NotFound(w, r)
	}
}

func (a *API) getComplianceReport(w http.ResponseWriter, r *http.Request, solutionID uuid.UUID) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	opts := service.ComplianceReportOptions{
		ProjectXMLID: r.URL.Query().Get("project_xml_id"),
		SchemeXMLID:  r.URL.Query().Get("scheme_xml_id"),
	}
	report, err := a.svc.BuildComplianceReport(r.Context(), solutionID, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, report)
}

func (a *API) searchSolution(w http.ResponseWriter, r *http.Request, solutionID uuid.UUID) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	start := time.Now()
	defer func() {
		slog.Info("searchSolution done", "solution_id", solutionID, "duration_ms", time.Since(start).Milliseconds())
	}()
	slog.Debug("searchSolution start", "solution_id", solutionID)

	q := r.URL.Query().Get("q")
	limit := 0
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}
	result, err := a.svc.SearchSolution(r.Context(), solutionID, q, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, result)
}

func (a *API) getSolutionTemplates(w http.ResponseWriter, r *http.Request, solutionID uuid.UUID) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	start := time.Now()
	defer func() {
		slog.Info("getSolutionTemplates done", "duration_ms", time.Since(start).Milliseconds())
	}()
	templates, err := a.svc.ListSolutionTemplates(r.Context(), solutionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, templates)
}

func (a *API) deleteSolution(w http.ResponseWriter, r *http.Request, solutionID uuid.UUID) {
	if err := a.svc.DeleteSolution(r.Context(), solutionID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "deleted"})
}

func (a *API) deleteAllSolutions(w http.ResponseWriter, r *http.Request) {
	count, err := a.svc.DeleteAllSolutionsForUser(r.Context(), service.UserIDFromContext(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"status": "deleted", "count": count})
}

func (a *API) getSolutionMeta(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	row, err := a.svc.GetSolutionMeta(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	out := map[string]any{
		"id": id.String(), "name": row.Name, "company": row.Company,
		"description": row.Description, "is_dirty": row.IsDirty,
		"import_status": row.ImportStatus,
	}
	if row.ImportStatus == "failed" && row.ImportError != "" {
		out["import_error"] = row.ImportError
	}
	attachImportProgressFields(out, a.svc, id, row.ImportStatus)
	attachSaveProgressFields(out, a.svc, id)
	writeJSON(w, out)
}

func (a *API) getTree(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	opts := service.TreeOptions{
		Depth:        r.URL.Query().Get("depth"),
		ProjectXMLID: r.URL.Query().Get("project_xml_id"),
	}
	tree, err := a.svc.BuildTree(r.Context(), id, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, tree)
}

func (a *API) getScheme(w http.ResponseWriter, r *http.Request, solutionID uuid.UUID, schemeXMLID string) {
	var sessionID *uuid.UUID
	if sid := r.URL.Query().Get("session_id"); sid != "" {
		parsed, err := uuid.Parse(sid)
		if err == nil {
			sessionID = &parsed
		}
	}
	data, err := a.svc.GetScheme(r.Context(), solutionID, schemeXMLID, sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (a *API) getObject(w http.ResponseWriter, r *http.Request, solutionID uuid.UUID, objectID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var sessionID *uuid.UUID
	if sid := r.URL.Query().Get("session_id"); sid != "" {
		parsed, err := uuid.Parse(sid)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session_id")
			return
		}
		sessionID = &parsed
	}
	details, err := a.svc.GetObjectDetails(r.Context(), solutionID, objectID, sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, details)
}

func (a *API) getObjectCommands(w http.ResponseWriter, r *http.Request, solutionID uuid.UUID, objectID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cmds, err := a.svc.GetObjectCommands(r.Context(), solutionID, objectID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, cmds)
}

func (a *API) getTypeSchema(w http.ResponseWriter, r *http.Request, solutionID uuid.UUID, template string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	schema, source, err := a.svc.GetTypeSchema(r.Context(), solutionID, template)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, map[string]any{
		"template": template,
		"source":   source,
		"schema":   json.RawMessage(schema),
	})
}

func (a *API) getSchemeObjectDetails(w http.ResponseWriter, r *http.Request, solutionID uuid.UUID, schemeXMLID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var sessionID *uuid.UUID
	if sid := r.URL.Query().Get("session_id"); sid != "" {
		if parsed, err := uuid.Parse(sid); err == nil {
			sessionID = &parsed
		}
	}
	details, err := a.svc.GetSchemeObjectsDetails(r.Context(), solutionID, schemeXMLID, sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, details)
}

func (a *API) handleSessions(w http.ResponseWriter, r *http.Request, solutionID uuid.UUID, parts []string) {
	if len(parts) == 0 {
		if r.Method == http.MethodPost {
			id, err := a.svc.CreateSession(r.Context(), solutionID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, map[string]string{"session_id": id.String()})
		}
		return
	}
	sessionID, err := uuid.Parse(parts[0])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	if len(parts) == 1 {
		http.NotFound(w, r)
		return
	}
	switch parts[1] {
	case "open-scheme":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			SchemeXMLID string `json:"scheme_xml_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := a.svc.OpenScheme(r.Context(), sessionID, body.SchemeXMLID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	case "close-scheme":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := a.svc.CloseScheme(r.Context(), sessionID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	case "objects":
		if len(parts) >= 3 && r.Method == http.MethodPatch {
			a.patchObject(w, r, sessionID, parts[2])
		}
	case "duplicate-tag-patch":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req service.PatchDuplicateTagRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := a.svc.PatchDuplicateTag(r.Context(), sessionID, solutionID, req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	case "mark-original":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req service.MarkOriginalRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		result, err := a.svc.MarkOriginalObject(r.Context(), sessionID, solutionID, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, result)
	default:
		http.NotFound(w, r)
	}
}

func (a *API) patchObject(w http.ResponseWriter, r *http.Request, sessionID uuid.UUID, objectID string) {
	var body struct {
		SchemeXMLID string          `json:"scheme_xml_id"`
		EntityType  string          `json:"entity_type"`
		Patch       json.RawMessage `json:"patch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.EntityType == "" {
		body.EntityType = "graphic_object"
	}
	if err := a.svc.PatchObject(r.Context(), sessionID, body.SchemeXMLID, body.EntityType, objectID, body.Patch); err != nil {
		if errors.Is(err, service.ErrSchemeXMLIDRequired) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func (a *API) saveSolution(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := a.svc.Save(r.Context(), id); err != nil {
		if errors.Is(err, service.ErrSaveBusy) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "saved"})
}

func (a *API) exportSolution(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	data, filename, err := a.svc.Export(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Write(data)
}

func (a *API) handleDisplaySettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	items, err := a.svc.ListDisplaySettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, items)
}

func (a *API) handleDisplaySettingItem(w http.ResponseWriter, r *http.Request) {
	template := strings.TrimPrefix(r.URL.Path, "/api/v1/display-settings/")
	if template == "" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var d repository.DisplaySetting
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	d.Template = template
	if err := a.svc.UpdateDisplaySetting(r.Context(), d); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func (a *API) handleComplianceRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := a.svc.ListComplianceRules(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, items)
	case http.MethodPost:
		var input service.ComplianceRuleInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		rule, err := a.svc.CreateComplianceRule(r.Context(), input)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, rule)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) handleComplianceRuleItem(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/compliance-rules/")
	if idStr == "" {
		http.NotFound(w, r)
		return
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid rule id")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var input service.ComplianceRuleInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		input.ID = id.String()
		if err := a.svc.UpdateComplianceRule(r.Context(), input); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	case http.MethodDelete:
		if err := a.svc.DeleteComplianceRule(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) exportObjects(w http.ResponseWriter, r *http.Request, solutionID uuid.UUID) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req service.ExportObjectsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Fields) == 0 {
		writeError(w, http.StatusBadRequest, "fields required")
		return
	}
	data, filename, err := a.svc.ExportObjects(r.Context(), solutionID, req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidSessionID) || errors.Is(err, service.ErrInvalidTagPresence) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	contentType := "text/csv"
	if strings.HasSuffix(filename, ".json") {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Write(data)
}

func (a *API) getTemplateParameters(w http.ResponseWriter, r *http.Request, solutionID uuid.UUID) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	template := r.URL.Query().Get("template")
	if template == "" {
		writeError(w, http.StatusBadRequest, "template required")
		return
	}
	var sessionID *uuid.UUID
	if sid := r.URL.Query().Get("session_id"); sid != "" {
		parsed, parseErr := uuid.Parse(sid)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid session_id")
			return
		}
		sessionID = &parsed
	}
	params, err := a.svc.ListTemplateParameters(r.Context(), solutionID, template, sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, params)
}

func (a *API) getTemplateCharacteristics(w http.ResponseWriter, r *http.Request, solutionID uuid.UUID) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	template := r.URL.Query().Get("template")
	if template == "" {
		writeError(w, http.StatusBadRequest, "template required")
		return
	}
	var sessionID *uuid.UUID
	if sid := r.URL.Query().Get("session_id"); sid != "" {
		parsed, parseErr := uuid.Parse(sid)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid session_id")
			return
		}
		sessionID = &parsed
	}
	params, err := a.svc.ListTemplateCharacteristics(r.Context(), solutionID, template, sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, params)
}

func parseOptionalSessionID(r *http.Request) (*uuid.UUID, error) {
	sid := r.URL.Query().Get("session_id")
	if sid == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(sid)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func (a *API) getComplianceTargets(w http.ResponseWriter, r *http.Request, solutionID uuid.UUID) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	objectTemplate := r.URL.Query().Get("object_template")
	scope := r.URL.Query().Get("scope")
	if objectTemplate == "" || scope == "" {
		writeError(w, http.StatusBadRequest, "object_template and scope required")
		return
	}
	sessionID, err := parseOptionalSessionID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session_id")
		return
	}
	targets, err := a.svc.ListComplianceTargets(r.Context(), solutionID, objectTemplate, scope, sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, targets)
}

func (a *API) getComplianceTargetParameters(w http.ResponseWriter, r *http.Request, solutionID uuid.UUID) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	scope := r.URL.Query().Get("scope")
	entityTemplate := r.URL.Query().Get("entity_template")
	if scope == "" || entityTemplate == "" {
		writeError(w, http.StatusBadRequest, "scope and entity_template required")
		return
	}
	sessionID, err := parseOptionalSessionID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session_id")
		return
	}
	params, err := a.svc.ListComplianceTargetParameters(r.Context(), solutionID, scope, entityTemplate, sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, params)
}

func (a *API) getChangedObjectsCount(w http.ResponseWriter, r *http.Request, solutionID uuid.UUID) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var sessionID *uuid.UUID
	if sid := r.URL.Query().Get("session_id"); sid != "" {
		if parsed, err := uuid.Parse(sid); err == nil {
			sessionID = &parsed
		}
	}
	count, err := a.svc.CountChangedObjects(r.Context(), solutionID, sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]int{"count": count})
}

func (a *API) handleChangedObjects(w http.ResponseWriter, r *http.Request, solutionID uuid.UUID, parts []string) {
	if len(parts) == 0 || parts[0] != "ack" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var sessionID *uuid.UUID
	if sid := r.URL.Query().Get("session_id"); sid != "" {
		parsed, err := uuid.Parse(sid)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session_id")
			return
		}
		sessionID = &parsed
	}
	count, err := a.svc.AckChangedObjects(r.Context(), solutionID, sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]int{"count": count})
}

func (a *API) handleTagEditRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := a.svc.ListTagEditRules(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, items)
	case http.MethodPost:
		var input service.TagEditRuleInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		rule, err := a.svc.CreateTagEditRule(r.Context(), input)
		if err != nil {
			if errors.Is(err, service.ErrTagEditEmptyPattern) || errors.Is(err, service.ErrTagEditEmptyTemplate) || errors.Is(err, service.ErrTagEditNoActions) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, rule)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) handleTagEditRuleItem(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/tag-edit-rules/")
	idStr = strings.Trim(idStr, "/")
	if idStr == "" {
		http.NotFound(w, r)
		return
	}
	if idStr == "export" {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		raw, err := a.svc.ExportTagEditRules(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="tag-edit-rules.json"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(raw)
		return
	}
	if idStr == "import" {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, service.TagEditImportReadLimit))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := a.svc.ImportTagEditRules(r.Context(), raw); err != nil {
			if errors.Is(err, service.ErrTagEditInvalidImport) || errors.Is(err, service.ErrTagEditImportTooLarge) || errors.Is(err, service.ErrTagEditEmptyPattern) || errors.Is(err, service.ErrTagEditEmptyTemplate) || errors.Is(err, service.ErrTagEditNoActions) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
		return
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid rule id")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var input service.TagEditRuleInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		input.ID = id.String()
		if err := a.svc.UpdateTagEditRule(r.Context(), input); err != nil {
			if errors.Is(err, service.ErrTagEditEmptyPattern) || errors.Is(err, service.ErrTagEditEmptyTemplate) || errors.Is(err, service.ErrTagEditNoActions) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	case http.MethodDelete:
		if err := a.svc.DeleteTagEditRule(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) handleTagEdit(w http.ResponseWriter, r *http.Request, solutionID uuid.UUID, parts []string) {
	if len(parts) == 0 {
		http.NotFound(w, r)
		return
	}
	switch parts[0] {
	case "preview":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		preview, err := a.svc.PreviewTagEdit(r.Context(), solutionID)
		if err != nil {
			if errors.Is(err, service.ErrTagEditNoRules) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, preview)
	case "apply":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		jobID, err := a.svc.StartTagEditJob(r.Context(), solutionID)
		if err != nil {
			if errors.Is(err, service.ErrTagEditNoRules) || errors.Is(err, service.ErrTagEditJobActive) || errors.Is(err, service.ErrDuplicateScanActive) || errors.Is(err, service.ErrBackgroundScanActive) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]string{"job_id": jobID.String()})
	case "jobs":
		if len(parts) < 2 {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		jobID, err := uuid.Parse(parts[1])
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid job id")
			return
		}
		job, err := a.svc.GetTagEditJob(r.Context(), solutionID, jobID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, job)
	default:
		http.NotFound(w, r)
	}
}

func (a *API) handleComplianceDuplicateScan(w http.ResponseWriter, r *http.Request, solutionID uuid.UUID, parts []string) {
	if len(parts) == 0 {
		http.NotFound(w, r)
		return
	}
	switch parts[0] {
	case "scan":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			RuleID string `json:"rule_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		ruleID, err := uuid.Parse(body.RuleID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid rule_id")
			return
		}
		sessionID, err := parseOptionalSessionID(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session_id")
			return
		}
		jobID, err := a.svc.StartDuplicateScanJob(r.Context(), solutionID, ruleID, sessionID)
		if err != nil {
			if errors.Is(err, service.ErrDuplicateNoRule) || errors.Is(err, service.ErrDuplicateScanActive) || errors.Is(err, service.ErrBackgroundScanActive) || errors.Is(err, service.ErrTagEditJobActive) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]string{"job_id": jobID.String()})
	case "background":
		a.handleComplianceDuplicateBackground(w, r, solutionID)
	case "results":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		result, err := a.svc.GetBackgroundDuplicateScanResults(r.Context(), solutionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]interface{}{
			"violations_by_object": result.ViolationsByObject,
			"duplicate_tags":       result.DuplicateTags,
			"nodes":                result.Nodes,
			"rows":                 result.Rows,
		})
	case "recheck":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			RuleID   string `json:"rule_id"`
			ObjectID string `json:"object_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		ruleID, err := uuid.Parse(body.RuleID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid rule_id")
			return
		}
		sessionID, err := parseOptionalSessionID(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session_id")
			return
		}
		result, err := a.svc.RunDuplicateScanSync(r.Context(), solutionID, ruleID, sessionID)
		if err != nil {
			if errors.Is(err, service.ErrDuplicateNoRule) || errors.Is(err, service.ErrDuplicateScanActive) || errors.Is(err, service.ErrBackgroundScanActive) || errors.Is(err, service.ErrTagEditJobActive) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		objectHasDuplicates := false
		if body.ObjectID != "" {
			objectHasDuplicates = result.ViolationsByObject[body.ObjectID]
		}
		writeJSON(w, map[string]interface{}{
			"violations_by_object":  result.ViolationsByObject,
			"duplicate_tags":        result.DuplicateTags,
			"nodes":                 result.Nodes,
			"rows":                  result.Rows,
			"object_has_duplicates": objectHasDuplicates,
		})
	case "jobs":
		if len(parts) < 2 {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		jobID, err := uuid.Parse(parts[1])
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid job id")
			return
		}
		job, err := a.svc.GetDuplicateScanJob(r.Context(), solutionID, jobID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, job)
	default:
		http.NotFound(w, r)
	}
}

func (a *API) handleComplianceDuplicateBackground(w http.ResponseWriter, r *http.Request, solutionID uuid.UUID) {
	switch r.Method {
	case http.MethodGet:
		status, err := a.svc.GetBackgroundDuplicateScanStatus(r.Context(), solutionID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, status)
	case http.MethodPost:
		var body struct {
			RuleID  string `json:"rule_id"`
			Enabled bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		ruleID := uuid.Nil
		if body.Enabled {
			parsed, err := uuid.Parse(body.RuleID)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid rule_id")
				return
			}
			ruleID = parsed
		} else if body.RuleID != "" {
			if parsed, err := uuid.Parse(body.RuleID); err == nil {
				ruleID = parsed
			}
		}
		sessionID, err := parseOptionalSessionID(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session_id")
			return
		}
		if err := a.svc.SetBackgroundDuplicateScan(r.Context(), solutionID, ruleID, sessionID, body.Enabled); err != nil {
			if errors.Is(err, service.ErrDuplicateNoRule) || errors.Is(err, service.ErrDuplicateScanActive) || errors.Is(err, service.ErrBackgroundScanActive) || errors.Is(err, service.ErrTagEditJobActive) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		status, err := a.svc.GetBackgroundDuplicateScanStatus(r.Context(), solutionID)
		if err != nil {
			writeJSON(w, map[string]interface{}{"enabled": body.Enabled})
			return
		}
		writeJSON(w, status)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	writeJSONStatus(w, http.StatusOK, v)
}

func writeJSONStatus(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
