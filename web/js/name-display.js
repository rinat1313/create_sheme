const NameDisplay = {
  MODES: ['user', 'tech', 'both'],
  MODE_LABELS: {
    user: 'Пользовательское',
    tech: 'Технологическое',
    both: 'Оба',
  },
  mode: 'user',

  normalize(mode) {
    return this.MODES.includes(mode) ? mode : 'user';
  },

  cycle(mode) {
    const current = this.normalize(mode);
    const idx = this.MODES.indexOf(current);
    return this.MODES[(idx + 1) % this.MODES.length];
  },

  modeLabel(mode) {
    const key = this.normalize(mode);
    return this.MODE_LABELS[key] || this.MODE_LABELS.user;
  },

  techName(entity) {
    if (entity == null) return '';
    if (typeof entity === 'string') return entity;
    return String(entity.Template || entity.template || entity.Name || entity.name || '').trim();
  },

  userName(entity) {
    if (entity == null) return '';
    if (typeof entity === 'string') return entity;
    return String(entity.Description || entity.description || entity.Name || entity.name || '').trim();
  },

  format(entity, mode) {
    const m = this.normalize(mode || this.mode);
    const tech = this.techName(entity);
    const user = this.userName(entity);
    if (m === 'tech') return tech || user;
    if (m === 'user') return user || tech;
    if (tech && user && tech !== user) return `${tech} — ${user}`;
    return user || tech;
  },

  itemValue(item) {
    if (item == null) return '';
    if (typeof item === 'string') return item;
    return String(item.name || item.Name || item.template || item.Template || '').trim();
  },

  itemEntity(item) {
    if (item == null) return {};
    if (typeof item === 'string') return { Template: item, Name: item };
    return {
      Template: item.template || item.Template || item.name || item.Name || '',
      Name: item.name || item.Name || '',
      Description: item.description || item.Description || '',
    };
  },

  formatItem(item, mode) {
    return this.format(this.itemEntity(item), mode);
  },

  names(items) {
    if (!Array.isArray(items)) return [];
    return items.map((item) => this.itemValue(item)).filter(Boolean);
  },
};
