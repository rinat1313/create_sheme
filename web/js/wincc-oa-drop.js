(function (root, factory) {
  const api = factory();
  if (typeof module === 'object' && module.exports) {
    module.exports = api;
  }
  root.WinCcOaDrop = api;
})(typeof globalThis !== 'undefined' ? globalThis : this, function () {
  function parseWinCcOaDropText(raw) {
    if (raw == null) return null;
    const trimmed = String(raw).trim();
    if (!trimmed) return null;
    const colon = trimmed.indexOf(':');
    if (colon < 0) return null;
    const afterColon = trimmed.slice(colon + 1);
    const dot = afterColon.indexOf('.');
    const segment = (dot < 0 ? afterColon : afterColon.slice(0, dot)).trim();
    if (!segment) return null;
    return segment;
  }

  function readDropText(dataTransfer) {
    if (!dataTransfer) return '';
    return dataTransfer.getData('text/plain')
      || dataTransfer.getData('Text')
      || dataTransfer.getData('text')
      || '';
  }

  function clearDropHighlights() {
    document.querySelectorAll('.scheme-object--drop-target').forEach((el) => {
      el.classList.remove('scheme-object--drop-target');
    });
    document.querySelectorAll('.props-table__textarea--drop-target, .props-field__input--drop-target').forEach((el) => {
      el.classList.remove('props-table__textarea--drop-target', 'props-field__input--drop-target');
    });
  }

  function findSchemeObjectTarget(el) {
    if (!el || typeof el.closest !== 'function') return null;
    return el.closest('.scheme-object');
  }

  function findEditableFieldTarget(el) {
    if (!el || typeof el.closest !== 'function') return null;
    const field = el.closest('textarea.props-table__textarea, textarea.props-field__input');
    if (!field || field.disabled || field.readOnly) return null;
    return field;
  }

  function resolveDropTarget(el) {
    const field = findEditableFieldTarget(el);
    if (field) return { kind: 'field', el: field };
    const obj = findSchemeObjectTarget(el);
    if (obj) return { kind: 'object', el: obj };
    return null;
  }

  function highlightTarget(target) {
    clearDropHighlights();
    if (!target) return;
    if (target.kind === 'object') {
      target.el.classList.add('scheme-object--drop-target');
      return;
    }
    target.el.classList.add('props-table__textarea--drop-target');
    target.el.classList.add('props-field__input--drop-target');
  }

  async function applyTagToSchemeObject(objectEl, tag) {
    const objectId = objectEl.getAttribute('data-id');
    if (!objectId || typeof App === 'undefined') return;
    const obj = App.currentScheme?.GraphicObject?.find((o) => o.Id === objectId);
    if (!obj) return;
    const template = typeof Renderer !== 'undefined' ? Renderer.getTemplate(obj) : null;
    if (template && typeof Renderer !== 'undefined' && Renderer.canInteractWithObject(template)) {
      await App.selectObject(obj);
    } else if (App.solutionId) {
      await Inspector.loadObject(App.solutionId, objectId);
    }
    const base = Inspector.currentObject?.graphic_object;
    if (!base || base.Id !== objectId) return;
    if (!base.TechnologyObject?.[0] && obj.TechnologyObject?.[0]) {
      base.TechnologyObject = JSON.parse(JSON.stringify(obj.TechnologyObject));
    }
    Inspector.applyMainFieldPatch(base, 'to-field', 'Tag', tag);
    if (Inspector.currentObject) {
      Inspector.render(Inspector.currentObject);
    }
  }

  function applyValueToEditableField(field, value) {
    field.value = value;
    field.dispatchEvent(new Event('input', { bubbles: true }));
  }

  function install() {
    if (install._bound) return;
    install._bound = true;

    document.addEventListener('dragover', (e) => {
      const target = resolveDropTarget(e.target);
      if (!target) {
        clearDropHighlights();
        return;
      }
      e.preventDefault();
      if (e.dataTransfer) e.dataTransfer.dropEffect = 'copy';
      highlightTarget(target);
    });

    document.addEventListener('dragleave', (e) => {
      const related = e.relatedTarget;
      if (related && resolveDropTarget(related)) return;
      clearDropHighlights();
    });

    document.addEventListener('drop', (e) => {
      const target = resolveDropTarget(e.target);
      clearDropHighlights();
      if (!target) return;
      e.preventDefault();
      e.stopPropagation();
      const parsed = parseWinCcOaDropText(readDropText(e.dataTransfer));
      if (!parsed) return;
      if (target.kind === 'field') {
        applyValueToEditableField(target.el, parsed);
        return;
      }
      applyTagToSchemeObject(target.el, parsed);
    });

    document.addEventListener('dragend', () => {
      clearDropHighlights();
    });
  }

  return {
    parseWinCcOaDropText,
    readDropText,
    install,
  };
});
