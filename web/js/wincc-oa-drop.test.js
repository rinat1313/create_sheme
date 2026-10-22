const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const { parseWinCcOaDropText } = require('./wincc-oa-drop.js');

describe('parseWinCcOaDropText', () => {
  it('parses WinCC OA dp path after first colon until first dot', () => {
    assert.equal(parseWinCcOaDropText('TNVP1:NAME_DAP.State.Status'), 'NAME_DAP');
    assert.equal(parseWinCcOaDropText('N:DP_P.S.s'), 'DP_P');
  });

  it('takes everything after first colon as second half', () => {
    assert.equal(parseWinCcOaDropText('A:B:C.D'), 'B:C');
    assert.equal(parseWinCcOaDropText('sys:DP.rest.more'), 'DP');
  });

  it('trims raw input and segment', () => {
    assert.equal(parseWinCcOaDropText('  TNVP1:NAME_DAP.State  '), 'NAME_DAP');
    assert.equal(parseWinCcOaDropText('X:  DP_P  .S'), 'DP_P');
  });

  it('returns null for invalid formats', () => {
    assert.equal(parseWinCcOaDropText(null), null);
    assert.equal(parseWinCcOaDropText(''), null);
    assert.equal(parseWinCcOaDropText('   '), null);
    assert.equal(parseWinCcOaDropText('NoColon'), null);
    assert.equal(parseWinCcOaDropText('Tag'), null);
    assert.equal(parseWinCcOaDropText('Name'), null);
    assert.equal(parseWinCcOaDropText('prefix:'), null);
    assert.equal(parseWinCcOaDropText('prefix:  '), null);
    assert.equal(parseWinCcOaDropText('prefix:.State'), null);
  });

  it('accepts segment without trailing dotted path', () => {
    assert.equal(parseWinCcOaDropText('N:DP_ONLY'), 'DP_ONLY');
  });
});
