const Renderer = {
  displaySettings: {},
  complianceRules: [],
  zoom: 1,
  panX: 0,
  panY: 0,
  baseViewBox: null,
  layoutOffset: { x: 0, y: 0 },
  layoutScale: 1,
  objectSymbolScale: 1.2,
  schemeOrigin: { x: 0, y: 0 },
  _renderJob: 0,
  labelFontSize: 11,
  labelOffsets: new Map(),
  showLabels: false,
  labelsEnabled: true,

  cancelRender() {
    this._renderJob += 1;
  },

  captureBaseViewBox(svgEl) {
    const vb = svgEl.viewBox.baseVal;
    this.baseViewBox = { x: vb.x, y: vb.y, w: vb.width, h: vb.height };
    this.zoom = 1;
  },

  getZoomLevel(svgEl) {
    if (!this.baseViewBox || !svgEl?.viewBox?.baseVal?.width) return 1;
    return this.baseViewBox.w / svgEl.viewBox.baseVal.width;
  },

  getZoomPercent(svgEl) {
    return Math.round(this.getZoomLevel(svgEl) * 100);
  },

  resetViewBox(svgEl) {
    if (!this.baseViewBox) return;
    const b = this.baseViewBox;
    svgEl.setAttribute('viewBox', `${b.x} ${b.y} ${b.w} ${b.h}`);
    this.zoom = 1;
  },

  getViewBoxScreenScale(svgEl) {
    const vb = svgEl.viewBox.baseVal;
    const rect = svgEl.getBoundingClientRect();
    if (!rect.width) return 1;
    return vb.width / rect.width;
  },

  zoomViewBoxAt(svgEl, clientX, clientY, factor) {
    const vb = svgEl.viewBox.baseVal;
    const rect = svgEl.getBoundingClientRect();
    if (!rect.width || !rect.height) return;
    const scaleX = vb.width / rect.width;
    const scaleY = vb.height / rect.height;
    const px = vb.x + (clientX - rect.left) * scaleX;
    const py = vb.y + (clientY - rect.top) * scaleY;
    const nextZoom = this.getZoomLevel(svgEl) * factor;
    if (nextZoom > 5 || nextZoom < 0.1) return;
    const newW = vb.width / factor;
    const newH = vb.height / factor;
    const newX = px - (px - vb.x) / factor;
    const newY = py - (py - vb.y) / factor;
    svgEl.setAttribute('viewBox', `${newX} ${newY} ${newW} ${newH}`);
    this.zoom = nextZoom;
  },

  panViewBox(svgEl, dxScreen, dyScreen) {
    const vb = svgEl.viewBox.baseVal;
    const rect = svgEl.getBoundingClientRect();
    if (!rect.width) return;
    const dx = dxScreen * (vb.width / rect.width);
    const dy = dyScreen * (vb.height / rect.height);
    svgEl.setAttribute('viewBox', `${vb.x - dx} ${vb.y - dy} ${vb.width} ${vb.height}`);
  },

  centerObjectInView(svgEl, objectId) {
    if (!svgEl || !objectId) return false;
    const el = svgEl.querySelector(`[data-id="${CSS.escape(objectId)}"]`);
    if (!el) return false;
    let cx;
    let cy;
    try {
      const bbox = el.getBBox();
      cx = bbox.x + bbox.width / 2;
      cy = bbox.y + bbox.height / 2;
    } catch (_) {
      return false;
    }
    const vb = svgEl.viewBox.baseVal;
    if (!vb.width || !vb.height) return false;
    const newX = cx - vb.width / 2;
    const newY = cy - vb.height / 2;
    svgEl.setAttribute('viewBox', `${newX} ${newY} ${vb.width} ${vb.height}`);
    return true;
  },

  async loadDisplaySettings() {
    try {
      const settings = await API.getDisplaySettings();
      this.displaySettings = {};
      for (const s of settings) {
        this.displaySettings[s.template] = s;
      }
    } catch (_) {
      this.displaySettings = {};
    }
  },

  async loadComplianceRules() {
    try {
      const rules = await API.getComplianceRules();
      const all = Array.isArray(rules) ? rules : [];
      this.complianceRules = all.filter(r => r.text_condition !== 'duplicate_tag');
      this.duplicateRule = all.find(r => r.text_condition === 'duplicate_tag') || null;
    } catch (_) {
      this.complianceRules = [];
      this.duplicateRule = null;
    }
  },

  getTemplate(obj) {
    if (obj.TechnologyObject && obj.TechnologyObject.length > 0) {
      return obj.TechnologyObject[0].Template || '';
    }
    return '';
  },

  parseCoord(val) {
    if (!val) return 0;
    return parseFloat(String(val).replace(',', '.')) || 0;
  },

  getRawAnchor(obj) {
    if (!obj.AnchorPoint || obj.AnchorPoint.length === 0) return { x: 0, y: 0 };
    return {
      x: this.parseCoord(obj.AnchorPoint[0].X),
      y: this.parseCoord(obj.AnchorPoint[0].Y),
    };
  },

  rawToScreen(raw) {
    const ox = this.schemeOrigin.x;
    const oy = this.schemeOrigin.y;
    return {
      x: (raw.x - ox) * this.layoutScale + ox + this.layoutOffset.x,
      y: (raw.y - oy) * this.layoutScale + oy + this.layoutOffset.y,
    };
  },

  toScreenCoord(x, y) {
    return this.rawToScreen({ x, y });
  },

  getAnchor(obj) {
    const template = this.getTemplate(obj);
    return this.applyTypeOffset(this.rawToScreen(this.getRawAnchor(obj)), template);
  },

  getRawAbsolutePoints(obj) {
    const anchor = this.getRawAnchor(obj);
    const points = [];
    if (!obj.Points) return [{ x: anchor.x, y: anchor.y }];
    for (const block of obj.Points) {
      if (!block.Point) continue;
      for (const p of block.Point) {
        points.push({
          x: anchor.x + this.parseCoord(p.X),
          y: anchor.y + this.parseCoord(p.Y),
        });
      }
    }
    return points.length ? points : [{ x: anchor.x, y: anchor.y }];
  },

  getAbsolutePoints(obj) {
    const template = this.getTemplate(obj);
    return this.getRawAbsolutePoints(obj).map(p => this.applyTypeOffset(this.rawToScreen(p), template));
  },

  setSchemeOrigin(objects) {
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    for (const obj of objects) {
      const pts = this.getRawAbsolutePoints(obj);
      for (const p of pts) {
        minX = Math.min(minX, p.x);
        minY = Math.min(minY, p.y);
        maxX = Math.max(maxX, p.x);
        maxY = Math.max(maxY, p.y);
      }
      const anchor = this.getRawAnchor(obj);
      minX = Math.min(minX, anchor.x - 60);
      minY = Math.min(minY, anchor.y - 60);
      maxX = Math.max(maxX, anchor.x + 60);
      maxY = Math.max(maxY, anchor.y + 60);
    }
    if (!isFinite(minX) || !isFinite(minY)) {
      this.schemeOrigin = { x: 0, y: 0 };
      return;
    }
    this.schemeOrigin = { x: minX, y: minY };
  },

  getTypeOffset(template) {
    const style = this.getStyle(template);
    return {
      x: Number(style.offsetX) || 0,
      y: Number(style.offsetY) || 0,
    };
  },

  applyTypeOffset(point, template) {
    const off = this.getTypeOffset(template);
    return { x: point.x + off.x, y: point.y + off.y };
  },

  isPipeLike(template) {
    return ['Pipe', 'Bridge', 'Pipeline'].includes(template);
  },

  isFlagTrue(val) {
    return String(val || '').toLowerCase() === 'true';
  },

  getObjectPose(obj) {
    const angle = this.parseCoord(obj.Angle);
    let sx = this.isFlagTrue(obj.IsHorizontalReflected) ? -1 : 1;
    if (this.isFlagTrue(obj.IsReversed)) sx *= -1;
    const sy = this.isFlagTrue(obj.IsVerticalReflected) ? -1 : 1;
    return { angle, sx, sy };
  },

  applyLocalPose(dx, dy, pose) {
    const qx = dx * pose.sx;
    const qy = dy * pose.sy;
    const rad = pose.angle * Math.PI / 180;
    const c = Math.cos(rad);
    const sn = Math.sin(rad);
    return {
      x: qx * c - qy * sn,
      y: qx * sn + qy * c,
    };
  },

  drawInstIcon(g, template, anchor) {
    const href = typeof InstIcons !== 'undefined' ? InstIcons.url(template) : '';
    if (!href) return false;
    const size = this.symbolSize(InstIcons.NATIVE_SIZE);
    const img = document.createElementNS('http://www.w3.org/2000/svg', 'image');
    img.setAttribute('href', href);
    img.setAttributeNS('http://www.w3.org/1999/xlink', 'href', href);
    img.setAttribute('x', String(anchor.x - size / 2));
    img.setAttribute('y', String(anchor.y - size / 2));
    img.setAttribute('width', String(size));
    img.setAttribute('height', String(size));
    img.setAttribute('class', 'scheme-object__inst-icon');
    img.setAttribute('preserveAspectRatio', 'xMidYMid meet');
    g.appendChild(img);
    return true;
  },

  applyShapePose(shapeG, obj, template, anchor) {
    if (this.isPipeLike(template)) return;
    const pose = this.getObjectPose(obj);
    const letter = this.getRenderer(template) === 'letter';
    const sx = letter ? 1 : pose.sx;
    const sy = letter ? 1 : pose.sy;
    const angle = pose.angle;
    if (!angle && sx === 1 && sy === 1) return;
    const ax = anchor.x;
    const ay = anchor.y;
    shapeG.setAttribute('transform', `translate(${ax} ${ay}) rotate(${angle}) scale(${sx} ${sy}) translate(${-ax} ${-ay})`);
  },

  countRelativePoints(obj) {
    let n = 0;
    if (!obj.Points) return 0;
    for (const block of obj.Points) {
      if (!block.Point) continue;
      n += block.Point.length;
    }
    return n;
  },

  renderInstPoints(g, obj) {
    const pts = this.getAbsolutePoints(obj);
    if (pts.length < 2 || this.countRelativePoints(obj) < 2) return;
    const d = pts.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x} ${p.y}`).join(' ');
    const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
    path.setAttribute('d', d);
    path.setAttribute('fill', 'none');
    path.setAttribute('stroke', 'var(--pipe-fill)');
    path.setAttribute('stroke-width', '1.2');
    path.setAttribute('stroke-opacity', '0.55');
    path.setAttribute('stroke-linecap', 'round');
    path.setAttribute('stroke-linejoin', 'round');
    path.setAttribute('pointer-events', 'none');
    path.setAttribute('class', 'scheme-object__points');
    g.appendChild(path);
  },

  symbolSize(base) {
    return base * this.objectSymbolScale;
  },

  getPropValue(to, fieldName) {
    if (!to || !to.TechnologyProperty) return '';
    for (const p of to.TechnologyProperty) {
      if (p.Name === fieldName || p.Template === fieldName) {
        if (p.RealValue !== undefined && p.RealValue !== null && p.RealValue !== '') return String(p.RealValue);
        if (p.StringValue) return p.StringValue;
        if (p.IntegerValue !== undefined && p.IntegerValue !== null && p.IntegerValue !== '') return String(p.IntegerValue);
        if (p.BooleanValue) return p.BooleanValue;
      }
    }
    return '';
  },

  OBJECT_SCALAR_PARAMS: new Set(['Tag', 'Name', 'Description', 'Id', 'ExternalCode']),

  isObjectScalarParameter(key) {
    return this.OBJECT_SCALAR_PARAMS.has(key);
  },

  getPropTag(to, fieldName) {
    if (!to || !to.TechnologyProperty) return '';
    for (const p of to.TechnologyProperty) {
      if (p.Name === fieldName || p.Template === fieldName) {
        return p.Tag || '';
      }
    }
    return '';
  },

  resolveComplianceValue(rule, obj, to) {
    const scope = rule.parameter_scope || 'object';
    if (scope === 'property') {
      const src = rule.value_source || 'field';
      if (src === 'tag') {
        return this.getPropTag(to, rule.parameter);
      }
      if (!to || !to.TechnologyProperty) return '';
      for (const p of to.TechnologyProperty) {
        if (p.Name === rule.parameter || p.Template === rule.parameter) {
          if (p.RealValue !== undefined && p.RealValue !== null && p.RealValue !== '') return String(p.RealValue);
          if (p.StringValue) return p.StringValue;
          if (p.IntegerValue !== undefined && p.IntegerValue !== null && p.IntegerValue !== '') return String(p.IntegerValue);
          if (p.BooleanValue) return p.BooleanValue;
          if (p.PointsValue) return p.PointsValue;
        }
      }
      return '';
    }
    if (scope === 'command' || scope === 'protection') {
      const cached = typeof App !== 'undefined' ? App.objectDetailsCache?.get(obj.Id) : null;
      const entities = scope === 'command'
        ? (cached?.commands || [])
        : (cached?.automations || []);
      const entity = entities.find(e => e.Template === rule.entity_template);
      if (!entity) return '';
      return this.resolveEntityComplianceValue(rule, entity);
    }
    const src = rule.value_source || 'field';
    if (src === 'tag') {
      if (this.isObjectScalarParameter(rule.parameter)) {
        return this.resolveFieldValue(rule.parameter, obj, to);
      }
      return this.getPropTag(to, rule.parameter);
    }
    return this.resolveFieldValue(rule.parameter, obj, to);
  },

  resolveEntityComplianceValue(rule, entity) {
    const src = rule.value_source || 'field';
    const scalar = (field) => {
      switch (field) {
        case 'Tag': return entity.Tag || '';
        case 'Name': return entity.Name || '';
        case 'Description': return entity.Description || '';
        case 'Id': return entity.Id || '';
        case 'ExternalCode': return entity.ExternalCode || '';
        default: return '';
      }
    };
    if (src === 'tag') {
      if (this.isObjectScalarParameter(rule.parameter)) {
        return scalar(rule.parameter);
      }
      const props = entity.TechnologyProperty || [];
      for (const p of props) {
        if (p.Name === rule.parameter || p.Template === rule.parameter) {
          return p.Tag || '';
        }
      }
      return '';
    }
    if (this.isObjectScalarParameter(rule.parameter)) {
      return scalar(rule.parameter);
    }
    const props = entity.TechnologyProperty || [];
    for (const p of props) {
      if (p.Name === rule.parameter || p.Template === rule.parameter) {
        if (p.RealValue !== undefined && p.RealValue !== null && p.RealValue !== '') return String(p.RealValue);
        if (p.StringValue) return p.StringValue;
        if (p.IntegerValue !== undefined && p.IntegerValue !== null && p.IntegerValue !== '') return String(p.IntegerValue);
        if (p.BooleanValue) return p.BooleanValue;
      }
    }
    return '';
  },

  getSchemeObjects(scheme) {
    const objects = scheme?.GraphicObject || scheme?.graphic_object || [];
    return Array.isArray(objects) ? objects : [];
  },

  parseVisibleFields(raw) {
    if (!raw) return [];
    if (Array.isArray(raw)) return raw;
    try {
      const fields = JSON.parse(raw);
      return Array.isArray(fields) ? fields : [];
    } catch (_) {
      return [];
    }
  },

  getVisibleFields(template) {
    const s = this.displaySettings[template];
    if (!s) return [];
    return this.parseVisibleFields(s.visible_fields);
  },

  getRenderer(template) {
    const s = this.displaySettings[template];
    if (!s || !s.renderer) return 'symbol';
    if (s.renderer === 'letter' || s.renderer === 'glyph') return 'letter';
    if (s.renderer === 'pipe_strip' || s.renderer === 'pipe_line') return 'pipe';
    return 'symbol';
  },

  getStyle(template) {
    const s = this.displaySettings[template];
    if (!s || !s.style) return {};
    try {
      return typeof s.style === 'string' ? JSON.parse(s.style) : s.style;
    } catch (_) {
      return {};
    }
  },

  isTemplateVisible(template) {
    const style = this.getStyle(template);
    return style.visible !== false;
  },

  canInteractWithObject(template) {
    if (this.isPipeLike(template) && typeof App !== 'undefined' && App.schemeMovementLocked) {
      return false;
    }
    return true;
  },

  getStrokeWidth(template) {
    const style = this.getStyle(template);
    if (style.strokeWidth) return Number(style.strokeWidth);
    if (template === 'Bridge') return 4;
    if (this.isPipeLike(template)) return 3;
    return 3;
  },

  getLetterText(obj, template) {
    const style = this.getStyle(template);
    const source = style.letterSource || 'Tag';
    const to = obj.TechnologyObject?.[0];
    let val = this.resolveFieldValue(source, obj, to);
    if (!val && style.letterFallback) {
      val = this.resolveFieldValue(style.letterFallback, obj, to);
    }
    if (!val) val = template;
    return String(val).slice(0, 2).toUpperCase();
  },

  resolveFieldValue(field, obj, to) {
    switch (field) {
      case 'Tag': return to?.Tag || obj.Tag || '';
      case 'Name': return to?.Name || obj.Name || '';
      case 'Description': return to?.Description || obj.Description || '';
      case 'Id': return (to?.Id != null && String(to.Id).trim() !== '') ? String(to.Id).trim() : '';
      case 'ExternalCode': return to?.ExternalCode || obj.ExternalCode || '';
      default: return this.getPropValue(to, field);
    }
  },

  getLabelAnchor(obj, template, anchor) {
    if (this.isPipeLike(template)) {
      const pts = this.getAbsolutePoints(obj);
      if (pts.length >= 2) return pts[Math.floor(pts.length / 2)];
    }
    return anchor;
  },

  getLabelPosition(template) {
    const s = this.displaySettings[template];
    if (!s || !s.style) return 'bottom';
    try {
      const style = typeof s.style === 'string' ? JSON.parse(s.style) : s.style;
      return style.labelPosition || 'bottom';
    } catch (_) {
      return 'bottom';
    }
  },

  getLabelFontSize(vbW) {
    return Math.min(15, Math.max(10, vbW / 100));
  },

  computeLabelOffsets(objects) {
    const offsets = new Map();
    const occupied = [];
    const minDist = 63;
    const sorted = [...objects].sort((a, b) => {
      const aa = this.getLabelAnchor(a, this.getTemplate(a), this.getAnchor(a));
      const bb = this.getLabelAnchor(b, this.getTemplate(b), this.getAnchor(b));
      return aa.y - bb.y || aa.x - bb.x;
    });
    for (const obj of sorted) {
      const template = this.getTemplate(obj);
      const anchor = this.getAnchor(obj);
      const base = this.getLabelAnchor(obj, template, anchor);
      let ox = 0;
      let oy = 33;
      for (let attempt = 0; attempt < 16; attempt++) {
        const px = base.x + ox;
        const py = base.y + oy;
        const clash = occupied.some(o => Math.hypot(o.x - px, o.y - py) < minDist);
        if (!clash) {
          occupied.push({ x: px, y: py });
          offsets.set(obj.Id, { ox, oy, x: px, y: py });
          break;
        }
        const angle = attempt * 0.75;
        const radius = 33 + attempt * 10;
        ox = Math.cos(angle) * radius;
        oy = Math.sin(angle) * radius + 33;
      }
      if (!offsets.has(obj.Id)) {
        offsets.set(obj.Id, { ox: 0, oy: 33, x: base.x, y: base.y + 33 });
      }
    }
    return offsets;
  },

  renderScheme(svgEl, scheme, onSelect) {
    return this.renderSchemeAsync(svgEl, scheme, onSelect);
  },

  async renderSchemeAsync(svgEl, scheme, onSelect, onProgress) {
    this.cancelRender();
    const jobId = this._renderJob;
    svgEl.innerHTML = '';
    const objects = this.getSchemeObjects(scheme);
    const pad = 120;
    this.setSchemeOrigin(objects);

    if (objects.length === 0) {
      svgEl.setAttribute('viewBox', '0 0 800 600');
      svgEl.setAttribute('preserveAspectRatio', 'xMidYMid meet');
      this.captureBaseViewBox(svgEl);
      const text = document.createElementNS('http://www.w3.org/2000/svg', 'text');
      text.setAttribute('x', '400');
      text.setAttribute('y', '300');
      text.setAttribute('text-anchor', 'middle');
      text.setAttribute('class', 'scheme-empty-message');
      text.textContent = 'На схеме нет объектов';
      svgEl.appendChild(text);
      document.getElementById('statusObjects').textContent = 'Объектов: 0';
      return null;
    }

    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    for (const obj of objects) {
      const pts = this.getAbsolutePoints(obj);
      for (const p of pts) {
        minX = Math.min(minX, p.x);
        minY = Math.min(minY, p.y);
        maxX = Math.max(maxX, p.x);
        maxY = Math.max(maxY, p.y);
      }
      const anchor = this.getAnchor(obj);
      minX = Math.min(minX, anchor.x - 60);
      minY = Math.min(minY, anchor.y - 60);
      maxX = Math.max(maxX, anchor.x + 60);
      maxY = Math.max(maxY, anchor.y + 60);
    }

    if (!isFinite(minX) || !isFinite(minY)) {
      minX = 0; minY = 0; maxX = 800; maxY = 600;
    }

    const vbX = minX - pad;
    const vbY = minY - pad;
    const vbW = Math.max(maxX - minX + pad * 2, 200);
    const vbH = Math.max(maxY - minY + pad * 2, 200);

    svgEl.setAttribute('viewBox', `${vbX} ${vbY} ${vbW} ${vbH}`);
    svgEl.setAttribute('preserveAspectRatio', 'xMidYMid meet');
    this.captureBaseViewBox(svgEl);
    this.labelFontSize = this.getLabelFontSize(vbW);
    this.labelOffsets = this.computeLabelOffsets(objects);

    this.ensureGridPattern(svgEl);
    const gridRect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
    gridRect.setAttribute('x', vbX);
    gridRect.setAttribute('y', vbY);
    gridRect.setAttribute('width', vbW);
    gridRect.setAttribute('height', vbH);
    gridRect.setAttribute('fill', 'url(#scheme-grid-pattern)');
    gridRect.setAttribute('class', 'scheme-grid');
    svgEl.appendChild(gridRect);

    const objectsG = document.createElementNS('http://www.w3.org/2000/svg', 'g');
    objectsG.setAttribute('id', 'objectsLayer');
    svgEl.appendChild(objectsG);

    const batchSize = objects.length > 80 ? 15 : objects.length;
    for (let i = 0; i < objects.length; i += batchSize) {
      if (jobId !== this._renderJob) return null;
      await new Promise(resolve => requestAnimationFrame(resolve));
      const batch = objects.slice(i, i + batchSize);
      for (const obj of batch) {
        const template = this.getTemplate(obj);
        if (!this.isTemplateVisible(template)) continue;
        const g = this.renderObject(obj, this.labelOffsets.get(obj.Id));
        g.addEventListener('click', (e) => {
          e.stopPropagation();
          if (!this.canInteractWithObject(template)) return;
          onSelect(obj);
        });
        objectsG.appendChild(g);
      }
      if (onProgress) {
        onProgress(Math.min(100, Math.round(((i + batch.length) / objects.length) * 100)));
      }
    }

    document.getElementById('statusObjects').textContent = `Объектов: ${objects.length}`;
    return { vbX, vbY, vbW, vbH };
  },

  replaceObject(svgEl, obj, onSelect) {
    const template = this.getTemplate(obj);
    if (!this.isTemplateVisible(template)) {
      const existing = svgEl.querySelector(`[data-id="${obj.Id}"]`);
      if (existing) existing.remove();
      return;
    }
    const existing = svgEl.querySelector(`[data-id="${obj.Id}"]`);
    const offset = this.labelOffsets.get(obj.Id) || this.computeLabelOffsets([obj]).get(obj.Id);
    const g = this.renderObject(obj, offset);
    g.addEventListener('click', (e) => {
      e.stopPropagation();
      if (!this.canInteractWithObject(template)) return;
      onSelect(obj);
    });
    if (existing) existing.replaceWith(g);
    else svgEl.querySelector('#objectsLayer')?.appendChild(g);
  },

  setLabelsVisible(svgEl, visible) {
    this.showLabels = visible;
    svgEl.querySelectorAll('.scheme-object__labels').forEach(el => {
      el.style.display = visible ? '' : 'none';
    });
  },

  ensureGridPattern(svgEl) {
    let defs = svgEl.querySelector('defs');
    if (!defs) {
      defs = document.createElementNS('http://www.w3.org/2000/svg', 'defs');
      svgEl.insertBefore(defs, svgEl.firstChild);
    }
    if (defs.querySelector('#scheme-grid-pattern')) return;
    const pattern = document.createElementNS('http://www.w3.org/2000/svg', 'pattern');
    pattern.setAttribute('id', 'scheme-grid-pattern');
    pattern.setAttribute('patternUnits', 'userSpaceOnUse');
    pattern.setAttribute('width', '50');
    pattern.setAttribute('height', '50');
    const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
    path.setAttribute('d', 'M 50 0 L 0 0 0 50');
    path.setAttribute('fill', 'none');
    path.setAttribute('stroke', 'var(--grid-line)');
    path.setAttribute('stroke-width', '0.5');
    pattern.appendChild(path);
    defs.appendChild(pattern);
  },

  renderObject(obj, labelOffset) {
    const template = this.getTemplate(obj);
    const g = document.createElementNS('http://www.w3.org/2000/svg', 'g');
    g.setAttribute('class', `scheme-object scheme-object--${template.toLowerCase()}`);
    g.setAttribute('data-id', obj.Id);
    g.setAttribute('data-template', template);
    if (obj.IsLocked === 'True') g.classList.add('scheme-object--locked');
    if (this.isPipeLike(template)) g.classList.add('scheme-object--pipe-like');
    if (this.isPipeLike(template) && typeof App !== 'undefined' && App.schemeMovementLocked) {
      g.classList.add('scheme-object--pipe-no-select');
    }
    if (obj.IsVisibled === 'False') g.classList.add('scheme-object--hidden');

    const anchor = this.getAnchor(obj);

    const shapeG = document.createElementNS('http://www.w3.org/2000/svg', 'g');
    shapeG.setAttribute('class', 'scheme-object__shape');

    const renderer = this.getRenderer(template);
    const symbolCtx = {
      scale: (n) => this.symbolSize(n),
      getProp: (to, field) => this.getPropValue(to, field),
      obj,
    };
    if (this.isPipeLike(template)) {
      this.renderPipe(shapeG, obj, template);
    } else if (renderer === 'letter') {
      this.renderInstPoints(shapeG, obj);
      Symbols.drawLetter(shapeG, anchor, symbolCtx.scale, this.getLetterText(obj, template));
    } else {
      this.renderInstPoints(shapeG, obj);
      if (!this.drawInstIcon(shapeG, template, anchor)) {
        Symbols.draw(shapeG, template, anchor, symbolCtx);
      }
    }
    this.applyShapePose(shapeG, obj, template, anchor);
    g.appendChild(shapeG);

    this.renderComplianceIndication(g, obj, template, anchor);
    this.renderLabels(g, obj, template, anchor, labelOffset);
    this.renderHitArea(g, obj, anchor, template);
    return g;
  },

  renderPipe(g, obj, template) {
    const pts = this.getAbsolutePoints(obj);
    if (pts.length < 2) return;
    const d = pts.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x} ${p.y}`).join(' ');
    const strokeW = this.getStrokeWidth(template);
    const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
    path.setAttribute('d', d);
    path.setAttribute('fill', 'none');
    path.setAttribute('stroke', 'var(--pipe-fill)');
    path.setAttribute('stroke-width', String(strokeW));
    path.setAttribute('stroke-linecap', 'round');
    path.setAttribute('stroke-linejoin', 'round');
    path.setAttribute('vector-effect', 'non-scaling-stroke');
    path.setAttribute('class', 'scheme-pipe__segment');
    g.appendChild(path);

    for (const p of pts) {
      const joint = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
      joint.setAttribute('cx', p.x);
      joint.setAttribute('cy', p.y);
      joint.setAttribute('r', String(strokeW / 2));
      joint.setAttribute('fill', 'var(--pipe-fill)');
      joint.setAttribute('class', 'scheme-pipe__joint');
      g.appendChild(joint);
    }
  },

  renderLabels(g, obj, template, anchor, labelOffset) {
    if (!this.showLabels || !this.labelsEnabled) return;
    const fields = this.getVisibleFields(template);
    const to = obj.TechnologyObject?.[0];
    const items = [];
    for (const f of fields) {
      const val = this.resolveFieldValue(f, obj, to);
      if (val) items.push({ field: f, val: String(val) });
    }
    if (items.length === 0) return;

    const pos = this.getLabelPosition(template);
    const pt = this.getLabelAnchor(obj, template, anchor);
    const off = labelOffset || { ox: 0, oy: 33 };
    const labelX = pt.x + off.ox;
    const baseY = pos === 'top' ? pt.y + off.oy - 28 : pt.y + off.oy;

    const labelG = document.createElementNS('http://www.w3.org/2000/svg', 'g');
    labelG.setAttribute('class', 'scheme-object__labels scheme-object__labels--horizontal');

    const fontSize = this.labelFontSize;
    const fullText = items.map(i => i.val).join(' | ');
    const charW = fontSize * 0.58;
    const approxWidth = fullText.length * charW + 12;
    const rectH = fontSize + 8;
    const rectX = labelX - approxWidth / 2;
    const rectY = baseY - fontSize - 2;

    const bg = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
    bg.setAttribute('x', rectX);
    bg.setAttribute('y', rectY);
    bg.setAttribute('width', approxWidth);
    bg.setAttribute('height', rectH);
    bg.setAttribute('rx', '4');
    bg.setAttribute('class', 'scheme-object__labels-bg');
    labelG.appendChild(bg);

    const text = document.createElementNS('http://www.w3.org/2000/svg', 'text');
    text.setAttribute('x', labelX);
    text.setAttribute('y', baseY);
    text.setAttribute('text-anchor', 'middle');
    text.setAttribute('font-size', String(fontSize));
    text.setAttribute('class', 'scheme-object__label scheme-object__label--horizontal');

    items.forEach((item, idx) => {
      if (idx > 0) {
        const sepTspan = document.createElementNS('http://www.w3.org/2000/svg', 'tspan');
        sepTspan.setAttribute('class', 'scheme-object__label-sep');
        sepTspan.textContent = ' | ';
        text.appendChild(sepTspan);
      }
      const tspan = document.createElementNS('http://www.w3.org/2000/svg', 'tspan');
      tspan.setAttribute('class', 'scheme-object__label-part');
      tspan.textContent = item.val;
      text.appendChild(tspan);
    });

    labelG.appendChild(text);
    g.appendChild(labelG);
  },

  evaluateComplianceAll(obj, template) {
    const rules = (this.complianceRules || []).filter(r => r.template === template);
    const to = obj.TechnologyObject?.[0];
    const result = [];
    for (const rule of rules) {
      if (rule.text_condition === 'duplicate_tag') continue;
      const val = this.resolveComplianceValue(rule, obj, to);
      const isEmpty = !val || String(val).trim() === '';
      let violated = false;
      if (isEmpty) {
        if (!rule.empty_allowed) violated = true;
      } else {
        const text = String(val);
        const expected = rule.expected_text || '';
        if (rule.text_condition === 'must_not_contain' && text.includes(expected)) violated = true;
        if (rule.text_condition === 'must_contain' && expected && !text.includes(expected)) violated = true;
        if (rule.text_condition === 'exact_match' && text !== expected) violated = true;
        if (rule.text_condition === 'must_not_exact' && text === expected) violated = true;
      }
      if (violated) {
        const ind = this.parseIndication(rule.indication);
        result.push({ ruleId: rule.id, indication: ind });
      }
    }
    const duplicateHit = typeof App !== 'undefined' && App.getDuplicateViolations
      ? App.getDuplicateViolations(obj.Id)
      : null;
    if (duplicateHit) {
      result.push({ ruleId: duplicateHit.ruleId, indication: duplicateHit.indication });
    }
    return result;
  },

  evaluateCompliance(obj, template) {
    const all = this.evaluateComplianceAll(obj, template);
    return all.length > 0 ? all[0].indication : null;
  },

  parseIndication(raw) {
    if (!raw) return { type: 'solid', color: '#F85149' };
    if (typeof raw === 'string') {
      try { return JSON.parse(raw); } catch (_) { return { type: 'solid', color: '#F85149' }; }
    }
    return raw;
  },

  getObjectBounds(obj, template, anchor) {
    if (this.isPipeLike(template)) {
      const pts = this.getAbsolutePoints(obj);
      if (pts.length < 2) {
        return { x: anchor.x - 14, y: anchor.y - 14, width: 28, height: 28 };
      }
      let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
      for (const p of pts) {
        minX = Math.min(minX, p.x);
        minY = Math.min(minY, p.y);
        maxX = Math.max(maxX, p.x);
        maxY = Math.max(maxY, p.y);
      }
      const strokeW = this.getStrokeWidth(template);
      const pad = strokeW / 2;
      return { x: minX - pad, y: minY - pad, width: maxX - minX + strokeW, height: maxY - minY + strokeW };
    }
    if (this.getRenderer(template) === 'letter') {
      const half = this.symbolSize(14);
      return this.posedBounds(obj, template, anchor, {
        x: anchor.x - half, y: anchor.y - half, width: half * 2, height: half * 2,
      });
    }
    if (typeof InstIcons !== 'undefined' && InstIcons.url(template)) {
      const half = this.symbolSize(InstIcons.NATIVE_SIZE) / 2;
      return this.posedBounds(obj, template, anchor, {
        x: anchor.x - half, y: anchor.y - half, width: half * 2, height: half * 2,
      });
    }
    return this.posedBounds(obj, template, anchor, Symbols.getBounds(template, anchor, { scale: (n) => this.symbolSize(n) }));
  },

  posedBounds(obj, template, anchor, box) {
    const pose = this.getObjectPose(obj);
    const letter = this.getRenderer(template) === 'letter';
    const sx = letter ? 1 : pose.sx;
    const sy = letter ? 1 : pose.sy;
    if (!pose.angle && sx === 1 && sy === 1) return box;
    const corners = [
      { x: box.x, y: box.y },
      { x: box.x + box.width, y: box.y },
      { x: box.x + box.width, y: box.y + box.height },
      { x: box.x, y: box.y + box.height },
    ];
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    for (const c of corners) {
      const p = this.applyLocalPose(c.x - anchor.x, c.y - anchor.y, { angle: pose.angle, sx, sy });
      const x = anchor.x + p.x;
      const y = anchor.y + p.y;
      minX = Math.min(minX, x);
      minY = Math.min(minY, y);
      maxX = Math.max(maxX, x);
      maxY = Math.max(maxY, y);
    }
    return { x: minX, y: minY, width: maxX - minX, height: maxY - minY };
  },

  polarToXY(cx, cy, r, deg) {
    const rad = deg * Math.PI / 180;
    return { x: cx + r * Math.cos(rad), y: cy + r * Math.sin(rad) };
  },

  describeAnnulusSector(cx, cy, rOuter, rInner, startDeg, endDeg) {
    const span = endDeg - startDeg;
    const largeArc = span > 180 ? 1 : 0;
    const startOuter = this.polarToXY(cx, cy, rOuter, startDeg);
    const endOuter = this.polarToXY(cx, cy, rOuter, endDeg);
    const endInner = this.polarToXY(cx, cy, rInner, endDeg);
    const startInner = this.polarToXY(cx, cy, rInner, startDeg);
    return [
      `M ${startOuter.x} ${startOuter.y}`,
      `A ${rOuter} ${rOuter} 0 ${largeArc} 1 ${endOuter.x} ${endOuter.y}`,
      `L ${endInner.x} ${endInner.y}`,
      `A ${rInner} ${rInner} 0 ${largeArc} 0 ${startInner.x} ${startInner.y}`,
      'Z',
    ].join(' ');
  },

  describeFullAnnulus(cx, cy, rOuter, rInner) {
    return [
      `M ${cx + rOuter} ${cy}`,
      `A ${rOuter} ${rOuter} 0 1 1 ${cx - rOuter} ${cy}`,
      `A ${rOuter} ${rOuter} 0 1 1 ${cx + rOuter} ${cy}`,
      `M ${cx + rInner} ${cy}`,
      `A ${rInner} ${rInner} 0 1 0 ${cx - rInner} ${cy}`,
      `A ${rInner} ${rInner} 0 1 0 ${cx + rInner} ${cy}`,
    ].join(' ');
  },

  describePieSector(cx, cy, r, startDeg, endDeg) {
    const span = endDeg - startDeg;
    if (span >= 359.9) {
      return [
        `M ${cx - r} ${cy}`,
        `A ${r} ${r} 0 1 1 ${cx + r} ${cy}`,
        `A ${r} ${r} 0 1 1 ${cx - r} ${cy}`,
        'Z',
      ].join(' ');
    }
    const largeArc = span > 180 ? 1 : 0;
    const start = this.polarToXY(cx, cy, r, startDeg);
    const end = this.polarToXY(cx, cy, r, endDeg);
    return [
      `M ${cx} ${cy}`,
      `L ${start.x} ${start.y}`,
      `A ${r} ${r} 0 ${largeArc} 1 ${end.x} ${end.y}`,
      'Z',
    ].join(' ');
  },

  applyComplianceIndicationFill(overlay, indication, inner) {
    const color = indication.color || '#F85149';
    const solidOpacity = inner ? '0.20' : '0.38';
    const hatchOpacity = inner ? '0.30' : '0.45';
    const pulseOpacity = inner ? '0.25' : '0.40';

    if (indication.type === 'hatch') {
      this.ensureHatchPattern(color);
      overlay.setAttribute('fill', `url(#compliance-hatch-${color.replace('#', '')})`);
      overlay.setAttribute('fill-opacity', hatchOpacity);
    } else if (indication.type === 'pulse') {
      overlay.setAttribute('fill', color);
      overlay.setAttribute('fill-opacity', pulseOpacity);
      overlay.classList.add(
        inner ? 'scheme-object__compliance--inner-pulse' : 'scheme-object__compliance--segment-pulse',
      );
      const period = indication.pulsePeriodMs || 1000;
      overlay.style.animation = `compliance-pulse ${period}ms ease-in-out infinite`;
    } else {
      overlay.setAttribute('fill', color);
      overlay.setAttribute('fill-opacity', solidOpacity);
    }
  },

  buildComplianceSegments(violations) {
    const sorted = [...violations].sort((a, b) => String(a.ruleId).localeCompare(String(b.ruleId)));
    const n = sorted.length;
    const gapDeg = n > 1 ? 4 : 0;
    const totalGap = n > 1 ? n * gapDeg : 0;
    const segmentDeg = (360 - totalGap) / n;
    const segments = [];
    let currentDeg = -90;
    for (let i = 0; i < n; i++) {
      const startDeg = currentDeg;
      const endDeg = currentDeg + segmentDeg;
      segments.push({
        violation: sorted[i],
        indication: sorted[i].indication,
        startDeg,
        endDeg,
      });
      currentDeg = endDeg + gapDeg;
    }
    return segments;
  },

  renderComplianceInnerSegment(g, cx, cy, rInner, segment, indication) {
    const overlay = document.createElementNS('http://www.w3.org/2000/svg', 'path');
    overlay.setAttribute(
      'd',
      this.describePieSector(cx, cy, rInner, segment.startDeg, segment.endDeg),
    );
    overlay.setAttribute('class', 'scheme-object__compliance scheme-object__compliance--inner scheme-object__compliance--inner-segment');
    overlay.setAttribute('pointer-events', 'none');
    this.applyComplianceIndicationFill(overlay, indication, true);
    g.appendChild(overlay);
  },

  renderComplianceSegment(g, cx, cy, rOuter, rInner, segment, indication) {
    const color = indication.color || '#F85149';
    const span = segment.endDeg - segment.startDeg;
    const isFullRing = span >= 359.9;

    const overlay = document.createElementNS('http://www.w3.org/2000/svg', 'path');
    overlay.setAttribute(
      'd',
      isFullRing
        ? this.describeFullAnnulus(cx, cy, rOuter, rInner)
        : this.describeAnnulusSector(cx, cy, rOuter, rInner, segment.startDeg, segment.endDeg),
    );
    if (isFullRing) overlay.setAttribute('fill-rule', 'evenodd');
    overlay.setAttribute('class', 'scheme-object__compliance scheme-object__compliance--ring scheme-object__compliance--segment');
    overlay.setAttribute('pointer-events', 'none');
    overlay.setAttribute('stroke', color);
    overlay.setAttribute('stroke-width', '1');
    overlay.setAttribute('stroke-opacity', '1');
    this.applyComplianceIndicationFill(overlay, indication, false);
    g.appendChild(overlay);
  },

  renderComplianceIndication(g, obj, template, anchor) {
    const violations = this.evaluateComplianceAll(obj, template);
    if (violations.length === 0) return;

    const bounds = this.getObjectBounds(obj, template, anchor);
    const cx = bounds.x + bounds.width / 2;
    const cy = bounds.y + bounds.height / 2;
    const maxDim = Math.max(bounds.width, bounds.height);
    const rOuter = Math.max(maxDim * 0.72 * 1.25 * 0.8, 14);
    const ringThickness = Math.min(5, Math.max(3, Math.round(maxDim * 0.10)));
    const rInner = rOuter - ringThickness;

    const segments = this.buildComplianceSegments(violations);
    const pipeLike = this.isPipeLike(template);

    if (!pipeLike) {
      for (const segment of segments) {
        this.renderComplianceInnerSegment(g, cx, cy, rInner, segment, segment.indication);
      }
    }

    if (segments.length === 1) {
      const glowColor = segments[0].indication.color || '#F85149';
      const hasPulse = segments[0].indication.type === 'pulse';
      const glow = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
      glow.setAttribute('cx', String(cx));
      glow.setAttribute('cy', String(cy));
      glow.setAttribute('r', String(rOuter + 6));
      glow.setAttribute(
        'class',
        `scheme-object__compliance scheme-object__compliance--glow${hasPulse ? ' scheme-object__compliance--glow-pulse' : ''}`,
      );
      glow.setAttribute('pointer-events', 'none');
      glow.setAttribute('fill', glowColor);
      glow.setAttribute('fill-opacity', '0.28');
      glow.setAttribute('stroke', 'none');
      g.appendChild(glow);
    }

    for (const segment of segments) {
      this.renderComplianceSegment(g, cx, cy, rOuter, rInner, segment, segment.indication);
    }
  },

  ensureHatchPattern(color) {
    const svg = document.getElementById('schemeSvg');
    if (!svg) return;
    let defs = svg.querySelector('defs');
    if (!defs) {
      defs = document.createElementNS('http://www.w3.org/2000/svg', 'defs');
      svg.insertBefore(defs, svg.firstChild);
    }
    const id = `compliance-hatch-${color.replace('#', '')}`;
    if (defs.querySelector(`#${id}`)) return;
    const pattern = document.createElementNS('http://www.w3.org/2000/svg', 'pattern');
    pattern.setAttribute('id', id);
    pattern.setAttribute('patternUnits', 'userSpaceOnUse');
    pattern.setAttribute('width', '6');
    pattern.setAttribute('height', '6');
    pattern.setAttribute('patternTransform', 'rotate(45)');
    const line = document.createElementNS('http://www.w3.org/2000/svg', 'line');
    line.setAttribute('x1', '0');
    line.setAttribute('y1', '0');
    line.setAttribute('x2', '0');
    line.setAttribute('y2', '6');
    line.setAttribute('stroke', color);
    line.setAttribute('stroke-width', '3');
    pattern.appendChild(line);
    defs.appendChild(pattern);
  },

  renderHitArea(g, obj, anchor, template) {
    const hit = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
    if (this.isPipeLike(template)) {
      const pts = this.getAbsolutePoints(obj);
      if (pts.length >= 2) {
        const mid = pts[Math.floor(pts.length / 2)];
        hit.setAttribute('cx', mid.x); hit.setAttribute('cy', mid.y);
      } else {
        hit.setAttribute('cx', anchor.x); hit.setAttribute('cy', anchor.y);
      }
    } else {
      hit.setAttribute('cx', anchor.x); hit.setAttribute('cy', anchor.y);
    }
    hit.setAttribute('r', String(this.symbolSize(22)));
    hit.setAttribute('fill', 'transparent');
    hit.setAttribute('class', 'scheme-object__hit');
    g.appendChild(hit);
  },

  selectObject(svgEl, objectId) {
    svgEl.querySelectorAll('.scheme-object').forEach(el => el.classList.remove('scheme-object--selected'));
    const el = svgEl.querySelector(`[data-id="${objectId}"]`);
    if (el) el.classList.add('scheme-object--selected');
  },
};
