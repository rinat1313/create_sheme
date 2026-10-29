const LocatorGeom = {
  bboxIntersectsViewBox(box, view) {
    if (!box || !view) return false;
    return !(
      box.x + box.w <= view.x
      || box.x >= view.x + view.w
      || box.y + box.h <= view.y
      || box.y >= view.y + view.h
    );
  },

  chevronSize(w, h) {
    const side = Math.min(Math.max(w, 0), Math.max(h, 0));
    const fromMax = Math.max(w, h) * 0.2;
    const raw = Math.max(side, fromMax);
    return Math.max(12, Math.min(raw, 48));
  },

  chevronPolygons(box) {
    if (!box) return [];
    const w = Math.max(box.w, 0);
    const h = Math.max(box.h, 0);
    const size = this.chevronSize(w, h);
    const gap = Math.max(4, size * 0.12);
    const cx = box.x + w / 2;
    const cy = box.y + h / 2;
    const half = size / 2;
    return [
      {
        side: 'top',
        points: [
          [cx, box.y - gap],
          [cx - half, box.y - gap - size],
          [cx + half, box.y - gap - size],
        ],
      },
      {
        side: 'bottom',
        points: [
          [cx, box.y + h + gap],
          [cx - half, box.y + h + gap + size],
          [cx + half, box.y + h + gap + size],
        ],
      },
      {
        side: 'left',
        points: [
          [box.x - gap, cy],
          [box.x - gap - size, cy - half],
          [box.x - gap - size, cy + half],
        ],
      },
      {
        side: 'right',
        points: [
          [box.x + w + gap, cy],
          [box.x + w + gap + size, cy - half],
          [box.x + w + gap + size, cy + half],
        ],
      },
    ];
  },
};

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { LocatorGeom };
}
