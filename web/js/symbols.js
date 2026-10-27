const Symbols = {
  NS: 'http://www.w3.org/2000/svg',

  P: {
    ink: '#1E2428',
    g: '#2DB24A',
    gL: '#73DE7C',
    gD: '#1A8A32',
    y: '#F2D12B',
    yD: '#C9A61A',
    st: '#8E99A4',
    stL: '#D5DCE2',
    stD: '#5A646E',
    cyan: '#3EC3E6',
    cyanD: '#1E8AAB',
    wh: '#F4F8F2',
    blu: '#3A8FD4',
  },

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

  paint(extra) {
    return Object.assign({
      fill: this.P.g,
      stroke: this.P.ink,
      'stroke-width': 0.95,
      'stroke-linejoin': 'round',
      'stroke-linecap': 'round',
    }, extra || {});
  },

  add(g, tag, attrs) {
    const node = this.el(tag, this.paint(attrs));
    g.appendChild(node);
    return node;
  },

  flange(g, cx, cy, rx, ry) {
    this.add(g, 'ellipse', {
      cx, cy, rx, ry, fill: this.P.y, stroke: this.P.yD, 'stroke-width': 0.8,
    });
    this.add(g, 'ellipse', {
      cx, cy, rx: rx * 0.35, ry: ry * 0.55, fill: this.P.yD, stroke: 'none',
    });
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
      Tank: () => this.drawTankLite(g, anchor, s),
      TankBattery: () => this.drawTankBattery(g, anchor, s),
      TankDump: () => this.drawTank(g, anchor, s, ctx, true),
      Enter: () => this.drawFlowPort(g, anchor, s, 'in'),
      Exit: () => this.drawFlowPort(g, anchor, s, 'out'),
      Filter: () => this.drawFilter(g, anchor, s),
      PumpStation: () => this.drawStationLite(g, anchor, s),
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
      Valve: { w: half(44), h: half(56) },
      BackValve: { w: half(40), h: half(28) },
      ReliefValve: { w: half(40), h: half(48) },
      PressureWaveSmoothingValve: { w: half(44), h: half(56) },
      PipelinePump: { w: half(40), h: half(36) },
      LeakPump: { w: half(44), h: half(48) },
      BoosterPump: { w: half(44), h: half(48) },
      Tank: { w: half(40), h: half(64) },
      TankBattery: { w: half(56), h: half(52) },
      TankDump: { w: half(48), h: half(80) },
      Enter: { w: half(32), h: half(28) },
      Exit: { w: half(32), h: half(28) },
      Filter: { w: half(36), h: half(32) },
      PumpStation: { w: half(56), h: half(40) },
      CompositePumpStation: { w: half(72), h: half(52) },
      ShopMainstream: { w: half(72), h: half(52) },
      Sensor: { w: half(16), h: half(22) },
      UnitMeasureQuality: { w: half(32), h: half(28) },
      PressureControlUnit: { w: half(36), h: half(28) },
      LeakDetectionSystem: { w: half(36), h: half(36) },
      CleanDiagnSystemCamera: { w: half(28), h: half(36) },
      CleanDiagnSystemCameraSensor: { w: half(36), h: half(28) },
      CleanDiagnSystemCameraPass: { w: half(36), h: half(28) },
      LeaseAutomaticCustodyTransfer: { w: half(36), h: half(32) },
      LinearPartOfTrunkPipeline: { w: half(48), h: half(16) },
      TechnologicalArea: { w: half(64), h: half(48) },
      CompositeGeneralObject: { w: half(40), h: half(32) },
      PipelineTieIn: { w: half(32), h: half(28) },
      PointOilHeating: { w: half(36), h: half(28) },
      AreaControlCenter: { w: half(56), h: half(44) },
    };
    const p = presets[template] || { w: half(40), h: half(40) };
    return { x: anchor.x - p.w / 2, y: anchor.y - p.h / 2, width: p.w, height: p.h };
  },

  drawGateValve(g, anchor, s, damped) {
    const x = anchor.x;
    const y = anchor.y;
    const hw = s(15);
    const hh = s(11);
    this.flange(g, x - hw, y, s(4), hh);
    this.flange(g, x + hw, y, s(4), hh);
    this.add(g, 'polygon', {
      points: `${x - hw + s(2)},${y - hh} ${x},${y} ${x - hw + s(2)},${y + hh}`,
      fill: this.P.gD,
    });
    this.add(g, 'polygon', {
      points: `${x - hw + s(2)},${y - hh} ${x},${y} ${x - hw + s(3)},${y}`,
      fill: this.P.gL, stroke: 'none',
    });
    this.add(g, 'polygon', {
      points: `${x + hw - s(2)},${y - hh} ${x},${y} ${x + hw - s(2)},${y + hh}`,
      fill: this.P.g,
    });
    this.add(g, 'polygon', {
      points: `${x + hw - s(2)},${y - hh} ${x},${y} ${x + hw - s(3)},${y}`,
      fill: this.P.gL, stroke: 'none',
    });
    const stemTop = y - hh - s(10);
    this.add(g, 'rect', {
      x: x - s(1.4), y: y - hh - s(1), width: s(2.8), height: s(11),
      fill: this.P.stL, stroke: this.P.stD,
    });
    this.add(g, 'rect', {
      x: x - s(8), y: stemTop - s(7), width: s(16), height: s(10), rx: s(1.5),
      fill: this.P.st, stroke: this.P.ink,
    });
    this.add(g, 'rect', {
      x: x + s(4), y: stemTop - s(4), width: s(10), height: s(6), rx: s(2),
      fill: this.P.stL, stroke: this.P.stD,
    });
    this.add(g, 'rect', {
      x: x - s(6), y: stemTop - s(5), width: s(12), height: s(3),
      fill: this.P.y, stroke: this.P.yD,
    });
    if (damped) {
      this.add(g, 'circle', {
        cx: x - s(10), cy: stemTop, r: s(3.5), fill: this.P.blu, stroke: this.P.ink,
      });
    }
  },

  drawCheckValve(g, anchor, s) {
    const x = anchor.x;
    const y = anchor.y;
    const r = s(11);
    this.flange(g, x - r - s(3), y, s(3.2), s(9));
    this.flange(g, x + r + s(3), y, s(3.2), s(9));
    this.add(g, 'circle', { cx: x, cy: y, r, fill: this.P.g });
    this.add(g, 'ellipse', {
      cx: x - s(3), cy: y - s(3), rx: s(5), ry: s(4), fill: this.P.gL, stroke: 'none',
    });
    this.add(g, 'polygon', {
      points: `${x - s(5)},${y - s(6)} ${x + s(7)},${y} ${x - s(5)},${y + s(6)}`,
      fill: this.P.wh,
    });
  },

  drawReliefValve(g, anchor, s) {
    this.drawGateValve(g, anchor, s);
    const x = anchor.x;
    const y = anchor.y - s(26);
    this.add(g, 'polygon', {
      points: `${x - s(5)},${y} ${x},${y - s(7)} ${x + s(5)},${y}`,
      fill: this.P.y,
    });
  },

  drawCentrifugalPump(g, anchor, s, label) {
    const x = anchor.x;
    const y = anchor.y;
    const r = s(14);
    this.flange(g, x - r - s(2), y, s(3), s(8));
    this.add(g, 'circle', { cx: x, cy: y, r, fill: this.P.g });
    this.add(g, 'ellipse', {
      cx: x - s(4), cy: y - s(4), rx: s(6), ry: s(5), fill: this.P.gL, stroke: 'none',
    });
    this.add(g, 'polygon', {
      points: `${x - s(4)},${y - s(6)} ${x + s(9)},${y} ${x - s(4)},${y + s(6)}`,
      fill: this.P.wh,
    });
    this.add(g, 'rect', {
      x: x - s(3), y: y - r - s(7), width: s(6), height: s(7),
      fill: this.P.st, stroke: this.P.ink,
    });
    if (label) {
      this.add(g, 'text', {
        x, y: y + r + s(10), 'text-anchor': 'middle', 'font-size': s(8), 'font-weight': 700,
        fill: this.P.ink, stroke: 'none',
      }).textContent = label;
    }
  },

  drawTankLite(g, anchor, s) {
    const w = s(34);
    const h = s(52);
    const x = anchor.x - w / 2;
    const y = anchor.y - h / 2;
    this.add(g, 'rect', {
      x, y: y + s(6), width: w, height: h - s(10), fill: this.P.st,
    });
    this.add(g, 'rect', {
      x: x + s(2), y: y + h * 0.45, width: w - s(4), height: h * 0.38,
      fill: this.P.cyan, stroke: 'none',
    });
    this.add(g, 'ellipse', {
      cx: anchor.x, cy: y + h - s(4), rx: w / 2, ry: s(6), fill: this.P.stD,
    });
    this.add(g, 'ellipse', {
      cx: anchor.x, cy: y + s(8), rx: w / 2, ry: s(8), fill: this.P.cyan,
    });
    this.add(g, 'ellipse', {
      cx: anchor.x, cy: y + s(6), rx: w / 2 - s(4), ry: s(4), fill: this.P.wh, stroke: 'none', opacity: 0.35,
    });
  },

  drawTank(g, anchor, s, ctx, withDrain) {
    this.drawTankLite(g, anchor, s);
    if (withDrain) {
      this.add(g, 'path', {
        d: `M ${anchor.x} ${anchor.y + s(32)} L ${anchor.x} ${anchor.y + s(40)} L ${anchor.x + s(8)} ${anchor.y + s(40)}`,
        fill: 'none', stroke: this.P.stD, 'stroke-width': 1.6,
      });
    }
  },

  drawTankBattery(g, anchor, s) {
    const w = s(18);
    const h = s(36);
    for (let i = -1; i <= 1; i++) {
      const cx = anchor.x + i * s(18);
      const x = cx - w / 2;
      const y = anchor.y - h / 2;
      this.add(g, 'rect', {
        x, y: y + s(5), width: w, height: h - s(8), fill: this.P.st,
      });
      this.add(g, 'ellipse', {
        cx, cy: y + s(6), rx: w / 2, ry: s(5), fill: this.P.cyan,
      });
    }
  },

  drawFlowPort(g, anchor, s, dir) {
    const a = s(11);
    const x = anchor.x;
    const y = anchor.y;
    this.add(g, 'rect', {
      x: x - a, y: y - a, width: a * 2, height: a * 2, rx: s(1.5), fill: this.P.y,
    });
    this.add(g, 'rect', {
      x: x - a + s(2), y: y - a + s(2), width: a * 2 - s(4), height: s(4),
      fill: this.P.yD, stroke: 'none',
    });
    const pts = dir === 'in'
      ? `${x + s(6)},${y - s(6)} ${x - s(6)},${y} ${x + s(6)},${y + s(6)}`
      : `${x - s(6)},${y - s(6)} ${x + s(6)},${y} ${x - s(6)},${y + s(6)}`;
    this.add(g, 'polygon', { points: pts, fill: this.P.g });
  },

  drawFilter(g, anchor, s) {
    const x = anchor.x;
    const y = anchor.y;
    this.add(g, 'polygon', {
      points: `${x - s(16)},${y - s(10)} ${x + s(16)},${y - s(10)} ${x},${y + s(14)}`,
      fill: this.P.g,
    });
    this.add(g, 'polygon', {
      points: `${x - s(16)},${y - s(10)} ${x},${y - s(10)} ${x},${y + s(14)}`,
      fill: this.P.gL, stroke: 'none',
    });
    this.add(g, 'line', {
      x1: x - s(10), y1: y - s(2), x2: x + s(10), y2: y - s(2),
      stroke: this.P.y, 'stroke-width': 2, fill: 'none',
    });
    this.add(g, 'rect', {
      x: x - s(16), y: y + s(12), width: s(32), height: s(4), fill: this.P.y,
    });
  },

  drawStationLite(g, anchor, s) {
    const w = s(52);
    const h = s(34);
    const x = anchor.x - w / 2;
    const y = anchor.y - h / 2;
    this.add(g, 'rect', { x, y, width: w, height: h, rx: s(2), fill: this.P.g });
    this.add(g, 'rect', {
      x, y, width: w, height: s(7), fill: this.P.gL, stroke: 'none',
    });
    this.add(g, 'circle', {
      cx: anchor.x, cy: anchor.y + s(4), r: s(9), fill: this.P.wh,
    });
    this.add(g, 'polygon', {
      points: `${anchor.x - s(4)},${anchor.y} ${anchor.x + s(6)},${anchor.y + s(4)} ${anchor.x - s(4)},${anchor.y + s(8)}`,
      fill: this.P.gD, stroke: 'none',
    });
  },

  drawPumpStation(g, anchor, s, composite) {
    this.drawStationLite(g, anchor, s);
    const positions = composite ? [-s(16), 0, s(16)] : [-s(10), s(10)];
    for (const ox of positions) {
      this.add(g, 'circle', {
        cx: anchor.x + ox, cy: anchor.y + s(4), r: s(6),
        fill: this.P.wh, 'stroke-width': 1.2,
      });
    }
  },

  drawSensor(g, anchor, s) {
    const x = anchor.x;
    const y = anchor.y;
    this.add(g, 'polygon', {
      points: `${x},${y - s(10)} ${x + s(8)},${y + s(7)} ${x - s(8)},${y + s(7)}`,
      fill: this.P.blu,
    });
    this.add(g, 'line', {
      x1: x, y1: y + s(7), x2: x, y2: y + s(13),
      stroke: this.P.ink, 'stroke-width': 1.4, fill: 'none',
    });
  },

  drawInstrument(g, anchor, s, letter) {
    const w = s(28);
    const h = s(22);
    this.add(g, 'rect', {
      x: anchor.x - w / 2, y: anchor.y - h / 2, width: w, height: h, rx: s(2), fill: this.P.g,
    });
    this.add(g, 'text', {
      x: anchor.x, y: anchor.y + s(4), 'text-anchor': 'middle', 'font-size': s(10), 'font-weight': 700,
      fill: this.P.wh, stroke: 'none',
    }).textContent = letter;
  },

  drawController(g, anchor, s) {
    const w = s(30);
    const h = s(22);
    this.add(g, 'rect', {
      x: anchor.x - w / 2, y: anchor.y - h / 2, width: w, height: h, fill: this.P.g,
    });
    this.add(g, 'polygon', {
      points: `${anchor.x},${anchor.y - s(6)} ${anchor.x + s(7)},${anchor.y} ${anchor.x},${anchor.y + s(6)} ${anchor.x - s(7)},${anchor.y}`,
      fill: this.P.y,
    });
  },

  drawLeakDetector(g, anchor, s) {
    this.add(g, 'circle', {
      cx: anchor.x, cy: anchor.y, r: s(14), fill: this.P.g,
    });
    this.add(g, 'path', {
      d: `M ${anchor.x - s(7)} ${anchor.y + s(3)} Q ${anchor.x} ${anchor.y - s(8)} ${anchor.x + s(7)} ${anchor.y + s(3)}`,
      fill: 'none', stroke: this.P.wh, 'stroke-width': 1.6,
    });
  },

  drawSensorFlag(g, anchor, s, dashed) {
    const poleX = anchor.x - s(8);
    const y = anchor.y;
    this.add(g, 'line', {
      x1: poleX, y1: y - s(12), x2: poleX, y2: y + s(12),
      stroke: this.P.ink, 'stroke-width': 2, fill: 'none',
    });
    this.add(g, 'polygon', {
      points: `${poleX},${y - s(8)} ${poleX},${y + s(8)} ${anchor.x + s(14)},${y}`,
      fill: dashed ? this.P.wh : this.P.g,
      'stroke-dasharray': dashed ? `${s(3)} ${s(2)}` : null,
    });
  },

  drawCamera(g, anchor, s) {
    this.add(g, 'circle', {
      cx: anchor.x, cy: anchor.y, r: s(11), fill: this.P.g,
    });
    this.add(g, 'circle', {
      cx: anchor.x, cy: anchor.y, r: s(5), fill: this.P.wh,
    });
    this.add(g, 'line', {
      x1: anchor.x, y1: anchor.y + s(11), x2: anchor.x, y2: anchor.y + s(17),
      stroke: this.P.ink, fill: 'none', 'stroke-width': 1.5,
    });
    this.add(g, 'line', {
      x1: anchor.x - s(6), y1: anchor.y + s(17), x2: anchor.x + s(6), y2: anchor.y + s(17),
      stroke: this.P.ink, fill: 'none', 'stroke-width': 1.5,
    });
  },

  drawCustodyTransfer(g, anchor, s) {
    const a = s(15);
    this.add(g, 'rect', {
      x: anchor.x - a, y: anchor.y - a + s(2), width: a * 2, height: a * 2 - s(4), rx: s(2), fill: this.P.g,
    });
    this.add(g, 'circle', {
      cx: anchor.x, cy: anchor.y + s(1), r: s(7), fill: this.P.y,
    });
    this.add(g, 'line', {
      x1: anchor.x, y1: anchor.y + s(1), x2: anchor.x + s(5), y2: anchor.y - s(4),
      stroke: this.P.ink, fill: 'none', 'stroke-width': 1.5,
    });
  },

  drawTrunkSection(g, anchor, s) {
    const w = s(44);
    const h = s(10);
    this.add(g, 'rect', {
      x: anchor.x - w / 2, y: anchor.y - h / 2, width: w, height: h, rx: h / 2, fill: this.P.g,
    });
    this.add(g, 'line', {
      x1: anchor.x - w / 2 + s(4), y1: anchor.y, x2: anchor.x + w / 2 - s(4), y2: anchor.y,
      stroke: this.P.gL, fill: 'none', 'stroke-width': 1.2,
    });
  },

  drawTechArea(g, anchor, s) {
    const w = s(64);
    const h = s(48);
    this.add(g, 'rect', {
      x: anchor.x - w / 2, y: anchor.y - h / 2, width: w, height: h, rx: s(4),
      fill: 'none', stroke: this.P.st, 'stroke-width': 1.5, 'stroke-dasharray': `${s(6)} ${s(4)}`,
    });
    this.add(g, 'text', {
      x: anchor.x, y: anchor.y + s(4), 'text-anchor': 'middle', 'font-size': s(9),
      fill: this.P.stD, stroke: 'none',
    }).textContent = 'Тех. зона';
  },

  drawComposite(g, anchor, s) {
    const w = s(26);
    const h = s(20);
    this.add(g, 'rect', {
      x: anchor.x - w / 2 - s(4), y: anchor.y - h / 2 - s(3), width: w, height: h, fill: this.P.g,
    });
    this.add(g, 'rect', {
      x: anchor.x - w / 2 + s(4), y: anchor.y - h / 2 + s(3), width: w, height: h, fill: this.P.gL,
    });
  },

  drawTieIn(g, anchor, s) {
    this.add(g, 'line', {
      x1: anchor.x - s(14), y1: anchor.y, x2: anchor.x + s(14), y2: anchor.y,
      stroke: this.P.g, 'stroke-width': 3, fill: 'none',
    });
    this.add(g, 'line', {
      x1: anchor.x, y1: anchor.y, x2: anchor.x, y2: anchor.y - s(12),
      stroke: this.P.g, 'stroke-width': 3, fill: 'none',
    });
    this.add(g, 'rect', {
      x: anchor.x - s(5), y: anchor.y - s(5), width: s(10), height: s(10), fill: this.P.y,
    });
  },

  drawHeater(g, anchor, s) {
    const w = s(32);
    const h = s(22);
    this.add(g, 'rect', {
      x: anchor.x - w / 2, y: anchor.y - h / 2, width: w, height: h, fill: this.P.g,
    });
    this.add(g, 'path', {
      d: `M ${anchor.x - s(10)} ${anchor.y + s(3)} Q ${anchor.x - s(5)} ${anchor.y - s(7)} ${anchor.x} ${anchor.y + s(3)} Q ${anchor.x + s(5)} ${anchor.y - s(7)} ${anchor.x + s(10)} ${anchor.y + s(3)}`,
      fill: 'none', stroke: this.P.y, 'stroke-width': 1.6,
    });
  },

  drawControlCenter(g, anchor, s) {
    const w = s(52);
    const h = s(40);
    this.add(g, 'rect', {
      x: anchor.x - w / 2, y: anchor.y - h / 2, width: w, height: h, rx: s(3), fill: this.P.g,
    });
    this.add(g, 'rect', {
      x: anchor.x - s(16), y: anchor.y - s(10), width: s(32), height: s(20), rx: s(2), fill: this.P.stL,
    });
    this.add(g, 'text', {
      x: anchor.x, y: anchor.y + s(5), 'text-anchor': 'middle', 'font-size': s(8), 'font-weight': 700,
      fill: this.P.ink, stroke: 'none',
    }).textContent = 'ЦУ';
  },

  drawGeneric(g, anchor, s, template) {
    const half = s(18);
    this.add(g, 'rect', {
      x: anchor.x - half, y: anchor.y - half, width: half * 2, height: half * 2, rx: s(3), fill: this.P.g,
    });
    const abbr = (template || '?').replace(/([A-Z])/g, ' $1').trim().split(/\s+/).map(w => w[0]).join('').slice(0, 2).toUpperCase();
    this.add(g, 'text', {
      x: anchor.x, y: anchor.y + s(4), 'text-anchor': 'middle', 'font-size': s(10), 'font-weight': 700,
      fill: this.P.wh, stroke: 'none',
    }).textContent = abbr;
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
