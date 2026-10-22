const DISPLAY_FIELD_OPTIONS = ['Name', 'Description', 'Tag', 'Id', 'ExternalCode'];
const DISPLAY_EXTRA_PROPS = ['PressureDifference', 'Capacity', 'FixedLevel', 'Power', 'Diameter', 'PipeDiameter'];

const DisplaySettingsForm = {
  cloneSetting(setting) {
    return JSON.parse(JSON.stringify(setting || {
      template: '',
      renderer: 'symbol',
      visible_fields: '[]',
      style: '{}',
    }));
  },

  parseStyle(raw) {
    try {
      return JSON.parse(raw || '{}');
    } catch (_) {
      return {};
    }
  },

  parseFields(raw) {
    try {
      const fields = JSON.parse(raw || '[]');
      return Array.isArray(fields) ? fields : [];
    } catch (_) {
      return [];
    }
  },

  buildTypeSettingsForm(template, setting, options = {}) {
    const onChange = typeof options.onChange === 'function' ? options.onChange : null;
    const onOffsetChange = typeof options.onOffsetChange === 'function' ? options.onOffsetChange : onChange;
    const includeTitle = options.includeTitle !== false;
    const s = setting || { template, renderer: 'symbol', visible_fields: '[]', style: '{}' };
    const fields = this.parseFields(s.visible_fields);
    const styleObj = this.parseStyle(s.style);

    const block = document.createElement('div');
    block.className = 'display-settings__type-block';
    block.dataset.template = template;

    if (includeTitle) {
      const title = document.createElement('div');
      title.className = 'display-settings__type-title';
      title.textContent = template;
      block.appendChild(title);
    }

    const rendererRow = document.createElement('div');
    rendererRow.className = 'display-settings__renderer-row';
    const rendererLabel = document.createElement('span');
    rendererLabel.className = 'props-field__label';
    rendererLabel.textContent = 'Режим отображения';
    const rendererSel = document.createElement('select');
    rendererSel.className = 'input display-settings__renderer';
    [['symbol', 'Детальный символ'], ['letter', 'Буква'], ['pipe_strip', 'Линия (труба)']].forEach(([v, l]) => {
      const opt = document.createElement('option');
      opt.value = v;
      opt.textContent = l;
      const cur = s.renderer || 'symbol';
      if (v === cur || (v === 'letter' && cur === 'glyph')) opt.selected = true;
      rendererSel.appendChild(opt);
    });
    if (onChange) rendererSel.addEventListener('change', () => onChange(block));
    rendererRow.appendChild(rendererLabel);
    rendererRow.appendChild(rendererSel);
    block.appendChild(rendererRow);

    const visibleRow = document.createElement('label');
    visibleRow.className = 'display-settings__visible-row';
    const visibleCb = document.createElement('input');
    visibleCb.type = 'checkbox';
    visibleCb.className = 'display-settings__visible-cb';
    visibleCb.checked = styleObj.visible !== false;
    if (onChange) visibleCb.addEventListener('change', () => onChange(block));
    visibleRow.appendChild(visibleCb);
    visibleRow.appendChild(document.createTextNode(' Показывать на схеме'));
    block.appendChild(visibleRow);

    const offsetRow = document.createElement('div');
    offsetRow.className = 'display-settings__offset-row';
    const offsetTitle = document.createElement('span');
    offsetTitle.className = 'props-field__label';
    offsetTitle.textContent = 'Позиция (от левого верхнего угла)';
    offsetRow.appendChild(offsetTitle);

    for (const [axis, label, def] of [['x', 'X', styleObj.offsetX || 0], ['y', 'Y', styleObj.offsetY || 0]]) {
      const field = document.createElement('div');
      field.className = 'display-settings__offset-field';
      const axisLabel = document.createElement('span');
      axisLabel.className = 'props-field__label';
      axisLabel.textContent = label;
      const input = document.createElement('input');
      input.type = 'range';
      input.min = '-100';
      input.max = '100';
      input.step = '1';
      input.className = `display-settings__offset-input display-settings__offset-${axis}`;
      input.value = String(def);
      const valueEl = document.createElement('span');
      valueEl.className = 'display-settings__offset-value';
      valueEl.textContent = `${input.value} px`;
      input.addEventListener('input', () => {
        valueEl.textContent = `${input.value} px`;
        if (onOffsetChange) onOffsetChange(block);
      });
      field.appendChild(axisLabel);
      field.appendChild(input);
      field.appendChild(valueEl);
      offsetRow.appendChild(field);
    }
    block.appendChild(offsetRow);

    if (Renderer.isPipeLike(template)) {
      const strokeRow = document.createElement('div');
      strokeRow.className = 'display-settings__stroke-row';
      const strokeLabel = document.createElement('span');
      strokeLabel.className = 'props-field__label';
      strokeLabel.textContent = 'Толщина линии';
      const strokeInput = document.createElement('input');
      strokeInput.type = 'range';
      strokeInput.min = '1';
      strokeInput.max = '12';
      strokeInput.step = '1';
      strokeInput.className = 'display-settings__stroke-width';
      strokeInput.value = String(styleObj.strokeWidth || Renderer.getStrokeWidth(template));
      const strokeValue = document.createElement('span');
      strokeValue.className = 'display-settings__stroke-value';
      strokeValue.textContent = `${strokeInput.value} px`;
      strokeInput.addEventListener('input', () => {
        strokeValue.textContent = `${strokeInput.value} px`;
        if (onChange) onChange(block);
      });
      strokeRow.appendChild(strokeLabel);
      strokeRow.appendChild(strokeInput);
      strokeRow.appendChild(strokeValue);
      block.appendChild(strokeRow);
    }

    const checks = document.createElement('div');
    checks.className = 'display-settings__checks';
    const allOpts = [...DISPLAY_FIELD_OPTIONS, ...DISPLAY_EXTRA_PROPS];
    for (const opt of allOpts) {
      const labelEl = document.createElement('label');
      const cb = document.createElement('input');
      cb.type = 'checkbox';
      cb.value = opt;
      cb.checked = fields.includes(opt);
      cb.className = 'display-settings__cb';
      if (onChange) cb.addEventListener('change', () => onChange(block));
      labelEl.appendChild(cb);
      labelEl.appendChild(document.createTextNode(opt));
      checks.appendChild(labelEl);
    }
    block.appendChild(checks);

    const extraLabel = document.createElement('div');
    extraLabel.className = 'props-field__label';
    extraLabel.textContent = 'Доп. параметры (через запятую)';
    block.appendChild(extraLabel);

    const extra = document.createElement('input');
    extra.className = 'props-field__input display-settings__extra';
    const known = new Set(allOpts);
    const custom = fields.filter(f => !known.has(f));
    extra.value = custom.join(', ');
    extra.dataset.template = template;
    if (onChange) {
      extra.addEventListener('input', () => onChange(block));
      extra.addEventListener('change', () => onChange(block));
    }
    block.appendChild(extra);

    return block;
  },

  readTypeSettingsForm(block, baseStyle = {}) {
    const template = block.dataset.template;
    const fields = [];
    block.querySelectorAll('.display-settings__cb:checked').forEach(cb => fields.push(cb.value));
    const extraInput = block.querySelector('.display-settings__extra');
    if (extraInput?.value) {
      extraInput.value.split(',').map(s => s.trim()).filter(Boolean).forEach(f => {
        if (!fields.includes(f)) fields.push(f);
      });
    }
    const rendererSel = block.querySelector('.display-settings__renderer');
    const style = { ...baseStyle };
    const strokeInput = block.querySelector('.display-settings__stroke-width');
    if (strokeInput) {
      style.strokeWidth = Number(strokeInput.value);
    }
    style.offsetX = Number(block.querySelector('.display-settings__offset-x')?.value || 0);
    style.offsetY = Number(block.querySelector('.display-settings__offset-y')?.value || 0);
    const visibleCb = block.querySelector('.display-settings__visible-cb');
    style.visible = visibleCb ? visibleCb.checked : true;
    return {
      template,
      renderer: rendererSel?.value || 'symbol',
      visible_fields: JSON.stringify(fields),
      style: JSON.stringify({
        ...style,
        labelPosition: style.labelPosition || 'bottom',
        layout: style.layout || 'horizontal',
      }),
    };
  },

  readOffsetsFromBlock(block) {
    return {
      offsetX: Number(block.querySelector('.display-settings__offset-x')?.value || 0),
      offsetY: Number(block.querySelector('.display-settings__offset-y')?.value || 0),
    };
  },
};
