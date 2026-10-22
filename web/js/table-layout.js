const TableLayout = {
  loadLayout(storageKey, columnDefs) {
    const defMap = Object.fromEntries(columnDefs.map(d => [d.key, d]));
    const fallback = {
      order: columnDefs.map(d => d.key),
      widths: Object.fromEntries(columnDefs.map(d => [d.key, d.width])),
    };
    try {
      const saved = JSON.parse(localStorage.getItem(storageKey) || 'null');
      if (!saved?.order?.length) return fallback;
      const order = saved.order.filter(key => defMap[key]);
      for (const def of columnDefs) {
        if (!order.includes(def.key)) order.push(def.key);
      }
      const widths = { ...fallback.widths, ...(saved.widths || {}) };
      for (const key of Object.keys(widths)) {
        const def = defMap[key];
        if (def) widths[key] = Math.max(def.minWidth, parseInt(widths[key], 10) || def.width);
      }
      return { order, widths };
    } catch (_) {
      return fallback;
    }
  },

  saveLayout(storageKey, order, widths) {
    localStorage.setItem(storageKey, JSON.stringify({ order, widths }));
  },

  buildTable({ columnDefs, layout, className = 'props-table', allowReorder = true }) {
    const defMap = Object.fromEntries(columnDefs.map(d => [d.key, d]));
    const table = document.createElement('table');
    table.className = className;

    const colgroup = document.createElement('colgroup');
    for (const key of layout.order) {
      const col = document.createElement('col');
      col.dataset.colKey = key;
      col.style.width = `${layout.widths[key] || defMap[key]?.width || 120}px`;
      colgroup.appendChild(col);
    }
    table.appendChild(colgroup);

    const thead = document.createElement('thead');
    const headRow = document.createElement('tr');
    for (const key of layout.order) {
      const def = defMap[key];
      if (!def) continue;
      const th = document.createElement('th');
      th.dataset.colKey = key;
      const inner = document.createElement('div');
      inner.className = 'props-table__th-inner';
      if (allowReorder) {
        const grip = document.createElement('span');
        grip.className = 'props-table__col-grip';
        grip.textContent = '⋮⋮';
        grip.title = 'Перетащите для изменения порядка колонок';
        inner.appendChild(grip);
      }
      const label = document.createElement('span');
      label.className = 'props-table__col-label';
      label.textContent = def.label;
      inner.appendChild(label);
      th.appendChild(inner);
      const resize = document.createElement('span');
      resize.className = 'props-table__col-resize';
      resize.title = 'Изменить ширину колонки';
      th.appendChild(resize);
      headRow.appendChild(th);
    }
    thead.appendChild(headRow);
    table.appendChild(thead);

    const tbody = document.createElement('tbody');
    table.appendChild(tbody);

    return { table, tbody, colgroup };
  },

  setupColumns(table, layout, columnDefs, storageKey, options = {}) {
    if (!table || table.dataset.columnsReady === '1') return;
    table.dataset.columnsReady = '1';
    const defMap = Object.fromEntries(columnDefs.map(d => [d.key, d]));
    const allowReorder = options.allowReorder !== false;
    let dragKey = null;

    const applyColumnWidth = (key, width) => {
      const col = table.querySelector(`colgroup col[data-col-key="${key}"]`);
      const def = defMap[key];
      const px = Math.max(def?.minWidth || 64, width);
      if (col) col.style.width = `${px}px`;
      layout.widths[key] = px;
    };

    table.querySelectorAll('.props-table__col-resize').forEach(handle => {
      handle.addEventListener('mousedown', (e) => {
        e.preventDefault();
        e.stopPropagation();
        const th = handle.closest('th');
        const key = th?.dataset.colKey;
        if (!key) return;
        const col = table.querySelector(`colgroup col[data-col-key="${key}"]`);
        const startX = e.clientX;
        const startWidth = col?.getBoundingClientRect().width || defMap[key]?.width || 120;
        const onMove = (ev) => {
          applyColumnWidth(key, startWidth + (ev.clientX - startX));
        };
        const onUp = () => {
          window.removeEventListener('mousemove', onMove);
          window.removeEventListener('mouseup', onUp);
          this.saveLayout(storageKey, layout.order, layout.widths);
        };
        window.addEventListener('mousemove', onMove);
        window.addEventListener('mouseup', onUp);
      });
      handle.addEventListener('dblclick', (e) => {
        e.preventDefault();
        e.stopPropagation();
        const th = handle.closest('th');
        const key = th?.dataset.colKey;
        if (!key || !defMap[key]) return;
        applyColumnWidth(key, defMap[key].width);
        this.saveLayout(storageKey, layout.order, layout.widths);
      });
    });

    if (allowReorder) {
      table.querySelectorAll('.props-table__col-grip').forEach(grip => {
        grip.addEventListener('mousedown', (e) => {
          e.preventDefault();
          e.stopPropagation();
        });
        grip.addEventListener('dragstart', (e) => {
          const th = grip.closest('th');
          dragKey = th?.dataset.colKey || null;
          if (!dragKey) {
            e.preventDefault();
            return;
          }
          e.dataTransfer.effectAllowed = 'move';
          e.dataTransfer.setData('text/plain', dragKey);
          th.classList.add('props-table__th--drag-over');
        });
        grip.addEventListener('dragend', () => {
          dragKey = null;
          table.querySelectorAll('.props-table__th--drag-over').forEach(el => el.classList.remove('props-table__th--drag-over'));
        });
        grip.setAttribute('draggable', 'true');
      });

      table.querySelectorAll('thead th').forEach(th => {
        th.addEventListener('dragover', (e) => {
          if (!dragKey) return;
          e.preventDefault();
          e.dataTransfer.dropEffect = 'move';
          table.querySelectorAll('.props-table__th--drag-over').forEach(el => {
            if (el !== th) el.classList.remove('props-table__th--drag-over');
          });
          th.classList.add('props-table__th--drag-over');
        });
        th.addEventListener('dragleave', () => {
          th.classList.remove('props-table__th--drag-over');
        });
        th.addEventListener('drop', (e) => {
          e.preventDefault();
          e.stopPropagation();
          const fromKey = e.dataTransfer.getData('text/plain') || dragKey;
          const toKey = th.dataset.colKey;
          table.querySelectorAll('.props-table__th--drag-over').forEach(el => el.classList.remove('props-table__th--drag-over'));
          if (!fromKey || !toKey || fromKey === toKey) return;
          const order = [...layout.order];
          const fromIdx = order.indexOf(fromKey);
          const toIdx = order.indexOf(toKey);
          if (fromIdx < 0 || toIdx < 0) return;
          order.splice(fromIdx, 1);
          order.splice(toIdx, 0, fromKey);
          layout.order = order;
          this.saveLayout(storageKey, layout.order, layout.widths);
          if (options.onLayoutChange) options.onLayoutChange(layout);
        });
      });
    }
  },
};
