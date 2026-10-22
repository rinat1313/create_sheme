const UserContext = {
  userId: null,
  displayName: null,

  load() {
    this.userId = localStorage.getItem('userId');
    this.displayName = localStorage.getItem('userDisplayName') || '';
  },

  save(id, displayName) {
    this.userId = id;
    this.displayName = displayName || '';
    localStorage.setItem('userId', id);
    localStorage.setItem('userDisplayName', this.displayName);
  },

  clear() {
    this.userId = null;
    this.displayName = null;
    localStorage.removeItem('userId');
    localStorage.removeItem('userDisplayName');
  },

  lastSolutionKey() {
    return this.userId ? `lastSolutionId:${this.userId}` : 'lastSolutionId';
  },
};

async function apiFetch(url, options = {}) {
  const headers = new Headers(options.headers || {});
  if (UserContext.userId && !headers.has('X-User-Id')) {
    headers.set('X-User-Id', UserContext.userId);
  }
  return fetch(url, { ...options, headers });
}

const API = {
  async listUsers() {
    const r = await apiFetch('/api/v1/users');
    if (!r.ok) return [];
    return r.json();
  },

  async createUser(login, displayName) {
    const r = await apiFetch('/api/v1/users', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ login, display_name: displayName || login }),
    });
    if (!r.ok) {
      const err = await r.json();
      throw new Error(err.error || 'create user failed');
    }
    return r.json();
  },

  async listSolutions() {
    const r = await apiFetch('/api/v1/me/solutions');
    if (!r.ok) {
      const fallback = await apiFetch('/api/v1/solutions');
      return fallback.json();
    }
    return r.json();
  },

  uploadSolution(file, onProgress) {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      const fd = new FormData();
      fd.append('file', file);
      xhr.open('POST', '/api/v1/solutions');
      if (UserContext.userId) {
        xhr.setRequestHeader('X-User-Id', UserContext.userId);
      }
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable && onProgress) {
          onProgress({ phase: 'upload', loaded: e.loaded, total: e.total });
        }
      };
      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          try {
            resolve(JSON.parse(xhr.responseText));
          } catch {
            reject(new Error('invalid server response'));
          }
          return;
        }
        let message = `upload failed (${xhr.status})`;
        try {
          const err = JSON.parse(xhr.responseText);
          if (err.error) message = err.error;
        } catch {
          if (xhr.status === 413) message = 'Файл превышает допустимый размер';
        }
        if (xhr.status === 409) message = 'Импорт уже выполняется. Дождитесь завершения.';
        if (xhr.status === 413) message = 'Файл превышает допустимый размер';
        reject(new Error(message));
      };
      xhr.onerror = () => reject(new Error('Соединение прервано. Загрузка не завершена.'));
      xhr.ontimeout = () => reject(new Error('Превышено время ожидания. Попробуйте снова.'));
      xhr.send(fd);
    });
  },

  async pollImportStatus(solutionId, onProgress, intervalMs = 750) {
    return new Promise((resolve, reject) => {
      const poll = async () => {
        try {
          const meta = await this.getSolutionMeta(solutionId);
          if (onProgress) onProgress(meta);
          if (meta.import_status === 'ready') {
            resolve(meta);
            return;
          }
          if (meta.import_status === 'failed') {
            reject(new Error(meta.import_error || 'Импорт завершился с ошибкой'));
            return;
          }
          setTimeout(poll, intervalMs);
        } catch (err) {
          reject(err);
        }
      };
      poll();
    });
  },

  async getSolutionMeta(id) {
    const r = await apiFetch(`/api/v1/solutions/${id}`);
    return r.json();
  },

  async getTree(id, options = {}) {
    const params = new URLSearchParams();
    if (options.depth) params.set('depth', options.depth);
    if (options.project_xml_id) params.set('project_xml_id', options.project_xml_id);
    const qs = params.toString();
    const r = await apiFetch(`/api/v1/solutions/${id}/tree${qs ? '?' + qs : ''}`);
    return r.json();
  },

  async searchSolution(solutionId, q, options = {}) {
    const params = new URLSearchParams();
    if (q != null) params.set('q', q);
    if (options.limit != null) params.set('limit', String(options.limit));
    if (options.offset != null) params.set('offset', String(options.offset));
    const qs = params.toString();
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/search${qs ? '?' + qs : ''}`);
    if (!r.ok) {
      const err = await r.json().catch(() => ({}));
      throw new Error(err.error || 'search failed');
    }
    return r.json();
  },

  async getProjectSchemes(solutionId, projectXmlId) {
    return this.getTree(solutionId, { project_xml_id: projectXmlId });
  },

  async getSolutionTemplates(solutionId) {
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/templates`);
    if (!r.ok) return [];
    return r.json();
  },

  async getScheme(solutionId, schemeXMLId, sessionId) {
    let url = `/api/v1/solutions/${solutionId}/schemes/${encodeURIComponent(schemeXMLId)}`;
    if (sessionId) url += `?session_id=${sessionId}`;
    const r = await apiFetch(url);
    if (!r.ok) throw new Error('scheme not found');
    return r.json();
  },

  async getObject(solutionId, objectId, sessionId) {
    let url = `/api/v1/solutions/${solutionId}/objects/${objectId}`;
    if (sessionId) url += `?session_id=${sessionId}`;
    const r = await apiFetch(url);
    if (!r.ok) throw new Error('object not found');
    return r.json();
  },

  async getSchemeObjectDetails(solutionId, schemeXMLId, sessionId) {
    let url = `/api/v1/solutions/${solutionId}/schemes/${encodeURIComponent(schemeXMLId)}/object-details`;
    if (sessionId) url += `?session_id=${sessionId}`;
    const r = await apiFetch(url);
    if (!r.ok) throw new Error('object details not found');
    return r.json();
  },

  async createSession(solutionId) {
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/sessions`, { method: 'POST' });
    return r.json();
  },

  async openScheme(solutionId, sessionId, schemeXMLId) {
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/sessions/${sessionId}/open-scheme`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ scheme_xml_id: schemeXMLId }),
    });
    return r.json();
  },

  async closeScheme(solutionId, sessionId) {
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/sessions/${sessionId}/close-scheme`, {
      method: 'POST',
    });
    return r.json();
  },

  async patchObject(solutionId, sessionId, objectId, schemeXMLId, patch, entityType = 'graphic_object') {
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/sessions/${sessionId}/objects/${objectId}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ scheme_xml_id: schemeXMLId, entity_type: entityType, patch }),
    });
    return r.json();
  },

  async save(solutionId) {
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/save`, { method: 'POST' });
    if (!r.ok) {
      let message = `save failed (${r.status})`;
      try {
        const err = await r.json();
        if (err.error) message = err.error;
      } catch {
      }
      if (r.status === 409) message = 'Сохранение уже выполняется. Дождитесь завершения.';
      throw new Error(message);
    }
    return r.json();
  },

  async export(solutionId) {
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/export`);
    return r.blob();
  },

  async getDisplaySettings() {
    const r = await apiFetch('/api/v1/display-settings');
    return r.json();
  },

  async updateDisplaySetting(template, data) {
    const r = await apiFetch(`/api/v1/display-settings/${template}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    return r.json();
  },

  async getComplianceRules() {
    const r = await apiFetch('/api/v1/compliance-rules');
    if (!r.ok) return [];
    const data = await r.json();
    return Array.isArray(data) ? data : [];
  },

  async getComplianceReport(solutionId, options = {}) {
    const params = new URLSearchParams();
    if (options.project_xml_id) params.set('project_xml_id', options.project_xml_id);
    if (options.scheme_xml_id) params.set('scheme_xml_id', options.scheme_xml_id);
    const qs = params.toString();
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/compliance-report${qs ? '?' + qs : ''}`);
    if (!r.ok) return { nodes: [] };
    return r.json();
  },

  async createComplianceRule(data) {
    const r = await apiFetch('/api/v1/compliance-rules', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!r.ok) {
      const err = await r.json();
      throw new Error(err.error || 'create failed');
    }
    return r.json();
  },

  async updateComplianceRule(id, data) {
    const r = await apiFetch(`/api/v1/compliance-rules/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!r.ok) {
      const err = await r.json();
      throw new Error(err.error || 'update failed');
    }
    return r.json();
  },

  async deleteComplianceRule(id) {
    const r = await apiFetch(`/api/v1/compliance-rules/${id}`, { method: 'DELETE' });
    if (!r.ok) {
      const err = await r.json();
      throw new Error(err.error || 'delete failed');
    }
    return r.json();
  },

  async exportObjects(solutionId, options) {
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/export-objects`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(options),
    });
    if (!r.ok) {
      const err = await r.json();
      throw new Error(err.error || 'export failed');
    }
    return r.blob();
  },

  async ackChangedObjects(solutionId, sessionId) {
    const q = sessionId ? `?session_id=${encodeURIComponent(sessionId)}` : '';
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/changed-objects/ack${q}`, {
      method: 'POST',
    });
    if (!r.ok) {
      const err = await r.json().catch(() => ({}));
      throw new Error(err.error || 'ack failed');
    }
    return r.json();
  },

  async getTemplateParameters(solutionId, template, sessionId) {
    const params = new URLSearchParams({ template });
    if (sessionId) params.set('session_id', sessionId);
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/template-parameters?${params}`);
    if (!r.ok) throw new Error('template parameters failed');
    return r.json();
  },

  async getTemplateCharacteristics(solutionId, template, sessionId) {
    const params = new URLSearchParams({ template });
    if (sessionId) params.set('session_id', sessionId);
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/template-characteristics?${params}`);
    if (!r.ok) throw new Error('template characteristics failed');
    return r.json();
  },

  async getComplianceTargets(solutionId, objectTemplate, scope, sessionId) {
    const params = new URLSearchParams({ object_template: objectTemplate, scope });
    if (sessionId) params.set('session_id', sessionId);
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/compliance-targets?${params}`);
    if (!r.ok) throw new Error('compliance targets failed');
    return r.json();
  },

  async getComplianceTargetParameters(solutionId, scope, entityTemplate, sessionId) {
    const params = new URLSearchParams({ scope, entity_template: entityTemplate });
    if (sessionId) params.set('session_id', sessionId);
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/compliance-target-parameters?${params}`);
    if (!r.ok) throw new Error('compliance target parameters failed');
    return r.json();
  },

  async getChangedObjectsCount(solutionId, sessionId) {
    const q = sessionId ? `?session_id=${encodeURIComponent(sessionId)}` : '';
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/changed-objects-count${q}`);
    if (!r.ok) throw new Error('changed count failed');
    return r.json();
  },

  async getTagEditRules() {
    const r = await apiFetch('/api/v1/tag-edit-rules');
    if (!r.ok) return [];
    const data = await r.json();
    return Array.isArray(data) ? data : [];
  },

  async createTagEditRule(data) {
    const r = await apiFetch('/api/v1/tag-edit-rules', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!r.ok) {
      const err = await r.json();
      throw new Error(err.error || 'create failed');
    }
    return r.json();
  },

  async updateTagEditRule(id, data) {
    const r = await apiFetch(`/api/v1/tag-edit-rules/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!r.ok) {
      const err = await r.json();
      throw new Error(err.error || 'update failed');
    }
    return r.json();
  },

  async deleteTagEditRule(id) {
    const r = await apiFetch(`/api/v1/tag-edit-rules/${id}`, { method: 'DELETE' });
    if (!r.ok) {
      const err = await r.json();
      throw new Error(err.error || 'delete failed');
    }
    return r.json();
  },

  async previewTagEdit(solutionId) {
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/tag-edit/preview`, { method: 'POST' });
    if (!r.ok) {
      const err = await r.json();
      throw new Error(err.error || 'preview failed');
    }
    return r.json();
  },

  async applyTagEdit(solutionId) {
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/tag-edit/apply`, { method: 'POST' });
    if (!r.ok) {
      const err = await r.json();
      throw new Error(err.error || 'apply failed');
    }
    return r.json();
  },

  async getTagEditJob(solutionId, jobId) {
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/tag-edit/jobs/${jobId}`);
    if (!r.ok) {
      const err = await r.json();
      throw new Error(err.error || 'job status failed');
    }
    return r.json();
  },

  async pollTagEditJob(solutionId, jobId, onProgress, intervalMs = 500) {
    return new Promise((resolve, reject) => {
      const poll = async () => {
        try {
          const job = await this.getTagEditJob(solutionId, jobId);
          if (onProgress) onProgress(job);
          if (job.status === 'completed') {
            resolve(job);
            return;
          }
          if (job.status === 'failed') {
            reject(new Error(job.error_message || 'tag edit job failed'));
            return;
          }
          setTimeout(poll, intervalMs);
        } catch (err) {
          reject(err);
        }
      };
      poll();
    });
  },

  async startDuplicateScan(solutionId, ruleId, sessionId) {
    const params = sessionId ? `?session_id=${encodeURIComponent(sessionId)}` : '';
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/compliance-duplicate-scan/scan${params}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ rule_id: ruleId }),
    });
    if (!r.ok) {
      const err = await r.json();
      throw new Error(err.error || 'duplicate scan failed');
    }
    return r.json();
  },

  async setDuplicateBackgroundScan(solutionId, ruleId, enabled, sessionId) {
    const params = sessionId ? `?session_id=${encodeURIComponent(sessionId)}` : '';
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/compliance-duplicate-scan/background${params}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ rule_id: ruleId || '', enabled: !!enabled }),
    });
    if (!r.ok) {
      const err = await r.json();
      throw new Error(err.error || 'background scan failed');
    }
    return r.json();
  },

  async getDuplicateBackgroundScan(solutionId) {
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/compliance-duplicate-scan/background`);
    if (!r.ok) {
      const err = await r.json();
      throw new Error(err.error || 'background status failed');
    }
    return r.json();
  },

  async getDuplicateBackgroundResults(solutionId) {
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/compliance-duplicate-scan/results`);
    if (!r.ok) {
      const err = await r.json();
      throw new Error(err.error || 'background results failed');
    }
    return r.json();
  },

  async recheckDuplicateScan(solutionId, ruleId, sessionId, objectId) {
    const params = sessionId ? `?session_id=${encodeURIComponent(sessionId)}` : '';
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/compliance-duplicate-scan/recheck${params}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ rule_id: ruleId, object_id: objectId || '' }),
    });
    if (!r.ok) {
      const err = await r.json();
      throw new Error(err.error || 'duplicate recheck failed');
    }
    return r.json();
  },

  async getDuplicateScanJob(solutionId, jobId) {
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/compliance-duplicate-scan/jobs/${jobId}`);
    if (!r.ok) {
      const err = await r.json();
      throw new Error(err.error || 'job status failed');
    }
    return r.json();
  },

  async pollDuplicateScanJob(solutionId, jobId, onProgress, intervalMs = 500) {
    return new Promise((resolve, reject) => {
      const poll = async () => {
        try {
          const job = await this.getDuplicateScanJob(solutionId, jobId);
          if (onProgress) onProgress(job);
          if (job.status === 'completed') {
            resolve(job);
            return;
          }
          if (job.status === 'failed') {
            reject(new Error(job.error_message || 'duplicate scan job failed'));
            return;
          }
          setTimeout(poll, intervalMs);
        } catch (err) {
          reject(err);
        }
      };
      poll();
    });
  },

  async patchDuplicateTag(solutionId, sessionId, payload) {
    const r = await apiFetch(`/api/v1/solutions/${solutionId}/sessions/${sessionId}/duplicate-tag-patch`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (!r.ok) {
      const err = await r.json();
      throw new Error(err.error || 'patch duplicate tag failed');
    }
    return r.json();
  },

  async deleteSolution(id) {
    const r = await apiFetch(`/api/v1/solutions/${id}`, { method: 'DELETE' });
    if (!r.ok) {
      const err = await r.json();
      throw new Error(err.error || 'delete failed');
    }
    return r.json();
  },

  async deleteAllSolutions() {
    const r = await apiFetch('/api/v1/solutions', { method: 'DELETE' });
    if (!r.ok) {
      const err = await r.json();
      throw new Error(err.error || 'delete failed');
    }
    return r.json();
  },
};
