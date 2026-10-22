const Symbols = {
  NS: 'http://www.w3.org/2000/svg',

  el(tag, attrs) {
    const node = document.createElementNS(this.NS, tag);
    if (attrs) {
      for (const [k, v] of Object.entries(attrs)) {
        if (v != null) node.setAttribute(k, String(v));
      }
    }
    return node;
  },

  scaleFn(baseScale) {
    return (n) => n * baseScale;
  },

  draw(g, template, anchor, ctx) {
    const s = ctx.scale;
    const map = {
      Valve: () => this.drawGateValve(g, anchor, s),
      BackValve: () => this.drawCheckValve(g, anchor, s),
      ReliefValve: () => this.drawReliefValve(g, anchor, s),
      PressureWaveSmoothingValve: () => this.drawGateValve(g, anchor, s, true),
      PipelinePump: () => this.drawCentrifugalPump(g, anchor, s),
      LeakPump: () => this.drawCentrifugalPump(g, anchor, s, 'LP'),
      BoosterPump: () => this.drawCentrifugalPump(g, anchor, s, 'BP'),
      Tank: () => this.drawTank(g, anchor, s, ctx),
      TankBattery: () => this.drawTankBattery(g, anchor, s),
      TankDump: () => this.drawTank(g, anchor, s, ctx, true),
      Enter: () => this.drawFlowPort(g, anchor, s, 'in'),
      Exit: () => this.drawFlowPort(g, anchor, s, 'out'),
      Filter: () => this.drawFilter(g, anchor, s),
      PumpStation: () => this.drawPumpStation(g, anchor, s),
      CompositePumpStation: () => this.drawPumpStation(g, anchor, s, true),
      ShopMainstream: () => this.drawPumpStation(g, anchor, s, true),
      Sensor: () => this.drawSensor(g, anchor, s),
      UnitMeasureQuality: () => this.drawInstrument(g, anchor, s, 'Q'),
      PressureControlUnit: () => this.drawController(g, anchor, s),
      LeakDetectionSystem: () => this.drawLeakDetector(g, anchor, s),
      CleanDiagnSystemCamera: () => this.drawCamera(g, anchor, s),
      CleanDiagnSystemCameraSensor: () => this.drawSensorFlag(g, anchor, s),
      CleanDiagnSystemCameraPass: () => this.drawSensorFlag(g, anchor, s, true),
      LeaseAutomaticCustodyTransfer: () => this.drawCustodyTransfer(g, anchor, s),
      LinearPartOfTrunkPipeline: () => this.drawTrunkSection(g, anchor, s),
      TechnologicalArea: () => this.drawTechArea(g, anchor, s),
      CompositeGeneralObject: () => this.drawComposite(g, anchor, s),
      PipelineTieIn: () => this.drawTieIn(g, anchor, s),
      PointOilHeating: () => this.drawHeater(g, anchor, s),
      AreaControlCenter: () => this.drawControlCenter(g, anchor, s),
    };
    const fn = map[template];
    if (fn) {
      fn();
      return;
    }
    this.drawGeneric(g, anchor, s, template);
  },

  getBounds(template, anchor, ctx) {
    const s = ctx.scale;
    const half = (n) => s(n);
    const presets = {
      Valve: { w: half(36), h: half(44) },
      BackValve: { w: half(32), h: half(32) },
      ReliefValve: { w: half(32), h: half(40) },
      PressureWaveSmoothingValve: { w: half(36), h: half(44) },
      PipelinePump: { w: half(44), h: half(44) },
      LeakPump: { w: half(44), h: half(44) },
      BoosterPump: { w: half(44), h: half(44) },
      Tank: { w: half(60), h: half(100) },
      TankBattery: { w: half(80), h: half(70) },
      TankDump: { w: half(60), h: half(100) },
      Enter: { w: half(36), h: half(36) },
      Exit: { w: half(36), h: half(36) },
      Filter: { w: half(40), h: half(36) },
      PumpStation: { w: half(72), h: half(52) },
      CompositePumpStation: { w: half(72), h: half(52) },
      ShopMainstream: { w: half(72), h: half(52) },
      Sensor: { w: half(14), h: half(18) },
      UnitMeasureQuality: { w: half(32), h: half(32) },
      PressureControlUnit: { w: half(44), h: half(36) },
      LeakDetectionSystem: { w: half(40), h: half(40) },
      CleanDiagnSystemCamera: { w: half(40), h: half(32) },
      CleanDiagnSystemCameraSensor: { w: half(36), h: half(28) },
      CleanDiagnSystemCameraPass: { w: half(36), h: half(28) },
      LeaseAutomaticCustodyTransfer: { w: half(44), h: half(36) },
      LinearPartOfTrunkPipeline: { w: half(48), h: half(20) },
      TechnologicalArea: { w: half(64), h: half(48) },
      CompositeGeneralObject: { w: half(52), h: half(40) },
      PipelineTieIn: { w: half(36), h: half(36) },
      PointOilHeating: { w: half(40), h: half(40) },
      AreaControlCenter: { w: half(56), h: half(44) },
    };
    const p = presets[template] || { w: half(40), h: half(40) };
    return { x: anchor.x - p.w / 2, y: anchor.y - p.h / 2, width: p.w, height: p.h };
  },

  drawGateValve(g, anchor, s, damped) {
    const bw = s(16);
    const stem = s(12);
    const x = anchor.x;
    const y = anchor.y;
    g.appendChild(this.el('polygon', {
      points: `${x - bw},${y - bw} ${x},${y} ${x - bw},${y + bw}`,
      fill: 'var(--equipment-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 1.5,
      class: 'sym-valve__body',
    }));
    g.appendChild(this.el('polygon', {
      points: `${x + bw},${y - bw} ${x},${y} ${x + bw},${y + bw}`,
      fill: 'var(--equipment-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 1.5,
      class: 'sym-valve__body',
    }));
    g.appendChild(this.el('line', {
      x1: x, y1: y - bw, x2: x, y2: y - bw - stem,
      stroke: 'var(--equipment-stroke)', 'stroke-width': 2, class: 'sym-valve__stem',
    }));
    if (damped) {
      g.appendChild(this.el('line', {
        x1: x - s(4), y1: y - bw - stem, x2: x + s(4), y2: y - bw - stem,
        stroke: 'var(--state-info)', 'stroke-width': 2,
      }));
    }
  },

  drawCheckValve(g, anchor, s) {
    const r = s(14);
    const x = anchor.x;
    const y = anchor.y;
    g.appendChild(this.el('circle', {
      cx: x, cy: y, r, fill: 'var(--equipment-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 1.5,
    }));
    g.appendChild(this.el('polygon', {
      points: `${x - s(8)},${y + s(4)} ${x + s(8)},${y} ${x - s(8)},${y - s(4)}`,
      fill: 'var(--state-normal)', stroke: 'var(--equipment-stroke)', 'stroke-width': 1,
    }));
    g.appendChild(this.el('line', {
      x1: x - r, y1: y, x2: x + r, y2: y,
      stroke: 'var(--equipment-stroke)', 'stroke-width': 1.5,
    }));
  },

  drawReliefValve(g, anchor, s) {
    this.drawGateValve(g, anchor, s);
    g.appendChild(this.el('path', {
      d: `M ${anchor.x - s(6)} ${anchor.y - s(22)} L ${anchor.x} ${anchor.y - s(28)} L ${anchor.x + s(6)} ${anchor.y - s(22)}`,
      fill: 'none', stroke: 'var(--state-warning)', 'stroke-width': 1.5,
    }));
  },

  drawCentrifugalPump(g, anchor, s, label) {
    const r = s(20);
    const x = anchor.x;
    const y = anchor.y;
    g.appendChild(this.el('circle', {
      cx: x, cy: y, r, fill: 'var(--equipment-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 2,
      class: 'sym-pump__casing',
    }));
    g.appendChild(this.el('path', {
      d: `M ${x} ${y - s(10)} L ${x + s(8)} ${y + s(6)} L ${x - s(8)} ${y + s(6)} Z`,
      fill: 'var(--state-normal)', class: 'sym-pump__rotor',
    }));
    g.appendChild(this.el('rect', {
      x: x - s(3), y: y - r - s(6), width: s(6), height: s(6),
      fill: 'var(--equipment-stroke)',
    }));
    if (label) {
      g.appendChild(this.el('text', {
        x, y: y + r + s(10), 'text-anchor': 'middle', 'font-size': s(9), 'font-weight': 600,
        fill: 'var(--text-secondary)', class: 'sym-pump__label',
      })).textContent = label;
    }
  },

  drawTank(g, anchor, s, ctx, withDrain) {
    const w = s(60);
    const h = s(100);
    const x = anchor.x - w / 2;
    const y = anchor.y - h / 2;
    g.appendChild(this.el('rect', {
      x, y, width: w, height: h, rx: s(8),
      fill: 'var(--equipment-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 2,
      class: 'sym-tank__shell',
    }));
    const to = ctx.obj?.TechnologyObject?.[0];
    const capacity = parseFloat(ctx.getProp(to, 'Capacity')) || 100;
    const level = parseFloat(ctx.getProp(to, 'FixedLevel')) || capacity * 0.6;
    const fillH = (level / capacity) * h;
    g.appendChild(this.el('rect', {
      x: x + s(2), y: y + h - fillH - s(8), width: w - s(4), height: fillH,
      fill: 'var(--tank-liquid)', opacity: 0.7, class: 'sym-tank__liquid',
    }));
    g.appendChild(this.el('ellipse', {
      cx: anchor.x, cy: y, rx: w / 2, ry: s(8),
      fill: 'var(--equipment-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 1.5,
    }));
    if (withDrain) {
      g.appendChild(this.el('path', {
        d: `M ${anchor.x} ${y + h} L ${anchor.x} ${y + h + s(10)} L ${anchor.x + s(8)} ${y + h + s(10)}`,
        fill: 'none', stroke: 'var(--equipment-stroke)', 'stroke-width': 2,
      }));
    }
  },

  drawTankBattery(g, anchor, s) {
    const w = s(24);
    const h = s(56);
    const gap = s(8);
    for (let i = -1; i <= 1; i++) {
      const cx = anchor.x + i * (w + gap);
      const x = cx - w / 2;
      const y = anchor.y - h / 2;
      g.appendChild(this.el('rect', {
        x, y, width: w, height: h, rx: s(4),
        fill: 'var(--equipment-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 1.5,
      }));
      g.appendChild(this.el('ellipse', {
        cx, cy: y, rx: w / 2, ry: s(5),
        fill: 'var(--equipment-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 1,
      }));
    }
  },

  drawFlowPort(g, anchor, s, dir) {
    const r = s(16);
    const x = anchor.x;
    const y = anchor.y;
    g.appendChild(this.el('circle', {
      cx: x, cy: y, r, fill: 'var(--equipment-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 2,
    }));
    const arrow = dir === 'in'
      ? `M ${x + s(10)} ${y} L ${x - s(4)} ${y - s(8)} L ${x - s(4)} ${y + s(8)} Z`
      : `M ${x - s(10)} ${y} L ${x + s(4)} ${y - s(8)} L ${x + s(4)} ${y + s(8)} Z`;
    g.appendChild(this.el('path', {
      d: arrow, fill: 'var(--state-normal)', stroke: 'var(--equipment-stroke)', 'stroke-width': 1,
    }));
  },

  drawFilter(g, anchor, s) {
    const w = s(18);
    const h = s(14);
    const x = anchor.x;
    const y = anchor.y;
    g.appendChild(this.el('polygon', {
      points: `${x - w},${y - h} ${x + w},${y - h} ${x},${y + h}`,
      fill: 'var(--equipment-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 1.5,
    }));
    g.appendChild(this.el('line', {
      x1: x - w, y1: y + h, x2: x + w, y2: y + h,
      stroke: 'var(--equipment-stroke)', 'stroke-width': 2,
    }));
    for (let i = -1; i <= 1; i++) {
      g.appendChild(this.el('line', {
        x1: x + i * s(6), y1: y - h + s(2), x2: x + i * s(4), y2: y + h - s(2),
        stroke: 'var(--text-muted)', 'stroke-width': 1,
      }));
    }
  },

  drawPumpStation(g, anchor, s, composite) {
    const w = s(72);
    const h = s(52);
    const x = anchor.x - w / 2;
    const y = anchor.y - h / 2;
    g.appendChild(this.el('rect', {
      x, y, width: w, height: h, rx: s(4),
      fill: 'var(--equipment-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 2,
    }));
    const pumpR = s(10);
    const positions = composite ? [-s(18), 0, s(18)] : [-s(12), s(12)];
    for (const ox of positions) {
      g.appendChild(this.el('circle', {
        cx: anchor.x + ox, cy: anchor.y + s(4), r: pumpR,
        fill: 'none', stroke: 'var(--state-normal)', 'stroke-width': 1.5,
      }));
    }
    g.appendChild(this.el('text', {
      x: anchor.x, y: y - s(4), 'text-anchor': 'middle', 'font-size': s(9), 'font-weight': 600,
      fill: 'var(--text-secondary)',
    })).textContent = composite ? 'НПС' : 'Станция';
  },

  drawSensor(g, anchor, s) {
    const w = s(10);
    const h = s(12);
    const x = anchor.x;
    const y = anchor.y;
    g.appendChild(this.el('polygon', {
      points: `${x - w / 2},${y - h} ${x + w / 2},${y - h} ${x},${y}`,
      fill: 'var(--state-info)',
      stroke: 'var(--equipment-stroke)',
      'stroke-width': 1.5,
      'stroke-linejoin': 'round',
      class: 'sym-sensor__triangle',
    }));
    g.appendChild(this.el('line', {
      x1: x, y1: y, x2: x, y2: y + s(4),
      stroke: 'var(--equipment-stroke)', 'stroke-width': 1.5, 'stroke-linecap': 'round',
      class: 'sym-sensor__stem',
    }));
  },

  drawInstrument(g, anchor, s, letter) {
    const r = s(14);
    g.appendChild(this.el('circle', {
      cx: anchor.x, cy: anchor.y, r,
      fill: 'var(--equipment-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 1.5,
    }));
    g.appendChild(this.el('text', {
      x: anchor.x, y: anchor.y + s(4), 'text-anchor': 'middle', 'font-size': s(11), 'font-weight': 700,
      fill: 'var(--text-primary)',
    })).textContent = letter;
  },

  drawController(g, anchor, s) {
    const w = s(44);
    const h = s(36);
    const x = anchor.x - w / 2;
    const y = anchor.y - h / 2;
    g.appendChild(this.el('rect', {
      x, y, width: w, height: h, rx: s(3),
      fill: 'var(--equipment-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 1.5,
    }));
    g.appendChild(this.el('path', {
      d: `M ${anchor.x - s(8)} ${anchor.y} L ${anchor.x} ${anchor.y - s(6)} L ${anchor.x + s(8)} ${anchor.y} L ${anchor.x} ${anchor.y + s(6)} Z`,
      fill: 'var(--state-warning)', stroke: 'none',
    }));
  },

  drawLeakDetector(g, anchor, s) {
    const r = s(18);
    g.appendChild(this.el('circle', {
      cx: anchor.x, cy: anchor.y, r,
      fill: 'none', stroke: 'var(--state-alarm)', 'stroke-width': 2, 'stroke-dasharray': `${s(4)} ${s(3)}`,
    }));
    g.appendChild(this.el('path', {
      d: `M ${anchor.x - s(8)} ${anchor.y + s(2)} Q ${anchor.x} ${anchor.y - s(8)} ${anchor.x + s(8)} ${anchor.y + s(2)}`,
      fill: 'none', stroke: 'var(--state-alarm)', 'stroke-width': 1.5,
    }));
  },

  drawSensorFlag(g, anchor, s, dashed) {
    const flagH = s(22);
    const flagLen = s(26);
    const poleX = anchor.x - flagLen * 0.35;
    const tipX = anchor.x + flagLen * 0.65;
    const y = anchor.y;
    const top = y - flagH / 2;
    const bottom = y + flagH / 2;

    g.appendChild(this.el('line', {
      x1: poleX, y1: top - s(5), x2: poleX, y2: bottom + s(5),
      stroke: 'var(--equipment-stroke)', 'stroke-width': 2, 'stroke-linecap': 'round',
      class: 'sym-sensor-flag__pole',
    }));

    const banner = this.el('polygon', {
      points: `${poleX},${top} ${poleX},${bottom} ${tipX},${y}`,
      fill: dashed ? 'none' : 'var(--equipment-fill)',
      stroke: 'var(--equipment-stroke)',
      'stroke-width': 1.5,
      'stroke-linejoin': 'round',
      class: 'sym-sensor-flag__banner',
    });
    if (dashed) {
      banner.setAttribute('stroke-dasharray', `${s(4)} ${s(3)}`);
    }
    g.appendChild(banner);

    g.appendChild(this.el('circle', {
      cx: poleX, cy: y, r: s(2.5),
      fill: 'var(--state-info)', class: 'sym-sensor-flag__point',
    }));
  },

  drawCamera(g, anchor, s) {
    const w = s(36);
    const h = s(24);
    const x = anchor.x - w / 2;
    const y = anchor.y - h / 2;
    g.appendChild(this.el('rect', {
      x, y, width: w, height: h, rx: s(3),
      fill: 'var(--equipment-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 1.5,
    }));
    g.appendChild(this.el('circle', {
      cx: anchor.x, cy: anchor.y, r: s(6),
      fill: 'var(--state-info)', opacity: 0.6,
    }));
    g.appendChild(this.el('polygon', {
      points: `${x + w} ${anchor.y - s(6)} ${x + w + s(8)} ${anchor.y - s(10)} ${x + w + s(8)} ${anchor.y + s(10)} ${x + w} ${anchor.y + s(6)}`,
      fill: 'var(--equipment-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 1,
    }));
  },

  drawCustodyTransfer(g, anchor, s) {
    const w = s(40);
    g.appendChild(this.el('line', {
      x1: anchor.x - w / 2, y1: anchor.y + s(8), x2: anchor.x + w / 2, y2: anchor.y + s(8),
      stroke: 'var(--equipment-stroke)', 'stroke-width': 2,
    }));
    g.appendChild(this.el('line', {
      x1: anchor.x, y1: anchor.y + s(8), x2: anchor.x, y2: anchor.y - s(14),
      stroke: 'var(--equipment-stroke)', 'stroke-width': 2,
    }));
    g.appendChild(this.el('path', {
      d: `M ${anchor.x - s(12)} ${anchor.y - s(14)} L ${anchor.x + s(12)} ${anchor.y - s(14)} L ${anchor.x} ${anchor.y - s(22)} Z`,
      fill: 'var(--equipment-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 1.5,
    }));
  },

  drawTrunkSection(g, anchor, s) {
    const w = s(48);
    const h = s(8);
    g.appendChild(this.el('rect', {
      x: anchor.x - w / 2, y: anchor.y - h / 2, width: w, height: h, rx: h / 2,
      fill: 'var(--pipe-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 1,
    }));
    g.appendChild(this.el('line', {
      x1: anchor.x - w / 2 + s(4), y1: anchor.y, x2: anchor.x + w / 2 - s(4), y2: anchor.y,
      stroke: 'var(--text-muted)', 'stroke-width': 1, 'stroke-dasharray': `${s(3)} ${s(2)}`,
    }));
  },

  drawTechArea(g, anchor, s) {
    const w = s(64);
    const h = s(48);
    g.appendChild(this.el('rect', {
      x: anchor.x - w / 2, y: anchor.y - h / 2, width: w, height: h, rx: s(4),
      fill: 'none', stroke: 'var(--text-muted)', 'stroke-width': 1.5, 'stroke-dasharray': `${s(6)} ${s(4)}`,
    }));
    g.appendChild(this.el('text', {
      x: anchor.x, y: anchor.y + s(4), 'text-anchor': 'middle', 'font-size': s(9),
      fill: 'var(--text-muted)',
    })).textContent = 'Тех. зона';
  },

  drawComposite(g, anchor, s) {
    const w = s(52);
    const h = s(40);
    g.appendChild(this.el('rect', {
      x: anchor.x - w / 2, y: anchor.y - h / 2, width: w, height: h, rx: s(4),
      fill: 'var(--equipment-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 1.5,
    }));
    g.appendChild(this.el('rect', {
      x: anchor.x - w / 2 + s(4), y: anchor.y - h / 2 + s(4), width: w / 2 - s(6), height: h - s(8), rx: s(2),
      fill: 'none', stroke: 'var(--text-muted)', 'stroke-width': 1,
    }));
    g.appendChild(this.el('rect', {
      x: anchor.x + s(2), y: anchor.y - h / 2 + s(4), width: w / 2 - s(6), height: h - s(8), rx: s(2),
      fill: 'none', stroke: 'var(--text-muted)', 'stroke-width': 1,
    }));
  },

  drawTieIn(g, anchor, s) {
    const r = s(14);
    g.appendChild(this.el('circle', {
      cx: anchor.x, cy: anchor.y, r,
      fill: 'var(--equipment-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 1.5,
    }));
    g.appendChild(this.el('path', {
      d: `M ${anchor.x - s(10)} ${anchor.y} L ${anchor.x} ${anchor.y - s(10)} L ${anchor.x + s(10)} ${anchor.y} L ${anchor.x} ${anchor.y + s(10)} Z`,
      fill: 'none', stroke: 'var(--state-normal)', 'stroke-width': 1.5,
    }));
  },

  drawHeater(g, anchor, s) {
    const w = s(36);
    const h = s(28);
    g.appendChild(this.el('rect', {
      x: anchor.x - w / 2, y: anchor.y - h / 2, width: w, height: h, rx: s(3),
      fill: 'var(--equipment-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 1.5,
    }));
    for (let i = 0; i < 3; i++) {
      g.appendChild(this.el('path', {
        d: `M ${anchor.x - s(10) + i * s(10)} ${anchor.y - s(6)} Q ${anchor.x - s(5) + i * s(10)} ${anchor.y} ${anchor.x - s(10) + i * s(10)} ${anchor.y + s(6)}`,
        fill: 'none', stroke: 'var(--state-warning)', 'stroke-width': 1.5,
      }));
    }
  },

  drawControlCenter(g, anchor, s) {
    const w = s(56);
    const h = s(44);
    g.appendChild(this.el('rect', {
      x: anchor.x - w / 2, y: anchor.y - h / 2, width: w, height: h, rx: s(4),
      fill: 'var(--equipment-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 2,
    }));
    g.appendChild(this.el('rect', {
      x: anchor.x - s(16), y: anchor.y - s(10), width: s(32), height: s(20), rx: s(2),
      fill: 'var(--bg-elevated)', stroke: 'var(--state-info)', 'stroke-width': 1,
    }));
    g.appendChild(this.el('text', {
      x: anchor.x, y: anchor.y + s(5), 'text-anchor': 'middle', 'font-size': s(8), 'font-weight': 600,
      fill: 'var(--text-secondary)',
    })).textContent = 'ЦУ';
  },

  drawGeneric(g, anchor, s, template) {
    const half = s(20);
    g.appendChild(this.el('rect', {
      x: anchor.x - half, y: anchor.y - half, width: half * 2, height: half * 2, rx: s(4),
      fill: 'var(--equipment-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 1.5,
    }));
    const abbr = (template || '?').replace(/([A-Z])/g, ' $1').trim().split(/\s+/).map(w => w[0]).join('').slice(0, 2).toUpperCase();
    g.appendChild(this.el('text', {
      x: anchor.x, y: anchor.y + s(4), 'text-anchor': 'middle', 'font-size': s(10), 'font-weight': 600,
      fill: 'var(--text-primary)',
    })).textContent = abbr;
  },

  drawLetter(g, anchor, s, text) {
    const size = s(28);
    const half = size / 2;
    g.appendChild(this.el('rect', {
      x: anchor.x - half, y: anchor.y - half, width: size, height: size, rx: s(4),
      fill: 'var(--equipment-fill)', stroke: 'var(--equipment-stroke)', 'stroke-width': 1.5,
      class: 'scheme-letter__box',
    }));
    g.appendChild(this.el('text', {
      x: anchor.x, y: anchor.y + s(4), 'text-anchor': 'middle', 'font-size': s(11), 'font-weight': 600,
      fill: 'var(--text-primary)', class: 'scheme-letter__text',
    })).textContent = text;
  },
};
