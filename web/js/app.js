const App = {
  solutionId: null,
  sessionId: null,
  currentSchemeId: null,
  currentScheme: null,
  isDirty: false,
  saving: false,
  dragState: null,
  complianceReport: null,
  duplicateCompliance: null,
  duplicateRows: [],
  duplicatePanelDismissed: false,
  duplicatePanelCollapsed: false,
  duplicatePanelUserClosedDuringScan: false,
  duplicateBackgroundPollTimer: null,
  duplicateBackgroundEnabled: false,
  lastTree: null,
  schemeProjectMap: {},
  projectFolderMap: {},
  objectDetailsCache: new Map(),
  complianceParamsCache: new Map(),
  cachedSchemeId: null,
  schemeMovementLocked: false,
  tagEditApplying: false,
  selectedObjectId: null,
  locatorTimer: null,
  _panGesture: { active: false, moved: false, startX: 0, startY: 0, lastX: 0, lastY: 0 },
  _suppressSvgClick: false,
  objectSettings: {
    open: false,
    objectId: null,
    template: null,
    draft: null,
    baseline: null,
    dirty: false,
  },
  _objectSettingsPreviewToken: 0,

  _users: [],
  _solutions: [],
  _uiHandlersBound: false,
  solutionSearchOpen: false,
  solutionSearchTimer: null,
  solutionSearchReq: 0,
  solutionSearchQuery: '',
  solutionSearchItems: [],

  bindUiHandlers() {
    if (this._uiHandlersBound) return;
    this._uiHandlersBound = true;

    document.getElementById('fileInput').addEventListener('change', (e) => this.uploadFile(e));
    document.getElementById('addSolutionLabel').addEventListener('click', (e) => {
      if (!UserContext.userId) {
        e.preventDefault();
        e.stopPropagation();
        this.openUserMenu();
        const btn = document.getElementById('btnUserMenu');
        if (btn) btn.title = 'Сначала выберите или создайте пользователя';
      }
    });
    document.getElementById('btnSolutionMenu').addEventListener('click', (e) => {
      e.preventDefault();
      e.stopPropagation();
      this.toggleSolutionMenu();
    });
    document.getElementById('btnSave').addEventListener('click', () => this.save());
    document.getElementById('btnExport').addEventListener('click', () => this.exportXML());
    document.getElementById('btnCloseScheme').addEventListener('click', () => this.closeScheme());
    document.getElementById('btnZoomIn').addEventListener('click', () => this.adjustZoom(1.2));
    document.getElementById('btnZoomOut').addEventListener('click', () => this.adjustZoom(0.8));
    document.getElementById('btnFit').addEventListener('click', () => this.fitView());
    document.getElementById('btnLockScheme').addEventListener('click', () => this.toggleSchemeMovementLock());
    document.getElementById('btnObjectSettings').addEventListener('click', () => this.toggleObjectSettingsPanel());
    document.getElementById('btnObjectSettingsAccept').addEventListener('click', () => this.acceptObjectSettings({ keepOpen: true }));
    document.getElementById('btnObjectSettingsClose').addEventListener('click', () => this.closeObjectSettingsPanel());
    document.getElementById('btnDisplaySettings').addEventListener('click', () => this.showDisplaySettings());
    document.getElementById('btnCloseModal').addEventListener('click', () => this.hideDisplaySettings());
    document.getElementById('btnComplianceSettings').addEventListener('click', () => this.showComplianceSettings());
    document.getElementById('btnCloseComplianceModal').addEventListener('click', () => this.hideComplianceSettings());
    document.getElementById('btnDuplicateSettings').addEventListener('click', () => this.showDuplicateSettings());
    document.getElementById('btnCloseDuplicateModal').addEventListener('click', () => this.hideDuplicateSettings());
    document.getElementById('btnCloseDuplicatePanel').addEventListener('click', (e) => {
      e.stopPropagation();
      this.hideDuplicatePanel();
    });
    document.getElementById('btnToggleDuplicatePanel').addEventListener('click', (e) => {
      e.stopPropagation();
      this.toggleDuplicatePanelCollapse();
    });
    document.getElementById('btnShowDuplicatePanel').addEventListener('click', () => this.showDuplicatePanel());
    document.getElementById('btnRecheckDuplicates').addEventListener('click', () => this.recheckObjectDuplicates());
    document.getElementById('duplicatePanelHeader').addEventListener('click', (e) => {
      if (!this.duplicatePanelCollapsed) return;
      if (e.target.closest('button')) return;
      this.duplicatePanelCollapsed = false;
      localStorage.setItem('duplicatePanelCollapsed', 'false');
      this.updateDuplicatePanel();
    });
    document.getElementById('btnTagEditSettings').addEventListener('click', () => this.showTagEditSettings());
    document.getElementById('btnCloseTagEditModal').addEventListener('click', () => this.hideTagEditSettings());
    document.getElementById('btnExportObjects').addEventListener('click', () => this.showExportObjects());
    document.getElementById('btnCloseExportModal').addEventListener('click', () => this.hideExportObjects());
    document.getElementById('btnDeleteSolution').addEventListener('click', () => this.deleteCurrentSolution());
    document.getElementById('btnClearAll').addEventListener('click', () => this.clearAllSolutions());
    document.getElementById('btnHelp').addEventListener('click', () => this.showHelp());
    document.getElementById('btnCloseHelpModal').addEventListener('click', () => this.hideHelp());
    document.getElementById('btnTheme').addEventListener('click', () => this.toggleTheme());
    document.getElementById('btnUserMenu').addEventListener('click', (e) => {
      e.preventDefault();
      e.stopPropagation();
      this.toggleUserMenu();
    });
    document.getElementById('btnCreateUser').addEventListener('click', (e) => {
      e.preventDefault();
      e.stopPropagation();
      this.promptCreateUser();
    });

    this.duplicatePanelCollapsed = localStorage.getItem('duplicatePanelCollapsed') === 'true';
    this.initTheme();
    this.initSolutionSearch();

    const helpModal = document.getElementById('helpModal');
    const displayModal = document.getElementById('displayModal');
    const complianceModal = document.getElementById('complianceModal');
    const duplicateModal = document.getElementById('duplicateModal');
    const tagEditModal = document.getElementById('tagEditModal');
    const exportModal = document.getElementById('exportObjectsModal');
    helpModal.addEventListener('click', (e) => {
      if (e.target === helpModal) this.hideHelp();
    });
    displayModal.addEventListener('click', (e) => {
      if (e.target === displayModal) this.hideDisplaySettings();
    });
    complianceModal.addEventListener('click', (e) => {
      if (e.target === complianceModal) this.hideComplianceSettings();
    });
    duplicateModal.addEventListener('click', (e) => {
      if (e.target === duplicateModal) this.hideDuplicateSettings();
    });
    tagEditModal.addEventListener('click', (e) => {
      if (e.target === tagEditModal) this.hideTagEditSettings();
    });
    exportModal.addEventListener('click', (e) => {
      if (e.target === exportModal) this.hideExportObjects();
    });
    document.addEventListener('click', (e) => {
      const userMenu = document.getElementById('userMenu');
      if (userMenu && !userMenu.hidden && !e.target.closest('.topbar__user')) {
        this.closeUserMenu();
      }
      const solutionMenu = document.getElementById('solutionMenu');
      if (solutionMenu && !solutionMenu.hidden && !e.target.closest('.topbar__solutions')) {
        this.closeSolutionMenu();
      }
    });
    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') {
        if (this.saving || this.tagEditApplying) {
          e.preventDefault();
          return;
        }
        const userGate = document.getElementById('userGateOverlay');
        if (userGate && !userGate.hidden) {
          e.preventDefault();
          if (typeof this._userGateOnCancel === 'function') this._userGateOnCancel();
          else userGate.hidden = true;
          return;
        }
        const modalOpen = !helpModal.hidden
          || !displayModal.hidden
          || !complianceModal.hidden
          || !duplicateModal.hidden
          || !tagEditModal.hidden
          || !exportModal.hidden;
        if (modalOpen) {
          if (!helpModal.hidden) this.hideHelp();
          if (!displayModal.hidden) this.hideDisplaySettings();
          if (!complianceModal.hidden) this.hideComplianceSettings();
          if (!duplicateModal.hidden) this.hideDuplicateSettings();
          if (!tagEditModal.hidden) this.hideTagEditSettings();
          if (!exportModal.hidden) this.hideExportObjects();
          return;
        }
        if (this.objectSettings.open) {
          this.closeObjectSettingsPanel();
          return;
        }
        const duplicatePanel = document.getElementById('duplicatePanel');
        if (duplicatePanel && !duplicatePanel.hidden) {
          if (this.duplicatePanelCollapsed) {
            this.hideDuplicatePanel();
          } else {
            this.toggleDuplicatePanelCollapse(true);
          }
          return;
        }
        const duplicateScanOverlay = document.getElementById('duplicateScanOverlay');
        if (duplicateScanOverlay && !duplicateScanOverlay.hidden) {
          return;
        }
        if (this.solutionSearchOpen) {
          const input = document.getElementById('solutionSearchInput');
          if (input && input.value) {
            this.clearSolutionSearch();
            return;
          }
          this.setSolutionSearchOpen(false);
          return;
        }
        this.closeUserMenu();
        this.closeSolutionMenu();
      }
    });

    const svg = document.getElementById('schemeSvg');
    svg.addEventListener('click', async () => {
      if (this._suppressSvgClick) {
        this._suppressSvgClick = false;
        return;
      }
      if (this.objectSettings.open) {
        const closed = await this.tryCloseObjectSettingsPanel();
        if (!closed) return;
      }
      this.clearObjectLocator();
      svg.querySelectorAll('.scheme-object--selected').forEach(el => el.classList.remove('scheme-object--selected'));
      this.selectedObjectId = null;
      if (this.currentScheme) Inspector.showSchemeInfo(this.currentScheme);
      document.getElementById('app').classList.remove('app--object-selected');
    });

    this.setupPanZoom(svg);
    this.setupDrag(svg);
    this.setupPanelResizers();
  },

  updateUserMenuButton(displayName) {
    const btnUserMenu = document.getElementById('btnUserMenu');
    if (!btnUserMenu) return;
    if (UserContext.userId) {
      const name = displayName || UserContext.displayName || 'Пользователь';
      const short = name.length > 12 ? name.slice(0, 11) + '…' : name;
      btnUserMenu.textContent = '👤 ' + short;
      btnUserMenu.title = name;
      btnUserMenu.classList.remove('topbar__user-btn--needs-user');
    } else {
      btnUserMenu.textContent = '👤 Выбрать…';
      btnUserMenu.title = 'Выбрать…';
      btnUserMenu.classList.add('topbar__user-btn--needs-user');
    }
    this.updateUserGuards();
    this.updateEmptyTreeHint();
  },

  updateUserGuards() {
    const hasUser = !!UserContext.userId;
    const msg = 'Сначала выберите пользователя';
    const addLabel = document.getElementById('addSolutionLabel');
    const fileInput = document.getElementById('fileInput');
    const btnClearAll = document.getElementById('btnClearAll');
    const btnSolutionMenu = document.getElementById('btnSolutionMenu');
    if (addLabel) {
      addLabel.title = hasUser ? 'Добавить решение (Solution.xml или .rar)' : msg;
      addLabel.setAttribute('aria-disabled', hasUser ? 'false' : 'true');
    }
    if (fileInput) fileInput.disabled = !hasUser;
    if (btnClearAll) {
      btnClearAll.title = hasUser ? 'Удалить все мои решения' : msg;
      btnClearAll.disabled = !hasUser;
    }
    if (btnSolutionMenu) {
      btnSolutionMenu.title = hasUser ? 'Выбрать решение' : msg;
    }
  },

  emptyTreeMessage() {
    if (!UserContext.userId) {
      return 'Сначала выберите или создайте пользователя (кнопка слева вверху)';
    }
    return 'Загрузите Solution.xml или .rar с Solution.xml внутри';
  },

  updateEmptyTreeHint() {
    const tree = document.getElementById('treeContainer');
    if (!tree) return;
    const empty = tree.querySelector('.props-empty');
    if (!empty) return;
    if (!this.solutionId) empty.textContent = this.emptyTreeMessage();
  },

  requireUser() {
    if (UserContext.userId) return true;
    this.openUserMenu();
    const btn = document.getElementById('btnUserMenu');
    if (btn) btn.title = 'Сначала выберите или создайте пользователя';
    return false;
  },

  closeUserMenu() {
    const userMenu = document.getElementById('userMenu');
    const btnUserMenu = document.getElementById('btnUserMenu');
    if (!userMenu || !btnUserMenu) return;
    userMenu.hidden = true;
    btnUserMenu.classList.remove('topbar__user-btn--open');
    btnUserMenu.setAttribute('aria-expanded', 'false');
  },

  renderUserMenu() {
    const userMenuList = document.getElementById('userMenuList');
    if (!userMenuList) return;
    userMenuList.innerHTML = '';
    if (this._users.length === 0) {
      const empty = document.createElement('div');
      empty.className = 'topbar__user-menu-empty';
      empty.textContent = 'Нет пользователей';
      userMenuList.appendChild(empty);
      return;
    }
    for (const u of this._users) {
      const item = document.createElement('button');
      item.type = 'button';
      item.className = 'topbar__user-menu-item';
      if (u.id === UserContext.userId) {
        item.classList.add('topbar__user-menu-item--active');
      }
      item.textContent = u.display_name || u.login;
      item.addEventListener('click', (e) => {
        e.stopPropagation();
        this.closeUserMenu();
        this.switchToUser(u.id);
      });
      userMenuList.appendChild(item);
    }
  },

  openUserMenu() {
    this.renderUserMenu();
    const userMenu = document.getElementById('userMenu');
    const btnUserMenu = document.getElementById('btnUserMenu');
    if (!userMenu || !btnUserMenu) return;
    userMenu.hidden = false;
    btnUserMenu.classList.add('topbar__user-btn--open');
    btnUserMenu.setAttribute('aria-expanded', 'true');
  },

  toggleUserMenu() {
    const userMenu = document.getElementById('userMenu');
    if (!userMenu) return;
    if (userMenu.hidden) this.openUserMenu();
    else this.closeUserMenu();
  },

  closeSolutionMenu() {
    const solutionMenu = document.getElementById('solutionMenu');
    const btnSolutionMenu = document.getElementById('btnSolutionMenu');
    if (!solutionMenu || !btnSolutionMenu) return;
    solutionMenu.hidden = true;
    btnSolutionMenu.classList.remove('topbar__solutions-btn--open');
    btnSolutionMenu.setAttribute('aria-expanded', 'false');
  },

  renderSolutionMenu() {
    const solutionMenuList = document.getElementById('solutionMenuList');
    if (!solutionMenuList) return;
    solutionMenuList.innerHTML = '';
    if (this._solutions.length === 0) {
      const empty = document.createElement('div');
      empty.className = 'topbar__solutions-menu-empty';
      empty.textContent = 'Нет загруженных решений';
      solutionMenuList.appendChild(empty);
      return;
    }
    for (const s of this._solutions) {
      const item = document.createElement('button');
      item.type = 'button';
      item.className = 'topbar__solutions-menu-item';
      if (s.id === this.solutionId) {
        item.classList.add('topbar__solutions-menu-item--active');
      }
      if (s.is_dirty) {
        item.classList.add('topbar__solutions-menu-item--dirty');
      }
      let label = s.name || 'Без имени';
      if (s.import_status === 'importing') {
        label += ' (импорт…)';
      } else if (s.import_status === 'failed') {
        label += ' (ошибка импорта)';
      }
      item.textContent = label;
      item.title = label;
      item.addEventListener('click', (e) => {
        e.stopPropagation();
        this.closeSolutionMenu();
        if (s.id !== this.solutionId) {
          this.selectSolution(s.id);
        }
      });
      solutionMenuList.appendChild(item);
    }
  },

  openSolutionMenu() {
    if (!this.requireUser()) return;
    this.renderSolutionMenu();
    const solutionMenu = document.getElementById('solutionMenu');
    const btnSolutionMenu = document.getElementById('btnSolutionMenu');
    if (!solutionMenu || !btnSolutionMenu) return;
    solutionMenu.hidden = false;
    btnSolutionMenu.classList.add('topbar__solutions-btn--open');
    btnSolutionMenu.setAttribute('aria-expanded', 'true');
  },

  toggleSolutionMenu() {
    const solutionMenu = document.getElementById('solutionMenu');
    if (!solutionMenu) return;
    if (solutionMenu.hidden) this.openSolutionMenu();
    else this.closeSolutionMenu();
  },

  updateSolutionMenuButton() {
    const btnSolutionMenu = document.getElementById('btnSolutionMenu');
    if (!btnSolutionMenu) return;
    btnSolutionMenu.hidden = this._solutions.length === 0;
  },

  applyUserContext(id, displayName) {
    UserContext.save(id, displayName);
    const gate = document.getElementById('userGateOverlay');
    if (gate) gate.hidden = true;
    this.updateUserMenuButton(displayName);
    this.closeUserMenu();
  },

  async switchToUser(id) {
    if (!id || id === UserContext.userId) return;
    if (UserContext.userId && this.isDirty && !confirm('Несохранённые изменения будут потеряны. Сменить пользователя?')) {
      return;
    }
    const selected = this._users.find((u) => u.id === id);
    this.resetAppState();
    this.applyUserContext(id, selected ? (selected.display_name || selected.login) : '');
    await Renderer.loadDisplaySettings();
    await Renderer.loadComplianceRules();
    await this.loadSolutionList();
  },

  async promptCreateUser() {
    this.closeUserMenu();
    if (UserContext.userId && this.isDirty && !confirm('Несохранённые изменения будут потеряны. Создать нового пользователя?')) {
      return;
    }
    try {
      await this.openUserGate();
      this.resetAppState();
      await Renderer.loadDisplaySettings();
      await Renderer.loadComplianceRules();
      await this.loadSolutionList();
    } catch (_) {}
  },

  openUserGate() {
    const gate = document.getElementById('userGateOverlay');
    const gateLogin = document.getElementById('userGateLogin');
    const gateContinue = document.getElementById('userGateContinue');
    const gateCancel = document.getElementById('userGateCancel');
    const gateTitle = gate?.querySelector('.upload-overlay__title');

    return new Promise((resolve, reject) => {
      if (gateTitle) gateTitle.textContent = 'Создать пользователя';
      if (gateLogin) {
        gateLogin.hidden = false;
        gateLogin.value = '';
      }
      if (gateContinue) gateContinue.textContent = 'Создать';
      if (gateCancel) gateCancel.hidden = false;
      gate.hidden = false;
      if (gateLogin) gateLogin.focus();

      const cleanup = () => {
        gateContinue.removeEventListener('click', onContinue);
        gateCancel.removeEventListener('click', onCancel);
        this._userGateOnCancel = null;
        if (gateLogin) gateLogin.value = '';
      };

      const onCancel = () => {
        cleanup();
        gate.hidden = true;
        reject(new Error('cancelled'));
      };

      const onContinue = async () => {
        try {
          const login = (gateLogin?.value || '').trim();
          if (!login) {
            alert('Введите логин');
            return;
          }
          const user = await API.createUser(login, login);
          this._users.push(user);
          this.applyUserContext(user.id, user.display_name || user.login);
          cleanup();
          gate.hidden = true;
          resolve({ action: 'created', user });
        } catch (err) {
          alert('Ошибка: ' + err.message);
        }
      };

      this._userGateOnCancel = onCancel;
      gateContinue.addEventListener('click', onContinue);
      gateCancel.addEventListener('click', onCancel);
    });
  },

  async init() {
    UserContext.load();
    this.bindUiHandlers();
    this.updateUserMenuButton(UserContext.displayName || '');

    try {
      await this.initUser();
      await Renderer.loadDisplaySettings();
      await Renderer.loadComplianceRules();

      Explorer.init(
        document.getElementById('treeContainer'),
        document.getElementById('treeFooter'),
        (node) => this.openScheme(node.xml_id, node.name),
      );

      Inspector.init(
        document.getElementById('propsBody'),
        document.getElementById('propsTitle'),
        document.getElementById('propsTabs'),
      );

      Inspector.onPatch = (objectId, patch) => this.patchObject(objectId, patch, { lightweight: true });
      Inspector.onEntityPatch = (entityType, entityId, patch) => this.patchEntity(entityType, entityId, patch);

      if (typeof WinCcOaDrop !== 'undefined') WinCcOaDrop.install();

      await this.loadSolutionList();
    } catch (err) {
      console.error('App init failed:', err);
      this.updateUserMenuButton(UserContext.displayName || '');
    }
  },

  async initUser() {
    try {
      this._users = await API.listUsers();
    } catch (_) {
      this._users = [];
    }

    if (UserContext.userId && this._users.some((u) => u.id === UserContext.userId)) {
      const current = this._users.find((u) => u.id === UserContext.userId);
      this.updateUserMenuButton(current ? (current.display_name || current.login) : '');
    } else {
      UserContext.clear();
      this.updateUserMenuButton('');
    }
  },

  async loadSolutionList(options = {}) {
    if (!UserContext.userId) {
      this._solutions = [];
      this.updateSolutionMenuButton();
      return;
    }
    try {
      const list = await API.listSolutions();
      this._solutions = list;
      this.updateSolutionMenuButton();

      if (options.skipAutoLoad) return;

      const readyList = list.filter((s) => !s.import_status || s.import_status === 'ready');
      const savedId = localStorage.getItem(UserContext.lastSolutionKey());
      const idToLoad = (savedId && readyList.some(s => s.id === savedId))
        ? savedId
        : (readyList.length === 1 ? readyList[0].id : null);
      if (idToLoad) {
        await this.selectSolution(idToLoad);
      }
    } catch (_) {}
  },

  async selectSolution(id) {
    if (!this.requireUser()) return;
    if (!id) return;
    const item = (this._solutions || []).find((s) => s.id === id);
    if (item && item.import_status === 'importing') {
      await this.resumeImportProgress(item);
      return;
    }
    if (item && item.import_status === 'failed') {
      alert('Импорт решения не удался' + (item.import_error ? ': ' + item.import_error : '.'));
      return;
    }
    this.clearDuplicateScanState();
    this.solutionId = id;
    this.invalidateComplianceParamsCache();
    localStorage.setItem(UserContext.lastSolutionKey(), id);
    this.currentSchemeId = null;
    this.currentScheme = null;
    document.getElementById('schemeSvg').hidden = true;
    document.getElementById('schemeSvg').innerHTML = '';
    document.getElementById('canvasPlaceholder').style.display = '';
    document.getElementById('canvasToolbar').hidden = true;
    document.getElementById('canvasStatusbar').hidden = true;
    document.getElementById('app').classList.remove('app--scheme-open', 'app--object-selected');
    this.selectedObjectId = null;
    this.clearObjectLocator();
    if (this.objectSettings.open) this.forceCloseObjectSettingsPanel();
    await this.loadSolution();
  },

  importPhaseTitle(phase) {
    switch (phase) {
      case 'upload': return 'Загрузка файла…';
      case 'unpack': return 'Распаковка архива…';
      case 'parse': return 'Разбор XML…';
      case 'persist': return 'Сохранение схем…';
      case 'index': return 'Построение индексов…';
      case 'catalog': return 'Подготовка списков параметров…';
      default: return 'Обработка на сервере…';
    }
  },

  formatImportEta(etaSeconds) {
    const eta = Number(etaSeconds) || 0;
    if (eta <= 0) return '';
    if (eta < 60) return `осталось ~${eta} с`;
    return `осталось ~${Math.max(1, Math.round(eta / 60))} мин`;
  },

  formatSaveStatus(meta) {
    const percent = Math.round(Number(meta?.save_percent) || 0);
    const main = `Сохранение… ${percent}%`;
    const eta = this.formatImportEta(meta?.save_eta_seconds);
    return eta ? `${main} · ${eta}` : main;
  },

  formatImportStatus(meta, detailFallback) {
    const phaseLabels = {
      unpack: 'Распаковка архива',
      parse: 'Разбор XML',
      persist: 'Сохранение схем',
      index: 'Построение индексов',
      catalog: 'Подготовка списков параметров',
    };
    const detail = phaseLabels[meta?.import_phase] || detailFallback || 'Обработка';
    let main = `${detail}…`;
    const processed = Number(meta?.import_processed) || 0;
    const total = Number(meta?.import_total) || 0;
    const hasPercent = meta && Object.prototype.hasOwnProperty.call(meta, 'import_percent');
    const percent = Number(meta?.import_percent) || 0;
    const countablePhases = ['persist', 'index', 'catalog'];
    if (countablePhases.includes(meta?.import_phase) && total > 0) {
      main = `${detail}… ${processed}/${total}`;
    } else if (hasPercent) {
      main = `${detail}… ${Math.round(percent)}%`;
    }
    const eta = this.formatImportEta(meta?.import_eta_seconds);
    return eta ? `${main} · ${eta}` : main;
  },

  compositeImportBar(uploadFraction, importPercent, shownBar) {
    let bar = 0;
    if (uploadFraction != null) {
      bar = uploadFraction * 10;
    } else {
      bar = 10 + (Number(importPercent) || 0) * 0.9;
    }
    bar = Math.max(0, Math.min(100, bar));
    if (shownBar != null) bar = Math.max(shownBar, bar);
    return bar;
  },

  async resumeImportProgress(item) {
    const overlay = document.getElementById('uploadOverlay');
    const overlayTitle = document.getElementById('uploadOverlayTitle');
    const overlayFile = document.getElementById('uploadOverlayFile');
    const overlayBar = document.getElementById('uploadOverlayBar');
    const overlayStatus = document.getElementById('uploadOverlayStatus');
    let shownBar = 0;

    const paint = (meta) => {
      const phase = meta?.import_phase || 'parse';
      shownBar = this.compositeImportBar(null, meta?.import_percent, shownBar);
      overlayTitle.textContent = this.importPhaseTitle(phase);
      overlayFile.textContent = item.name || 'Импорт решения';
      overlayStatus.textContent = this.formatImportStatus(meta);
      overlayBar.style.width = `${Math.round(shownBar)}%`;
      overlay.hidden = false;
    };

    paint(item);
    document.body.style.cursor = 'wait';
    try {
      await API.pollImportStatus(item.id, paint);
      this.clearDuplicateScanState();
      this.solutionId = item.id;
      this.invalidateComplianceParamsCache();
      localStorage.setItem(UserContext.lastSolutionKey(), item.id);
      localStorage.setItem('lastSolutionId', item.id);
      await this.loadSolution();
      await this.loadSolutionList({ skipAutoLoad: true });
    } catch (err) {
      alert('Ошибка импорта: ' + (err?.message || String(err)));
    } finally {
      overlay.hidden = true;
      document.body.style.cursor = '';
    }
  },

  async uploadFile(e) {
    const file = e.target.files[0];
    if (!file) return;
    if (!this.requireUser()) {
      e.target.value = '';
      return;
    }
    const addLabel = document.getElementById('addSolutionLabel');
    const overlay = document.getElementById('uploadOverlay');
    const overlayTitle = document.getElementById('uploadOverlayTitle');
    const overlayFile = document.getElementById('uploadOverlayFile');
    const overlayBar = document.getElementById('uploadOverlayBar');
    const overlayStatus = document.getElementById('uploadOverlayStatus');
    let shownBar = 0;

    const paintUpload = (loaded, total) => {
      const fraction = total > 0 ? loaded / total : 0;
      shownBar = this.compositeImportBar(fraction, null, shownBar);
      const pct = total > 0 ? Math.round(fraction * 100) : 0;
      overlayTitle.textContent = this.importPhaseTitle('upload');
      overlayFile.textContent = `${file.name} (${this.formatFileSize(file.size)})`;
      overlayStatus.textContent = `Передача файла… ${pct}%`;
      overlayBar.style.width = `${Math.round(shownBar)}%`;
      overlay.hidden = false;
    };

    const paintServer = (meta) => {
      const phase = meta?.import_phase;
      shownBar = this.compositeImportBar(null, meta?.import_percent, shownBar);
      overlayTitle.textContent = phase ? this.importPhaseTitle(phase) : 'Обработка на сервере…';
      overlayFile.textContent = `${file.name} (${this.formatFileSize(file.size)})`;
      overlayStatus.textContent = phase
        ? this.formatImportStatus(meta)
        : 'Ожидание статуса…';
      overlayBar.style.width = `${Math.round(shownBar)}%`;
      overlay.hidden = false;
    };

    addLabel.classList.add('btn--disabled');
    document.body.style.cursor = 'wait';
    paintUpload(0, file.size || 1);

    try {
      if (!/\.(xml|rar)$/i.test(file.name)) {
        throw new Error('не xml/не rar');
      }
      const result = await API.uploadSolution(file, ({ loaded, total }) => {
        paintUpload(loaded, total);
      });
      shownBar = this.compositeImportBar(1, null, shownBar);
      paintServer({});
      await API.pollImportStatus(result.id, paintServer);
      this.clearDuplicateScanState();
      this.solutionId = result.id;
      this.invalidateComplianceParamsCache();
      localStorage.setItem(UserContext.lastSolutionKey(), result.id);
      localStorage.setItem('lastSolutionId', result.id);
      await this.loadSolution();
      await this.loadSolutionList({ skipAutoLoad: true });
    } catch (err) {
      const message = this.normalizeUploadError(err);
      alert('Ошибка загрузки: ' + message);
    } finally {
      overlay.hidden = true;
      addLabel.classList.remove('btn--disabled');
      document.body.style.cursor = '';
      e.target.value = '';
    }
  },

  formatFileSize(bytes) {
    if (bytes >= 1 << 30) return `${(bytes / (1 << 30)).toFixed(1)} GB`;
    if (bytes >= 1 << 20) return `${Math.round(bytes / (1 << 20))} MB`;
    return `${Math.round(bytes / 1024)} KB`;
  },

  normalizeUploadError(err) {
    const msg = err?.message || String(err);
    if (msg === 'Load failed' || msg === 'Failed to fetch') {
      return 'Соединение прервано. Загрузка не завершена.';
    }
    if (msg === 'не xml/не rar' || /unsupported import format/i.test(msg)) {
      return 'Поддерживаются только Solution.xml и архивы .rar';
    }
    if (/Solution\.xml not found in archive/i.test(msg)) {
      return 'В архиве не найден Solution.xml';
    }
    if (
      /invalid archive entry path/i.test(msg) ||
      /encrypted archives are not supported/i.test(msg) ||
      /archive has too many entries/i.test(msg) ||
      /zip:/i.test(msg) ||
      /not a valid zip/i.test(msg)
    ) {
      return 'Не удалось открыть архив. Проверьте файл .rar';
    }
    return msg;
  },

  async loadSolution() {
    const meta = await API.getSolutionMeta(this.solutionId);
    const tree = await API.getTree(this.solutionId, { depth: 'skeleton' });
    this.rebuildSchemeProjectMap(tree);
    const session = await API.createSession(this.solutionId);
    this.sessionId = session.session_id;

    document.getElementById('solutionName').textContent = meta.name;
    document.getElementById('btnSave').disabled = false;
    document.getElementById('btnExport').disabled = false;
    document.getElementById('btnExportObjects').disabled = false;
    document.getElementById('btnTagEditSettings').disabled = false;
    document.getElementById('btnDuplicateSettings').disabled = false;
    document.getElementById('btnDeleteSolution').hidden = false;
    document.getElementById('app').classList.remove('app--empty');
    document.getElementById('app').classList.add('app--loaded');

    Explorer.setSolutionId(this.solutionId);
    Explorer.collapsedNodes.clear();
    Explorer.setComplianceHints([]);
    this.lastTree = tree;
    Explorer.render(tree);
    Inspector.showSolutionMeta(meta);
    this.setDirty(meta.is_dirty);
    this.updateSolutionSearchEnabled();
    document.getElementById('treeFooter').textContent = 'Проверка…';
    document.getElementById('treeFooter').style.color = '';
    await this.refreshComplianceReport();
    await this.restoreDuplicateScanSnapshot();
  },

  clearDuplicateScanState() {
    this.stopDuplicateBackgroundPolling();
    this.duplicateBackgroundEnabled = false;
    this.duplicateCompliance = null;
    this.duplicateRows = [];
    this.duplicatePanelDismissed = false;
    this.duplicatePanelUserClosedDuringScan = false;
    this.updateDuplicatePanel();
    const wrap = document.getElementById('duplicateTableWrap');
    if (wrap) {
      wrap.innerHTML = '<div class="duplicate-table__empty">Запустите поиск, чтобы увидеть дубликаты Tag</div>';
    }
    this.syncDuplicateExportButton();
  },

  async restoreDuplicateScanSnapshot() {
    if (!this.solutionId) return;
    try {
      const result = await API.getDuplicateBackgroundResults(this.solutionId);
      const rows = result?.rows || [];
      const violations = result?.violations_by_object || {};
      if (!rows.length && !Object.keys(violations).length) {
        return;
      }
      await this.applyDuplicateScanResult(result, { soft: true, reopenPanel: false });
    } catch (_) {}
  },

  updateComplianceFooter(report, force = false) {
    if (this.currentSchemeId && !force) return;
    const footer = document.getElementById('treeFooter');
    const nodes = report?.nodes || [];
    const violations = nodes.filter(n => n.indicators && n.indicators.length > 0).length;
    const dupObjects = Object.keys(this.duplicateCompliance?.violationsByObjectId || {}).length;
    if (violations > 0) {
      footer.textContent = dupObjects > 0
        ? `Нарушений: ${violations} (дубликатов tag: ${dupObjects} объектов)`
        : `Нарушений: ${violations}`;
      footer.style.color = 'var(--state-alarm)';
    } else {
      footer.textContent = 'Проверки пройдены';
      footer.style.color = 'var(--state-normal)';
    }
  },

  async refreshComplianceForProject(projectXmlId) {
    if (!this.solutionId || !projectXmlId) return;
    try {
      const report = await API.getComplianceReport(this.solutionId, { project_xml_id: projectXmlId });
      const scope = this.getProjectComplianceScope(projectXmlId);
      Explorer.mergeComplianceHints(report.nodes || [], scope);
      this.applyDuplicateComplianceToExplorer(scope);
      this.syncComplianceReportFromHints();
      Explorer.applyComplianceToTree();
    } catch (_) {}
  },

  rebuildSchemeProjectMap(tree) {
    this.schemeProjectMap = {};
    this.projectFolderMap = {};
    const walk = (nodes, folderXmlId) => {
      for (const node of nodes || []) {
        if (node.type === 'folder') {
          walk(node.children, node.xml_id);
        } else if (node.type === 'project') {
          if (folderXmlId) {
            this.projectFolderMap[node.xml_id] = folderXmlId;
          }
          for (const child of node.children || []) {
            if (child.type === 'graphic_scheme') {
              this.schemeProjectMap[child.xml_id] = node.xml_id;
            }
          }
        } else if (node.children?.length) {
          walk(node.children, folderXmlId);
        }
      }
    };
    walk(tree, null);
  },

  onProjectSchemesLoaded(projectXmlId, schemes) {
    for (const scheme of schemes || []) {
      this.schemeProjectMap[scheme.xml_id] = projectXmlId;
    }
  },

  findSchemeContext(schemeXmlId) {
    const projectXmlId = this.schemeProjectMap[schemeXmlId] || null;
    const folderXmlId = projectXmlId ? (this.projectFolderMap[projectXmlId] || null) : null;
    return { projectXmlId, folderXmlId };
  },

  computeSchemeComplianceIndicators(scheme) {
    const ruleMap = {};
    for (const obj of scheme.GraphicObject || []) {
      const template = Renderer.getTemplate(obj);
      const violations = Renderer.evaluateComplianceAll(obj, template);
      for (const v of violations) {
        const ind = v.indication || {};
        ruleMap[v.ruleId] = {
          rule_id: v.ruleId,
          color: ind.color || '#F85149',
          type: ind.type || 'solid',
        };
      }
    }
    return Object.values(ruleMap);
  },

  getDuplicateViolations(objectId) {
    if (!this.duplicateCompliance?.violationsByObjectId?.[String(objectId)]) return null;
    return {
      ruleId: this.duplicateCompliance.ruleId,
      indication: this.duplicateCompliance.indication || { type: 'solid', color: '#F85149' },
    };
  },

  getDuplicateRelatedRows(objectId) {
    if (!objectId || !this.duplicateRows?.length) return [];
    const id = String(objectId);
    if (!this.duplicateCompliance?.violationsByObjectId?.[id]) return [];

    const objectTags = new Set(
      this.duplicateRows
        .filter(r => String(r.object_id) === id)
        .map(r => r.tag)
        .filter(Boolean),
    );
    if (!objectTags.size) return [];

    return this.duplicateRows.filter(r => objectTags.has(r.tag));
  },

  duplicateSourceLabel(row) {
    const scope = row.scope_label || '';
    const source = row.source_label || '';
    if (scope && source) return `${scope} · ${source}`;
    return scope || source || '—';
  },

  parseDuplicateExcludedScopes(indication) {
    const ind = indication && typeof indication === 'string'
      ? (() => { try { return JSON.parse(indication); } catch (_) { return {}; } })()
      : (indication || {});
    const excluded = new Set(Array.isArray(ind.excluded_scopes) ? ind.excluded_scopes : []);
    return excluded;
  },

  parseDuplicateExcludedTagPatterns(indication) {
    const ind = indication && typeof indication === 'string'
      ? (() => { try { return JSON.parse(indication); } catch (_) { return {}; } })()
      : (indication || {});
    const raw = Array.isArray(ind.excluded_tag_patterns) ? ind.excluded_tag_patterns : [];
    const patterns = [];
    for (const item of raw) {
      if (!item || typeof item !== 'object') continue;
      const mode = String(item.mode || '').trim();
      const value = String(item.value ?? '').trim();
      if (!value) continue;
      if (this.duplicateTagPatternNeedsSingleChar(mode)) {
        if ([...value].length !== 1) continue;
      } else if (mode === 'contains_text') {
      } else {
        continue;
      }
      patterns.push({ mode, value });
    }
    return patterns;
  },

  duplicateTagPatternNeedsSingleChar(mode) {
    return mode === 'contains_char' || mode === 'exact_one_char' || mode === 'first_char';
  },

  buildDuplicateExcludedScopesFromFields(fields) {
    const excluded = [];
    if (!fields.excludeObject?.checked) excluded.push('object');
    if (!fields.excludeProperty?.checked) excluded.push('property');
    if (!fields.excludeCommand?.checked) excluded.push('command');
    if (!fields.excludeProtection?.checked) excluded.push('protection');
    return excluded;
  },

  buildDuplicateExcludedTagPatternsFromList(listEl) {
    if (!listEl) return [];
    const patterns = [];
    for (const row of listEl.querySelectorAll('.duplicate-tag-exclusion')) {
      const mode = String(row._fields?.modeSel?.value || '').trim();
      const value = String(row._fields?.valueInput?.value || '').trim();
      if (!value) continue;
      if (this.duplicateTagPatternNeedsSingleChar(mode)) {
        if ([...value].length !== 1) continue;
      } else if (mode === 'contains_text') {
      } else {
        continue;
      }
      patterns.push({ mode, value });
    }
    return patterns;
  },

  syncDuplicateTagExclusionsEmpty(listEl) {
    if (!listEl) return;
    let empty = listEl.querySelector('.duplicate-tag-exclusions-empty');
    const hasRows = listEl.querySelectorAll('.duplicate-tag-exclusion').length > 0;
    if (hasRows) {
      if (empty) empty.remove();
      return;
    }
    if (!empty) {
      empty = document.createElement('div');
      empty.className = 'duplicate-tag-exclusions-empty';
      empty.textContent = 'Нет исключений — учитываются все Tag';
      listEl.appendChild(empty);
    }
  },

  buildDuplicateTagExclusionRow(pattern) {
    const row = document.createElement('div');
    row.className = 'duplicate-tag-exclusion';

    const modeSel = document.createElement('select');
    modeSel.className = 'input duplicate-tag-exclusion__field';
    for (const [val, label] of [
      ['contains_char', 'Содержит символ'],
      ['contains_text', 'Содержит текст'],
      ['exact_one_char', 'Содержит один символ'],
      ['first_char', 'Первый символ'],
    ]) {
      const opt = document.createElement('option');
      opt.value = val;
      opt.textContent = label;
      if (val === (pattern?.mode || 'contains_char')) opt.selected = true;
      modeSel.appendChild(opt);
    }

    const valueInput = document.createElement('input');
    valueInput.type = 'text';
    valueInput.className = 'input duplicate-tag-exclusion__field';
    valueInput.value = pattern?.value || '';

    const delBtn = document.createElement('button');
    delBtn.type = 'button';
    delBtn.className = 'btn btn--ghost duplicate-tag-exclusion__delete';
    delBtn.textContent = '✕';
    delBtn.addEventListener('click', () => {
      const list = row.parentElement;
      row.remove();
      this.syncDuplicateTagExclusionsEmpty(list);
    });

    const valueWrap = this.labeledBlock('Символ', valueInput);
    const valueLabel = valueWrap.querySelector('.props-field__label');

    const syncValueFieldUI = () => {
      const isChar = this.duplicateTagPatternNeedsSingleChar(modeSel.value);
      if (valueLabel) valueLabel.textContent = isChar ? 'Символ' : 'Текст';
      if (isChar) {
        valueInput.maxLength = 1;
        valueInput.placeholder = 'Символ';
        const runes = [...valueInput.value];
        if (runes.length > 1) valueInput.value = runes[0] || '';
      } else {
        valueInput.removeAttribute('maxlength');
        valueInput.placeholder = 'часть Tag для исключения';
      }
    };
    modeSel.addEventListener('change', syncValueFieldUI);
    syncValueFieldUI();

    row.appendChild(this.labeledBlock('Тип', modeSel));
    row.appendChild(valueWrap);
    row.appendChild(delBtn);
    row._fields = { modeSel, valueInput };
    return row;
  },

  reaggregateProjectCompliance(projectXmlId) {
    const ruleMap = {};
    for (const [schemeXmlId, projId] of Object.entries(this.schemeProjectMap)) {
      if (projId !== projectXmlId) continue;
      const hint = Explorer.complianceHints[schemeXmlId];
      if (!hint?.indicators) continue;
      for (const ind of hint.indicators) {
        ruleMap[ind.rule_id] = ind;
      }
    }
    const indicators = Object.values(ruleMap);
    if (indicators.length > 0) {
      Explorer.complianceHints[projectXmlId] = {
        xml_id: projectXmlId,
        type: 'project',
        indicators,
      };
    } else {
      delete Explorer.complianceHints[projectXmlId];
    }
  },

  reaggregateFolderCompliance(folderXmlId) {
    const ruleMap = {};
    for (const [projectXmlId, folderId] of Object.entries(this.projectFolderMap)) {
      if (folderId !== folderXmlId) continue;
      const hint = Explorer.complianceHints[projectXmlId];
      if (!hint?.indicators) continue;
      for (const ind of hint.indicators) {
        ruleMap[ind.rule_id] = ind;
      }
    }
    const indicators = Object.values(ruleMap);
    if (indicators.length > 0) {
      Explorer.complianceHints[folderXmlId] = {
        xml_id: folderXmlId,
        type: 'folder',
        indicators,
      };
    } else {
      delete Explorer.complianceHints[folderXmlId];
    }
  },

  mergeComplianceIndicators(existingIndicators, newIndicators) {
    const ruleMap = {};
    for (const ind of [...(existingIndicators || []), ...(newIndicators || [])]) {
      if (ind?.rule_id) ruleMap[ind.rule_id] = ind;
    }
    return Object.values(ruleMap);
  },

  applyDuplicateComplianceToExplorer(scopeXmlIds = null) {
    const dupRuleId = this.duplicateCompliance?.ruleId || null;
    const dupNodes = this.duplicateCompliance?.nodes || [];
    const dupNodeById = {};
    for (const node of dupNodes) {
      dupNodeById[node.xml_id] = node;
    }

    const scope = scopeXmlIds instanceof Set ? scopeXmlIds : (scopeXmlIds ? new Set(scopeXmlIds) : null);
    const idsToUpdate = new Set(Object.keys(dupNodeById));

    if (dupRuleId) {
      for (const [xmlId, hint] of Object.entries(Explorer.complianceHints)) {
        if (scope && !scope.has(xmlId)) continue;
        if (hint.indicators?.some(ind => ind.rule_id === dupRuleId)) {
          idsToUpdate.add(xmlId);
        }
      }
    }

    if (scope) {
      for (const xmlId of scope) {
        idsToUpdate.add(xmlId);
      }
    }

    const targetIds = scope ? [...idsToUpdate].filter(id => scope.has(id)) : [...idsToUpdate];
    for (const xmlId of targetIds) {
      const existing = Explorer.complianceHints[xmlId];
      let indicators = (existing?.indicators || []).filter(
        ind => !dupRuleId || ind.rule_id !== dupRuleId,
      );
      const dupNode = dupNodeById[xmlId];
      if (dupNode?.indicators?.length) {
        indicators = this.mergeComplianceIndicators(indicators, dupNode.indicators);
      }
      if (indicators.length > 0) {
        Explorer.complianceHints[xmlId] = {
          xml_id: xmlId,
          type: dupNode?.type || existing?.type,
          indicators,
        };
      } else {
        delete Explorer.complianceHints[xmlId];
      }
    }
  },

  syncComplianceReportFromHints() {
    const nodes = Object.values(Explorer.complianceHints).filter(
      h => h.indicators && h.indicators.length > 0,
    );
    if (!this.complianceReport) {
      this.complianceReport = { nodes: [] };
    }
    this.complianceReport.nodes = nodes;
  },

  getProjectComplianceScope(projectXmlId) {
    const scope = new Set([projectXmlId]);
    const folderXmlId = this.projectFolderMap[projectXmlId];
    if (folderXmlId) scope.add(folderXmlId);
    for (const [schemeXmlId, projId] of Object.entries(this.schemeProjectMap)) {
      if (projId === projectXmlId) scope.add(schemeXmlId);
    }
    return scope;
  },

  syncExplorerComplianceFromCurrentScheme() {
    if (!this.currentSchemeId || !this.currentScheme) return;
    const indicators = this.computeSchemeComplianceIndicators(this.currentScheme);
    const schemeXmlId = this.currentSchemeId;
    if (indicators.length > 0) {
      Explorer.complianceHints[schemeXmlId] = {
        xml_id: schemeXmlId,
        type: 'graphic_scheme',
        indicators,
      };
    } else {
      delete Explorer.complianceHints[schemeXmlId];
    }
    const { projectXmlId, folderXmlId } = this.findSchemeContext(schemeXmlId);
    if (projectXmlId) {
      this.reaggregateProjectCompliance(projectXmlId);
    }
    if (folderXmlId) {
      this.reaggregateFolderCompliance(folderXmlId);
    }
    this.applyDuplicateComplianceToExplorer();
    this.syncComplianceReportFromHints();
    Explorer.applyComplianceToTree();
  },

  async refreshComplianceReport() {
    if (!this.solutionId) return;
    try {
      const options = {};
      if (this.currentSchemeId) {
        const projectXmlId = this.schemeProjectMap[this.currentSchemeId];
        if (projectXmlId) {
          options.project_xml_id = projectXmlId;
        }
      }
      const report = await API.getComplianceReport(this.solutionId, options);
      this.complianceReport = report;
      Explorer.setComplianceHints(report.nodes || []);
      this.applyDuplicateComplianceToExplorer();
      this.syncComplianceReportFromHints();
      if (this.lastTree) {
        Explorer.applyComplianceToTree();
      }
      this.updateComplianceFooter({ nodes: Object.values(Explorer.complianceHints) });
    } catch (_) {
      Explorer.setComplianceHints([]);
      this.applyDuplicateComplianceToExplorer();
      Explorer.applyComplianceToTree();
      if (!this.currentSchemeId) {
        const footer = document.getElementById('treeFooter');
        footer.textContent = 'Ошибка проверки';
        footer.style.color = 'var(--state-alarm)';
      }
    }
  },

  clearObjectCache() {
    this.objectDetailsCache.clear();
    this.cachedSchemeId = null;
  },

  async preloadSchemeObjects(schemeXMLId) {
    this.clearObjectCache();
    this.cachedSchemeId = schemeXMLId;
    const items = await API.getSchemeObjectDetails(this.solutionId, schemeXMLId, this.sessionId);
    for (const details of items) {
      const id = details.graphic_object?.Id;
      if (id) this.objectDetailsCache.set(id, details);
    }
  },

  updateObjectCache(objectId, patch) {
    const cached = this.objectDetailsCache.get(objectId);
    if (!cached) return;
    cached.graphic_object = patch;
    cached.technology_objects = patch.TechnologyObject || [];
    this.objectDetailsCache.set(objectId, cached);
  },

  async openScheme(schemeXMLId, schemeName) {
    const viewport = document.getElementById('canvasViewport');
    const loading = document.getElementById('canvasLoading');
    loading.hidden = false;

    try {
      if (typeof Inspector.flushPendingEdits === 'function') {
        await Inspector.flushPendingEdits();
      }

      if (this.currentSchemeId && this.currentSchemeId !== schemeXMLId) {
        await API.closeScheme(this.solutionId, this.sessionId);
      }

      this.currentSchemeId = schemeXMLId;
      await API.openScheme(this.solutionId, this.sessionId, schemeXMLId);

      const scheme = await API.getScheme(this.solutionId, schemeXMLId, this.sessionId);
      this.currentScheme = scheme;

      loading.textContent = 'Загрузка объектов…';
      await this.preloadSchemeObjects(schemeXMLId);

      const svg = document.getElementById('schemeSvg');
      svg.hidden = false;
      document.getElementById('canvasPlaceholder').style.display = 'none';
      document.getElementById('canvasToolbar').hidden = false;
      document.getElementById('canvasStatusbar').hidden = false;

      Renderer.showLabels = false;
      loading.textContent = 'Загрузка схемы… 0%';
      await Renderer.renderSchemeAsync(svg, scheme, (obj) => this.selectObject(obj), (pct) => {
        loading.textContent = `Загрузка схемы… ${pct}%`;
      });
      this.applyTransform();
      Inspector.showSchemeInfo(scheme);

      document.getElementById('app').classList.add('app--scheme-open');
      document.getElementById('app').classList.remove('app--object-selected');
      this.selectedObjectId = null;
      if (this.objectSettings.open) this.forceCloseObjectSettingsPanel();
      document.getElementById('treeFooter').textContent = schemeName || scheme.Name;
      this.updateSchemeLockUI();
      this.updateDuplicatePanel();
    } catch (err) {
      this.currentSchemeId = null;
      this.currentScheme = null;
      this.clearObjectCache();
      alert('Ошибка открытия схемы: ' + err.message);
      document.getElementById('schemeSvg').hidden = true;
      document.getElementById('canvasPlaceholder').style.display = '';
      document.getElementById('canvasToolbar').hidden = true;
      document.getElementById('canvasStatusbar').hidden = true;
      document.getElementById('app').classList.remove('app--scheme-open');
    } finally {
      loading.hidden = true;
    }
  },

  async closeScheme() {
    if (!this.currentSchemeId) return;
    if (this.objectSettings.open) {
      const closed = await this.tryCloseObjectSettingsPanel();
      if (!closed) return;
    }
    await API.closeScheme(this.solutionId, this.sessionId);
    this.currentSchemeId = null;
    this.currentScheme = null;
    this.selectedObjectId = null;
    this.clearObjectLocator();
    this.clearObjectCache();

    document.getElementById('schemeSvg').hidden = true;
    document.getElementById('schemeSvg').innerHTML = '';
    document.getElementById('canvasPlaceholder').style.display = '';
    document.getElementById('canvasToolbar').hidden = true;
    document.getElementById('canvasStatusbar').hidden = true;

    document.getElementById('app').classList.remove('app--scheme-open', 'app--object-selected');
    this.hideDuplicatePanel();
    this.setDirty(true);
    Inspector.showEmpty('Схема закрыта. Изменения в буфере.');
    if (this.complianceReport) {
      this.updateComplianceFooter(this.complianceReport);
    }
  },

  clearObjectLocator() {
    if (this.locatorTimer) {
      clearTimeout(this.locatorTimer);
      this.locatorTimer = null;
    }
    const svg = document.getElementById('schemeSvg');
    Renderer.clearLocatorArrows(svg);
  },

  revealSelectedFromList(objectId) {
    const svg = document.getElementById('schemeSvg');
    if (!svg || svg.hidden || !objectId) return;
    const run = () => {
      if (!Renderer.objectBBoxInView(svg, objectId)) {
        this.centerObjectInViewport(objectId);
      }
      Renderer.showLocatorArrows(svg, objectId);
      if (this.locatorTimer) clearTimeout(this.locatorTimer);
      this.locatorTimer = setTimeout(() => {
        Renderer.clearLocatorArrows(svg);
        this.locatorTimer = null;
      }, 2000);
    };
    requestAnimationFrame(() => requestAnimationFrame(run));
  },

  async selectObject(obj, options = {}) {
    const template = Renderer.getTemplate(obj);
    if (!Renderer.canInteractWithObject(template)) return;

    if (this.objectSettings.open) {
      const panel = this.objectSettings;
      if (panel.objectId === obj.Id) {
        Renderer.selectObject(document.getElementById('schemeSvg'), obj.Id);
        Inspector.loadObject(this.solutionId, obj.Id);
        this.selectedObjectId = obj.Id;
        document.getElementById('app').classList.add('app--object-selected');
        if (options.fromList) this.revealSelectedFromList(obj.Id);
        return;
      }
      if (panel.template === template) {
        panel.objectId = obj.Id;
      } else if (panel.dirty) {
        const choice = await this.confirmObjectSettingsDirty();
        if (choice === 'cancel') {
          if (panel.objectId) {
            const prev = this.currentScheme?.GraphicObject?.find(o => o.Id === panel.objectId);
            if (prev) {
              Renderer.selectObject(document.getElementById('schemeSvg'), panel.objectId);
              Inspector.loadObject(this.solutionId, panel.objectId);
              this.selectedObjectId = panel.objectId;
              document.getElementById('app').classList.add('app--object-selected');
            }
          }
          return;
        }
        if (choice === 'accept') {
          const saved = await this.acceptObjectSettings({ keepOpen: true });
          if (!saved) return;
        } else {
          await this.discardObjectSettingsPreview();
        }
        await this.bindObjectSettingsToObject(obj);
        await this.previewObjectSettings();
      } else {
        await this.bindObjectSettingsToObject(obj);
        await this.previewObjectSettings();
      }
    }

    Renderer.selectObject(document.getElementById('schemeSvg'), obj.Id);
    Inspector.loadObject(this.solutionId, obj.Id);
    this.selectedObjectId = obj.Id;
    document.getElementById('app').classList.add('app--object-selected');
    if (options.fromList) this.revealSelectedFromList(obj.Id);
  },

  async patchObject(objectId, patch, options = {}) {
    if (!this.currentSchemeId) {
      console.error('patchObject: нет активной схемы (scheme_xml_id)');
      return;
    }
    await API.patchObject(this.solutionId, this.sessionId, objectId, this.currentSchemeId, patch);
    this.setDirty(true);

    const idx = this.currentScheme.GraphicObject.findIndex(o => o.Id === objectId);
    if (idx >= 0) this.currentScheme.GraphicObject[idx] = patch;

    if (Inspector.currentObject?.graphic_object?.Id === objectId) {
      Inspector.currentObject.graphic_object = patch;
      Inspector.currentObject.technology_objects = patch.TechnologyObject || [];
    }
    this.updateObjectCache(objectId, patch);

    if (options.skipFullRender) {
      this.syncExplorerComplianceFromCurrentScheme();
      return;
    }

    const svg = document.getElementById('schemeSvg');
    if (options.lightweight) {
      Renderer.replaceObject(svg, patch, (obj) => this.selectObject(obj));
      Renderer.selectObject(svg, objectId);
      this.syncExplorerComplianceFromCurrentScheme();
      return;
    }
    await Renderer.renderSchemeAsync(svg, this.currentScheme, (obj) => this.selectObject(obj));
    Renderer.selectObject(svg, objectId);
    this.applyTransform();
    this.syncExplorerComplianceFromCurrentScheme();
  },

  async patchEntity(entityType, entityId, patch) {
    const objectId = Inspector.currentObject?.graphic_object?.Id;
    if (!objectId) return;
    if (!this.currentSchemeId) {
      console.error('patchEntity: нет активной схемы (scheme_xml_id)');
      return;
    }
    await API.patchObject(this.solutionId, this.sessionId, entityId, this.currentSchemeId, patch, entityType);
    this.setDirty(true);

    const listKey = entityType === 'control_command' ? 'commands' : 'automations';
    if (Inspector.currentObject?.[listKey]) {
      const idx = Inspector.currentObject[listKey].findIndex(e => e.Id === entityId);
      if (idx >= 0) Inspector.currentObject[listKey][idx] = patch;
    }

    const cached = this.objectDetailsCache.get(objectId);
    if (cached?.[listKey]) {
      const idx = cached[listKey].findIndex(e => e.Id === entityId);
      if (idx >= 0) cached[listKey][idx] = patch;
      this.objectDetailsCache.set(objectId, cached);
    }

    const svg = document.getElementById('schemeSvg');
    const obj = this.currentScheme?.GraphicObject?.find(o => o.Id === objectId);
    if (obj && svg) {
      Renderer.replaceObject(svg, obj, (o) => this.selectObject(o));
      Renderer.selectObject(svg, objectId);
    }
    this.syncExplorerComplianceFromCurrentScheme();
  },

  async save() {
    if (!this.solutionId || this.saving || this.tagEditApplying) return;

    const overlay = document.getElementById('saveOverlay');
    const overlayTitle = document.getElementById('saveOverlayTitle');
    const overlayFile = document.getElementById('saveOverlayFile');
    const overlayBar = document.getElementById('saveOverlayBar');
    const overlayStatus = document.getElementById('saveOverlayStatus');
    const overlayClose = document.getElementById('saveOverlayClose');
    const btnSave = document.getElementById('btnSave');
    let shownBar = 0;

    const paint = (meta) => {
      const percent = Math.max(0, Math.min(100, Number(meta?.save_percent) || 0));
      shownBar = Math.max(shownBar, percent);
      overlayTitle.textContent = 'Сохранение решения…';
      overlayFile.textContent = '';
      overlayStatus.textContent = this.formatSaveStatus(meta);
      overlayBar.style.width = `${Math.round(shownBar)}%`;
      overlayBar.style.background = '';
    };

    this.saving = true;
    btnSave.disabled = true;
    document.body.style.cursor = 'wait';
    overlayClose.hidden = true;
    overlayFile.textContent = '';
    overlayBar.style.width = '0%';
    overlayBar.style.background = '';
    overlayTitle.textContent = 'Сохранение решения…';
    overlayStatus.textContent = 'Сохранение… 0%';
    overlay.hidden = false;

    let pollActive = true;
    const pollPromise = (async () => {
      while (pollActive) {
        try {
          const meta = await API.getSolutionMeta(this.solutionId);
          if (meta.save_status === 'saving' || Object.prototype.hasOwnProperty.call(meta, 'save_percent')) {
            paint(meta);
          }
        } catch (_) {
        }
        if (!pollActive) break;
        await new Promise((r) => setTimeout(r, 400));
      }
    })();

    try {
      if (typeof Inspector.flushPendingEdits === 'function') {
        await Inspector.flushPendingEdits();
      }
      const restoreSchemeId = this.currentSchemeId;
      const restoreSchemeName = document.getElementById('treeFooter')?.textContent
        || this.currentScheme?.Name
        || '';
      await API.save(this.solutionId);
      pollActive = false;
      await pollPromise;
      overlayTitle.textContent = 'Решение сохранено';
      overlayStatus.textContent = 'Сохранение… 100%';
      overlayBar.style.width = '100%';
      overlayBar.style.background = 'var(--state-normal)';
      this.invalidateComplianceParamsCache();
      this.setDirty(false);
      if (restoreSchemeId) {
        await this.openScheme(restoreSchemeId, restoreSchemeName);
      }
      await new Promise((r) => setTimeout(r, 1300));
      overlay.hidden = true;
    } catch (err) {
      pollActive = false;
      await pollPromise;
      overlayTitle.textContent = 'Ошибка сохранения';
      overlayStatus.textContent = err?.message || String(err);
      overlayClose.hidden = false;
      await new Promise((resolve) => {
        const onClose = () => {
          overlayClose.removeEventListener('click', onClose);
          resolve();
        };
        overlayClose.addEventListener('click', onClose);
      });
      overlay.hidden = true;
    } finally {
      this.saving = false;
      if (this.solutionId) btnSave.disabled = false;
      document.body.style.cursor = '';
      overlayBar.style.background = '';
      overlayClose.hidden = true;
    }
  },

  async exportXML() {
    if (this.tagEditApplying) return;
    const blob = await API.export(this.solutionId);
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'solution.xml';
    a.click();
    URL.revokeObjectURL(url);
  },

  setDirty(dirty) {
    this.isDirty = dirty;
    document.getElementById('dirtyBadge').hidden = !dirty;
  },

  adjustZoom(factor) {
    const svg = document.getElementById('schemeSvg');
    const rect = svg.getBoundingClientRect();
    Renderer.zoomViewBoxAt(svg, rect.left + rect.width / 2, rect.top + rect.height / 2, factor);
    this.applyTransform();
  },

  toggleSchemeMovementLock() {
    this.schemeMovementLocked = !this.schemeMovementLocked;
    this.updateSchemeLockUI();
  },

  updateSchemeLockUI() {
    const btn = document.getElementById('btnLockScheme');
    const viewport = document.getElementById('canvasViewport');
    const locked = this.schemeMovementLocked;
    if (btn) {
      btn.textContent = locked ? '🔒 Заблокировано' : '🔓 Движение';
      btn.classList.toggle('btn--active', locked);
      btn.title = locked ? 'Разблокировать перемещение объектов' : 'Заблокировать перемещение объектов';
    }
    viewport?.classList.toggle('canvas-viewport--movement-locked', locked);
    document.getElementById('app').classList.toggle('app--scheme-movement-locked', locked && !!this.currentSchemeId);
    const status = document.getElementById('statusCoords');
    if (status && this.currentSchemeId) {
      status.textContent = locked ? 'Движение заблокировано' : '';
    }
    this.refreshPipeSelectability();
  },

  refreshPipeSelectability() {
    const svg = document.getElementById('schemeSvg');
    if (!svg) return;
    const locked = this.schemeMovementLocked;
    svg.querySelectorAll('.scheme-object--pipe-like').forEach(el => {
      el.classList.toggle('scheme-object--pipe-no-select', locked);
    });
    if (locked) {
      const selected = svg.querySelector('.scheme-object--selected.scheme-object--pipe-like');
      if (selected) {
        selected.classList.remove('scheme-object--selected');
        this.selectedObjectId = null;
        if (this.objectSettings.open) {
          this.forceCloseObjectSettingsPanel();
        }
        if (this.currentScheme) Inspector.showSchemeInfo(this.currentScheme);
        document.getElementById('app').classList.remove('app--object-selected');
      }
    }
  },

  fitView() {
    const svg = document.getElementById('schemeSvg');
    Renderer.resetViewBox(svg);
    svg.style.transform = 'none';
    this.applyTransform();
  },

  applyTransform() {
    const svg = document.getElementById('schemeSvg');
    svg.style.transform = 'none';
    const zoomLevel = Renderer.getZoomLevel(svg);
    const hasLabelFields = Object.values(Renderer.displaySettings).some(s => {
      try {
        const fields = JSON.parse(s.visible_fields || '[]');
        return Array.isArray(fields) && fields.length > 0;
      } catch (_) { return false; }
    });
    const showLabels = Renderer.labelsEnabled && hasLabelFields && zoomLevel >= 0.35;
    if (Renderer.showLabels !== showLabels) {
      Renderer.setLabelsVisible(svg, showLabels);
    }
    const pct = Renderer.getZoomPercent(svg);
    document.getElementById('zoomLabel').textContent = pct + '%';
    document.getElementById('statusZoom').textContent = `Масштаб: ${pct}%`;
  },

  setupPanZoom(svg) {
    const viewport = document.getElementById('canvasViewport');
    const panPx = 4;

    viewport.addEventListener('mousedown', (e) => {
      if (e.button !== 0) return;
      if (e.target.closest('.scheme-object')) return;
      this._panGesture = {
        active: true,
        moved: false,
        startX: e.clientX,
        startY: e.clientY,
        lastX: e.clientX,
        lastY: e.clientY,
      };
    });
    window.addEventListener('mousemove', (e) => {
      const g = this._panGesture;
      if (!g.active) return;
      const dist = Math.hypot(e.clientX - g.startX, e.clientY - g.startY);
      if (!g.moved && dist < panPx) return;
      if (!g.moved) {
        g.moved = true;
        g.lastX = e.clientX;
        g.lastY = e.clientY;
      }
      const dx = e.clientX - g.lastX;
      const dy = e.clientY - g.lastY;
      g.lastX = e.clientX;
      g.lastY = e.clientY;
      Renderer.panViewBox(svg, dx, dy);
      this.applyTransform();
    });
    window.addEventListener('mouseup', () => {
      if (this._panGesture.moved) this._suppressSvgClick = true;
      this._panGesture.active = false;
      this._panGesture.moved = false;
    });

    viewport.addEventListener('wheel', (e) => {
      e.preventDefault();
      const factor = e.deltaY < 0 ? 1.1 : 0.9;
      Renderer.zoomViewBoxAt(svg, e.clientX, e.clientY, factor);
      this.applyTransform();
    }, { passive: false });
  },

  setupDrag(svg) {
    svg.addEventListener('mousedown', (e) => {
      if (this.schemeMovementLocked) return;
      const objEl = e.target.closest('.scheme-object');
      if (!objEl || objEl.classList.contains('scheme-object--locked')) return;
      if (objEl.classList.contains('scheme-object--pipe-no-select')) return;
      e.stopPropagation();
      const id = objEl.dataset.id;
      const obj = this.currentScheme?.GraphicObject?.find(o => o.Id === id);
      if (!obj) return;

      this.dragState = { obj, objEl, startX: e.clientX, startY: e.clientY, pendingPatch: null };
    });

    window.addEventListener('mousemove', (e) => {
      if (!this.dragState) return;
      const screenScale = Renderer.getViewBoxScreenScale(svg);
      const dx = (e.clientX - this.dragState.startX) * screenScale / Renderer.layoutScale;
      const dy = (e.clientY - this.dragState.startY) * screenScale / Renderer.layoutScale;
      if (Math.abs(dx) < 2 && Math.abs(dy) < 2) return;

      const patch = JSON.parse(JSON.stringify(this.dragState.obj));
      if (!patch.AnchorPoint) patch.AnchorPoint = [{ X: '0', Y: '0' }];
      const curX = Renderer.parseCoord(patch.AnchorPoint[0].X);
      const curY = Renderer.parseCoord(patch.AnchorPoint[0].Y);
      patch.AnchorPoint[0].X = String(curX + dx);
      patch.AnchorPoint[0].Y = String(curY + dy);

      this.dragState.startX = e.clientX;
      this.dragState.startY = e.clientY;
      this.dragState.obj = patch;
      this.dragState.pendingPatch = patch;

      const idx = this.currentScheme.GraphicObject.findIndex(o => o.Id === patch.Id);
      if (idx >= 0) this.currentScheme.GraphicObject[idx] = patch;

      Renderer.replaceObject(svg, patch, (obj) => this.selectObject(obj));
      Renderer.selectObject(svg, patch.Id);
    });

    window.addEventListener('mouseup', async () => {
      if (!this.dragState?.pendingPatch) {
        this.dragState = null;
        return;
      }
      const patch = this.dragState.pendingPatch;
      this.dragState = null;
      await this.patchObject(patch.Id, patch, { skipFullRender: true });
    });
  },

  async showDisplaySettings() {
    if (this.tagEditApplying) return;
    if (this.objectSettings.open) {
      const closed = await this.tryCloseObjectSettingsPanel();
      if (!closed) return;
    }

    const [settings, templates] = await Promise.all([
      API.getDisplaySettings(),
      this.solutionId ? API.getSolutionTemplates(this.solutionId) : [],
    ]);
    const settingsMap = {};
    for (const s of settings) settingsMap[s.template] = s;

    const templateNames = new Set(settings.map(s => s.template));
    for (const t of templates) templateNames.add(t.template);

    const body = document.getElementById('displaySettingsBody');
    body.innerHTML = '<p style="color:var(--text-secondary);font-size:12px;margin-bottom:12px;">Настройка применяется ко всем объектам выбранного типа. Реперная точка схемы — левый верхний угол. Смещение позиции видно сразу при движении ползунков.</p>';

    const globalToggle = document.createElement('label');
    globalToggle.className = 'display-settings__global-toggle';
    const globalCb = document.createElement('input');
    globalCb.type = 'checkbox';
    globalCb.checked = Renderer.labelsEnabled;
    globalCb.id = 'displayLabelsGlobal';
    globalToggle.appendChild(globalCb);
    globalToggle.appendChild(document.createTextNode(' Показывать подписи на схеме'));
    body.appendChild(globalToggle);

    for (const template of [...templateNames].sort()) {
      const s = settingsMap[template] || { template, renderer: 'symbol', visible_fields: '[]', style: '{}' };
      const block = DisplaySettingsForm.buildTypeSettingsForm(template, s, {
        includeTitle: true,
        onOffsetChange: () => this.previewDisplayOffsets(),
      });
      body.appendChild(block);
    }

    const btnRow = document.createElement('div');
    btnRow.style.marginTop = '12px';

    const saveBtn = document.createElement('button');
    saveBtn.className = 'btn btn--primary';
    saveBtn.textContent = 'Применить';
    saveBtn.addEventListener('click', () => this.saveDisplaySettings());
    btnRow.appendChild(saveBtn);

    const cancelBtn = document.createElement('button');
    cancelBtn.className = 'btn btn--ghost';
    cancelBtn.textContent = 'Закрыть';
    cancelBtn.style.marginLeft = '8px';
    cancelBtn.addEventListener('click', () => this.hideDisplaySettings());
    btnRow.appendChild(cancelBtn);

    body.appendChild(btnRow);
    document.getElementById('displayModal').hidden = false;
  },

  async previewDisplayOffsets() {
    const blocks = document.querySelectorAll('#displaySettingsBody .display-settings__type-block');
    for (const block of blocks) {
      const template = block.dataset.template;
      const { offsetX, offsetY } = DisplaySettingsForm.readOffsetsFromBlock(block);
      if (!Renderer.displaySettings[template]) {
        Renderer.displaySettings[template] = { template, renderer: 'symbol', visible_fields: '[]', style: '{}' };
      }
      let style = {};
      try { style = JSON.parse(Renderer.displaySettings[template].style || '{}'); } catch (_) {}
      style.offsetX = offsetX;
      style.offsetY = offsetY;
      Renderer.displaySettings[template].style = JSON.stringify(style);
    }
    if (!this.currentScheme) return;
    const svg = document.getElementById('schemeSvg');
    await Renderer.renderSchemeAsync(svg, this.currentScheme, (obj) => this.selectObject(obj));
    this.applyTransform();
  },

  async saveDisplaySettings() {
    Renderer.labelsEnabled = document.getElementById('displayLabelsGlobal')?.checked ?? false;
    const blocks = document.querySelectorAll('#displaySettingsBody .display-settings__type-block');
    const existing = await API.getDisplaySettings();
    for (const block of blocks) {
      const template = block.dataset.template;
      const cur = existing.find(s => s.template === template);
      let baseStyle = {};
      try { baseStyle = JSON.parse(cur?.style || '{}'); } catch (_) {}
      const payload = DisplaySettingsForm.readTypeSettingsForm(block, baseStyle);
      if (!payload.renderer) payload.renderer = cur?.renderer || 'symbol';
      await API.updateDisplaySetting(template, payload);
    }
    await Renderer.loadDisplaySettings();
    if (this.currentScheme) {
      const svg = document.getElementById('schemeSvg');
      await Renderer.renderSchemeAsync(svg, this.currentScheme, (obj) => this.selectObject(obj));
      this.applyTransform();
    }
    this.hideDisplaySettings();
  },

  hideDisplaySettings() {
    document.getElementById('displayModal').hidden = true;
  },

  confirmObjectSettingsDirty(message) {
    return new Promise((resolve) => {
      const overlay = document.createElement('div');
      overlay.className = 'dirty-confirm-overlay';
      const dialog = document.createElement('div');
      dialog.className = 'dirty-confirm-dialog';
      const text = document.createElement('p');
      text.className = 'dirty-confirm-dialog__text';
      text.textContent = message || 'Есть несохранённые настройки объекта. Принять изменения, сбросить или отменить?';
      const actions = document.createElement('div');
      actions.className = 'dirty-confirm-dialog__actions';

      const finish = (value) => {
        overlay.remove();
        resolve(value);
      };

      const acceptBtn = document.createElement('button');
      acceptBtn.type = 'button';
      acceptBtn.className = 'btn btn--primary';
      acceptBtn.textContent = 'Принять';
      acceptBtn.addEventListener('click', () => finish('accept'));

      const discardBtn = document.createElement('button');
      discardBtn.type = 'button';
      discardBtn.className = 'btn btn--ghost';
      discardBtn.textContent = 'Сбросить';
      discardBtn.addEventListener('click', () => finish('discard'));

      const cancelBtn = document.createElement('button');
      cancelBtn.type = 'button';
      cancelBtn.className = 'btn btn--ghost';
      cancelBtn.textContent = 'Отмена';
      cancelBtn.addEventListener('click', () => finish('cancel'));

      actions.appendChild(acceptBtn);
      actions.appendChild(discardBtn);
      actions.appendChild(cancelBtn);
      dialog.appendChild(text);
      dialog.appendChild(actions);
      overlay.appendChild(dialog);
      overlay.addEventListener('click', (e) => {
        if (e.target === overlay) finish('cancel');
      });
      document.body.appendChild(overlay);
    });
  },

  getSelectedSchemeObject() {
    const id = this.selectedObjectId;
    if (!id || !this.currentScheme?.GraphicObject) return null;
    return this.currentScheme.GraphicObject.find(o => o.Id === id) || null;
  },

  async toggleObjectSettingsPanel() {
    if (this.objectSettings.open) {
      await this.closeObjectSettingsPanel();
      return;
    }
    const obj = this.getSelectedSchemeObject();
    if (!obj) return;
    await this.openObjectSettingsPanel(obj);
  },

  async openObjectSettingsPanel(obj) {
    if (!obj) return;
    if (!document.getElementById('displayModal').hidden) {
      this.hideDisplaySettings();
    }
    await this.bindObjectSettingsToObject(obj);
    this.objectSettings.open = true;
    document.getElementById('objectSettingsPanel').hidden = false;
    await this.previewObjectSettings();
  },

  async bindObjectSettingsToObject(obj) {
    const template = Renderer.getTemplate(obj);
    let existing = Renderer.displaySettings[template]
      || { template, renderer: 'symbol', visible_fields: '[]', style: '{}' };
    try {
      const all = await API.getDisplaySettings();
      const fromApi = all.find(s => s.template === template);
      if (fromApi) existing = fromApi;
    } catch (_) {}
    const baseline = DisplaySettingsForm.cloneSetting(existing);
    baseline.template = template;
    this.objectSettings.objectId = obj.Id;
    this.objectSettings.template = template;
    this.objectSettings.baseline = baseline;
    this.objectSettings.draft = DisplaySettingsForm.cloneSetting(baseline);
    this.objectSettings.dirty = false;
    Renderer.displaySettings[template] = DisplaySettingsForm.cloneSetting(baseline);
    document.getElementById('objectSettingsType').textContent = template || '—';
    const body = document.getElementById('objectSettingsBody');
    body.innerHTML = '';
    const block = DisplaySettingsForm.buildTypeSettingsForm(template, this.objectSettings.draft, {
      includeTitle: false,
      onChange: (formBlock) => this.onObjectSettingsFormChange(formBlock),
    });
    body.appendChild(block);
  },

  onObjectSettingsFormChange(block) {
    const panel = this.objectSettings;
    if (!panel.open || !panel.template) return;
    let baseStyle = {};
    try { baseStyle = JSON.parse(panel.baseline?.style || '{}'); } catch (_) {}
    const draft = DisplaySettingsForm.readTypeSettingsForm(block, baseStyle);
    panel.draft = draft;
    panel.dirty = JSON.stringify(draft) !== JSON.stringify(panel.baseline);
    Renderer.displaySettings[panel.template] = DisplaySettingsForm.cloneSetting(draft);
    this.previewObjectSettings();
  },

  async previewObjectSettings() {
    if (!this.currentScheme) return;
    const token = ++this._objectSettingsPreviewToken;
    const svg = document.getElementById('schemeSvg');
    const selectedId = this.selectedObjectId || this.objectSettings.objectId;
    await Renderer.renderSchemeAsync(svg, this.currentScheme, (obj) => this.selectObject(obj));
    if (token !== this._objectSettingsPreviewToken) return;
    if (selectedId) {
      Renderer.selectObject(svg, selectedId);
    }
    this.applyTransform();
  },

  async discardObjectSettingsPreview() {
    const panel = this.objectSettings;
    if (panel.template && panel.baseline) {
      Renderer.displaySettings[panel.template] = DisplaySettingsForm.cloneSetting(panel.baseline);
    }
    panel.draft = panel.baseline ? DisplaySettingsForm.cloneSetting(panel.baseline) : null;
    panel.dirty = false;
    await this.previewObjectSettings();
  },

  async acceptObjectSettings(options = {}) {
    const panel = this.objectSettings;
    if (!panel.open || !panel.template) return false;
    const block = document.querySelector('#objectSettingsBody .display-settings__type-block');
    if (!block) return false;
    let baseStyle = {};
    try { baseStyle = JSON.parse(panel.baseline?.style || '{}'); } catch (_) {}
    const payload = DisplaySettingsForm.readTypeSettingsForm(block, baseStyle);
    await API.updateDisplaySetting(panel.template, payload);
    await Renderer.loadDisplaySettings();
    panel.baseline = DisplaySettingsForm.cloneSetting(Renderer.displaySettings[panel.template] || payload);
    panel.draft = DisplaySettingsForm.cloneSetting(panel.baseline);
    panel.dirty = false;
    if (this.currentScheme) {
      const svg = document.getElementById('schemeSvg');
      const selectedId = this.selectedObjectId || panel.objectId;
      await Renderer.renderSchemeAsync(svg, this.currentScheme, (obj) => this.selectObject(obj));
      if (selectedId) Renderer.selectObject(svg, selectedId);
      this.applyTransform();
    }
    if (!options.keepOpen) {
      this.resetObjectSettingsState();
      document.getElementById('objectSettingsPanel').hidden = true;
    } else {
      document.getElementById('objectSettingsType').textContent = panel.template || '—';
      const body = document.getElementById('objectSettingsBody');
      body.innerHTML = '';
      body.appendChild(DisplaySettingsForm.buildTypeSettingsForm(panel.template, panel.draft, {
        includeTitle: false,
        onChange: (formBlock) => this.onObjectSettingsFormChange(formBlock),
      }));
    }
    return true;
  },

  async closeObjectSettingsPanel() {
    await this.tryCloseObjectSettingsPanel();
  },

  async tryCloseObjectSettingsPanel() {
    if (!this.objectSettings.open) return true;
    if (this.objectSettings.dirty) {
      const choice = await this.confirmObjectSettingsDirty();
      if (choice === 'cancel') return false;
      if (choice === 'accept') {
        const saved = await this.acceptObjectSettings({ keepOpen: false });
        return saved;
      }
      await this.discardObjectSettingsPreview();
      this.resetObjectSettingsState();
      document.getElementById('objectSettingsPanel').hidden = true;
      document.getElementById('objectSettingsBody').innerHTML = '';
      document.getElementById('objectSettingsType').textContent = '';
      return true;
    }
    this.forceCloseObjectSettingsPanel();
    return true;
  },

  forceCloseObjectSettingsPanel() {
    const panel = this.objectSettings;
    const needsRestore = !!(panel.dirty && panel.template && panel.baseline);
    if (panel.template && panel.baseline) {
      Renderer.displaySettings[panel.template] = DisplaySettingsForm.cloneSetting(panel.baseline);
    }
    this.resetObjectSettingsState();
    document.getElementById('objectSettingsPanel').hidden = true;
    document.getElementById('objectSettingsBody').innerHTML = '';
    document.getElementById('objectSettingsType').textContent = '';
    if (needsRestore && this.currentScheme) {
      const svg = document.getElementById('schemeSvg');
      const selectedId = this.selectedObjectId;
      Renderer.renderSchemeAsync(svg, this.currentScheme, (obj) => this.selectObject(obj)).then(() => {
        if (selectedId) Renderer.selectObject(svg, selectedId);
        this.applyTransform();
      });
    }
  },

  resetObjectSettingsState() {
    this.objectSettings.open = false;
    this.objectSettings.objectId = null;
    this.objectSettings.template = null;
    this.objectSettings.draft = null;
    this.objectSettings.baseline = null;
    this.objectSettings.dirty = false;
  },

  async showComplianceSettings() {
    if (this.tagEditApplying) return;
    const rules = await API.getComplianceRules();
    const fieldRules = (rules || []).filter(r => r.text_condition !== 'duplicate_tag');
    const templates = this.solutionId
      ? (await API.getSolutionTemplates(this.solutionId)).map(t => t.template)
      : (await API.getDisplaySettings()).map(s => s.template);
    const body = document.getElementById('complianceSettingsBody');
    body.innerHTML = '<p class="modal-hint">Настройте проверки параметров объектов, характеристик, команд и защит. «Значение» сравнивает значение выбранного параметра или характеристики; «Tag параметра» — Tag выбранной характеристики (для параметра Tag — Tag объекта/команды/защиты). При нарушении объект подсвечивается на схеме.</p>';

    const list = document.createElement('div');
    list.className = 'compliance-rules-list';
    list.id = 'complianceRulesList';

    for (const rule of fieldRules) {
      list.appendChild(this.buildComplianceRuleRow(rule, templates));
    }
    body.appendChild(list);

    const addBtn = document.createElement('button');
    addBtn.className = 'btn btn--ghost';
    addBtn.textContent = '+ Добавить проверку';
    addBtn.addEventListener('click', () => {
      list.appendChild(this.buildComplianceRuleRow({
        id: '',
        template: templates[0] || 'Valve',
        parameter_scope: 'object',
        entity_template: '',
        parameter: 'Tag',
        value_source: 'field',
        text_condition: 'must_contain',
        expected_text: '',
        empty_allowed: true,
        indication: { type: 'solid', color: '#F85149' },
      }, templates, true));
    });
    body.appendChild(addBtn);

    const btnRow = document.createElement('div');
    btnRow.className = 'modal-actions';
    const saveBtn = document.createElement('button');
    saveBtn.className = 'btn btn--primary';
    saveBtn.textContent = 'Применить';
    saveBtn.addEventListener('click', () => this.saveComplianceSettings());
    btnRow.appendChild(saveBtn);
    const cancelBtn = document.createElement('button');
    cancelBtn.className = 'btn btn--ghost';
    cancelBtn.textContent = 'Закрыть';
    cancelBtn.addEventListener('click', () => this.hideComplianceSettings());
    btnRow.appendChild(cancelBtn);
    body.appendChild(btnRow);

    document.getElementById('complianceModal').hidden = false;
  },

  buildComplianceRuleRow(rule, templates, isNew) {
    const row = document.createElement('div');
    row.className = 'compliance-rule';
    row.dataset.id = rule.id || '';
    row._stash = {
      entity_template: rule.entity_template || '',
      parameter: rule.parameter || '',
      value_source: rule.value_source || 'field',
    };

    const templateSel = document.createElement('select');
    templateSel.className = 'input compliance-rule__field';
    for (const t of templates) {
      const opt = document.createElement('option');
      opt.value = t;
      opt.textContent = t;
      if (t === rule.template) opt.selected = true;
      templateSel.appendChild(opt);
    }
    row.appendChild(this.labeledField('Тип объекта', templateSel));

    const scopeSel = document.createElement('select');
    scopeSel.className = 'input compliance-rule__field';
    [['object', 'Основные'], ['property', 'Характеристики'], ['command', 'Команды'], ['protection', 'Защиты']].forEach(([v, l]) => {
      const opt = document.createElement('option');
      opt.value = v;
      opt.textContent = l;
      if ((rule.parameter_scope || 'object') === v) opt.selected = true;
      scopeSel.appendChild(opt);
    });
    row.appendChild(this.labeledField('Область', scopeSel));

    const paramSel = document.createElement('select');
    paramSel.className = 'input compliance-rule__field compliance-rule__parameter';
    const paramRetryBtn = document.createElement('button');
    paramRetryBtn.type = 'button';
    paramRetryBtn.className = 'btn btn--ghost btn--compact compliance-rule__retry';
    paramRetryBtn.textContent = '⟳ Повторить';
    paramRetryBtn.title = 'Повторить загрузку списка параметров';
    paramRetryBtn.hidden = true;
    row.appendChild(this.labeledActionField('Параметр', paramSel, paramRetryBtn));

    const sourceSel = document.createElement('select');
    sourceSel.className = 'input compliance-rule__field';
    sourceSel.title = 'Значение — значение характеристики; Tag параметра — Tag выбранной характеристики';
    [['field', 'Значение'], ['tag', 'Tag параметра']].forEach(([v, l]) => {
      const opt = document.createElement('option');
      opt.value = v;
      opt.textContent = l;
      if ((rule.value_source || 'field') === v) opt.selected = true;
      sourceSel.appendChild(opt);
    });
    row.appendChild(this.labeledField('Атрибут параметра', sourceSel));

    const OBJECT_SCALAR_PARAMS = new Set(['Tag', 'Name', 'Description', 'Id', 'ExternalCode']);
    const tagSourceAllowed = (param) => {
      if (scopeSel.value === 'property' || scopeSel.value === 'command' || scopeSel.value === 'protection') return true;
      return !OBJECT_SCALAR_PARAMS.has(param) || param === 'Tag';
    };
    const syncSourceOptions = () => {
      const paramReady = !paramSel.disabled && !!paramSel.value;
      sourceSel.disabled = !paramReady;
      const tagOpt = sourceSel.querySelector('option[value="tag"]');
      if (!tagOpt) return;
      if (scopeSel.value === 'command' || scopeSel.value === 'protection') {
        tagOpt.hidden = false;
        tagOpt.disabled = false;
        return;
      }
      const allowed = tagSourceAllowed(paramSel.value);
      tagOpt.hidden = !allowed;
      tagOpt.disabled = !allowed;
      if (!allowed && sourceSel.value === 'tag') {
        sourceSel.value = 'field';
      }
    };

    const PARAM_LOADING_PLACEHOLDER_DELAY_MS = 200;
    const PARAM_STATE_MODIFIERS = ['loading', 'muted', 'error'];
    const PARAM_EMPTY_LABELS = {
      object: { text: 'Нет вариантов', title: 'Список параметров пуст' },
      property: { text: 'Нет характеристик', title: 'У выбранного типа объекта нет характеристик' },
      command: { text: 'Нет команд', title: 'У выбранного типа объекта нет команд' },
      protection: { text: 'Нет защит', title: 'У выбранного типа объекта нет защит' },
    };

    let paramLoadSeq = 0;
    let paramPlaceholderTimer = null;

    const applyParamState = (state) => {
      for (const modifier of PARAM_STATE_MODIFIERS) {
        paramSel.classList.toggle(`compliance-rule__parameter--${modifier}`, modifier === state);
      }
      paramRetryBtn.hidden = state !== 'error';
    };

    const setParamPlaceholder = (text, state, title = '') => {
      paramSel.innerHTML = '';
      const opt = document.createElement('option');
      opt.value = '';
      opt.textContent = text;
      opt.disabled = true;
      opt.selected = true;
      paramSel.appendChild(opt);
      paramSel.disabled = true;
      paramSel.title = title;
      applyParamState(state);
      syncSourceOptions();
    };

    const setParamError = () => {
      setParamPlaceholder('Ошибка загрузки', 'error', 'Не удалось получить список параметров. Нажмите «Повторить»');
    };

    const fillParamOptions = (items, current, scope) => {
      if (!items || items.length === 0) {
        const empty = PARAM_EMPTY_LABELS[scope] || PARAM_EMPTY_LABELS.object;
        setParamPlaceholder(empty.text, 'muted', empty.title);
        return;
      }
      paramSel.innerHTML = '';
      for (const item of items) {
        const opt = document.createElement('option');
        opt.value = item;
        opt.textContent = item;
        if (item === current) opt.selected = true;
        paramSel.appendChild(opt);
      }
      paramSel.disabled = false;
      paramSel.title = '';
      applyParamState(null);
      if (!paramSel.value) paramSel.value = items[0];
      syncSourceOptions();
    };

    const beginParamLoad = () => {
      const seq = ++paramLoadSeq;
      clearTimeout(paramPlaceholderTimer);
      paramSel.setAttribute('aria-busy', 'true');
      paramPlaceholderTimer = setTimeout(() => {
        if (seq !== paramLoadSeq) return;
        setParamPlaceholder('Загрузка…', 'loading');
      }, PARAM_LOADING_PLACEHOLDER_DELAY_MS);
      return seq;
    };

    const endParamLoad = () => {
      clearTimeout(paramPlaceholderTimer);
      paramPlaceholderTimer = null;
      paramSel.removeAttribute('aria-busy');
    };

    const paramCurrentForScope = (scope) => {
      if (scope === 'command' || scope === 'protection') {
        return paramSel.value || row._stash.entity_template || '';
      }
      return paramSel.value || row._stash.parameter || rule.parameter || '';
    };

    const paramFallbackForScope = (scope) => (
      scope === 'object' ? ['Tag', 'Name', 'Description', 'Id', 'ExternalCode'] : []
    );

    const loadParamsForScope = async (scope, force = false) => {
      const tpl = templateSel.value;
      const current = paramCurrentForScope(scope) || (scope === 'object' ? 'Tag' : '');
      const cacheKey = this.complianceParamsCacheKey(scope, tpl);
      const seq = beginParamLoad();

      if (!this.solutionId || !tpl) {
        endParamLoad();
        fillParamOptions(paramFallbackForScope(scope), current, scope);
        return;
      }
      if (!force && this.complianceParamsCache.has(cacheKey)) {
        endParamLoad();
        fillParamOptions(this.complianceParamsCache.get(cacheKey), current, scope);
        return;
      }

      try {
        const items = await this.loadComplianceParameters(scope, tpl, force);
        if (seq !== paramLoadSeq) return;
        endParamLoad();
        fillParamOptions(items, current, scope);
      } catch (_) {
        if (seq !== paramLoadSeq) return;
        endParamLoad();
        setParamError();
      }
    };

    const syncScopeUI = async (force = false) => {
      const scope = scopeSel.value;
      if (!scope) {
        setParamPlaceholder('Выберите область', 'muted');
        return;
      }
      await loadParamsForScope(scope, force);
    };

    setParamPlaceholder('Выберите область', 'muted');
    templateSel.addEventListener('change', () => syncScopeUI(false));
    scopeSel.addEventListener('change', () => syncScopeUI(false));
    paramSel.addEventListener('change', () => syncSourceOptions());
    sourceSel.addEventListener('change', () => syncSourceOptions());
    paramRetryBtn.addEventListener('click', (e) => {
      e.preventDefault();
      syncScopeUI(true);
    });
    syncScopeUI(false);

    const condSel = document.createElement('select');
    condSel.className = 'input compliance-rule__field';
    [['must_contain', 'Должен содержать'], ['must_not_contain', 'Не должен содержать'], ['exact_match', 'Строгое соответствие'], ['must_not_exact', 'Не должен совпадать строго']].forEach(([v, l]) => {
      const opt = document.createElement('option');
      opt.value = v;
      opt.textContent = l;
      if (v === rule.text_condition) opt.selected = true;
      condSel.appendChild(opt);
    });
    row.appendChild(this.labeledField('Условие текста', condSel));

    const textInput = document.createElement('input');
    textInput.className = 'input compliance-rule__field';
    textInput.value = rule.expected_text || '';
    row.appendChild(this.labeledField('Текст', textInput));

    const emptySel = document.createElement('select');
    emptySel.className = 'input compliance-rule__field';
    [['true', 'Пустое допустимо'], ['false', 'Пустое недопустимо']].forEach(([v, l]) => {
      const opt = document.createElement('option');
      opt.value = v;
      opt.textContent = l;
      if ((rule.empty_allowed ? 'true' : 'false') === v) opt.selected = true;
      emptySel.appendChild(opt);
    });
    row.appendChild(this.labeledField('Пустое поле', emptySel));

    const indication = (() => {
      if (!rule.indication) return {};
      if (typeof rule.indication === 'string') {
        try { return JSON.parse(rule.indication); } catch (_) { return {}; }
      }
      return rule.indication;
    })();
    const typeSel = document.createElement('select');
    typeSel.className = 'input compliance-rule__field';
    [['solid', 'Цвет'], ['pulse', 'Пульсация'], ['hatch', 'Решётка']].forEach(([v, l]) => {
      const opt = document.createElement('option');
      opt.value = v;
      opt.textContent = l;
      if (v === (indication.type || 'solid')) opt.selected = true;
      typeSel.appendChild(opt);
    });
    row.appendChild(this.labeledField('Индикация', typeSel));

    const colorInput = document.createElement('input');
    colorInput.type = 'color';
    colorInput.className = 'compliance-rule__color';
    colorInput.value = indication.color || '#F85149';
    row.appendChild(this.labeledField('Цвет', colorInput));

    const periodInput = document.createElement('input');
    periodInput.type = 'number';
    periodInput.className = 'input compliance-rule__field';
    periodInput.value = indication.pulsePeriodMs || 1000;
    periodInput.min = '200';
    periodInput.step = '100';
    row.appendChild(this.labeledField('Период (мс)', periodInput));

    const delBtn = document.createElement('button');
    delBtn.className = 'btn btn--ghost compliance-rule__delete';
    delBtn.textContent = '✕';
    delBtn.addEventListener('click', async () => {
      if (rule.id && !isNew) await API.deleteComplianceRule(rule.id);
      row.remove();
    });
    row.appendChild(delBtn);

    row._fields = { templateSel, scopeSel, paramSel, sourceSel, condSel, textInput, emptySel, typeSel, colorInput, periodInput };
    return row;
  },

  labeledField(label, el) {
    const wrap = document.createElement('div');
    wrap.className = 'compliance-rule__group';
    const lbl = document.createElement('span');
    lbl.className = 'props-field__label';
    lbl.textContent = label;
    wrap.appendChild(lbl);
    wrap.appendChild(el);
    return wrap;
  },

  labeledActionField(label, el, action) {
    const wrap = document.createElement('div');
    wrap.className = 'compliance-rule__group';
    const labelRow = document.createElement('div');
    labelRow.className = 'compliance-rule__label-row';
    const lbl = document.createElement('span');
    lbl.className = 'props-field__label';
    lbl.textContent = label;
    labelRow.appendChild(lbl);
    labelRow.appendChild(action);
    wrap.appendChild(labelRow);
    wrap.appendChild(el);
    return wrap;
  },

  complianceParamsCacheKey(scope, template) {
    return `${this.solutionId || ''}|${scope}|${template}`;
  },

  fetchComplianceParameters(scope, template) {
    if (scope === 'property') {
      return API.getTemplateCharacteristics(this.solutionId, template, this.sessionId);
    }
    if (scope === 'command' || scope === 'protection') {
      return API.getComplianceTargets(this.solutionId, template, scope, this.sessionId);
    }
    return API.getTemplateParameters(this.solutionId, template, this.sessionId);
  },

  async loadComplianceParameters(scope, template, force = false) {
    const key = this.complianceParamsCacheKey(scope, template);
    if (!force && this.complianceParamsCache.has(key)) {
      return this.complianceParamsCache.get(key);
    }
    const items = (await this.fetchComplianceParameters(scope, template)) || [];
    this.complianceParamsCache.set(key, Array.isArray(items) ? items : []);
    return this.complianceParamsCache.get(key);
  },

  invalidateComplianceParamsCache() {
    this.complianceParamsCache.clear();
  },

  async saveComplianceSettings() {
    const rows = document.querySelectorAll('.compliance-rule');
    let skippedWithoutParameter = 0;
    for (const row of rows) {
      const f = row._fields;
      const scope = f.scopeSel.value;
      if (!f.paramSel.value) {
        skippedWithoutParameter++;
        continue;
      }
      const isEntityScope = scope === 'command' || scope === 'protection';
      const stash = row._stash || { entity_template: '', parameter: '', value_source: 'field' };
      let entityTemplate = '';
      let parameter = f.paramSel.value;
      let valueSource = f.sourceSel.value;

      if (isEntityScope) {
        entityTemplate = f.paramSel.value;
        valueSource = f.sourceSel.value;
        const entityChanged = entityTemplate !== stash.entity_template;
        const attrChanged = valueSource !== stash.value_source;
        const isNewRow = !row.dataset.id;
        if (isNewRow || entityChanged || attrChanged) {
          parameter = valueSource === 'tag' ? 'Tag' : 'Name';
        } else {
          parameter = stash.parameter;
        }
      } else {
        entityTemplate = '';
        parameter = f.paramSel.value;
        valueSource = f.sourceSel.value;
      }

      const data = {
        template: f.templateSel.value,
        parameter_scope: scope,
        entity_template: entityTemplate,
        parameter,
        value_source: valueSource,
        text_condition: f.condSel.value,
        expected_text: f.textInput.value,
        empty_allowed: f.emptySel.value === 'true',
        indication: {
          type: f.typeSel.value,
          color: f.colorInput.value,
          pulsePeriodMs: parseInt(f.periodInput.value) || 1000,
        },
      };
      if (row.dataset.id) {
        await API.updateComplianceRule(row.dataset.id, data);
      } else {
        const created = await API.createComplianceRule(data);
        row.dataset.id = created.id;
      }
      row._stash = {
        entity_template: entityTemplate,
        parameter,
        value_source: valueSource,
      };
    }
    await Renderer.loadComplianceRules();
    await this.refreshComplianceReport();
    if (this.currentScheme) {
      const svg = document.getElementById('schemeSvg');
      await Renderer.renderSchemeAsync(svg, this.currentScheme, (obj) => this.selectObject(obj));
      this.applyTransform();
    }
    this.hideComplianceSettings();
    if (skippedWithoutParameter > 0) {
      alert('Правило без параметра не сохранено: у выбранного типа объекта нет доступных параметров.');
    }
  },

  async showDuplicateSettings() {
    if (this.tagEditApplying) return;
    const rules = await API.getComplianceRules();
    const duplicateRule = (rules || []).find(r => r.text_condition === 'duplicate_tag') || null;
    const body = document.getElementById('duplicateSettingsBody');
    body.innerHTML = '';

    const hint = document.createElement('p');
    hint.className = 'modal-hint';
    hint.textContent = 'Поиск одинаковых Tag среди основных параметров, характеристик, команд и защит. Результаты отображаются в таблице и на схемах.';
    body.appendChild(hint);

    const settings = document.createElement('div');
    settings.className = 'duplicate-settings';
    settings.id = 'duplicateSettingsSection';

    const indication = (() => {
      if (!duplicateRule?.indication) return {};
      if (typeof duplicateRule.indication === 'string') {
        try { return JSON.parse(duplicateRule.indication); } catch (_) { return {}; }
      }
      return duplicateRule.indication;
    })();
    const excludedScopes = this.parseDuplicateExcludedScopes(indication);
    const excludedTagPatterns = this.parseDuplicateExcludedTagPatterns(indication);

    const typeSel = document.createElement('select');
    typeSel.className = 'input compliance-rule__field';
    [['solid', 'Цвет'], ['pulse', 'Пульсация'], ['hatch', 'Решётка']].forEach(([v, l]) => {
      const opt = document.createElement('option');
      opt.value = v;
      opt.textContent = l;
      if (v === (indication.type || 'solid')) opt.selected = true;
      typeSel.appendChild(opt);
    });
    settings.appendChild(this.labeledField('Индикация', typeSel));

    const colorInput = document.createElement('input');
    colorInput.type = 'color';
    colorInput.className = 'compliance-rule__color';
    colorInput.value = indication.color || '#F85149';
    settings.appendChild(this.labeledField('Цвет', colorInput));

    const exclusions = document.createElement('div');
    exclusions.className = 'duplicate-settings__exclusions';
    const exclusionsTitle = document.createElement('div');
    exclusionsTitle.className = 'props-field__label';
    exclusionsTitle.textContent = 'Области поиска';
    exclusions.appendChild(exclusionsTitle);
    const exclusionsHint = document.createElement('div');
    exclusionsHint.className = 'duplicate-settings__exclusions-hint';
    exclusionsHint.textContent = 'Отмеченные области участвуют в поиске дубликатов Tag';
    exclusions.appendChild(exclusionsHint);
    const checks = document.createElement('div');
    checks.className = 'duplicate-settings__exclusion-checks display-settings__checks';
    const excludeObject = document.createElement('input');
    excludeObject.type = 'checkbox';
    excludeObject.checked = !excludedScopes.has('object');
    const excludeProperty = document.createElement('input');
    excludeProperty.type = 'checkbox';
    excludeProperty.checked = !excludedScopes.has('property');
    const excludeCommand = document.createElement('input');
    excludeCommand.type = 'checkbox';
    excludeCommand.checked = !excludedScopes.has('command');
    const excludeProtection = document.createElement('input');
    excludeProtection.type = 'checkbox';
    excludeProtection.checked = !excludedScopes.has('protection');
    [
      [excludeObject, 'Основные'],
      [excludeProperty, 'Характеристики'],
      [excludeCommand, 'Команды'],
      [excludeProtection, 'Защиты'],
    ].forEach(([input, label]) => {
      const wrap = document.createElement('label');
      wrap.appendChild(input);
      wrap.appendChild(document.createTextNode(label));
      checks.appendChild(wrap);
    });
    exclusions.appendChild(checks);
    settings.appendChild(exclusions);

    const tagExclusions = document.createElement('div');
    tagExclusions.className = 'duplicate-settings__tag-exclusions';
    const tagExclusionsTitle = document.createElement('div');
    tagExclusionsTitle.className = 'props-field__label';
    tagExclusionsTitle.textContent = 'Исключения Tag';
    tagExclusions.appendChild(tagExclusionsTitle);
    const tagExclusionsHint = document.createElement('div');
    tagExclusionsHint.className = 'duplicate-settings__exclusions-hint';
    tagExclusionsHint.textContent = 'Tag по выбранному типу не участвуют в поиске дубликатов. Несколько условий — через ИЛИ.';
    tagExclusions.appendChild(tagExclusionsHint);
    const tagExclusionsList = document.createElement('div');
    tagExclusionsList.className = 'duplicate-tag-exclusions-list';
    tagExclusionsList.id = 'duplicateTagExclusionsList';
    for (const pattern of excludedTagPatterns) {
      tagExclusionsList.appendChild(this.buildDuplicateTagExclusionRow(pattern));
    }
    this.syncDuplicateTagExclusionsEmpty(tagExclusionsList);
    tagExclusions.appendChild(tagExclusionsList);
    const addTagExclusionBtn = document.createElement('button');
    addTagExclusionBtn.type = 'button';
    addTagExclusionBtn.className = 'btn btn--ghost';
    addTagExclusionBtn.textContent = '+ Добавить исключение';
    addTagExclusionBtn.addEventListener('click', () => {
      tagExclusionsList.appendChild(this.buildDuplicateTagExclusionRow({ mode: 'contains_char', value: '' }));
      this.syncDuplicateTagExclusionsEmpty(tagExclusionsList);
    });
    tagExclusions.appendChild(addTagExclusionBtn);
    settings.appendChild(tagExclusions);

    const actions = document.createElement('div');
    actions.className = 'duplicate-settings__actions';

    const bgLabel = document.createElement('label');
    bgLabel.className = 'duplicate-settings__bg-label';
    const bgCheck = document.createElement('input');
    bgCheck.type = 'checkbox';
    bgCheck.id = 'duplicateBackgroundCheck';
    bgCheck.disabled = !this.solutionId;
    bgLabel.appendChild(bgCheck);
    bgLabel.appendChild(document.createTextNode('Поиск в фоне'));
    actions.appendChild(bgLabel);

    const bgStatus = document.createElement('span');
    bgStatus.className = 'duplicate-settings__bg-status duplicate-settings__exclusions-hint';
    bgStatus.id = 'duplicateBackgroundStatus';
    actions.appendChild(bgStatus);

    const runBtn = document.createElement('button');
    runBtn.className = 'btn btn--primary';
    runBtn.id = 'duplicateStartBtn';
    runBtn.textContent = 'Начать поиск';
    runBtn.disabled = !this.solutionId;
    runBtn.addEventListener('click', () => this.runDuplicateCheck());
    actions.appendChild(runBtn);
    const closeBtn = document.createElement('button');
    closeBtn.className = 'btn btn--ghost';
    closeBtn.textContent = 'Закрыть';
    closeBtn.addEventListener('click', () => this.hideDuplicateSettings());
    actions.appendChild(closeBtn);
    settings.appendChild(actions);

    settings._fields = {
      typeSel, colorInput, excludeObject, excludeProperty, excludeCommand, excludeProtection,
      tagExclusionsList, bgCheck, runBtn, bgStatus,
    };
    settings.dataset.ruleId = duplicateRule?.id || '';
    body.appendChild(settings);

    const tableWrap = document.createElement('div');
    tableWrap.className = 'duplicate-table-wrap';
    tableWrap.id = 'duplicateTableWrap';
    body.appendChild(tableWrap);
    body.appendChild(this.buildDuplicateExportSection());
    this.renderDuplicateTable(tableWrap);

    bgCheck.addEventListener('change', () => this.onDuplicateBackgroundToggle(bgCheck.checked));

    document.getElementById('duplicateModal').hidden = false;
    await this.syncDuplicateBackgroundUI();
  },

  hideDuplicateSettings() {
    document.getElementById('duplicateModal').hidden = true;
  },

  updateDuplicateBackgroundStatusText(status) {
    const el = document.getElementById('duplicateBackgroundStatus');
    if (!el) return;
    if (!status?.enabled) {
      el.textContent = '';
      return;
    }
    const found = status.found || 0;
    const pct = Math.round(status.percent || 0);
    if (status.mode === 'full' && status.status === 'running') {
      el.textContent = `Сканирование… ${pct}% · Найдено: ${found}`;
      return;
    }
    if (status.status === 'running' || status.mode === 'incremental') {
      el.textContent = `Поиск… · Найдено: ${found}`;
      return;
    }
    el.textContent = `Найдено: ${found}`;
  },

  async syncDuplicateBackgroundUI() {
    if (!this.solutionId) return;
    const section = document.getElementById('duplicateSettingsSection');
    const fields = section?._fields;
    try {
      const status = await API.getDuplicateBackgroundScan(this.solutionId);
      this.duplicateBackgroundEnabled = !!status.enabled;
      if (fields?.bgCheck) {
        fields.bgCheck.checked = !!status.enabled;
      }
      if (fields?.runBtn) {
        fields.runBtn.disabled = !!status.enabled || !this.solutionId;
      }
      this.updateDuplicateBackgroundStatusText(status);
      if (status.enabled) {
        this.startDuplicateBackgroundPolling();
        if (status.found > 0 || (status.processed || 0) > 0) {
          await this.refreshDuplicateBackgroundResults({ soft: true });
        }
      } else {
        this.stopDuplicateBackgroundPolling();
      }
    } catch (_) {
      this.duplicateBackgroundEnabled = false;
    }
  },

  async onDuplicateBackgroundToggle(enabled) {
    const section = document.getElementById('duplicateSettingsSection');
    const fields = section?._fields;
    if (!this.solutionId || !fields) return;
    if (enabled && this.tagEditApplying) {
      fields.bgCheck.checked = false;
      alert('Сейчас выполняется редактирование TAG. Дождитесь окончания.');
      return;
    }
    try {
      if (enabled) {
        const ruleId = await this.saveDuplicateComplianceSettings();
        if (!ruleId) {
          fields.bgCheck.checked = false;
          alert('Не удалось сохранить настройки');
          return;
        }
        fields.runBtn.disabled = true;
        await API.setDuplicateBackgroundScan(this.solutionId, ruleId, true, this.sessionId);
        this.duplicateBackgroundEnabled = true;
        this.updateDuplicateBackgroundStatusText({ enabled: true, mode: 'full', status: 'running', percent: 0, found: 0 });
        this.startDuplicateBackgroundPolling();
      } else {
        await API.setDuplicateBackgroundScan(this.solutionId, section.dataset.ruleId || '', false, this.sessionId);
        this.duplicateBackgroundEnabled = false;
        this.stopDuplicateBackgroundPolling();
        fields.runBtn.disabled = !this.solutionId;
        this.updateDuplicateBackgroundStatusText({ enabled: false });
      }
    } catch (err) {
      fields.bgCheck.checked = !enabled;
      fields.runBtn.disabled = this.duplicateBackgroundEnabled || !this.solutionId;
      const msg = String(err.message || '');
      if (msg.includes('tag edit job already')) {
        alert('Сейчас выполняется редактирование TAG. Дождитесь окончания.');
      } else {
        alert('Ошибка фонового поиска: ' + err.message);
      }
    }
  },

  startDuplicateBackgroundPolling() {
    this.stopDuplicateBackgroundPolling();
    const tick = async () => {
      if (!this.solutionId || !this.duplicateBackgroundEnabled) return;
      try {
        const status = await API.getDuplicateBackgroundScan(this.solutionId);
        this.duplicateBackgroundEnabled = !!status.enabled;
        const section = document.getElementById('duplicateSettingsSection');
        const fields = section?._fields;
        if (fields?.bgCheck) fields.bgCheck.checked = !!status.enabled;
        if (fields?.runBtn) fields.runBtn.disabled = !!status.enabled || !this.solutionId;
        this.updateDuplicateBackgroundStatusText(status);
        if (status.enabled) {
          await this.refreshDuplicateBackgroundResults({ soft: true });
        } else {
          this.stopDuplicateBackgroundPolling();
        }
      } catch (_) {}
    };
    tick();
    this.duplicateBackgroundPollTimer = setInterval(tick, 3000);
  },

  stopDuplicateBackgroundPolling() {
    if (this.duplicateBackgroundPollTimer) {
      clearInterval(this.duplicateBackgroundPollTimer);
      this.duplicateBackgroundPollTimer = null;
    }
  },

  async refreshDuplicateBackgroundResults(options = {}) {
    if (!this.solutionId) return;
    try {
      const result = await API.getDuplicateBackgroundResults(this.solutionId);
      await this.applyDuplicateScanResult(result || { violations_by_object: {}, duplicate_tags: [], nodes: [], rows: [] }, {
        reopenPanel: false,
        soft: !!options.soft,
      });
    } catch (_) {}
  },

  showDuplicatePanel() {
    this.duplicatePanelDismissed = false;
    this.updateDuplicatePanel();
  },

  hideDuplicatePanel() {
    this.duplicatePanelDismissed = true;
    const panel = document.getElementById('duplicatePanel');
    panel.hidden = true;
    if (!document.getElementById('duplicateScanOverlay').hidden) {
      this.duplicatePanelUserClosedDuringScan = true;
    }
    this.updateDuplicatePanel();
  },

  toggleDuplicatePanelCollapse(forceCollapsed) {
    if (typeof forceCollapsed === 'boolean') {
      this.duplicatePanelCollapsed = forceCollapsed;
    } else {
      this.duplicatePanelCollapsed = !this.duplicatePanelCollapsed;
    }
    localStorage.setItem('duplicatePanelCollapsed', String(this.duplicatePanelCollapsed));
    this.updateDuplicatePanel();
  },

  duplicateRowKey(row) {
    return [row.tag, row.object_id, row.scope, row.source_label, row.property_id || '', row.entity_id || ''].join('|');
  },

  duplicateTableColumnDefs() {
    return [
      { key: 'tag', label: 'Tag', width: 140, minWidth: 100 },
      { key: 'object_id', label: 'Id', width: 180, minWidth: 64, mono: true },
      { key: 'object_name', label: 'Имя', width: 160, minWidth: 64 },
      { key: 'object_type', label: 'Тип', width: 80, minWidth: 64 },
      { key: 'scope_label', label: 'Область', width: 100, minWidth: 64 },
      { key: 'source_label', label: 'Параметр', width: 140, minWidth: 64, mono: true },
      { key: 'scheme', label: 'Схема', width: 160, minWidth: 64 },
    ];
  },

  loadDuplicateTableLayout() {
    const defs = this.duplicateTableColumnDefs();
    const defMap = Object.fromEntries(defs.map(d => [d.key, d]));
    const fallback = { order: defs.map(d => d.key), widths: Object.fromEntries(defs.map(d => [d.key, d.width])) };
    try {
      const saved = JSON.parse(localStorage.getItem('duplicateTableColumns') || 'null');
      if (!saved?.order?.length) return fallback;
      const order = saved.order.filter(key => defMap[key]);
      for (const def of defs) {
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

  saveDuplicateTableLayout(order, widths) {
    localStorage.setItem('duplicateTableColumns', JSON.stringify({ order, widths }));
  },

  duplicateCellValue(row, key) {
    switch (key) {
      case 'tag': return row.tag || '';
      case 'object_id': return row.technology_object_id || '';
      case 'object_name': return row.object_name || '';
      case 'object_type': return row.object_type || '';
      case 'scope_label': return row.scope_label || '';
      case 'source_label': return row.source_label || '';
      case 'scheme': return row.scheme_name || row.scheme_xml_id || '';
      default: return '';
    }
  },

  renderDuplicateTable(container) {
    const wrap = container || document.getElementById('duplicateTableWrap');
    if (!wrap) return;
    const rows = this.duplicateRows || [];
    if (rows.length === 0) {
      wrap.innerHTML = '<div class="duplicate-table__empty">Запустите поиск, чтобы увидеть дубликаты Tag</div>';
      this.syncDuplicateExportButton();
      return;
    }
    const defs = this.duplicateTableColumnDefs();
    const defMap = Object.fromEntries(defs.map(d => [d.key, d]));
    const layout = this.loadDuplicateTableLayout();
    const table = document.createElement('table');
    table.className = 'duplicate-table';

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
      inner.className = 'duplicate-table__th-inner';
      const grip = document.createElement('span');
      grip.className = 'duplicate-table__col-grip';
      grip.textContent = '⋮⋮';
      grip.title = 'Перетащите для изменения порядка колонок';
      inner.appendChild(grip);
      const label = document.createElement('span');
      label.className = 'duplicate-table__col-label';
      label.textContent = def.label;
      inner.appendChild(label);
      th.appendChild(inner);
      const resize = document.createElement('span');
      resize.className = 'duplicate-table__col-resize';
      resize.title = 'Изменить ширину колонки';
      th.appendChild(resize);
      headRow.appendChild(th);
    }
    thead.appendChild(headRow);
    table.appendChild(thead);

    const tbody = document.createElement('tbody');
    for (const row of rows) {
      const tr = document.createElement('tr');
      tr.dataset.rowKey = this.duplicateRowKey(row);
      tr.dataset.schemeId = row.scheme_xml_id || '';
      tr.dataset.objectId = row.object_id || '';

      for (const key of layout.order) {
        const def = defMap[key];
        if (!def) continue;
        const td = document.createElement('td');
        if (def.mono) td.classList.add('duplicate-table__cell--mono');
        if (key === 'tag') {
          const tagInput = document.createElement('input');
          tagInput.className = 'input duplicate-table__tag-input';
          tagInput.value = row.tag || '';
          tagInput.addEventListener('click', (e) => e.stopPropagation());
          tagInput.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') tagInput.blur();
          });
          tagInput.addEventListener('change', () => this.patchDuplicateTagFromRow(row, tagInput.value, tr));
          td.appendChild(tagInput);
        } else {
          const text = this.duplicateCellValue(row, key) || '—';
          td.textContent = text;
          if (text !== '—') td.title = text;
        }
        tr.appendChild(td);
      }

      tr.addEventListener('click', () => this.navigateToDuplicateRow(row, tr));
      tr.classList.add('duplicate-table__row');
      tbody.appendChild(tr);
    }
    table.appendChild(tbody);
    wrap.innerHTML = '';
    wrap.appendChild(table);
    this.setupDuplicateTableColumns(table, layout);
    this.syncDuplicateExportButton();
  },

  buildDuplicateExportSection() {
    const section = document.createElement('div');
    section.className = 'duplicate-export';
    section.id = 'duplicateExportSection';

    const title = document.createElement('div');
    title.className = 'props-field__label';
    title.textContent = 'Выгрузка дубликатов Tag';
    section.appendChild(title);

    const hint = document.createElement('div');
    hint.className = 'duplicate-settings__exclusions-hint';
    hint.textContent = 'В отчёт попадают все строки текущей таблицы. Отметьте колонки и нажмите «Выгрузить».';
    section.appendChild(hint);

    const formatSel = document.createElement('select');
    formatSel.className = 'input duplicate-export__format';
    formatSel.id = 'duplicateExportFormat';
    [['csv', 'CSV'], ['txt', 'TXT'], ['xml', 'XML']].forEach(([v, l]) => {
      const opt = document.createElement('option');
      opt.value = v;
      opt.textContent = l;
      formatSel.appendChild(opt);
    });
    section.appendChild(this.labeledBlock('Формат', formatSel));

    const checks = document.createElement('div');
    checks.className = 'duplicate-export__columns display-settings__checks';
    checks.id = 'duplicateExportColumns';
    for (const def of this.duplicateTableColumnDefs()) {
      const wrap = document.createElement('label');
      const cb = document.createElement('input');
      cb.type = 'checkbox';
      cb.value = def.key;
      cb.checked = true;
      wrap.appendChild(cb);
      wrap.appendChild(document.createTextNode(def.label));
      checks.appendChild(wrap);
    }
    section.appendChild(this.labeledBlock('Колонки отчёта', checks));

    const btnRow = document.createElement('div');
    btnRow.className = 'duplicate-export__actions';
    const exportBtn = document.createElement('button');
    exportBtn.type = 'button';
    exportBtn.className = 'btn btn--primary';
    exportBtn.id = 'duplicateExportBtn';
    exportBtn.textContent = 'Выгрузить';
    exportBtn.addEventListener('click', () => this.exportDuplicateRows());
    btnRow.appendChild(exportBtn);
    section.appendChild(btnRow);
    return section;
  },

  selectedDuplicateExportColumns() {
    const checks = document.getElementById('duplicateExportColumns');
    if (!checks) return [];
    const selected = [];
    for (const cb of checks.querySelectorAll('input[type="checkbox"]')) {
      if (cb.checked) selected.push(cb.value);
    }
    return selected;
  },

  syncDuplicateExportButton() {
    const btn = document.getElementById('duplicateExportBtn');
    if (!btn) return;
    const hasRows = (this.duplicateRows || []).length > 0;
    btn.disabled = !hasRows;
  },

  csvEscapeDuplicateCell(value) {
    const text = String(value ?? '');
    if (/[;"\n\r]/.test(text)) {
      return `"${text.replace(/"/g, '""')}"`;
    }
    return text;
  },

  xmlEscapeDuplicateCell(value) {
    return String(value ?? '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&apos;');
  },

  txtEscapeDuplicateCell(value) {
    return String(value ?? '').replace(/[\t\r\n]+/g, ' ');
  },

  buildDuplicateExportText(format, columnKeys, rows) {
    const defs = this.duplicateTableColumnDefs();
    const defMap = Object.fromEntries(defs.map(d => [d.key, d]));
    const headers = columnKeys.map(key => defMap[key]?.label || key);
    if (format === 'csv') {
      const lines = [headers.map(h => this.csvEscapeDuplicateCell(h)).join(';')];
      for (const row of rows) {
        lines.push(columnKeys.map(key => this.csvEscapeDuplicateCell(this.duplicateCellValue(row, key))).join(';'));
      }
      return lines.join('\r\n');
    }
    if (format === 'txt') {
      const lines = [headers.map(h => this.txtEscapeDuplicateCell(h)).join('\t')];
      for (const row of rows) {
        lines.push(columnKeys.map(key => this.txtEscapeDuplicateCell(this.duplicateCellValue(row, key))).join('\t'));
      }
      return lines.join('\r\n');
    }
    const parts = ['<?xml version="1.0" encoding="UTF-8"?>', '<duplicates>'];
    for (const row of rows) {
      parts.push('  <row>');
      for (const key of columnKeys) {
        const tag = key.replace(/[^A-Za-z0-9_]/g, '_');
        parts.push(`    <${tag}>${this.xmlEscapeDuplicateCell(this.duplicateCellValue(row, key))}</${tag}>`);
      }
      parts.push('  </row>');
    }
    parts.push('</duplicates>');
    return parts.join('\n');
  },

  exportDuplicateRows() {
    const rows = this.duplicateRows || [];
    if (!rows.length) {
      alert('Сначала выполните поиск дубликатов Tag');
      return;
    }
    const columns = this.selectedDuplicateExportColumns();
    if (!columns.length) {
      alert('Выберите хотя бы одну колонку');
      return;
    }
    const format = document.getElementById('duplicateExportFormat')?.value || 'csv';
    const text = this.buildDuplicateExportText(format, columns, rows);
    const mime = format === 'xml' ? 'application/xml;charset=utf-8' : 'text/plain;charset=utf-8';
    const blob = new Blob([text], { type: mime });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `tag-duplicates.${format}`;
    a.click();
    URL.revokeObjectURL(url);
  },

  setupDuplicateTableColumns(table, layout) {
    if (!table || table.dataset.columnsReady === '1') return;
    table.dataset.columnsReady = '1';
    const defs = this.duplicateTableColumnDefs();
    const defMap = Object.fromEntries(defs.map(d => [d.key, d]));
    let dragKey = null;

    const applyColumnWidth = (key, width) => {
      const col = table.querySelector(`colgroup col[data-col-key="${key}"]`);
      const def = defMap[key];
      const px = Math.max(def?.minWidth || 64, width);
      if (col) col.style.width = `${px}px`;
      layout.widths[key] = px;
    };

    table.querySelectorAll('.duplicate-table__col-resize').forEach(handle => {
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
          this.saveDuplicateTableLayout(layout.order, layout.widths);
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
        this.saveDuplicateTableLayout(layout.order, layout.widths);
      });
    });

    table.querySelectorAll('.duplicate-table__col-grip').forEach(grip => {
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
        th.classList.add('duplicate-table__th--drag-over');
      });
      grip.addEventListener('dragend', () => {
        dragKey = null;
        table.querySelectorAll('.duplicate-table__th--drag-over').forEach(el => el.classList.remove('duplicate-table__th--drag-over'));
      });
      grip.setAttribute('draggable', 'true');
    });

    table.querySelectorAll('thead th').forEach(th => {
      th.addEventListener('dragover', (e) => {
        if (!dragKey) return;
        e.preventDefault();
        e.dataTransfer.dropEffect = 'move';
        table.querySelectorAll('.duplicate-table__th--drag-over').forEach(el => {
          if (el !== th) el.classList.remove('duplicate-table__th--drag-over');
        });
        th.classList.add('duplicate-table__th--drag-over');
      });
      th.addEventListener('dragleave', () => {
        th.classList.remove('duplicate-table__th--drag-over');
      });
      th.addEventListener('drop', (e) => {
        e.preventDefault();
        e.stopPropagation();
        const fromKey = e.dataTransfer.getData('text/plain') || dragKey;
        const toKey = th.dataset.colKey;
        table.querySelectorAll('.duplicate-table__th--drag-over').forEach(el => el.classList.remove('duplicate-table__th--drag-over'));
        if (!fromKey || !toKey || fromKey === toKey) return;
        const order = [...layout.order];
        const fromIdx = order.indexOf(fromKey);
        const toIdx = order.indexOf(toKey);
        if (fromIdx < 0 || toIdx < 0) return;
        order.splice(fromIdx, 1);
        order.splice(toIdx, 0, fromKey);
        layout.order = order;
        this.saveDuplicateTableLayout(layout.order, layout.widths);
        this.renderDuplicateTable(table.closest('.duplicate-table-wrap'));
      });
    });
  },

  async navigateToDuplicateRow(row, tr) {
    if (!row.scheme_xml_id) return;
    document.querySelectorAll('.duplicate-table__row--active, .props-table__row--active').forEach(el => el.classList.remove('duplicate-table__row--active', 'props-table__row--active'));
    if (tr) tr.classList.add('duplicate-table__row--active', 'props-table__row--active');

    const sameScheme = this.currentSchemeId === row.scheme_xml_id && this.currentScheme;
    if (!sameScheme) {
      await this.openScheme(row.scheme_xml_id, row.scheme_name || '');
    }

    const obj = this.currentScheme?.GraphicObject?.find(o => o.Id === row.object_id);
    if (obj) {
      Inspector.syncActiveTabFromDom();
      await this.selectObject(obj, { fromList: true });
    }
  },

  centerObjectInViewport(objectId) {
    const svg = document.getElementById('schemeSvg');
    if (!svg || svg.hidden) return;
    if (Renderer.centerObjectInView(svg, objectId)) {
      this.applyTransform();
    }
  },

  initSolutionSearch() {
    const panel = document.getElementById('solutionSearchPanel');
    const toggleBtn = document.getElementById('btnToggleSolutionSearch');
    const input = document.getElementById('solutionSearchInput');
    const clearBtn = document.getElementById('btnClearSolutionSearch');
    if (!panel || !toggleBtn || !input || !clearBtn) return;

    this.solutionSearchOpen = localStorage.getItem('solutionSearchOpen') === 'true';
    this.applySolutionSearchOpen();
    this.updateSolutionSearchEnabled();

    toggleBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      this.setSolutionSearchOpen(!this.solutionSearchOpen);
    });
    clearBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      this.clearSolutionSearch();
      input.focus();
    });
    input.addEventListener('input', () => {
      this.onSolutionSearchInput();
    });
    input.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        e.preventDefault();
        this.runSolutionSearch(input.value, { immediate: true });
      }
    });
  },

  setSolutionSearchOpen(open) {
    this.solutionSearchOpen = !!open;
    localStorage.setItem('solutionSearchOpen', String(this.solutionSearchOpen));
    this.applySolutionSearchOpen();
    if (this.solutionSearchOpen) {
      const input = document.getElementById('solutionSearchInput');
      if (input && !input.disabled) {
        input.focus();
        if (input.value.trim()) this.runSolutionSearch(input.value, { immediate: true });
      } else {
        this.renderSolutionSearchState('empty');
      }
    }
  },

  applySolutionSearchOpen() {
    const panel = document.getElementById('solutionSearchPanel');
    const toggleBtn = document.getElementById('btnToggleSolutionSearch');
    if (!panel || !toggleBtn) return;
    panel.hidden = !this.solutionSearchOpen;
    toggleBtn.setAttribute('aria-expanded', String(this.solutionSearchOpen));
    toggleBtn.classList.toggle('btn--active', this.solutionSearchOpen);
  },

  updateSolutionSearchEnabled() {
    const input = document.getElementById('solutionSearchInput');
    const toggleBtn = document.getElementById('btnToggleSolutionSearch');
    const enabled = Boolean(this.solutionId);
    if (input) input.disabled = !enabled;
    if (toggleBtn) toggleBtn.disabled = !enabled;
    if (!enabled) {
      this.clearSolutionSearch({ keepFocus: false });
      this.renderSolutionSearchState('empty');
    }
  },

  onSolutionSearchInput() {
    const input = document.getElementById('solutionSearchInput');
    const clearBtn = document.getElementById('btnClearSolutionSearch');
    if (clearBtn) clearBtn.hidden = !(input && input.value);
    this.runSolutionSearch(input?.value || '', { immediate: false });
  },

  clearSolutionSearch(options = {}) {
    if (this.solutionSearchTimer) {
      clearTimeout(this.solutionSearchTimer);
      this.solutionSearchTimer = null;
    }
    this.solutionSearchReq += 1;
    this.solutionSearchQuery = '';
    this.solutionSearchItems = [];
    const input = document.getElementById('solutionSearchInput');
    if (input) input.value = '';
    const clearBtn = document.getElementById('btnClearSolutionSearch');
    if (clearBtn) clearBtn.hidden = true;
    this.renderSolutionSearchState('empty');
    if (options.keepFocus !== false && input && !input.disabled && this.solutionSearchOpen) {
      input.focus();
    }
  },

  runSolutionSearch(rawQuery, options = {}) {
    if (this.solutionSearchTimer) {
      clearTimeout(this.solutionSearchTimer);
      this.solutionSearchTimer = null;
    }
    const query = (rawQuery || '').trim();
    const clearBtn = document.getElementById('btnClearSolutionSearch');
    if (clearBtn) clearBtn.hidden = !query;

    if (!this.solutionId) {
      this.renderSolutionSearchState('empty');
      return;
    }
    if (!query) {
      this.solutionSearchQuery = '';
      this.solutionSearchItems = [];
      this.renderSolutionSearchState('empty');
      return;
    }

    const exec = async () => {
      const reqId = ++this.solutionSearchReq;
      this.renderSolutionSearchState('loading');
      try {
        const result = await API.searchSolution(this.solutionId, query, { limit: 50, offset: 0 });
        if (reqId !== this.solutionSearchReq) return;
        this.solutionSearchQuery = result.query || query;
        this.solutionSearchItems = Array.isArray(result.items) ? result.items : [];
        if (!this.solutionSearchItems.length) {
          this.renderSolutionSearchState('zero', result);
          return;
        }
        this.renderSolutionSearchState('success', result);
      } catch (err) {
        if (reqId !== this.solutionSearchReq) return;
        this.renderSolutionSearchState('error', { error: err.message || 'Ошибка поиска' });
      }
    };

    if (options.immediate) {
      exec();
      return;
    }
    this.solutionSearchTimer = setTimeout(exec, 300);
  },

  renderSolutionSearchState(state, payload = {}) {
    const status = document.getElementById('solutionSearchStatus');
    const wrap = document.getElementById('solutionSearchTableWrap');
    const body = document.getElementById('solutionSearchBody');
    if (!status || !wrap || !body) return;
    status.classList.remove('solution-search__status--error');
    body.innerHTML = '';

    if (state === 'empty') {
      wrap.hidden = true;
      status.textContent = this.solutionId
        ? 'Введите имя, Tag или Id как во вкладке «Основные» (минимум 2 символа; короткий запрос — только Id)'
        : 'Загрузите решение, чтобы искать объекты';
      return;
    }
    if (state === 'loading') {
      wrap.hidden = true;
      status.textContent = 'Поиск…';
      return;
    }
    if (state === 'error') {
      wrap.hidden = true;
      status.classList.add('solution-search__status--error');
      status.textContent = payload.error || 'Ошибка поиска';
      return;
    }
    if (state === 'zero') {
      wrap.hidden = true;
      status.textContent = 'Ничего не найдено';
      return;
    }

    const items = Array.isArray(payload.items) ? payload.items : this.solutionSearchItems;
    const total = payload.total != null ? payload.total : items.length;
    status.textContent = total > items.length
      ? `Найдено: ${total} (показано ${items.length})`
      : `Найдено: ${items.length}`;
    wrap.hidden = false;

    for (const row of items) {
      const tr = document.createElement('tr');
      tr.className = 'solution-search__row duplicate-table__row';
      const cells = [
        { text: row.object_type || '', mono: false },
        { text: row.name || '', mono: false },
        { text: row.technology_object_id || '—', mono: true },
        { text: row.project_name || row.project_xml_id || '', mono: false },
        { text: row.scheme_name || row.scheme_xml_id || '', mono: false },
      ];
      for (const cell of cells) {
        const td = document.createElement('td');
        if (cell.mono) td.className = 'solution-search__cell--mono duplicate-table__cell--mono';
        td.textContent = cell.text;
        td.title = cell.text;
        tr.appendChild(td);
      }
      tr.addEventListener('click', () => this.navigateToSearchRow(row, tr));
      body.appendChild(tr);
    }
  },

  async navigateToSearchRow(row, tr) {
    if (!row?.scheme_xml_id) return;
    document.querySelectorAll('.solution-search__row.duplicate-table__row--active').forEach(el => {
      el.classList.remove('duplicate-table__row--active');
    });
    if (tr) tr.classList.add('duplicate-table__row--active');

    const sameScheme = this.currentSchemeId === row.scheme_xml_id && this.currentScheme;
    if (!sameScheme) {
      await this.openScheme(row.scheme_xml_id, row.scheme_name || '');
    }

    const obj = this.currentScheme?.GraphicObject?.find(o => o.Id === row.object_id);
    if (obj) {
      Inspector.syncActiveTabFromDom();
      await this.selectObject(obj, { fromList: true });
    }
  },

  async patchDuplicateTagFromRow(row, newTag, tr) {
    if (!this.solutionId || !this.sessionId) return;
    const trimmed = (newTag || '').trim();
    if (trimmed === row.tag) return;
    try {
      tr?.classList.add('duplicate-table__row--saving');
      await API.patchDuplicateTag(this.solutionId, this.sessionId, {
        scope: row.scope,
        object_id: row.object_id,
        scheme_xml_id: row.scheme_xml_id,
        property_id: row.property_id || '',
        entity_id: row.entity_id || '',
        new_tag: trimmed,
      });
      this.setDirty(true);
      row.tag = trimmed;
      if (row.scope === 'object' || row.scope === 'property') {
        if (this.currentSchemeId === row.scheme_xml_id) {
          const obj = this.currentScheme?.GraphicObject?.find(o => o.Id === row.object_id);
          if (obj && row.scope === 'object') {
            const patch = JSON.parse(JSON.stringify(obj));
            patch.Tag = trimmed;
            if (patch.TechnologyObject?.[0]) patch.TechnologyObject[0].Tag = trimmed;
            await this.patchObjectOnScheme(row.object_id, row.scheme_xml_id, patch, { lightweight: true });
          }
        }
        this.objectDetailsCache.delete(row.object_id);
      }
      await this.runDuplicateCheckSilent();
    } catch (err) {
      alert('Ошибка сохранения Tag: ' + err.message);
    } finally {
      tr?.classList.remove('duplicate-table__row--saving');
    }
  },

  async patchObjectOnScheme(objectId, schemeXMLId, patch, options = {}) {
    await API.patchObject(this.solutionId, this.sessionId, objectId, schemeXMLId, patch);
    if (this.currentSchemeId === schemeXMLId && this.currentScheme) {
      const idx = this.currentScheme.GraphicObject.findIndex(o => o.Id === objectId);
      if (idx >= 0) this.currentScheme.GraphicObject[idx] = patch;
      if (options.lightweight && this.currentScheme) {
        const svg = document.getElementById('schemeSvg');
        Renderer.replaceObject(svg, patch, (obj) => this.selectObject(obj));
        Renderer.selectObject(svg, objectId);
      }
    }
    this.updateObjectCache(objectId, patch);
  },

  async resolveDuplicateRuleId() {
    if (Renderer.duplicateRule?.id) return Renderer.duplicateRule.id;
    const rules = await API.getComplianceRules();
    const duplicateRule = (rules || []).find(r => r.text_condition === 'duplicate_tag');
    return duplicateRule?.id || null;
  },

  async recheckObjectDuplicates() {
    const objectId = Inspector.currentObject?.graphic_object?.Id;
    if (!objectId || !this.solutionId) return;
    if (!this.duplicateCompliance) {
      alert('Сначала выполните поиск дубликатов Tag');
      return;
    }
    await Inspector.flushPendingEdits();
    const ruleId = await this.resolveDuplicateRuleId();
    if (!ruleId) {
      alert('Настройте правило поиска дубликатов Tag');
      return;
    }
    Inspector.duplicateRecheckLoading = true;
    const obj = Inspector.currentObject.graphic_object;
    const to = Inspector.currentObject.technology_objects?.[0];
    Inspector.updateTitleRow(obj, to);
    try {
      const result = await API.recheckDuplicateScan(this.solutionId, ruleId, this.sessionId, objectId);
      await this.applyDuplicateScanResult(result, { reopenPanel: false });
      Inspector.completeDuplicateRecheck(Boolean(result.object_has_duplicates));
    } catch (err) {
      alert('Ошибка проверки дубликатов: ' + err.message);
    } finally {
      Inspector.duplicateRecheckLoading = false;
      if (Inspector.currentObject?.graphic_object) {
        Inspector.updateTitleRow(
          Inspector.currentObject.graphic_object,
          Inspector.currentObject.technology_objects?.[0],
        );
      }
    }
  },

  async runDuplicateCheckSilent() {
    if (!this.solutionId) return;
    const ruleId = await this.resolveDuplicateRuleId();
    if (!ruleId) return;
    const { job_id: jobId } = await API.startDuplicateScan(this.solutionId, ruleId, this.sessionId);
    const job = await API.pollDuplicateScanJob(this.solutionId, jobId);
    let result = null;
    if (job.result) {
      result = typeof job.result === 'string' ? JSON.parse(job.result) : job.result;
    }
    await this.applyDuplicateScanResult(
      result || { violations_by_object: {}, duplicate_tags: [], nodes: [], rows: [] },
      { reopenPanel: false },
    );
  },

  updateDuplicatePanel() {
    const panel = document.getElementById('duplicatePanel');
    const body = document.getElementById('duplicatePanelBody');
    const countEl = document.getElementById('duplicatePanelCount');
    const showBtn = document.getElementById('btnShowDuplicatePanel');
    const toggleBtn = document.getElementById('btnToggleDuplicatePanel');
    if (!panel || !body) return;

    const hasCurrentSchemeDuplicates = this.currentSchemeId
      && this.duplicateRows?.some(r => r.scheme_xml_id === this.currentSchemeId);

    if (showBtn) {
      showBtn.hidden = !this.duplicatePanelDismissed || !hasCurrentSchemeDuplicates;
    }

    if (this.duplicatePanelDismissed) {
      panel.hidden = true;
      return;
    }
    if (!hasCurrentSchemeDuplicates) {
      panel.hidden = true;
      return;
    }
    const rows = this.duplicateRows.filter(r => r.scheme_xml_id === this.currentSchemeId);
    panel.hidden = false;
    panel.classList.toggle('duplicate-panel--collapsed', this.duplicatePanelCollapsed);

    if (countEl) {
      countEl.textContent = `(${rows.length})`;
    }
    if (toggleBtn) {
      toggleBtn.textContent = this.duplicatePanelCollapsed ? '▲' : '▼';
      toggleBtn.title = this.duplicatePanelCollapsed ? 'Развернуть' : 'Свернуть';
      toggleBtn.setAttribute('aria-expanded', String(!this.duplicatePanelCollapsed));
    }

    if (this.duplicatePanelCollapsed) {
      body.innerHTML = '';
      return;
    }

    body.innerHTML = '';
    for (const row of rows) {
      const item = document.createElement('div');
      item.className = 'duplicate-panel__item';
      item.innerHTML = `
        <span class="duplicate-panel__item-tag">${this.escapeHtml(row.tag)}</span>
        <span class="duplicate-panel__item-meta">${this.escapeHtml(row.object_name)} · ${this.escapeHtml(row.scope_label)} · ${this.escapeHtml(row.source_label)}</span>
        <span class="duplicate-panel__item-meta">${this.escapeHtml(row.technology_object_id || '—')}</span>`;
      item.addEventListener('click', () => {
        const obj = this.currentScheme?.GraphicObject?.find(o => o.Id === row.object_id);
        if (obj) this.selectObject(obj, { fromList: true });
      });
      body.appendChild(item);
    }
  },

  escapeHtml(s) {
    const d = document.createElement('div');
    d.textContent = s || '';
    return d.innerHTML;
  },

  async saveDuplicateComplianceSettings() {
    const section = document.getElementById('duplicateSettingsSection');
    if (!section?._fields) return null;
    const f = section._fields;
    const data = {
      template: '*',
      parameter_scope: 'object',
      entity_template: '',
      parameter: '',
      value_source: 'field',
      text_condition: 'duplicate_tag',
      expected_text: '',
      empty_allowed: true,
      indication: {
        type: f.typeSel.value,
        color: f.colorInput.value,
        pulsePeriodMs: 1000,
        excluded_scopes: this.buildDuplicateExcludedScopesFromFields(f),
        excluded_tag_patterns: this.buildDuplicateExcludedTagPatternsFromList(f.tagExclusionsList),
      },
    };
    let ruleId = section.dataset.ruleId;
    if (ruleId) {
      await API.updateComplianceRule(ruleId, data);
    } else {
      const created = await API.createComplianceRule(data);
      ruleId = created.id;
      section.dataset.ruleId = ruleId;
    }
    await Renderer.loadComplianceRules();
    return ruleId;
  },

  async runDuplicateCheck() {
    if (!this.solutionId) return;
    if (this.tagEditApplying) {
      alert('Сейчас выполняется редактирование TAG. Дождитесь окончания.');
      return;
    }
    if (this.duplicateBackgroundEnabled) {
      alert('Обычный поиск недоступен, пока включён поиск в фоне');
      return;
    }

    const overlay = document.getElementById('duplicateScanOverlay');
    const overlayTitle = document.getElementById('duplicateScanTitle');
    const overlayDetail = document.getElementById('duplicateScanDetail');
    const overlayBar = document.getElementById('duplicateScanBar');
    const overlayStatus = document.getElementById('duplicateScanStatus');

    const showProgress = (title, detail, status, pct) => {
      overlayTitle.textContent = title;
      overlayDetail.textContent = detail;
      overlayStatus.textContent = status;
      overlayBar.style.width = `${pct}%`;
      overlay.hidden = false;
    };

    try {
      const ruleId = await this.saveDuplicateComplianceSettings();
      if (!ruleId) {
        alert('Не удалось сохранить настройки');
        return;
      }

      this.duplicatePanelUserClosedDuringScan = false;

      showProgress('Поиск дубликатов…', '', 'Запуск…', 0);

      const { job_id: jobId } = await API.startDuplicateScan(this.solutionId, ruleId, this.sessionId);
      const job = await API.pollDuplicateScanJob(this.solutionId, jobId, (j) => {
        const total = j.total_objects || 1;
        const pct = Math.round((j.processed_objects / total) * 100);
        const detail = j.duplicate_tag_count > 0 ? `Найдено дубликатов: ${j.duplicate_tag_count}` : '';
        showProgress(
          'Поиск дубликатов…',
          detail,
          `Сканирование… ${pct}% (${j.processed_objects} из ${j.total_objects})`,
          pct,
        );
      });

      showProgress('Поиск дубликатов…', '', 'Применение результатов…', 100);
      let result = null;
      if (job.result) {
        result = typeof job.result === 'string' ? JSON.parse(job.result) : job.result;
      }
      await this.applyDuplicateScanResult(
        result || { violations_by_object: {}, duplicate_tags: [], nodes: [], rows: [] },
        { reopenPanel: true },
      );
    } catch (err) {
      const msg = String(err.message || '');
      if (msg.includes('tag edit job already')) {
        alert('Сейчас выполняется редактирование TAG. Дождитесь окончания.');
      } else {
        alert('Ошибка поиска дубликатов: ' + err.message);
      }
    } finally {
      overlay.hidden = true;
    }
  },

  async applyDuplicateScanResult(result, options = {}) {
    const duplicateRule = Renderer.duplicateRule;
    let indication = { type: 'solid', color: '#F85149' };
    if (duplicateRule?.indication) {
      indication = Renderer.parseIndication(duplicateRule.indication);
    }

    this.duplicateRows = result.rows || [];
    if (options.reopenPanel && !this.duplicatePanelUserClosedDuringScan) {
      this.duplicatePanelDismissed = false;
    }
    this.duplicateCompliance = {
      ruleId: duplicateRule?.id || null,
      indication,
      violationsByObjectId: { ...(result.violations_by_object || {}) },
      nodes: result.nodes || [],
    };

    this.renderDuplicateTable();
    this.updateDuplicatePanel();

    const dupRuleId = this.duplicateCompliance.ruleId;
    const fieldNodes = (this.complianceReport?.nodes || []).filter(
      node => !dupRuleId || !node.indicators?.some(ind => ind.rule_id === dupRuleId),
    );
    Explorer.setComplianceHints(fieldNodes);
    this.applyDuplicateComplianceToExplorer();
    if (this.lastTree) {
      Explorer.applyComplianceToTree();
    }
    this.syncComplianceReportFromHints();
    this.updateComplianceFooter({ nodes: Object.values(Explorer.complianceHints) }, true);

    if (!options.soft && this.currentScheme) {
      const svg = document.getElementById('schemeSvg');
      await Renderer.renderSchemeAsync(svg, this.currentScheme, (obj) => this.selectObject(obj));
      this.applyTransform();
      this.syncExplorerComplianceFromCurrentScheme();
    }
    if (Inspector.currentObject?.graphic_object) {
      const objectId = Inspector.currentObject.graphic_object.Id;
      Inspector.updateDuplicateTabBadge(objectId);
      Inspector.syncActiveTabFromDom();
      if (Inspector.activeTab === 'duplicates') {
        Inspector.render(Inspector.currentObject);
      }
    }
  },

  hideComplianceSettings() {
    document.getElementById('complianceModal').hidden = true;
  },

  async showTagEditSettings() {
    if (this.tagEditApplying) return;
    const rules = await API.getTagEditRules();
    const templates = this.solutionId
      ? (await API.getSolutionTemplates(this.solutionId)).map(t => t.template)
      : (await API.getDisplaySettings()).map(s => s.template);
    const body = document.getElementById('tagEditSettingsBody');
    body.innerHTML = '<p class="modal-hint">Правила идут сверху вниз. Сначала выбираете, что проверять (как в «Проверках»), затем условие. Если условие «Пустое поле», текст не нужен. Галки блоков — дополнительный отбор: объект подходит, только если во всех отмеченных блоках есть непустой Tag. Ниже — одно или несколько действий: какое поле менять и что делать. Экспорт сохраняет правила в читаемый JSON, его можно править и загрузить снова.</p>';
    const fileRow = document.createElement('div');
    fileRow.className = 'tag-edit-file-actions';
    const exportBtn = document.createElement('button');
    exportBtn.type = 'button';
    exportBtn.className = 'btn btn--ghost';
    exportBtn.textContent = 'Экспорт правил';
    exportBtn.addEventListener('click', () => this.exportTagEditRulesFile());
    const importBtn = document.createElement('button');
    importBtn.type = 'button';
    importBtn.className = 'btn btn--ghost';
    importBtn.textContent = 'Импорт правил';
    const importInput = document.createElement('input');
    importInput.type = 'file';
    importInput.accept = 'application/json,.json';
    importInput.hidden = true;
    importBtn.addEventListener('click', () => importInput.click());
    importInput.addEventListener('change', async () => {
      const file = importInput.files && importInput.files[0];
      importInput.value = '';
      if (!file) return;
      try {
        await this.importTagEditRulesFile(file);
        this.hideTagEditSettings();
        await this.showTagEditSettings();
      } catch (err) {
        alert(err.message || 'Не удалось загрузить правила');
      }
    });
    fileRow.appendChild(exportBtn);
    fileRow.appendChild(importBtn);
    fileRow.appendChild(importInput);
    body.appendChild(fileRow);

    const list = document.createElement('div');
    list.className = 'tag-edit-rules-list';
    list.id = 'tagEditRulesList';

    for (const rule of rules) {
      list.appendChild(this.buildTagEditRuleRow(rule, templates));
    }
    body.appendChild(list);

    const addBtn = document.createElement('button');
    addBtn.className = 'btn btn--ghost';
    addBtn.textContent = '+ Добавить правило';
    addBtn.addEventListener('click', () => {
      list.appendChild(this.buildTagEditRuleRow({
        template: templates[0] || 'Valve',
        parameter_scope: 'object',
        parameter: 'Tag',
        value_source: 'field',
        text_condition: 'contains',
        pattern: '',
        check_properties: false,
        check_commands: false,
        check_protections: false,
        actions: [{ parameter_scope: 'object', parameter: 'Tag', value_source: 'field', action: 'delete', replacement: '', replace_mode: 'found' }],
      }, templates, true));
    });
    body.appendChild(addBtn);

    const actions = document.createElement('div');
    actions.className = 'modal-actions';
    const applyBtn = document.createElement('button');
    applyBtn.className = 'btn btn--primary';
    applyBtn.textContent = 'Применить';
    applyBtn.addEventListener('click', () => this.applyTagEditRules());
    const cancelBtn = document.createElement('button');
    cancelBtn.className = 'btn btn--ghost';
    cancelBtn.textContent = 'Закрыть';
    cancelBtn.addEventListener('click', () => this.hideTagEditSettings());
    actions.appendChild(applyBtn);
    actions.appendChild(cancelBtn);
    body.appendChild(actions);

    document.getElementById('tagEditModal').hidden = false;
  },

  bindTagEditScopeParams(templateSel, scopeSel, paramSel, sourceSel, stash) {
    const OBJECT_SCALAR_PARAMS = new Set(['Tag', 'Name', 'Description', 'Id', 'ExternalCode']);
    const tagSourceAllowed = (param) => {
      if (scopeSel.value === 'property' || scopeSel.value === 'command' || scopeSel.value === 'protection') return true;
      return !OBJECT_SCALAR_PARAMS.has(param) || param === 'Tag';
    };
    const syncSourceOptions = () => {
      const paramReady = !paramSel.disabled && !!paramSel.value;
      sourceSel.disabled = !paramReady;
      const tagOpt = sourceSel.querySelector('option[value="tag"]');
      if (!tagOpt) return;
      if (scopeSel.value === 'command' || scopeSel.value === 'protection') {
        tagOpt.hidden = false;
        tagOpt.disabled = false;
        return;
      }
      const allowed = tagSourceAllowed(paramSel.value);
      tagOpt.hidden = !allowed;
      tagOpt.disabled = !allowed;
      if (!allowed && sourceSel.value === 'tag') sourceSel.value = 'field';
    };
    const setPlaceholder = (text) => {
      paramSel.innerHTML = '';
      const opt = document.createElement('option');
      opt.value = '';
      opt.textContent = text;
      opt.disabled = true;
      opt.selected = true;
      paramSel.appendChild(opt);
      paramSel.disabled = true;
      syncSourceOptions();
    };
    const fill = (items, current) => {
      if (!items || !items.length) {
        setPlaceholder('Нет вариантов');
        return;
      }
      paramSel.innerHTML = '';
      for (const item of items) {
        const opt = document.createElement('option');
        opt.value = item;
        opt.textContent = item;
        if (item === current) opt.selected = true;
        paramSel.appendChild(opt);
      }
      paramSel.disabled = false;
      if (!paramSel.value) paramSel.value = items[0];
      syncSourceOptions();
    };
    const currentForScope = (scope) => {
      if (scope === 'command' || scope === 'protection') return paramSel.value || stash.entity_template || '';
      return paramSel.value || stash.parameter || 'Tag';
    };
    const load = async () => {
      const scope = scopeSel.value;
      const tpl = templateSel.value;
      const current = currentForScope(scope);
      if (!this.solutionId || !tpl) {
        fill(scope === 'object' ? ['Tag', 'Name', 'Description', 'Id', 'ExternalCode'] : [], current);
        return;
      }
      try {
        const items = await this.loadComplianceParameters(scope, tpl, false);
        fill(items, current);
      } catch (_) {
        setPlaceholder('Ошибка загрузки');
      }
    };
    templateSel.addEventListener('change', () => load());
    scopeSel.addEventListener('change', () => load());
    paramSel.addEventListener('change', () => syncSourceOptions());
    load();
    return () => this.collectTagEditTarget(scopeSel, paramSel, sourceSel, stash);
  },

  collectTagEditTarget(scopeSel, paramSel, sourceSel, stash) {
    const scope = scopeSel.value;
    const isEntity = scope === 'command' || scope === 'protection';
    let entityTemplate = '';
    let parameter = paramSel.value;
    let valueSource = sourceSel.value;
    if (isEntity) {
      entityTemplate = paramSel.value;
      const entityChanged = entityTemplate !== (stash.entity_template || '');
      if (entityChanged || !stash.parameter) {
        parameter = valueSource === 'tag' ? 'Tag' : 'Name';
      } else {
        parameter = stash.parameter;
      }
    }
    stash.entity_template = entityTemplate;
    stash.parameter = parameter;
    stash.value_source = valueSource;
    return { parameter_scope: scope, entity_template: entityTemplate, parameter, value_source: valueSource };
  },

  createTagEditScopeSelect(value) {
    const sel = document.createElement('select');
    sel.className = 'input tag-edit-rule__field';
    [['object', 'Основные характеристики'], ['property', 'Характеристики'], ['command', 'Команды'], ['protection', 'Защиты']].forEach(([v, l]) => {
      const opt = document.createElement('option');
      opt.value = v;
      opt.textContent = l;
      if ((value || 'object') === v) opt.selected = true;
      sel.appendChild(opt);
    });
    return sel;
  },

  createTagEditSourceSelect(value) {
    const sel = document.createElement('select');
    sel.className = 'input tag-edit-rule__field';
    [['field', 'Значение'], ['tag', 'Tag параметра']].forEach(([v, l]) => {
      const opt = document.createElement('option');
      opt.value = v;
      opt.textContent = l;
      if ((value || 'field') === v) opt.selected = true;
      sel.appendChild(opt);
    });
    return sel;
  },

  buildTagEditActionRow(action) {
    const row = document.createElement('div');
    row.className = 'tag-edit-action';
    const stash = {
      entity_template: action.entity_template || '',
      parameter: action.parameter || 'Tag',
      value_source: action.value_source || 'field',
    };
    const scopeSel = this.createTagEditScopeSelect(action.parameter_scope);
    const paramSel = document.createElement('select');
    paramSel.className = 'input tag-edit-rule__field';
    const sourceSel = this.createTagEditSourceSelect(action.value_source);
    const actionSel = document.createElement('select');
    actionSel.className = 'input tag-edit-rule__field';
    [['delete', 'Очистить'], ['replace', 'Заменить']].forEach(([v, l]) => {
      const opt = document.createElement('option');
      opt.value = v;
      opt.textContent = l;
      if ((action.action || 'delete') === v) opt.selected = true;
      actionSel.appendChild(opt);
    });
    const modeSel = document.createElement('select');
    modeSel.className = 'input tag-edit-rule__field';
    [['found', 'Только найденное'], ['whole', 'Всю строку'], ['first', 'Первое совпадение'], ['all', 'Все совпадения'], ['last', 'Последнее совпадение']].forEach(([v, l]) => {
      const opt = document.createElement('option');
      opt.value = v;
      opt.textContent = l;
      if ((action.replace_mode || 'found') === v) opt.selected = true;
      modeSel.appendChild(opt);
    });
    const replacementInput = document.createElement('input');
    replacementInput.type = 'text';
    replacementInput.className = 'input tag-edit-rule__field';
    replacementInput.value = action.replacement || '';
    replacementInput.placeholder = 'Заменить на';
    const replacementWrap = this.labeledField('Заменить на', replacementInput);
    const modeWrap = this.labeledField('Как заменить', modeSel);
    const syncReplace = () => {
      const show = actionSel.value === 'replace';
      replacementWrap.hidden = !show;
      modeWrap.hidden = !show;
      if (!show) replacementInput.value = '';
    };
    actionSel.addEventListener('change', syncReplace);
    syncReplace();
    const delBtn = document.createElement('button');
    delBtn.type = 'button';
    delBtn.className = 'btn btn--ghost';
    delBtn.textContent = '✕';
    delBtn.addEventListener('click', () => {
      const list = row.parentElement;
      if (list && list.querySelectorAll('.tag-edit-action').length <= 1) {
        alert('Нужно хотя бы одно действие');
        return;
      }
      row.remove();
    });
    row.appendChild(this.labeledField('Что меняем', scopeSel));
    row.appendChild(this.labeledField('Параметр', paramSel));
    row.appendChild(this.labeledField('Атрибут', sourceSel));
    row.appendChild(this.labeledField('Действие', actionSel));
    row.appendChild(modeWrap);
    row.appendChild(replacementWrap);
    row.appendChild(delBtn);
    row._collect = null;
    row._bind = (templateSel) => {
      row._collect = this.bindTagEditScopeParams(templateSel, scopeSel, paramSel, sourceSel, stash);
    };
    row._fields = { actionSel, replacementInput, modeSel };
    return row;
  },

  buildTagEditRuleRow(rule, templates, isNew) {
    const row = document.createElement('div');
    row.className = 'tag-edit-rule';
    if (rule.id) row.dataset.id = rule.id;
    const checkStash = {
      entity_template: rule.entity_template || '',
      parameter: rule.parameter || 'Tag',
      value_source: rule.value_source || 'field',
    };

    const templateSel = document.createElement('select');
    templateSel.className = 'input tag-edit-rule__field';
    const tmplList = templates && templates.length ? templates : [rule.template || 'Valve'];
    for (const t of tmplList) {
      const opt = document.createElement('option');
      opt.value = t;
      opt.textContent = t;
      if (t === rule.template) opt.selected = true;
      templateSel.appendChild(opt);
    }
    if (rule.template && !tmplList.includes(rule.template)) {
      const opt = document.createElement('option');
      opt.value = rule.template;
      opt.textContent = rule.template;
      opt.selected = true;
      templateSel.appendChild(opt);
    }

    const scopeSel = this.createTagEditScopeSelect(rule.parameter_scope);
    const paramSel = document.createElement('select');
    paramSel.className = 'input tag-edit-rule__field';
    const sourceSel = this.createTagEditSourceSelect(rule.value_source);

    const condSel = document.createElement('select');
    condSel.className = 'input tag-edit-rule__field';
    for (const [val, label] of [
      ['contains', 'Содержит текст'],
      ['starts_with', 'Текст начинается'],
      ['ends_with', 'Текст заканчивается'],
      ['empty', 'Пустое поле'],
    ]) {
      const opt = document.createElement('option');
      opt.value = val;
      opt.textContent = label;
      if (val === rule.text_condition) opt.selected = true;
      condSel.appendChild(opt);
    }

    const patternInput = document.createElement('input');
    patternInput.type = 'text';
    patternInput.className = 'input tag-edit-rule__field';
    patternInput.value = rule.pattern || '';
    patternInput.placeholder = 'Текст для проверки';
    const patternWrap = this.labeledField('Текст', patternInput);
    const syncPattern = () => {
      const empty = condSel.value === 'empty';
      patternWrap.hidden = empty;
      if (empty) patternInput.value = '';
    };
    condSel.addEventListener('change', syncPattern);
    syncPattern();

    const checkProps = document.createElement('input');
    checkProps.type = 'checkbox';
    checkProps.checked = !!rule.check_properties;
    const checkCmds = document.createElement('input');
    checkCmds.type = 'checkbox';
    checkCmds.checked = !!rule.check_commands;
    const checkProt = document.createElement('input');
    checkProt.type = 'checkbox';
    checkProt.checked = !!rule.check_protections;
    const nestedWrap = document.createElement('div');
    nestedWrap.className = 'tag-edit-rule__nested';
    [
      [checkProps, 'Характеристики'],
      [checkCmds, 'Команды'],
      [checkProt, 'Защиты'],
    ].forEach(([box, label]) => {
      const lab = document.createElement('label');
      lab.className = 'tag-edit-rule__check';
      lab.appendChild(box);
      lab.appendChild(document.createTextNode(label));
      nestedWrap.appendChild(lab);
    });

    const actionsWrap = document.createElement('div');
    actionsWrap.className = 'tag-edit-rule__actions';
    const actionsTitle = document.createElement('span');
    actionsTitle.className = 'props-field__label';
    actionsTitle.textContent = 'Действия';
    actionsWrap.appendChild(actionsTitle);
    const actionsList = document.createElement('div');
    actionsList.className = 'tag-edit-actions-list';
    const seedActions = (rule.actions && rule.actions.length)
      ? rule.actions
      : [{
          parameter_scope: 'object',
          parameter: rule.parameter || 'Tag',
          value_source: 'field',
          action: rule.action || 'delete',
          replacement: rule.replacement || '',
          replace_mode: 'found',
        }];
    seedActions.forEach((act) => {
      const actionRow = this.buildTagEditActionRow(act);
      actionRow._bind(templateSel);
      actionsList.appendChild(actionRow);
    });
    const addActionBtn = document.createElement('button');
    addActionBtn.type = 'button';
    addActionBtn.className = 'btn btn--ghost';
    addActionBtn.textContent = '+ Добавить действие';
    addActionBtn.addEventListener('click', () => {
      const actionRow = this.buildTagEditActionRow({
        parameter_scope: 'object',
        parameter: 'Tag',
        value_source: 'field',
        action: 'delete',
        replacement: '',
        replace_mode: 'found',
      });
      actionRow._bind(templateSel);
      actionsList.appendChild(actionRow);
    });
    actionsWrap.appendChild(actionsList);
    actionsWrap.appendChild(addActionBtn);

    const delBtn = document.createElement('button');
    delBtn.type = 'button';
    delBtn.className = 'btn btn--ghost tag-edit-rule__delete';
    delBtn.textContent = '✕';
    delBtn.addEventListener('click', async () => {
      if (rule.id && !isNew) await API.deleteTagEditRule(rule.id);
      row.remove();
    });

    row.appendChild(this.labeledField('Тип объекта', templateSel));
    row.appendChild(this.labeledField('Что проверяем', scopeSel));
    row.appendChild(this.labeledField('Параметр', paramSel));
    row.appendChild(this.labeledField('Атрибут параметра', sourceSel));
    row.appendChild(this.labeledField('Условие', condSel));
    row.appendChild(patternWrap);
    const nestedField = document.createElement('div');
    nestedField.className = 'tag-edit-rule__group tag-edit-rule__nested-group';
    const nestedLbl = document.createElement('span');
    nestedLbl.className = 'props-field__label';
    nestedLbl.textContent = 'Непустой Tag в блоках';
    nestedField.appendChild(nestedLbl);
    nestedField.appendChild(nestedWrap);
    row.appendChild(nestedField);
    row.appendChild(actionsWrap);
    row.appendChild(delBtn);

    const collectCheck = this.bindTagEditScopeParams(templateSel, scopeSel, paramSel, sourceSel, checkStash);
    row._fields = {
      templateSel, condSel, patternInput, checkProps, checkCmds, checkProt,
      collectCheck, actionsList,
    };
    return row;
  },

  collectTagEditRuleData(row) {
    const f = row._fields;
    const check = f.collectCheck();
    const actions = [];
    f.actionsList.querySelectorAll('.tag-edit-action').forEach((actionRow) => {
      const target = actionRow._collect ? actionRow._collect() : {};
      const af = actionRow._fields;
      actions.push({
        ...target,
        action: af.actionSel.value,
        replacement: af.actionSel.value === 'replace' ? af.replacementInput.value : '',
        replace_mode: af.modeSel.value,
      });
    });
    return {
      template: f.templateSel.value,
      ...check,
      text_condition: f.condSel.value,
      pattern: f.condSel.value === 'empty' ? '' : f.patternInput.value,
      check_properties: f.checkProps.checked,
      check_commands: f.checkCmds.checked,
      check_protections: f.checkProt.checked,
      actions,
    };
  },

  validateTagEditRules() {
    const rows = document.querySelectorAll('.tag-edit-rule');
    if (rows.length === 0) {
      alert('Добавьте хотя бы одно правило');
      return false;
    }
    for (const row of rows) {
      const data = this.collectTagEditRuleData(row);
      if (!data.template) {
        alert('Укажите тип объекта в каждом правиле');
        return false;
      }
      if (data.text_condition !== 'empty' && !String(data.pattern || '').trim()) {
        alert('Укажите текст для проверки или выберите «Пустое поле»');
        return false;
      }
      if (!data.actions || !data.actions.length) {
        alert('В каждом правиле нужно хотя бы одно действие');
        return false;
      }
    }
    return true;
  },

  async exportTagEditRulesFile() {
    const text = await API.exportTagEditRules();
    const blob = new Blob([text], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'tag-edit-rules.json';
    a.click();
    URL.revokeObjectURL(url);
  },

  async importTagEditRulesFile(file) {
    const text = await file.text();
    await API.importTagEditRules(text);
  },

  async saveTagEditRules() {
    const rows = document.querySelectorAll('.tag-edit-rule');
    for (let i = 0; i < rows.length; i++) {
      const row = rows[i];
      const data = { ...this.collectTagEditRuleData(row), sort_order: i };
      if (row.dataset.id) {
        await API.updateTagEditRule(row.dataset.id, data);
      } else {
        const created = await API.createTagEditRule(data);
        row.dataset.id = created.id;
      }
    }
  },

  async applyTagEditRules() {
    if (!this.solutionId || this.tagEditApplying) return;
    if (!this.validateTagEditRules()) return;

    const rows = document.querySelectorAll('.tag-edit-rule');
    const solutionName = document.getElementById('solutionName').textContent || 'текущее решение';
    const deleteRules = [...rows].filter(r => this.collectTagEditRuleData(r).actions?.some(a => a.action === 'delete')).length;
    let msg = `Применить ${rows.length} правил редактирования TAG к решению «${solutionName}»?\n\nПо условию отбираются объекты выбранного типа, затем выполняются все действия правила (поля объекта, характеристики, команды, защиты).`;
    if (deleteRules > 0) {
      msg += `\n\nВнимание: есть действия «Очистить» — выбранное поле станет пустым.`;
    }
    msg += '\n\nПродолжить?';
    if (!confirm(msg)) return;

    const overlay = document.getElementById('uploadOverlay');
    const overlayTitle = document.getElementById('uploadOverlayTitle');
    const overlayFile = document.getElementById('uploadOverlayFile');
    const overlayBar = document.getElementById('uploadOverlayBar');
    const overlayStatus = document.getElementById('uploadOverlayStatus');

    const showProgress = (title, status, pct) => {
      overlayTitle.textContent = title;
      overlayFile.textContent = '';
      overlayStatus.textContent = status;
      overlayBar.style.width = `${pct}%`;
      overlay.hidden = false;
    };

    this.tagEditApplying = true;
    document.body.style.cursor = 'wait';
    const tagBtn = document.getElementById('btnTagEditSettings');
    tagBtn.disabled = true;
    const extraBusy = [
      'btnSave', 'btnExport', 'btnExportObjects', 'btnDuplicateSettings',
      'btnComplianceSettings', 'btnDisplaySettings', 'btnHelp',
    ];
    extraBusy.forEach((id) => {
      const el = document.getElementById(id);
      if (el) el.disabled = true;
    });

    try {
      this.hideTagEditSettings();
      showProgress('Редактирование TAG…', 'Сохранение правил…', 0);

      await this.saveTagEditRules();
      showProgress('Редактирование TAG…', 'Запуск…', 0);

      const { job_id: jobId } = await API.applyTagEdit(this.solutionId);
      const job = await API.pollTagEditJob(this.solutionId, jobId, (progressJob) => {
        const total = progressJob.total_objects || 1;
        const pct = Math.round((progressJob.processed_objects / total) * 100);
        showProgress(
          'Редактирование TAG…',
          `Обработка… ${pct}% (${progressJob.processed_objects} из ${progressJob.total_objects}, изменено ${progressJob.changed_objects})`,
          pct,
        );
      });

      showProgress('Редактирование TAG…', 'Обновление данных…', 100);
      this.objectDetailsCache.clear();
      this.cachedSchemeId = null;
      const tree = await API.getTree(this.solutionId, { depth: 'skeleton' });
      this.rebuildSchemeProjectMap(tree);
      this.lastTree = tree;
      Explorer.render(tree);
      await this.refreshComplianceReport();
      if (this.currentSchemeId) {
        await this.openScheme(this.currentSchemeId, '');
      }
      if ((job.changed_objects || 0) > 0) {
        this.setDirty(true);
      }
    } catch (err) {
      const errMsg = String(err.message || '');
      if (errMsg.includes('duplicate scan already') || errMsg.includes('background duplicate scan')) {
        alert('Сейчас выполняется поиск дубликатов. Дождитесь окончания и повторите «Применить».');
      } else {
        alert('Ошибка редактирования TAG: ' + err.message);
      }
    } finally {
      overlay.hidden = true;
      this.tagEditApplying = false;
      document.body.style.cursor = '';
      if (this.solutionId) tagBtn.disabled = false;
      extraBusy.forEach((id) => {
        const el = document.getElementById(id);
        if (el) el.disabled = false;
      });
    }
  },

  hideTagEditSettings() {
    document.getElementById('tagEditModal').hidden = true;
  },

  async showExportObjects() {
    if (this.tagEditApplying) return;
    const openBtn = document.getElementById('btnExportObjects');
    const openBtnText = openBtn?.textContent || '';
    if (openBtn) {
      openBtn.disabled = true;
      openBtn.textContent = 'Загрузка…';
    }
    let settings;
    try {
      settings = await API.getDisplaySettings();
    } catch (err) {
      if (openBtn) {
        openBtn.disabled = false;
        openBtn.textContent = openBtnText;
      }
      alert('Не удалось открыть выгрузку: ' + err.message);
      return;
    }
    const settingsMap = {};
    for (const s of settings) settingsMap[s.template] = s;
    const templates = settings.map(s => s.template);
    const baseFields = [...DISPLAY_FIELD_OPTIONS, 'Template', ...DISPLAY_EXTRA_PROPS];
    const body = document.getElementById('exportObjectsBody');

    body.innerHTML = '<p class="modal-hint">Выберите тип объекта, параметры и порядок колонок. По умолчанию первыми идут Id и Tag.</p>';

    const templateSel = document.createElement('select');
    templateSel.className = 'input';
    templateSel.id = 'exportTemplate';
    const allOpt = document.createElement('option');
    allOpt.value = '';
    allOpt.textContent = '— Все типы —';
    templateSel.appendChild(allOpt);
    for (const t of templates) {
      const opt = document.createElement('option');
      opt.value = t;
      opt.textContent = t;
      templateSel.appendChild(opt);
    }
    body.appendChild(this.labeledBlock('Тип объекта', templateSel));

    const changedRow = document.createElement('div');
    changedRow.className = 'export-changed-row';
    const changedToggle = document.createElement('label');
    changedToggle.className = 'export-changed-toggle';
    const changedCb = document.createElement('input');
    changedCb.type = 'checkbox';
    changedCb.id = 'exportChangedOnly';
    changedToggle.appendChild(changedCb);
    const changedLabel = document.createElement('span');
    changedLabel.id = 'exportChangedLabel';
    changedLabel.textContent = 'Только изменённые объекты';
    changedToggle.appendChild(changedLabel);
    changedRow.appendChild(changedToggle);
    const ackBtn = document.createElement('button');
    ackBtn.type = 'button';
    ackBtn.className = 'btn btn--ghost export-changed-ack';
    ackBtn.id = 'exportAckChanged';
    ackBtn.textContent = 'Обнуление данных по изменённым объектам';
    ackBtn.disabled = true;
    changedRow.appendChild(ackBtn);
    body.appendChild(changedRow);

    const changedHint = document.createElement('div');
    changedHint.className = 'export-changed-hint';
    changedHint.id = 'exportChangedHint';
    body.appendChild(changedHint);

    const syncChangedAvailability = () => {
      if (changedCount === 0) {
        changedCb.checked = false;
        changedCb.disabled = true;
        ackBtn.disabled = true;
        changedHint.textContent = 'Нет изменённых объектов до обнуления счётчика';
        return;
      }
      changedCb.disabled = false;
      ackBtn.disabled = false;
      if (!changedCb.checked) {
        changedHint.textContent = '';
      }
    };

    const applyChangedCount = (count) => {
      changedCount = count;
      if (count === 0) {
        changedLabel.textContent = 'Только изменённые объекты';
      } else {
        changedLabel.textContent = `Только изменённые объекты (${count})`;
      }
      syncChangedAvailability();
    };

    let changedCount = 0;

    const checks = document.createElement('div');
    checks.className = 'display-settings__checks';
    checks.id = 'exportFieldsChecks';
    body.appendChild(this.labeledBlock('Параметры', checks));

    const orderList = document.createElement('ol');
    orderList.className = 'export-field-order';
    orderList.id = 'exportFieldOrder';
    body.appendChild(this.labeledBlock('Порядок колонок', orderList));

    const tagPresenceFallback = document.createElement('div');
    tagPresenceFallback.className = 'export-tag-presence export-tag-presence-fallback';
    tagPresenceFallback.id = 'exportTagPresenceFallback';
    tagPresenceFallback.hidden = true;
    const tagPresenceFallbackLabel = document.createElement('span');
    tagPresenceFallbackLabel.textContent = 'Фильтр Tag:';
    tagPresenceFallback.appendChild(tagPresenceFallbackLabel);
    body.appendChild(tagPresenceFallback);

    const tagPresenceHint = document.createElement('div');
    tagPresenceHint.className = 'export-tag-presence-hint';
    tagPresenceHint.textContent = 'Фильтр по Tag: «Только непустые» исключает объекты с очищенным Tag (после «Редактирование TAG» → очистить).';
    body.appendChild(tagPresenceHint);

    let tagPresence = 'non_empty';
    const createTagPresenceSelect = () => {
      const sel = document.createElement('select');
      sel.className = 'input export-tag-presence__select';
      sel.id = 'exportTagPresence';
      [
        ['all', 'Все'],
        ['non_empty', 'Только непустые'],
        ['empty', 'Только пустые'],
      ].forEach(([v, l]) => {
        const opt = document.createElement('option');
        opt.value = v;
        opt.textContent = l;
        if (v === tagPresence) opt.selected = true;
        sel.appendChild(opt);
      });
      sel.addEventListener('change', () => {
        tagPresence = sel.value;
        const other = document.querySelectorAll('#exportTagPresence');
        other.forEach((el) => {
          if (el !== sel) el.value = tagPresence;
        });
      });
      return sel;
    };

    const fieldOrder = ['Id', 'Tag'];
    let knownFields = new Set(baseFields);

    const syncOrderList = () => {
      orderList.innerHTML = '';
      const tagInOrder = fieldOrder.includes('Tag');
      tagPresenceFallback.hidden = tagInOrder;
      tagPresenceFallback.querySelectorAll('select').forEach((el) => el.remove());
      if (!tagInOrder) {
        tagPresenceFallback.appendChild(createTagPresenceSelect());
      }
      fieldOrder.forEach((field, idx) => {
        const li = document.createElement('li');
        li.className = 'export-field-order__item';
        li.dataset.field = field;
        const name = document.createElement('span');
        name.className = 'export-field-order__name';
        name.textContent = field;
        li.appendChild(name);
        if (field === 'Tag') {
          const presenceWrap = document.createElement('span');
          presenceWrap.className = 'export-tag-presence';
          presenceWrap.appendChild(createTagPresenceSelect());
          li.appendChild(presenceWrap);
        }
        const controls = document.createElement('span');
        controls.className = 'export-field-order__controls';
        const upBtn = document.createElement('button');
        upBtn.type = 'button';
        upBtn.className = 'btn btn--ghost btn--icon';
        upBtn.textContent = '↑';
        upBtn.setAttribute('aria-label', `Переместить колонку ${field} вверх`);
        upBtn.disabled = idx === 0;
        upBtn.addEventListener('click', () => {
          if (idx > 0) {
            [fieldOrder[idx - 1], fieldOrder[idx]] = [fieldOrder[idx], fieldOrder[idx - 1]];
            syncOrderList();
          }
        });
        const downBtn = document.createElement('button');
        downBtn.type = 'button';
        downBtn.className = 'btn btn--ghost btn--icon';
        downBtn.textContent = '↓';
        downBtn.setAttribute('aria-label', `Переместить колонку ${field} вниз`);
        downBtn.disabled = idx === fieldOrder.length - 1;
        downBtn.addEventListener('click', () => {
          if (idx < fieldOrder.length - 1) {
            [fieldOrder[idx + 1], fieldOrder[idx]] = [fieldOrder[idx], fieldOrder[idx + 1]];
            syncOrderList();
          }
        });
        controls.appendChild(upBtn);
        controls.appendChild(downBtn);
        li.appendChild(controls);
        orderList.appendChild(li);
      });
    };

    const toggleField = (field, checked) => {
      const idx = fieldOrder.indexOf(field);
      if (checked && idx < 0) fieldOrder.push(field);
      if (!checked && idx >= 0) fieldOrder.splice(idx, 1);
      syncOrderList();
    };

    const rebuildFieldChecks = (allFields, selectedFields) => {
      checks.innerHTML = '';
      knownFields = new Set(allFields);
      fieldOrder.length = 0;
      for (const f of selectedFields) {
        if (allFields.includes(f)) fieldOrder.push(f);
      }
      if (fieldOrder.length === 0) {
        for (const f of ['Id', 'Tag']) {
          if (allFields.includes(f)) fieldOrder.push(f);
        }
      }
      for (const opt of allFields) {
        const label = document.createElement('label');
        const cb = document.createElement('input');
        cb.type = 'checkbox';
        cb.value = opt;
        cb.checked = fieldOrder.includes(opt);
        cb.addEventListener('change', () => toggleField(opt, cb.checked));
        label.appendChild(cb);
        label.appendChild(document.createTextNode(opt));
        checks.appendChild(label);
      }
      syncOrderList();
    };

    const loadFieldsForTemplate = async (template) => {
      let allFields = baseFields;
      if (template && this.solutionId) {
        try {
          allFields = await API.getTemplateParameters(this.solutionId, template, this.sessionId);
        } catch (_) {}
      }
      if (!allFields.includes('Template')) {
        allFields = [...allFields, 'Template'];
      }
      let selectedFields = template ? ['Id', 'Tag'] : ['Id', 'Tag', 'Template'];
      if (template && settingsMap[template]) {
        try {
          const vf = JSON.parse(settingsMap[template].visible_fields || '[]');
          if (vf.length > 0) selectedFields = vf;
        } catch (_) {}
      }
      rebuildFieldChecks(allFields, selectedFields);
    };

    templateSel.addEventListener('change', async () => {
      await loadFieldsForTemplate(templateSel.value);
    });
    changedCb.addEventListener('change', () => {
      if (changedCount === 0) {
        changedHint.textContent = 'Нет изменённых объектов до обнуления счётчика';
      } else {
        changedHint.textContent = '';
      }
    });

    ackBtn.addEventListener('click', async () => {
      if (!confirm('Обнуление данных по изменённым объектам? Список и счётчик изменённых объектов будут очищены. Сохранение решения это не делает. Сами объекты и правки в решении не удаляются.')) {
        return;
      }
      ackBtn.disabled = true;
      try {
        const { count } = await API.ackChangedObjects(this.solutionId, this.sessionId);
        applyChangedCount(count);
      } catch (err) {
        alert('Не удалось обнулить счётчик: ' + err.message);
        ackBtn.disabled = changedCount === 0;
      }
    });

    if (this.solutionId) {
      try {
        const { count } = await API.getChangedObjectsCount(this.solutionId, this.sessionId);
        applyChangedCount(count);
      } catch (_) {
        changedCb.disabled = true;
        ackBtn.disabled = true;
        changedHint.textContent = 'Не удалось загрузить список изменённых объектов';
      }
    }

    await loadFieldsForTemplate('');

    const extra = document.createElement('input');
    extra.className = 'props-field__input';
    extra.id = 'exportExtraFields';
    extra.placeholder = 'Доп. параметры через запятую';
    extra.addEventListener('change', () => {
      extra.value.split(',').map(s => s.trim()).filter(Boolean).forEach(f => {
        if (!knownFields.has(f) && !fieldOrder.includes(f)) {
          knownFields.add(f);
          fieldOrder.push(f);
          const label = document.createElement('label');
          const cb = document.createElement('input');
          cb.type = 'checkbox';
          cb.value = f;
          cb.checked = true;
          cb.addEventListener('change', () => toggleField(f, cb.checked));
          label.appendChild(cb);
          label.appendChild(document.createTextNode(f));
          checks.appendChild(label);
        }
      });
      syncOrderList();
    });
    body.appendChild(this.labeledBlock('Доп. параметры', extra));

    const formatSel = document.createElement('select');
    formatSel.className = 'input';
    formatSel.id = 'exportFormat';
    [['csv', 'CSV'], ['json', 'JSON']].forEach(([v, l]) => {
      const opt = document.createElement('option');
      opt.value = v;
      opt.textContent = l;
      formatSel.appendChild(opt);
    });
    body.appendChild(this.labeledBlock('Формат', formatSel));

    const btnRow = document.createElement('div');
    btnRow.className = 'modal-actions';
    const exportBtn = document.createElement('button');
    exportBtn.className = 'btn btn--primary';
    exportBtn.id = 'exportObjectsSubmit';
    exportBtn.textContent = 'Выгрузить';
    exportBtn.addEventListener('click', () => this.doExportObjects(fieldOrder, exportBtn, () => tagPresence));
    btnRow.appendChild(exportBtn);
    const cancelBtn = document.createElement('button');
    cancelBtn.className = 'btn btn--ghost';
    cancelBtn.textContent = 'Закрыть';
    cancelBtn.addEventListener('click', () => this.hideExportObjects());
    btnRow.appendChild(cancelBtn);
    body.appendChild(btnRow);

    document.getElementById('exportObjectsModal').hidden = false;
    if (openBtn) {
      openBtn.disabled = false;
      openBtn.textContent = openBtnText;
    }
  },

  labeledBlock(label, el) {
    const wrap = document.createElement('div');
    wrap.className = 'export-field-block';
    const lbl = document.createElement('div');
    lbl.className = 'props-field__label';
    lbl.textContent = label;
    wrap.appendChild(lbl);
    wrap.appendChild(el);
    return wrap;
  },

  async doExportObjects(fieldOrder, exportBtn, getTagPresence) {
    const fields = fieldOrder ? [...fieldOrder] : [];
    const extra = document.getElementById('exportExtraFields')?.value;
    if (extra) {
      extra.split(',').map(s => s.trim()).filter(Boolean).forEach(f => {
        if (!fields.includes(f)) fields.push(f);
      });
    }
    if (fields.length === 0) {
      alert('Выберите хотя бы один параметр');
      return;
    }
    const template = document.getElementById('exportTemplate').value;
    const format = document.getElementById('exportFormat').value;
    const changedOnly = document.getElementById('exportChangedOnly')?.checked || false;
    const tagPresenceSel = document.getElementById('exportTagPresence');
    const tagPresence = (typeof getTagPresence === 'function' ? getTagPresence() : null)
      || tagPresenceSel?.value
      || 'non_empty';
    const btn = exportBtn || document.getElementById('exportObjectsSubmit');
    const originalText = btn?.textContent || 'Выгрузить';
    if (btn) {
      btn.disabled = true;
      btn.textContent = 'Выгрузка…';
    }
    try {
      const blob = await API.exportObjects(this.solutionId, {
        template,
        fields,
        format,
        changed_only: changedOnly,
        session_id: this.sessionId || '',
        tag_presence: tagPresence,
      });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = format === 'json' ? 'objects.json' : 'objects.csv';
      a.click();
      URL.revokeObjectURL(url);
      this.hideExportObjects();
    } catch (err) {
      alert('Ошибка выгрузки: ' + err.message);
    } finally {
      if (btn) {
        btn.disabled = false;
        btn.textContent = originalText;
      }
    }
  },

  showHelp() {
    if (this.tagEditApplying) return;
    const body = document.getElementById('helpBody');
    body.innerHTML = `
      <section class="help-section">
        <h4>Начало работы</h4>
        <p>При открытии сразу главное окно — без полноэкранного выбора пользователя. После сброса БД снова выберите или создайте пользователя через кнопку <strong>👤 Выбрать…</strong> слева вверху: меню со списком и «+ Создать нового» (форма с отменой / Escape). Без выбранного пользователя загрузка и выбор решений недоступны; клик по <strong>+</strong> открывает то же меню. Затем нажмите <strong>+ Добавить решение</strong> и выберите файл Solution.xml или .rar с Solution.xml внутри. Во время передачи файла оверлей показывает процент загрузки; после приёма сервером — фазы обработки (распаковка, разбор XML, сохранение схем, построение индексов, подготовка списков параметров) с процентом и оценкой оставшегося времени. Если импорт ещё идёт, повторный клик по решению в списке снова открывает оверлей прогресса. Готовое решение появится в списке и в обозревателе слева — откройте схему кликом по узлу дерева. Списки параметров для проверок строятся в каталог при импорте (или при первом запросе) и дальше открываются быстро. Сохранённый пользователь подставляется тихо при следующем запуске.</p>
      </section>
      <section class="help-section">
        <h4>Обозреватель и поиск</h4>
        <ul class="help-list">
          <li>Дерево слева показывает проекты и схемы загруженного решения. Клик по схеме открывает её на холсте.</li>
          <li>Кнопка <strong>⌕</strong> в заголовке обозревателя открывает или скрывает панель поиска. Состояние панели запоминается.</li>
          <li>В поиске можно искать по имени, Tag, Id и типу объекта. Id в таблице — технологический Id, как во вкладке «Основные» (если пусто — «—»). Введите запрос (не короче 2 символов; короткий запрос ищет только по Id графического или технологического объекта) — таблица покажет тип, название, Id, project и схему без повторов.</li>
          <li>Клик по строке открывает нужную схему и выделяет объект; если схема уже открыта, она не перезагружается. Если объект не виден в кадре — схема сдвигается к нему. На две секунды вокруг объекта появляются четыре стрелки (сверху, снизу и по бокам); повторный клик по строке показывает их снова. Клик по объекту на самой схеме стрелок не запускает.</li>
          <li>Индекс поиска обновляется при последнем сохранении или импорте решения. Несохранённые правки на открытой схеме (draft) в поиске не отражаются.</li>
        </ul>
      </section>
      <section class="help-section">
        <h4>Панель инструментов</h4>
        <ul class="help-list">
          <li><strong>👤 Выбрать…</strong> — выбор или создание пользователя (слева вверху). Настройки и решения привязаны к пользователю. Кнопка всегда доступна; без пользователя текст «Выбрать…», после выбора — укороченный логин.</li>
          <li><strong>⚙ Отображение</strong> — для каждого типа объекта: режим знака (детальный символ, буква, линия для труб), показ на схеме, смещение от угла, поля подписи (Name, Tag, Id и др.), для труб — толщина линии. Общий переключатель «Показывать подписи на схеме».</li>
          <li><strong>⚠ Проверки</strong> — порядок полей: Тип объекта → Область → Параметр → Атрибут параметра → Условие текста → Текст → Пустое поле → Индикация → Цвет → Период. Область задаёт список Параметра (Основные, Характеристики, Команды, Защиты); при нарушении объект подсвечивается на схеме. Условие текста: «Должен / Не должен содержать» — по вхождению; «Строгое соответствие» — значение должно быть равно тексту; «Не должен совпадать строго» — нарушение, если значение ровно равно тексту (подсветка объекта). Списки параметров берутся из каталога решения и на повторных открытиях загружаются быстро. Если список не загрузился, нажмите <strong>⟳ Повторить</strong> рядом с полем «Параметр». Надпись «Нет вариантов» означает, что для выбранного типа объекта и области список пуст.</li>
          <li><strong>🏷 Редактирование TAG</strong> — правила сверху вниз. После типа объекта выбираете, что проверять (как в «Проверках»: основные характеристики, характеристики, команды, защиты и конкретное поле). Условие: содержит / начинается / заканчивается / «Пустое поле» — тогда «Текст» скрыт. Галки «Непустой Tag в блоках»: объект в выборке, только если во всех отмеченных блоках есть непустой Tag. Затем одно или несколько действий: какое поле менять, «Очистить» (без «Заменить на») или «Заменить» («Заменить на» и способ: найденное / вся строка / первое / все / последнее совпадение). Экспорт и импорт — читаемый JSON правил. «Применить» держит оверлей до конца. Изменения — «● Несохранено» и «Выгрузить объекты → Только изменённые».</li>
          <li><strong>Выгрузить объекты</strong> — таблица CSV/JSON по выбранным полям; можно выгрузить только изменённые объекты и настроить порядок колонок. В параметрах можно включить колонку <strong>Template</strong> (тип объекта); select «Тип объекта» сверху — фильтр строк, его можно сочетать с «только изменённые» (AND). В строке Tag (или отдельно, если Tag снят) — фильтр: Все / Только непустые / Только пустые (по умолчанию — только непустые). Рядом с «Только изменённые объекты» — кнопка <strong>Обнуление данных по изменённым объектам</strong>: скрывает учтённые изменения из счётчика и фильтра «только изменённые», не удаляя правки в решении. Сохранение решения список и счётчик не сбрасывает — сброс только этой кнопкой.</li>
          <li><strong>Дубликаты</strong> — поиск одинаковых Tag по основным параметрам, характеристикам, командам и защитам. Клик по строке в таблице, на вкладке инспектора или в плавающей панели выделяет объект на схеме и на две секунды показывает четыре стрелки вокруг него. В таблице, на вкладке «Дубликаты» инспектора и в плавающей панели колонка Id — технологический Id объекта (как во вкладке «Основные»); если пусто — «—»; навигация по строке использует системный ключ графического объекта. В настройках: области поиска (Основные / Характеристики / Команды / Защиты — отмеченные участвуют в поиске) и исключения Tag (несколько условий через ИЛИ; подходящие Tag не участвуют в поиске). Типы исключения: «Содержит символ» — символ встречается в Tag; «Содержит текст» — фрагмент встречается в Tag; «Содержит один символ» — Tag целиком равен одному заданному символу (например «.» исключает только Tag «.», а не «тест.это.тест»); «Первый символ» — первый символ Tag равен заданному. «Начать поиск» — полный принудительный проход с оверлеем; на больших решениях может занять заметное время — дождитесь окончания по оверлею. «Поиск в фоне» — щадящий фоновый сканер без оверлея: сначала весь проект, затем только изменения; найденные дубликаты появляются в таблице по мере сканирования. Обычный и фоновый поиск взаимно блокируются. Смена областей поиска и исключений Tag сохраняется при запуске поиска или включении фона и требует полного пересчёта. После поиска блок «Выгрузка дубликатов Tag»: формат CSV (разделитель «;» и строка заголовка), TXT (колонки через табуляцию) или XML (корень duplicates, строки row, выбранные колонки — теги); отметьте колонки отчёта (Tag, Id, Имя, Тип, Область, Параметр, Схема; по умолчанию все включены) и нажмите <strong>Выгрузить</strong> — скачивается файл UTF-8 со всеми строками текущей таблицы. Без результатов поиска кнопка недоступна. Результаты поиска сохраняются и после обновления страницы снова видны на схемах и в обозревателе; полный пересчёт — только повторным запуском поиска (или фоном, если включён).</li>
          <li><strong>Сохранить</strong> — записать правки из буфера сессии в решение. Открытая схема после успеха остаётся открытой (перечитывается с сервера). Во время сохранения интерфейс блокируется оверлеем с процентом и оценкой оставшегося времени; успех показывается на оверлее без отдельного окна, при ошибке правки остаются несохранёнными.</li>
          <li><strong>Выгрузить XML</strong> — полный Solution.xml с учётом несохранённых рабочих изменений.</li>
        </ul>
      </section>
      <section class="help-section">
        <h4>Схема</h4>
        <ul class="help-list">
          <li>Режим «Детальный символ» — картинки объектов схемы (как в АРМ Инструктора). Угол и отражения из решения поворачивают картинку вокруг якоря; у труб таких картинок нет — только линия. Если для типа нет файла картинки, рисуется контурный знак.</li>
          <li>Режим «Буква» — только поворот по углу, без зеркала.</li>
          <li>У оборудования с несколькими точками на схеме рисуется тонкая ломаная между ними.</li>
          <li><strong>Угол</strong> во вкладке «Основные» сразу меняет ориентацию знака на схеме.</li>
          <li>Смещение знака по типу — в <strong>⚙ Отображение</strong>, от левого верхнего угла, ползунками. Оверлея сверки с оригиналом нет.</li>
          <li>Подписи на схеме только из <strong>⚙ Отображение</strong>: какие поля выводить и включён ли «Показывать подписи на схеме».</li>
          <li><strong>+ / − / Fit</strong> — масштаб и обзор всей схемы.</li>
          <li><strong>🔓 Движение</strong> — отдельный режим холста: при активном режиме можно выбирать и перетаскивать трубы, мосты и перемычки; при блокировке режима — только оборудование. Не путать с блокировкой объекта в решении (<code>IsLocked</code>).</li>
          <li>Клик по объекту открывает панель свойств справа (Основные, Характеристики, Команды, Защиты). Во вкладках можно править свойства и сохранять как обычно — в том числе у объектов, заблокированных на перемещение в решении. У таких объектов на схеме перетаскивание недоступно; во вкладке «Основные» поля X, Y и Angle только для просмотра. Во вкладке «Основные» после типа показывается Id технологического объекта из solution.xml; id графического объекта не показывается. Подписи на схеме и выгрузка по полю Id используют тот же технологический Id (если пусто — поле не заполняется / «—»). Правки полей живут в буфере сессии: при переключении схем они не пропадают; «Сохранить» записывает их в решение.</li>
          <li>Строка в Поиске, в таблице Дубликатов, на вкладке «Дубликаты» инспектора или в плавающей панели дубликатов выделяет объект и на две секунды ставит четыре стрелки вокруг него. Если объекта нет в кадре, схема сначала сдвигается к нему.</li>
          <li>Перетаскивание фона (зажать и сдвинуть) — панорамирование, выбранный объект остаётся выбранным. Короткий клик по пустому месту схемы снимает выбор. Колёсико мыши — масштаб к курсору.</li>
          <li><strong>Drop из WinCC OA</strong> — перетащите текст datapoint из WinCC OA 3.17 на объект схемы: в Tag запишется имя DP. На editable поле инспектора — значение вставится в это поле. Формат: <code>prefix:DP.rest</code> → берётся <code>DP</code> (часть после первого <code>:</code> до первой <code>.</code>). Неверный формат игнорируется.</li>
        </ul>
      </section>
      <section class="help-section">
        <h4>Изменения и выгрузка</h4>
        <p>После правки объекта на схеме появляется метка <strong>● Несохранено</strong>; она держится при смене схем, пока правки не записаны <strong>Сохранить</strong>. Пока идёт «Редактирование TAG», экран заблокирован оверлеем. Массовое «Редактирование TAG» ставит «● Несохранено» только если были изменения; такие объекты видны в <strong>Выгрузить объекты → Только изменённые</strong>. <strong>Сохранить</strong> не сбрасывает список изменённых; сброс — кнопкой обнуления. Фильтр Tag «Только непустые» исключает объекты с очищенным Tag.</p>
      </section>`;
    document.getElementById('helpModal').hidden = false;
  },

  hideHelp() {
    document.getElementById('helpModal').hidden = true;
  },

  hideExportObjects() {
    document.getElementById('exportObjectsModal').hidden = true;
  },

  initTheme() {
    const saved = localStorage.getItem('theme') || 'dark';
    document.documentElement.setAttribute('data-theme', saved);
    this.updateThemeButton(saved);
  },

  toggleTheme() {
    const current = document.documentElement.getAttribute('data-theme') || 'dark';
    const next = current === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem('theme', next);
    this.updateThemeButton(next);
    if (this.currentScheme) {
      const svg = document.getElementById('schemeSvg');
      Renderer.renderSchemeAsync(svg, this.currentScheme, (obj) => this.selectObject(obj)).then(() => this.applyTransform());
    }
  },

  updateThemeButton(theme) {
    const btn = document.getElementById('btnTheme');
    btn.textContent = theme === 'dark' ? '🌙' : '☀';
  },

  async deleteCurrentSolution() {
    if (!this.solutionId) return;
    const name = document.getElementById('solutionName').textContent || 'решение';
    if (!confirm(`Удалить решение «${name}» из базы данных?`)) return;
    try {
      await API.deleteSolution(this.solutionId);
      localStorage.removeItem(UserContext.lastSolutionKey());
      this.resetAppState();
      await this.loadSolutionList();
    } catch (err) {
      alert('Ошибка удаления: ' + err.message);
    }
  },

  async clearAllSolutions() {
    if (!this.requireUser()) return;
    const userName = UserContext.displayName || 'текущего пользователя';
    if (!confirm(`Удалить ВСЕ решения пользователя «${userName}»? Это действие необратимо.`)) return;
    try {
      const result = await API.deleteAllSolutions();
      localStorage.removeItem(UserContext.lastSolutionKey());
      this.resetAppState();
      await this.loadSolutionList();
      alert(`Удалено решений: ${result.count || 0}`);
    } catch (err) {
      alert('Ошибка очистки: ' + err.message);
    }
  },

  setupPanelResizers() {
    const explorer = document.getElementById('explorer');
    const properties = document.getElementById('properties');
    const resizerLeft = document.getElementById('resizerLeft');
    const resizerRight = document.getElementById('resizerRight');

    const saved = JSON.parse(localStorage.getItem('panelWidths') || '{}');
    if (saved.explorer) document.documentElement.style.setProperty('--explorer-width', saved.explorer + 'px');
    if (saved.properties) document.documentElement.style.setProperty('--properties-width', saved.properties + 'px');

    const saveWidths = () => {
      localStorage.setItem('panelWidths', JSON.stringify({
        explorer: parseInt(getComputedStyle(document.documentElement).getPropertyValue('--explorer-width')),
        properties: parseInt(getComputedStyle(document.documentElement).getPropertyValue('--properties-width')),
      }));
    };

    const setupDrag = (resizer, panel, side) => {
      let startX, startW;
      resizer.addEventListener('mousedown', (e) => {
        e.preventDefault();
        startX = e.clientX;
        startW = panel.getBoundingClientRect().width;
        const onMove = (ev) => {
          const dx = side === 'left' ? ev.clientX - startX : startX - ev.clientX;
          const newW = Math.max(200, Math.min(window.innerWidth * 0.45, startW + dx));
          const varName = side === 'left' ? '--explorer-width' : '--properties-width';
          document.documentElement.style.setProperty(varName, newW + 'px');
        };
        const onUp = () => {
          window.removeEventListener('mousemove', onMove);
          window.removeEventListener('mouseup', onUp);
          saveWidths();
        };
        window.addEventListener('mousemove', onMove);
        window.addEventListener('mouseup', onUp);
      });
      resizer.addEventListener('dblclick', () => {
        const def = side === 'left' ? 280 : 320;
        const varName = side === 'left' ? '--explorer-width' : '--properties-width';
        document.documentElement.style.setProperty(varName, def + 'px');
        saveWidths();
      });
    };

    setupDrag(resizerLeft, explorer, 'left');
    setupDrag(resizerRight, properties, 'right');
  },

  resetAppState() {
    this.clearDuplicateScanState();
    this.solutionId = null;
    this.invalidateComplianceParamsCache();
    this.sessionId = null;
    this.currentSchemeId = null;
    this.currentScheme = null;
    document.getElementById('schemeSvg').hidden = true;
    document.getElementById('schemeSvg').innerHTML = '';
    document.getElementById('canvasPlaceholder').style.display = '';
    document.getElementById('canvasToolbar').hidden = true;
    document.getElementById('canvasStatusbar').hidden = true;
    document.getElementById('btnSave').disabled = true;
    document.getElementById('btnExport').disabled = true;
    document.getElementById('btnExportObjects').disabled = true;
    document.getElementById('btnTagEditSettings').disabled = true;
    document.getElementById('btnDuplicateSettings').disabled = true;
    document.getElementById('btnDeleteSolution').hidden = true;
    document.getElementById('solutionName').textContent = '';
    this.closeSolutionMenu();
    this.updateSolutionMenuButton();
    document.getElementById('treeContainer').innerHTML = `<p class="props-empty">${this.emptyTreeMessage()}</p>`;
    document.getElementById('app').classList.add('app--empty');
    document.getElementById('app').classList.remove('app--loaded', 'app--scheme-open', 'app--object-selected');
    this.selectedObjectId = null;
    if (this.objectSettings.open) this.forceCloseObjectSettingsPanel();
    Inspector.showEmpty('Решение не выбрано');
    this.setDirty(false);
    this.updateSolutionSearchEnabled();
  },
};

document.addEventListener('DOMContentLoaded', () => App.init());
