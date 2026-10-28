const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { InstIcons } = require('./inst-icons.js');

describe('InstIcons', () => {
  it('maps Valve and BackValve to technology images', () => {
    assert.match(InstIcons.relativePath('Valve'), /technologyimages\/valve_32\.png$/);
    assert.match(InstIcons.relativePath('BackValve'), /technologyimages\/backvalve_32\.png$/);
  });

  it('maps PressureWaveSmoothingValve to visuals image not valve', () => {
    const rel = InstIcons.relativePath('PressureWaveSmoothingValve');
    assert.match(rel, /pressurewavesmoothingvalve32\.png$/);
    assert.ok(!rel.includes('valve_32'));
  });

  it('returns empty url for unknown template', () => {
    assert.equal(InstIcons.url('NoSuchTemplateType'), '');
  });

  it('points at files that exist on disk', () => {
    const root = path.join(__dirname, '..', 'assets', 'inst-icons');
    for (const rel of Object.values(InstIcons.FILES)) {
      assert.equal(fs.existsSync(path.join(root, rel)), true, rel);
    }
  });
});
