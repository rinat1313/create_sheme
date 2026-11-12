const Explorer = {
  container: null,
  footer: null,
  onSchemeSelect: null,
  collapsedNodes: new Set(),
  complianceHints: {},
  loadedProjects: new Set(),
  solutionId: null,
  filterMode: 'all',
  selectedIds: new Set(),
  picking: false,
  parentOf: {},
  childrenOf: {},
  typeOf: {},
  branchOf: {},
  emptyHint: null,

  init(container, footer, onSchemeSelect) {
    this.container = container;
    this.footer = footer;
    this.onSchemeSelect = onSchemeSelect;
  },

  setSolutionId(id) {
    this.solutionId = id;
    this.loadedProjects.clear();
    this.parentOf = {};
    this.childrenOf = {};
    this.typeOf = {};
    this.branchOf = {};
  },

  setTreeFilter(mode, ids, picking) {
    this.filterMode = mode || 'all';
    this.selectedIds = new Set(ids || []);
    this.picking = !!picking;
    this.applyVisibility();
  },

  nodeBranch(node, parentBranch) {
    if (node.type === 'workspace' || node.type === 'object_reference') return 'workspaces';
    if (parentBranch) return parentBranch;
    if (node.type === 'folder' && (node.children || []).some((c) => c.type === 'workspace')) {
      return 'workspaces';
    }
    if (node.type === 'folder' || node.type === 'project' || node.type === 'graphic_scheme') {
      return 'projects';
    }
    return '';
  },

  indexNode(node, parentXmlId, branch) {
    if (!node?.xml_id) return;
    this.parentOf[node.xml_id] = parentXmlId || '';
    this.typeOf[node.xml_id] = node.type;
    this.branchOf[node.xml_id] = branch || '';
    if (parentXmlId) {
      if (!this.childrenOf[parentXmlId]) this.childrenOf[parentXmlId] = [];
      if (!this.childrenOf[parentXmlId].includes(node.xml_id)) {
        this.childrenOf[parentXmlId].push(node.xml_id);
      }
    }
  },

  mountProjectsBranchChildren(childUl, projectsNode, parentXmlId, branch, depth) {
    this.indexNode(projectsNode, parentXmlId, branch);
    for (const child of projectsNode.children || []) {
      childUl.appendChild(this.renderNode(child, depth + 1, projectsNode.xml_id, branch));
    }
  },

  getDescendantSchemeIds(xmlId) {
    const out = [];
    const seenSchemes = new Set();
    const stack = [xmlId];
    const walkSeen = new Set();
    const projectsInWalk = new Set();
    while (stack.length) {
      const cur = stack.pop();
      if (walkSeen.has(cur)) continue;
      walkSeen.add(cur);
      const type = this.typeOf[cur];
      if (type === 'graphic_scheme') {
        if (!seenSchemes.has(cur)) {
          seenSchemes.add(cur);
          out.push(cur);
        }
        continue;
      }
      if (type === 'project') projectsInWalk.add(cur);
      for (const kid of this.childrenOf[cur] || []) {
        stack.push(kid);
      }
    }
    const map = typeof App !== 'undefined' && App.schemeProjectMap ? App.schemeProjectMap : null;
    if (map) {
      for (const projectId of projectsInWalk) {
        for (const [schemeId, pid] of Object.entries(map)) {
          if (pid === projectId && !seenSchemes.has(schemeId)) {
            seenSchemes.add(schemeId);
            out.push(schemeId);
          }
        }
      }
    }
    return out;
  },

  isXmlVisible(xmlId) {
    if (this.filterMode === 'all' || this.picking) return true;
    if (!xmlId) return false;
    if (this.branchOf[xmlId] === 'workspaces') return false;
    if (!this.selectedIds.size) return false;
    const seen = new Set();
    let cur = xmlId;
    while (cur && !seen.has(cur)) {
      if (this.selectedIds.has(cur)) return true;
      seen.add(cur);
      cur = this.parentOf[cur] || (typeof App !== 'undefined' && App.schemeProjectMap && App.schemeProjectMap[cur]) || (typeof App !== 'undefined' && App.projectFolderMap && App.projectFolderMap[cur]) || '';
    }
    for (const sel of this.selectedIds) {
      const walk = new Set();
      let p = this.parentOf[sel] || (typeof App !== 'undefined' && App.schemeProjectMap && App.schemeProjectMap[sel]) || (typeof App !== 'undefined' && App.projectFolderMap && App.projectFolderMap[sel]) || '';
      while (p && !walk.has(p)) {
        if (p === xmlId) return true;
        walk.add(p);
        p = this.parentOf[p] || (typeof App !== 'undefined' && App.schemeProjectMap && App.schemeProjectMap[p]) || (typeof App !== 'undefined' && App.projectFolderMap && App.projectFolderMap[p]) || '';
      }
    }
    return false;
  },

  isSelectable(type, branch, xmlId) {
    if (this.filterMode === 'all') return false;
    if (branch === 'workspaces') return false;
    if (type === 'solution') return false;
    if (xmlId === 'projects') return false;
    if (branch !== 'projects') return false;
    if (this.filterMode === 'schemes') {
      return type === 'project' || type === 'folder' || type === 'graphic_scheme';
    }
    return false;
  },

  checkboxStateForNode(xmlId, type) {
    if (type === 'graphic_scheme') {
      const on = this.selectedIds.has(xmlId);
      return { checked: on, indeterminate: false };
    }
    const schemes = this.getDescendantSchemeIds(xmlId);
    if (!schemes.length) {
      if (this.selectedIds.has(xmlId)) {
        return { checked: true, indeterminate: false };
      }
      return { checked: false, indeterminate: false };
    }
    let selectedCount = 0;
    for (const sid of schemes) {
      if (this.selectedIds.has(sid)) selectedCount += 1;
    }
    if (selectedCount === 0 && !this.selectedIds.has(xmlId)) {
      return { checked: false, indeterminate: false };
    }
    if (selectedCount === schemes.length) {
      return { checked: true, indeterminate: false };
    }
    return { checked: false, indeterminate: true };
  },

  applyVisibility() {
    if (!this.container) return;
    let visibleCount = 0;
    this.container.querySelectorAll(':scope > ul.tree > li, ul.tree-node__children > li').forEach((li) => {
      const div = li.querySelector(':scope > .tree-node');
      if (!div) return;
      const xmlId = div.dataset.xmlId;
      const type = div.dataset.type;
      const branch = div.dataset.branch || this.branchOf[xmlId] || '';
      const show = this.isXmlVisible(xmlId) || (type === 'solution' && this.hasVisibleDescendant(li));
      li.hidden = !show && !this.picking;
      if (!li.hidden) visibleCount += 1;
      this.syncCheckbox(div, xmlId, type, branch);
    });
    this.syncEmptyHint(visibleCount === 0 && this.filterMode !== 'all' && !this.picking);
  },

  hasVisibleDescendant(li) {
    const kids = li.querySelectorAll('.tree-node');
    for (const el of kids) {
      if (el === li.querySelector(':scope > .tree-node')) continue;
      if (this.isXmlVisible(el.dataset.xmlId)) return true;
    }
    return false;
  },

  syncEmptyHint(show) {
    if (!this.container) return;
    if (!this.emptyHint) {
      this.emptyHint = document.createElement('p');
      this.emptyHint.className = 'props-empty tree-filter-empty';
      this.emptyHint.textContent = 'Нет выбранных схем. Откройте фильтр в шапке обозревателя и отметьте схемы.';
    }
    if (show) {
      if (!this.emptyHint.isConnected) this.container.appendChild(this.emptyHint);
      this.emptyHint.hidden = false;
    } else if (this.emptyHint.isConnected) {
      this.emptyHint.hidden = true;
    }
  },

  syncCheckbox(div, xmlId, type, branch) {
    let box = div.querySelector('.tree-node__filter-check');
    const need = this.picking && this.isSelectable(type, branch, xmlId);
    if (!need) {
      if (box) box.remove();
      return;
    }
    if (!box) {
      box = document.createElement('input');
      box.type = 'checkbox';
      box.className = 'tree-node__filter-check';
      let clickWasIndeterminate = false;
      box.addEventListener('click', (e) => {
        e.stopPropagation();
        clickWasIndeterminate = box.indeterminate;
      });
      box.addEventListener('change', (e) => {
        e.stopPropagation();
        if (typeof App !== 'undefined' && App.onTreeFilterToggle) {
          const checked = clickWasIndeterminate ? true : box.checked;
          if (clickWasIndeterminate) {
            box.checked = true;
            box.indeterminate = false;
          }
          App.onTreeFilterToggle(xmlId, type, checked, clickWasIndeterminate);
          clickWasIndeterminate = false;
        }
      });
      div.insertBefore(box, div.querySelector('.tree-node__icon'));
    }
    const state = this.checkboxStateForNode(xmlId, type);
    box.checked = state.checked;
    box.indeterminate = state.indeterminate;
  },

  setComplianceHints(hints) {
    this.complianceHints = {};
    for (const h of hints || []) {
      this.complianceHints[h.xml_id] = h;
    }
  },

  mergeComplianceHints(hints, scopeXmlIds) {
    const byId = {};
    for (const h of hints || []) {
      byId[h.xml_id] = h;
    }
    if (scopeXmlIds) {
      const scope = scopeXmlIds instanceof Set ? scopeXmlIds : new Set(scopeXmlIds);
      for (const xmlId of scope) {
        const h = byId[xmlId];
        if (h && h.indicators && h.indicators.length > 0) {
          this.complianceHints[xmlId] = h;
        } else {
          delete this.complianceHints[xmlId];
        }
      }
    } else {
      for (const h of hints || []) {
        if (h.indicators && h.indicators.length > 0) {
          this.complianceHints[h.xml_id] = h;
        } else {
          delete this.complianceHints[h.xml_id];
        }
      }
    }
    this.applyComplianceToTree();
  },

  applyComplianceToTree() {
    this.container.querySelectorAll('.tree-node').forEach(el => {
      const xmlId = el.dataset.xmlId;
      const hint = this.complianceHints[xmlId];
      el.classList.remove('tree-node--violation');
      el.style.removeProperty('--violation-color');
      el.querySelector('.tree-node__compliance-dots')?.remove();
      if (hint && hint.indicators && hint.indicators.length > 0) {
        el.classList.add('tree-node--violation');
        el.style.setProperty('--violation-color', hint.indicators[0].color || '#F85149');
        const dots = document.createElement('span');
        dots.className = 'tree-node__compliance-dots';
        for (const ind of hint.indicators) {
          const dot = document.createElement('span');
          dot.className = 'tree-node__compliance-dot';
          dot.style.background = ind.color || '#F85149';
          dot.title = ind.rule_id || '';
          dots.appendChild(dot);
        }
        el.appendChild(dots);
      }
    });
  },

  render(tree) {
    this.collapsedNodes.clear();
    this.parentOf = {};
    this.childrenOf = {};
    this.typeOf = {};
    this.branchOf = {};
    this.container.innerHTML = '';
    const ul = document.createElement('ul');
    ul.className = 'tree';
    for (const node of tree) {
      if (node.xml_id === 'projects') {
        const branch = this.nodeBranch(node, '');
        this.indexNode(node, '', branch);
        for (const child of node.children || []) {
          ul.appendChild(this.renderNode(child, 0, node.xml_id, branch));
        }
      } else {
        ul.appendChild(this.renderNode(node, 0, '', ''));
      }
    }
    this.container.appendChild(ul);
    this.applyComplianceToTree();
    this.applyVisibility();
  },

  isCollapsible(node) {
    return node.children && node.children.length > 0 &&
      ['solution', 'folder', 'project', 'workspace'].includes(node.type);
  },

  isCollapsed(node) {
    if (this.collapsedNodes.has(node.id)) return true;
    if (node.type === 'workspace') return true;
    if (node.type === 'project') return true;
    return false;
  },

  async toggleNode(nodeId, childUl, toggleEl, node) {
    const collapsed = !childUl.classList.contains('tree-node__children--collapsed');
    if (!collapsed && node?.type === 'project' && !this.loadedProjects.has(node.xml_id)) {
      childUl.innerHTML = '<li class="tree-node__loading">Загрузка…</li>';
      childUl.classList.remove('tree-node__children--collapsed');
      toggleEl.textContent = '▼';
      this.collapsedNodes.delete(nodeId);
      try {
        const children = await API.getProjectSchemes(this.solutionId, node.xml_id);
        node.children = children;
        this.loadedProjects.add(node.xml_id);
        childUl.innerHTML = '';
        const branch = this.branchOf[node.xml_id] || 'projects';
        for (const child of children) {
          childUl.appendChild(this.renderNode(child, 0, node.xml_id, branch));
        }
        if (App.onProjectSchemesLoaded) {
          App.onProjectSchemesLoaded(node.xml_id, children);
        }
        this.applyComplianceToTree();
        this.applyVisibility();
        if (App.refreshComplianceForProject) {
          await App.refreshComplianceForProject(node.xml_id);
        }
      } catch (_) {
        childUl.innerHTML = '<li class="tree-node__loading tree-node__loading--error">Ошибка загрузки</li>';
      }
      return;
    }
    childUl.classList.toggle('tree-node__children--collapsed', collapsed);
    toggleEl.textContent = collapsed ? '▶' : '▼';
    if (collapsed) {
      this.collapsedNodes.add(nodeId);
    } else {
      this.collapsedNodes.delete(nodeId);
    }
  },

  renderNode(node, depth, parentXmlId, parentBranch) {
    const li = document.createElement('li');
    const icons = {
      solution: '📋', folder: '📁', project: '🏭',
      graphic_scheme: '📊', workspace: '🗂', object_reference: '🔗',
    };
    const hasChildren = node.children && node.children.length > 0;
    const collapsible = this.isCollapsible(node) || node.type === 'project';
    const collapsed = collapsible && this.isCollapsed(node);
    const hint = this.complianceHints[node.xml_id];
    const branch = this.nodeBranch(node, parentBranch);
    this.indexNode(node, parentXmlId, branch);

    const div = document.createElement('div');
    div.className = `tree-node tree-node--${node.type}`;
    div.dataset.xmlId = node.xml_id;
    div.dataset.type = node.type;
    div.dataset.branch = branch;
    if (hint && hint.indicators && hint.indicators.length > 0) {
      div.classList.add('tree-node--violation');
      div.style.setProperty('--violation-color', hint.indicators[0].color || '#F85149');
    }

    const toggle = document.createElement('span');
    toggle.className = 'tree-node__toggle' + (collapsible ? '' : ' tree-node__toggle--empty');
    toggle.textContent = collapsible ? (collapsed ? '▶' : '▼') : '';
    if (collapsible) {
      toggle.addEventListener('click', (e) => {
        e.stopPropagation();
        const childUl = li.querySelector(':scope > .tree-node__children');
        if (childUl) this.toggleNode(node.id, childUl, toggle, node);
      });
    }

    const icon = document.createElement('span');
    icon.className = 'tree-node__icon';
    icon.textContent = icons[node.type] || '•';

    const label = document.createElement('span');
    label.className = 'tree-node__label';
    label.textContent = node.name || '';

    div.appendChild(toggle);
    div.appendChild(icon);
    div.appendChild(label);

    if (hint && hint.indicators && hint.indicators.length > 0) {
      const dots = document.createElement('span');
      dots.className = 'tree-node__compliance-dots';
      for (const ind of hint.indicators) {
        const dot = document.createElement('span');
        dot.className = 'tree-node__compliance-dot';
        dot.style.background = ind.color || '#F85149';
        dots.appendChild(dot);
      }
      div.appendChild(dots);
    }

    if (node.type === 'graphic_scheme') {
      div.addEventListener('click', () => {
        this.container.querySelectorAll('.tree-node--active').forEach(el => el.classList.remove('tree-node--active'));
        div.classList.add('tree-node--active');
        this.footer.textContent = node.name;
        if (this.onSchemeSelect) this.onSchemeSelect(node);
      });
    } else if (collapsible) {
      div.addEventListener('click', () => {
        const childUl = li.querySelector(':scope > .tree-node__children');
        if (childUl) this.toggleNode(node.id, childUl, toggle, node);
      });
    }

    li.appendChild(div);

    if (hasChildren || node.type === 'project') {
      const childUl = document.createElement('ul');
      childUl.className = 'tree-node__children' + (collapsed ? ' tree-node__children--collapsed' : '');
      if (hasChildren) {
        for (const child of node.children) {
          if (child.xml_id === 'projects') {
            const projectsBranch = this.nodeBranch(child, branch);
            this.mountProjectsBranchChildren(childUl, child, node.xml_id, projectsBranch, depth);
          } else {
            childUl.appendChild(this.renderNode(child, depth + 1, node.xml_id, branch));
          }
        }
      }
      li.appendChild(childUl);
    }
    return li;
  },
};
