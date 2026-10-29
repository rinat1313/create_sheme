const Inspector = {
  body: null,
  title: null,
  tabs: null,
  activeTab: 'props',
  currentObject: null,
  onPatch: null,
  onEntityPatch: null,
  objectHadDuplicates: false,
  pendingDuplicateRecheck: false,
  duplicateRecheckStatus: null,
  duplicateRecheckLoading: false,
  markOriginalLoading: false,
  tagSnapshot: null,
  _pendingPatchPromise: null,
  sortState: {
    characteristics: { field: null, direction: null },
    commands: { field: null, direction: null },
    protections: { field: null, direction: null },
  },
  _debounceTimers: new Map(),

  init(body, title, tabs) {
    this.body = body;
    this.title = title;
    this.tabs = tabs;
    tabs.querySelectorAll('.props-tab').forEach(tab => {
      tab.addEventListener('click', () => this.switchTab(tab.dataset.tab));
    });
  },

  resetDuplicateRecheckState() {
    this.objectHadDuplicates = false;
    this.pendingDuplicateRecheck = false;
    this.duplicateRecheckStatus = null;
    this.duplicateRecheckLoading = false;
    this.markOriginalLoading = false;
    this.tagSnapshot = null;
  },

  collectObjectTags(details) {
    if (!details?.graphic_object) return {};
    const obj = details.graphic_object;
    const to = details.technology_objects?.[0];
    const tags = {};
    tags.main = (to?.Tag || obj.Tag || '').trim();
    for (const p of to?.TechnologyProperty || []) {
      if (p.Id) tags[`prop:${p.Id}`] = (p.Tag || '').trim();
    }
    for (const cmd of details.commands || []) {
      if (cmd.Id) tags[`cmd:${cmd.Id}`] = (cmd.Tag || '').trim();
      for (const p of cmd.TechnologyProperty || []) {
        if (p.Id) tags[`cmdprop:${cmd.Id}:${p.Id}`] = (p.Tag || '').trim();
      }
    }
    for (const auto of details.automations || []) {
      if (auto.Id) tags[`prot:${auto.Id}`] = (auto.Tag || '').trim();
      for (const p of auto.TechnologyProperty || []) {
        if (p.Id) tags[`protprop:${auto.Id}:${p.Id}`] = (p.Tag || '').trim();
      }
    }
    return tags;
  },

  tagsChanged(before, after) {
    const keys = new Set([...Object.keys(before || {}), ...Object.keys(after || {})]);
    for (const key of keys) {
      if ((before?.[key] || '') !== (after?.[key] || '')) return true;
    }
    return false;
  },

  refreshObjectDuplicateState(objectId) {
    this.objectHadDuplicates = Boolean(
      App.duplicateCompliance?.violationsByObjectId?.[String(objectId)]
      || App.getDuplicateRelatedRows(objectId).length > 0,
    );
    if (this.currentObject) {
      this.tagSnapshot = this.collectObjectTags(this.currentObject);
    }
  },

  onTagFieldsChanged() {
    if (!this.objectHadDuplicates || !this.currentObject) return;
    const currentTags = this.collectObjectTags(this.currentObject);
    if (!this.tagsChanged(this.tagSnapshot, currentTags)) return;
    this.pendingDuplicateRecheck = true;
    this.duplicateRecheckStatus = null;
    const obj = this.currentObject.graphic_object;
    const to = this.currentObject.technology_objects?.[0];
    this.updateTitleRow(obj, to);
  },

  completeDuplicateRecheck(hasDuplicates) {
    this.pendingDuplicateRecheck = false;
    this.objectHadDuplicates = hasDuplicates;
    this.duplicateRecheckStatus = hasDuplicates ? null : 'clean';
    if (this.currentObject) {
      this.tagSnapshot = this.collectObjectTags(this.currentObject);
      this.render(this.currentObject);
    }
  },

  hasMainTagDuplicates(objectId) {
    return App.getMainTagDuplicateSiblings(objectId).length > 0;
  },

  updateTitleRow(obj, to) {
    const statusEl = document.getElementById('propsDuplicateStatus');
    const btnEl = document.getElementById('btnRecheckDuplicates');
    const originalBtn = document.getElementById('btnMarkOriginal');
    const hideOriginal = () => {
      if (!originalBtn) return;
      originalBtn.hidden = true;
      originalBtn.disabled = false;
      originalBtn.textContent = 'Оригинальный объект';
    };
    if (!obj) {
      this.title.textContent = 'Свойства';
      if (statusEl) statusEl.hidden = true;
      if (btnEl) btnEl.hidden = true;
      hideOriginal();
      return;
    }
    this.title.textContent = to?.Name || obj.Name || 'Объект';
    const showOriginal = this.hasMainTagDuplicates(obj.Id);
    if (originalBtn) {
      originalBtn.hidden = !showOriginal;
      originalBtn.disabled = this.markOriginalLoading || this.duplicateRecheckLoading;
      originalBtn.textContent = this.markOriginalLoading ? 'Применение…' : 'Оригинальный объект';
    }
    if (!statusEl || !btnEl) return;
    if (this.pendingDuplicateRecheck && this.objectHadDuplicates) {
      btnEl.hidden = false;
      btnEl.disabled = this.duplicateRecheckLoading;
      btnEl.textContent = this.duplicateRecheckLoading ? 'Проверка…' : 'Проверить на дубликаты';
      statusEl.hidden = true;
      statusEl.onclick = null;
      return;
    }
    btnEl.hidden = true;
    if (this.duplicateRecheckStatus === 'clean') {
      statusEl.hidden = false;
      statusEl.textContent = 'Дубликатов нет';
      statusEl.className = 'props-duplicate-status props-duplicate-status--ok';
      statusEl.onclick = null;
      return;
    }
    const dupCount = App.getDuplicateRelatedRows(obj.Id).length;
    if (!this.pendingDuplicateRecheck && dupCount > 0) {
      statusEl.hidden = false;
      statusEl.textContent = `Дубликатов: ${dupCount}`;
      statusEl.className = 'props-duplicate-status props-duplicate-status--alarm';
      statusEl.title = 'Открыть вкладку «Дубликаты»';
      statusEl.onclick = () => this.switchTab('duplicates');
      return;
    }
    statusEl.hidden = true;
    statusEl.onclick = null;
  },

  trackPatch(promise) {
    const tracked = Promise.resolve(promise);
    this._pendingPatchPromise = tracked.finally(() => {
      if (this._pendingPatchPromise === tracked) {
        this._pendingPatchPromise = null;
      }
    });
    return this._pendingPatchPromise;
  },

  async flushPendingEdits() {
    const pending = [...this._debounceTimers.values()];
    for (const entry of pending) {
      clearTimeout(entry.timer);
    }
    this._debounceTimers.clear();
    for (const entry of pending) {
      entry.fn();
    }
    if (this._pendingPatchPromise) {
      await this._pendingPatchPromise;
    }
  },

  syncActiveTabFromDom() {
    const activeBtn = this.tabs?.querySelector('.props-tab--active');
    if (activeBtn?.dataset?.tab) {
      this.activeTab = activeBtn.dataset.tab;
    }
  },

  switchTab(tabName) {
    if (!tabName) return;
    this.activeTab = tabName;
    this.tabs.querySelectorAll('.props-tab').forEach(t => {
      t.classList.toggle('props-tab--active', t.dataset.tab === tabName);
    });
    if (this.currentObject) this.render(this.currentObject);
  },

  async loadObject(solutionId, objectId) {
    try {
      const cached = App.objectDetailsCache?.get(objectId);
      if (cached) {
        this.currentObject = cached;
      } else {
        const details = await API.getObject(solutionId, objectId, App.sessionId);
        this.currentObject = details;
        App.objectDetailsCache?.set(objectId, details);
      }
      this.resetDuplicateRecheckState();
      this.refreshObjectDuplicateState(objectId);
      this.syncActiveTabFromDom();
      this.render(this.currentObject);
      this.tabs.hidden = false;
      this.updateDuplicateTabBadge(objectId);
    } catch (e) {
      this.showEmpty('Ошибка загрузки объекта');
    }
  },

  showSolutionMeta(meta) {
    this.currentObject = null;
    this.resetDuplicateRecheckState();
    this.tabs.hidden = true;
    this.updateTitleRow(null);
    this.title.textContent = 'Решение';
    this.body.innerHTML = `
      <div class="props__header"><h4>${this.esc(meta.name)}</h4></div>
      <div class="props-section">
        <div class="props-field"><span class="props-field__label">Компания</span><span class="props-field__value">${this.esc(meta.company)}</span></div>
        <div class="props-field"><span class="props-field__label">Описание</span><span class="props-field__value">${this.esc(meta.description)}</span></div>
      </div>`;
  },

  showSchemeInfo(scheme) {
    this.currentObject = null;
    this.resetDuplicateRecheckState();
    this.tabs.hidden = true;
    this.updateTitleRow(null);
    this.title.textContent = 'Схема';
    const count = (scheme.GraphicObject || scheme.graphic_object || []).length;
    const types = {};
    for (const obj of scheme.GraphicObject || []) {
      const t = obj.TechnologyObject?.[0]?.Template || 'Unknown';
      types[t] = (types[t] || 0) + 1;
    }
    const typeList = Object.entries(types).map(([t, c]) => `${t}: ${c}`).join(', ');
    this.body.innerHTML = `
      <div class="props__header"><h4>${this.esc(scheme.Name)}</h4><span class="badge badge--template">${this.esc(scheme.Template)}</span></div>
      <div class="props-section">
        <div class="props-field"><span class="props-field__label">Объектов</span><span class="props-field__value">${count}</span></div>
        <div class="props-field"><span class="props-field__label">Типы</span><span class="props-field__value">${this.esc(typeList)}</span></div>
      </div>`;
  },

  render(details) {
    const obj = details?.graphic_object;
    const to = details?.technology_objects?.[0] || obj?.TechnologyObject?.[0];
    if (!obj) {
      this.body.innerHTML = '<p class="props-empty">Объект не выбран</p>';
      this.updateTitleRow(null);
      return;
    }
    this.updateTitleRow(obj, to);
    this.syncActiveTabFromDom();

    switch (this.activeTab) {
      case 'characteristics': this.renderCharacteristics(obj, to); break;
      case 'protections': this.renderProtections(obj, details.automations); break;
      case 'commands': this.renderCommands(obj, details.commands); break;
      case 'duplicates': this.renderDuplicates(details); break;
      default: this.renderMainProperties(obj, to); break;
    }
    this.updateDuplicateTabBadge(obj.Id);
  },

  duplicatesColumnDefs() {
    return [
      { key: 'tag', label: 'Tag', width: 100, minWidth: 64, mono: true },
      { key: 'scheme', label: 'Схема', width: 120, minWidth: 64 },
      { key: 'object_id', label: 'Id', width: 120, minWidth: 64, mono: true },
      { key: 'object_name', label: 'Имя', width: 100, minWidth: 64 },
      { key: 'object_type', label: 'Тип', width: 90, minWidth: 64 },
      { key: 'source', label: 'Источник Tag', width: 140, minWidth: 64 },
    ];
  },

  updateDuplicateTabBadge(objectId) {
    const tab = this.tabs?.querySelector('[data-tab="duplicates"]');
    if (!tab) return;
    const count = App.getDuplicateRelatedRows(objectId).length;
    tab.textContent = count > 0 ? `Дубликаты (${count})` : 'Дубликаты';
    tab.classList.toggle('props-tab--alarm', count > 0);
  },

  renderDuplicates(details) {
    const obj = details?.graphic_object;
    const objectId = obj?.Id;
    const rows = App.getDuplicateRelatedRows(objectId);
    if (!App.duplicateRows?.length) {
      this.body.innerHTML = '<p class="props-empty">Запустите поиск дубликатов Tag</p>';
      return;
    }
    if (!App.duplicateCompliance?.violationsByObjectId?.[String(objectId)]) {
      this.body.innerHTML = '<p class="props-empty">Дубликаты Tag для объекта не найдены</p>';
      return;
    }
    if (!rows.length) {
      this.body.innerHTML = '<p class="props-empty">Дубликаты Tag для объекта не найдены</p>';
      return;
    }

    const columnDefs = this.duplicatesColumnDefs();
    const layout = TableLayout.loadLayout('propsTableColumns:duplicates', columnDefs);
    this.body.innerHTML = '';
    const wrap = document.createElement('div');
    wrap.className = 'props-table-wrap';
    this.body.appendChild(wrap);

    const { table, tbody } = TableLayout.buildTable({ columnDefs, layout });
    for (const row of rows) {
      const tr = document.createElement('tr');
      tr.className = 'props-table__row--nav';
      tr.dataset.schemeId = row.scheme_xml_id || '';
      tr.dataset.objectId = row.object_id || '';
      if (String(row.object_id) === String(objectId)) {
        tr.classList.add('props-table__row--self');
      }
      for (const key of layout.order) {
        const def = columnDefs.find(c => c.key === key);
        if (!def) continue;
        const td = document.createElement('td');
        if (def.mono) td.classList.add('props-table__cell--mono');
        let text = '';
        switch (key) {
          case 'tag': text = row.tag || ''; break;
          case 'scheme': text = row.scheme_name || row.scheme_xml_id || ''; break;
          case 'object_id': text = row.technology_object_id || ''; break;
          case 'object_name': text = row.object_name || ''; break;
          case 'object_type': text = row.object_type || ''; break;
          case 'source': text = App.duplicateSourceLabel(row); break;
          default: text = '';
        }
        td.textContent = text === '—' ? '—' : (text || '—');
        if (text && text !== '—') td.title = text;
        if (key === 'tag' && text) td.style.color = 'var(--state-alarm)';
        tr.appendChild(td);
      }
      tr.addEventListener('click', (e) => {
        e.stopPropagation();
        table.querySelectorAll('.props-table__row--active').forEach(el => el.classList.remove('props-table__row--active'));
        tr.classList.add('props-table__row--active');
        App.navigateToDuplicateRow(row, tr);
      });
      tbody.appendChild(tr);
    }
    wrap.appendChild(table);
    TableLayout.setupColumns(table, layout, columnDefs, 'propsTableColumns:duplicates', {
      onLayoutChange: () => this.renderDuplicates(details),
    });
  },

  mainColumnDefs() {
    return [
      { key: 'Name', label: 'Имя', width: 120, minWidth: 64 },
      { key: 'Description', label: 'Описание', width: 140, minWidth: 64 },
      { key: 'Tag', label: 'Tag', width: 120, minWidth: 64 },
      { key: 'ExternalCode', label: 'ExternalCode', width: 140, minWidth: 64 },
      { key: 'X', label: 'X', width: 72, minWidth: 48 },
      { key: 'Y', label: 'Y', width: 72, minWidth: 48 },
      { key: 'Angle', label: 'Угол', width: 72, minWidth: 48 },
    ];
  },

  characteristicsColumnDefs() {
    return [
      { key: 'label', label: 'Параметр', width: 140, minWidth: 64 },
      { key: 'value', label: 'Значение', width: 120, minWidth: 64 },
      { key: 'tag', label: 'tag', width: 100, minWidth: 64 },
    ];
  },

  entityColumnDefs() {
    return [
      { key: 'name', label: 'Имя', width: 140, minWidth: 64 },
      { key: 'entity_tag', label: 'Tag', width: 120, minWidth: 64 },
      { key: 'prop_label', label: 'Параметр', width: 120, minWidth: 64 },
      { key: 'value', label: 'Значение', width: 100, minWidth: 64 },
      { key: 'prop_tag', label: 'tag', width: 100, minWidth: 64 },
    ];
  },

  buildObjectMetaHeader(obj, to) {
    const typeName = to?.Template || Renderer.getTemplate(obj) || '—';
    const toId = (to?.Id != null && String(to.Id).trim() !== '') ? String(to.Id).trim() : '';
    const shownId = toId || '—';
    const header = document.createElement('div');
    header.className = 'props-object-meta';
    const typeEl = document.createElement('span');
    typeEl.className = 'badge badge--template props-object-meta__type';
    typeEl.textContent = typeName;
    typeEl.title = typeName;
    const idEl = document.createElement('span');
    idEl.className = 'props-object-meta__id';
    const idLabel = document.createElement('span');
    idLabel.className = 'props-object-meta__id-label';
    idLabel.textContent = 'Id';
    const idValue = document.createElement('span');
    idValue.textContent = ` ${shownId}`;
    idEl.appendChild(idLabel);
    idEl.appendChild(idValue);
    idEl.title = `Id: ${shownId}`;
    header.appendChild(typeEl);
    header.appendChild(idEl);
    return header;
  },

  renderMainProperties(obj, to) {
    const locked = obj.IsLocked === 'True';
    const storageKey = 'propsTableColumns:main';
    const columnDefs = this.mainColumnDefs();
    const layout = TableLayout.loadLayout(storageKey, columnDefs);
    const anchor = obj.AnchorPoint?.[0] || {};

    this.body.innerHTML = '';
    this.body.appendChild(this.buildObjectMetaHeader(obj, to));
    const wrap = document.createElement('div');
    wrap.className = 'props-table-wrap';
    this.body.appendChild(wrap);

    const { table, tbody } = TableLayout.buildTable({ columnDefs, layout });
    const tr = document.createElement('tr');
    tr.dataset.rowKey = 'main';

    const values = {
      Name: to?.Name || obj.Name || '',
      Description: to?.Description || obj.Description || '',
      Tag: to?.Tag || obj.Tag || '',
      ExternalCode: to?.ExternalCode || obj.ExternalCode || '',
      X: anchor.X || '0',
      Y: anchor.Y || '0',
      Angle: obj.Angle || '0',
    };

    const fieldMeta = {
      Name: { type: 'to-field', key: 'Name' },
      Description: { type: 'to-field', key: 'Description' },
      Tag: { type: 'to-field', key: 'Tag' },
      ExternalCode: { type: 'to-field', key: 'ExternalCode' },
      X: { type: 'anchor', key: 'X' },
      Y: { type: 'anchor', key: 'Y' },
      Angle: { type: 'attr', key: 'Angle' },
    };

    for (const key of layout.order) {
      const td = document.createElement('td');
      const val = values[key] ?? '';
      const positionReadonly = locked && (key === 'X' || key === 'Y' || key === 'Angle');
      if (positionReadonly) {
        td.textContent = val || '—';
        if (val) td.title = val;
      } else {
        const input = this.createEditableCell(val, () => this.debounce(`main:${key}`, () => {
          this.applyMainFieldPatch(obj, fieldMeta[key].type, fieldMeta[key].key, input.value);
        }), {
          fieldType: fieldMeta[key]?.type || '',
          fieldKey: fieldMeta[key]?.key || key,
        });
        td.appendChild(input);
      }
      tr.appendChild(td);
    }
    tbody.appendChild(tr);
    wrap.appendChild(table);
    TableLayout.setupColumns(table, layout, columnDefs, storageKey, {
      onLayoutChange: () => this.renderMainProperties(obj, to),
    });
  },

  applyMainFieldPatch(obj, type, key, newVal) {
    this.applyPatch(obj, (patch) => {
      if (type === 'anchor') {
        if (!patch.AnchorPoint) patch.AnchorPoint = [{ X: '0', Y: '0' }];
        if (key === 'X') patch.AnchorPoint[0].X = newVal;
        if (key === 'Y') patch.AnchorPoint[0].Y = newVal;
      } else if (type === 'attr') {
        patch.Angle = newVal;
      } else if (type === 'to-field' && patch.TechnologyObject?.[0]) {
        patch.TechnologyObject[0][key] = newVal;
        if (key === 'Tag') patch.Tag = newVal;
        if (key === 'Name') patch.Name = newVal;
        if (key === 'Description') patch.Description = newVal;
        if (key === 'ExternalCode') patch.ExternalCode = newVal;
      }
    });
  },

  renderCharacteristics(obj, to) {
    if (!to?.TechnologyProperty || to.TechnologyProperty.length === 0) {
      this.body.innerHTML = '<p class="props-empty">Характеристики не найдены</p>';
      return;
    }
    const storageKey = 'propsTableColumns:characteristics';
    const columnDefs = this.characteristicsColumnDefs();
    const layout = TableLayout.loadLayout(storageKey, columnDefs);

    const rows = to.TechnologyProperty.map((p, idx) => ({
      propId: p.Id,
      label: p.Template || p.Name || p.Description || 'Параметр',
      value: this.propDisplayValue(p),
      tag: p.Tag || '',
      _xmlIndex: idx,
      _sortName: (p.Template || p.Name || p.Description || '').toLowerCase(),
      _sortTag: (p.Tag || '').toLowerCase(),
    }));
    this.sortRows(rows);

    this.body.innerHTML = '';
    this.body.appendChild(this.buildSortToolbar());
    const wrap = document.createElement('div');
    wrap.className = 'props-table-wrap';
    this.body.appendChild(wrap);

    const { table, tbody } = TableLayout.buildTable({ columnDefs, layout });

    for (const row of rows) {
      const tr = document.createElement('tr');
      tr.dataset.rowKey = row.propId;
      tr.dataset.propId = row.propId;

      for (const key of layout.order) {
        const td = document.createElement('td');
        const val = row[key] ?? '';
        if (key === 'label') {
          td.textContent = val || '—';
          if (val) td.title = val;
        } else {
          const input = this.createEditableCell(val, () => this.debounce(`char:${row.propId}:${key}`, () => {
            this.applyCharacteristicPatch(obj, row.propId, key, input.value);
          }), { propId: row.propId, colKey: key });
          td.appendChild(input);
        }
        tr.appendChild(td);
      }
      tbody.appendChild(tr);
    }

    wrap.appendChild(table);
    TableLayout.setupColumns(table, layout, columnDefs, storageKey, {
      onLayoutChange: () => this.renderCharacteristics(obj, to),
    });
  },

  applyCharacteristicPatch(obj, propId, colKey, newVal) {
    this.applyPatch(obj, (patch) => {
      const props = patch.TechnologyObject?.[0]?.TechnologyProperty;
      if (!props) return;
      for (const p of props) {
        if (p.Id !== propId) continue;
        if (colKey === 'tag') {
          p.Tag = newVal;
        } else if (colKey === 'value') {
          if (p.RealValue !== undefined) p.RealValue = newVal;
          else if (p.IntegerValue !== undefined) p.IntegerValue = parseInt(newVal, 10) || 0;
          else if (p.BooleanValue !== undefined) p.BooleanValue = newVal;
          else p.StringValue = newVal;
        }
      }
    });
  },

  buildEntityRows(entities, entityType) {
    const rows = [];
    entities.forEach((entity, entityIdx) => {
      const props = entity.TechnologyProperty || [];
      const base = {
        entityId: entity.Id,
        entityType,
        name: entity.Description || entity.Name || '—',
        entity_tag: entity.Tag || '',
        _xmlIndex: entityIdx,
        _sortName: (entity.Description || entity.Name || '').toLowerCase(),
        _sortTag: (entity.Tag || '').toLowerCase(),
      };
      if (props.length === 0) {
        rows.push({ ...base, propId: '', prop_label: '—', value: '', prop_tag: '' });
        return;
      }
      props.forEach((p, propIdx) => {
        rows.push({
          ...base,
          propId: p.Id,
          prop_label: p.Template || p.Name || p.Description || 'Параметр',
          value: this.propDisplayValue(p),
          prop_tag: p.Tag || '',
          _xmlIndex: entityIdx * 1000 + propIdx,
        });
      });
    });
    return rows;
  },

  renderEntityTable(obj, entities, entityType, storageKey, emptyMsg) {
    if (!entities || entities.length === 0) {
      this.body.innerHTML = `<p class="props-empty">${this.esc(emptyMsg)}</p>`;
      return;
    }
    const columnDefs = this.entityColumnDefs();
    const layout = TableLayout.loadLayout(storageKey, columnDefs);
    const rows = this.buildEntityRows(entities, entityType);
    this.sortRows(rows);

    this.body.innerHTML = '';
    this.body.appendChild(this.buildSortToolbar());
    const wrap = document.createElement('div');
    wrap.className = 'props-table-wrap';
    this.body.appendChild(wrap);

    const { table, tbody } = TableLayout.buildTable({ columnDefs, layout });

    for (const row of rows) {
      const tr = document.createElement('tr');
      tr.dataset.rowKey = `${row.entityId}:${row.propId || 'none'}`;
      tr.dataset.entityId = row.entityId;
      tr.dataset.entityType = row.entityType;
      if (row.propId) tr.dataset.propId = row.propId;

      for (const key of layout.order) {
        const td = document.createElement('td');
        const val = row[key] ?? '';
        const editable = key === 'entity_tag' || key === 'value' || key === 'prop_tag';
        if (!editable) {
          td.textContent = val || '—';
          if (val) td.title = val;
        } else {
          const input = this.createEditableCell(val, () => this.debounce(`entity:${row.entityId}:${row.propId}:${key}`, () => {
            this.applyEntityFieldPatch(row.entityType, row.entityId, row.propId, key, input.value);
          }), {
            entityId: row.entityId,
            entityType: row.entityType,
            colKey: key,
            ...(row.propId ? { propId: row.propId } : {}),
          });
          td.appendChild(input);
        }
        tr.appendChild(td);
      }
      tbody.appendChild(tr);
    }

    wrap.appendChild(table);
    TableLayout.setupColumns(table, layout, columnDefs, storageKey, {
      onLayoutChange: () => {
        if (entityType === 'control_command') {
          this.renderCommands(obj, this.currentObject?.commands);
        } else {
          this.renderProtections(obj, this.currentObject?.automations);
        }
      },
    });
  },

  renderProtections(obj, automations) {
    this.renderEntityTable(obj, automations, 'automation', 'propsTableColumns:protections', 'Защиты не найдены');
  },

  renderCommands(obj, commands) {
    this.renderEntityTable(obj, commands, 'control_command', 'propsTableColumns:commands', 'Команды не найдены');
  },

  applyEntityFieldPatch(entityType, entityId, propId, colKey, newVal) {
    const listKey = entityType === 'control_command' ? 'commands' : 'automations';
    const entities = this.currentObject?.[listKey];
    if (!entities) return;
    const entity = entities.find(e => e.Id === entityId);
    if (!entity) return;

    const patch = JSON.parse(JSON.stringify(entity));
    if (colKey === 'entity_tag') {
      patch.Tag = newVal;
    } else if (propId && patch.TechnologyProperty) {
      for (const p of patch.TechnologyProperty) {
        if (p.Id !== propId) continue;
        if (colKey === 'prop_tag') p.Tag = newVal;
        else if (colKey === 'value') {
          if (p.RealValue !== undefined) p.RealValue = newVal;
          else if (p.IntegerValue !== undefined) p.IntegerValue = parseInt(newVal, 10) || 0;
          else if (p.BooleanValue !== undefined) p.BooleanValue = newVal;
          else p.StringValue = newVal;
        }
      }
    }

    this.applyEntityPatch(entityType, entityId, patch);
    this.onTagFieldsChanged();
  },

  applyEntityPatch(entityType, entityId, patch) {
    if (this.onEntityPatch) {
      this.trackPatch(this.onEntityPatch(entityType, entityId, patch));
    }
    if (!this.currentObject) return;
    const listKey = entityType === 'control_command' ? 'commands' : 'automations';
    const list = this.currentObject[listKey];
    if (!list) return;
    const idx = list.findIndex(e => e.Id === entityId);
    if (idx >= 0) list[idx] = patch;
  },

  sortTabKey() {
    if (this.activeTab === 'characteristics' || this.activeTab === 'commands' || this.activeTab === 'protections') {
      return this.activeTab;
    }
    return null;
  },

  getSortState() {
    const tabKey = this.sortTabKey();
    if (!tabKey) return { field: null, direction: null };
    return this.sortState[tabKey] || { field: null, direction: null };
  },

  cycleSortField(fieldKey) {
    const tabKey = this.sortTabKey();
    if (!tabKey) return;
    const state = { ...(this.sortState[tabKey] || { field: null, direction: null }) };
    if (state.field === fieldKey) {
      if (state.direction === 'asc') {
        state.direction = 'desc';
      } else if (state.direction === 'desc') {
        state.field = null;
        state.direction = null;
      } else {
        state.field = fieldKey;
        state.direction = 'asc';
      }
    } else {
      state.field = fieldKey;
      state.direction = 'asc';
    }
    this.sortState[tabKey] = state;
  },

  sortButtonLabel(baseLabel, fieldKey) {
    const state = this.getSortState();
    if (state.field !== fieldKey || !state.direction) return baseLabel;
    return `${baseLabel} ${state.direction === 'asc' ? '↑' : '↓'}`;
  },

  buildSortToolbar() {
    const toolbar = document.createElement('div');
    toolbar.className = 'props-table-toolbar';
    const label = document.createElement('span');
    label.className = 'props-table-toolbar__label';
    label.textContent = 'Сортировка:';
    toolbar.appendChild(label);

    const modes = [
      { key: 'xml', label: 'Как в XML' },
      { key: 'name', label: 'По имени' },
      { key: 'tag', label: 'По tag' },
    ];
    for (const mode of modes) {
      const btn = document.createElement('button');
      btn.type = 'button';
      const state = this.getSortState();
      btn.className = 'props-table-sort';
      if (state.field === mode.key && state.direction === 'asc') {
        btn.classList.add('props-table-sort--active', 'props-table-sort--asc');
      } else if (state.field === mode.key && state.direction === 'desc') {
        btn.classList.add('props-table-sort--active', 'props-table-sort--desc');
      }
      btn.textContent = this.sortButtonLabel(mode.label, mode.key);
      btn.dataset.sort = mode.key;
      btn.title = state.field === mode.key && state.direction === 'asc'
        ? 'Сортировка по возрастанию. Нажмите для убывания.'
        : state.field === mode.key && state.direction === 'desc'
          ? 'Сортировка по убыванию. Нажмите для отключения.'
          : `Сортировать: ${mode.label}`;
      btn.addEventListener('click', () => {
        this.cycleSortField(mode.key);
        if (this.currentObject) this.render(this.currentObject);
      });
      toolbar.appendChild(btn);
    }
    return toolbar;
  },

  sortRows(rows) {
    const { field, direction } = this.getSortState();
    if (!field || !direction) {
      rows.sort((a, b) => (a._xmlIndex ?? 0) - (b._xmlIndex ?? 0));
      return;
    }
    const mul = direction === 'desc' ? -1 : 1;
    if (field === 'name') {
      rows.sort((a, b) => {
        const na = a._sortName || a.label || a.name || '';
        const nb = b._sortName || b.label || b.name || '';
        return mul * na.localeCompare(nb, 'ru');
      });
      return;
    }
    if (field === 'tag') {
      rows.sort((a, b) => {
        const ta = a._sortTag || a.tag || a.entity_tag || a.prop_tag || '';
        const tb = b._sortTag || b.tag || b.entity_tag || b.prop_tag || '';
        return mul * ta.localeCompare(tb, 'ru');
      });
      return;
    }
    rows.sort((a, b) => mul * ((a._xmlIndex ?? 0) - (b._xmlIndex ?? 0)));
  },

  createEditableCell(value, onInput, dataset = {}) {
    const el = document.createElement('textarea');
    el.className = 'props-field__input props-table__input props-table__textarea';
    el.rows = 1;
    el.value = value;
    for (const [k, v] of Object.entries(dataset)) {
      el.dataset[k] = v;
    }
    const resize = () => {
      el.style.height = 'auto';
      el.style.height = `${el.scrollHeight}px`;
    };
    el.addEventListener('click', (e) => e.stopPropagation());
    el.addEventListener('input', () => {
      resize();
      onInput();
    });
    requestAnimationFrame(resize);
    return el;
  },

  debounce(key, fn, delay = 300) {
    if (this._debounceTimers.has(key)) {
      clearTimeout(this._debounceTimers.get(key).timer);
    }
    const entry = { fn, timer: null };
    entry.timer = setTimeout(() => {
      this._debounceTimers.delete(key);
      entry.fn();
    }, delay);
    this._debounceTimers.set(key, entry);
  },

  propDisplayValue(p) {
    if (p.RealValue !== undefined && p.RealValue !== null && p.RealValue !== '') return String(p.RealValue);
    if (p.StringValue) return p.StringValue;
    if (p.IntegerValue !== undefined && p.IntegerValue !== null) return String(p.IntegerValue);
    if (p.BooleanValue) return p.BooleanValue;
    if (p.PointsValue) return p.PointsValue;
    return '';
  },

  applyPatch(obj, mutator) {
    const base = this.currentObject?.graphic_object || obj;
    const patch = JSON.parse(JSON.stringify(base));
    mutator(patch);
    if (this.onPatch) {
      this.trackPatch(this.onPatch(obj.Id, patch));
      if (this.currentObject) {
        this.currentObject.graphic_object = patch;
        if (patch.TechnologyObject?.[0]) {
          this.currentObject.technology_objects = patch.TechnologyObject;
        }
        this.onTagFieldsChanged();
      }
    }
  },

  showEmpty(msg) {
    this.tabs.hidden = true;
    this.resetDuplicateRecheckState();
    this.updateTitleRow(null);
    this.body.innerHTML = `<p class="props-empty">${this.esc(msg)}</p>`;
  },

  esc(s) {
    const d = document.createElement('div');
    d.textContent = s || '';
    return d.innerHTML;
  },
};
