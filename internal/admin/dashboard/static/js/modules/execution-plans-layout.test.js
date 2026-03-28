const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

function readFixture(relativePath) {
    return fs.readFileSync(path.join(__dirname, relativePath), 'utf8');
}

function readCSSRule(source, selector) {
    const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const match = source.match(new RegExp(`${escapedSelector}\\s*\\{([\\s\\S]*?)\\n\\}`, 'm'));
    assert.ok(match, `Expected CSS rule for ${selector}`);
    return match[1];
}

test('async pipeline branch spans full width and keeps the turn inline', () => {
    const template = readFixture('../../../templates/index.html');
    const css = readFixture('../../css/dashboard.css');

    assert.match(
        template,
        /<div class="ep-async-section"[\s\S]*?<div class="ep-async-row">[\s\S]*?<\/div>\s*<div class="ep-async-turn"><\/div>/
    );

    const asyncSectionRule = readCSSRule(css, '.ep-async-section');
    assert.match(asyncSectionRule, /width:\s*100%/);
    assert.doesNotMatch(asyncSectionRule, /flex-direction:\s*column/);
    assert.match(asyncSectionRule, /align-items:\s*center/);
    assert.doesNotMatch(asyncSectionRule, /margin-top:\s*[1-9]/);

    const asyncTurnRule = readCSSRule(css, '.ep-async-turn');
    assert.match(asyncTurnRule, /width:\s*\d/);
    assert.match(asyncTurnRule, /height:\s*2px/);
    assert.doesNotMatch(asyncTurnRule, /border-bottom:/);
    assert.doesNotMatch(asyncTurnRule, /border-right:/);

    const asyncRowRule = readCSSRule(css, '.ep-async-row');
    assert.match(asyncRowRule, /display:\s*flex/);
    assert.match(asyncRowRule, /margin-right:\s*7px/);

    const asyncTurnVerticalRule = readCSSRule(css, '.ep-async-turn::after');
    assert.match(asyncTurnVerticalRule, /border-right:/);
    assert.match(asyncTurnVerticalRule, /bottom:\s*1px/);
    assert.doesNotMatch(asyncTurnVerticalRule, /transform:/);
});

test('async label stays inline on the right side of the branch', () => {
    const template = readFixture('../../../templates/index.html');
    const css = readFixture('../../css/dashboard.css');

    assert.match(
        template,
        /<div class="ep-async-row">[\s\S]*ep-node-async-usage[\s\S]*ep-conn-async[\s\S]*ep-node-async-audit[\s\S]*<\/div>\s*<div class="ep-async-turn"><\/div>\s*<span class="ep-async-label">Async<\/span>/
    );

    const asyncLabelRule = readCSSRule(css, '.ep-async-label');
    assert.doesNotMatch(asyncLabelRule, /position:\s*absolute/);
});
