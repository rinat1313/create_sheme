const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const { LocatorGeom } = require('./locator-geom.js');

describe('LocatorGeom', () => {
  it('treats overlapping boxes as in view', () => {
    assert.equal(
      LocatorGeom.bboxIntersectsViewBox({ x: 10, y: 10, w: 20, h: 20 }, { x: 0, y: 0, w: 25, h: 25 }),
      true,
    );
  });

  it('treats a box that overlaps the view edge as in view', () => {
    assert.equal(
      LocatorGeom.bboxIntersectsViewBox({ x: 8, y: 0, w: 5, h: 5 }, { x: 0, y: 0, w: 10, h: 10 }),
      true,
    );
  });

  it('detects fully offscreen boxes', () => {
    assert.equal(
      LocatorGeom.bboxIntersectsViewBox({ x: 100, y: 100, w: 10, h: 10 }, { x: 0, y: 0, w: 50, h: 50 }),
      false,
    );
  });

  it('builds four inward chevrons around the bbox', () => {
    const polys = LocatorGeom.chevronPolygons({ x: 0, y: 0, w: 40, h: 40 });
    assert.equal(polys.length, 4);
    const top = polys.find((p) => p.side === 'top');
    assert.ok(top.points[0][1] < 0);
    const left = polys.find((p) => p.side === 'left');
    assert.ok(left.points[0][0] < 0);
  });

  it('clamps chevron size', () => {
    assert.equal(LocatorGeom.chevronSize(4, 4), 12);
    assert.equal(LocatorGeom.chevronSize(400, 10), 48);
  });
});
