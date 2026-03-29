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

test('sidebar and main content share the flex layout without manual content offsets', () => {
    const template = readFixture('../../../templates/layout.html');
    const css = readFixture('../../css/dashboard.css');

    assert.match(template, /<aside class="sidebar"[\s\S]*<div class="sidebar-toggle"[\s\S]*<main class="content"/);
    assert.match(template, /<main class="content" :class="\{ 'content-collapsed': sidebarCollapsed \}">/);

    const sidebarRule = readCSSRule(css, '.sidebar');
    assert.match(sidebarRule, /flex:\s*0 0 var\(--sidebar-width\)/);
    assert.match(sidebarRule, /position:\s*sticky/);
    assert.match(sidebarRule, /height:\s*100vh/);
    assert.doesNotMatch(sidebarRule, /position:\s*fixed/);

    const toggleRule = readCSSRule(css, '.sidebar-toggle');
    assert.match(toggleRule, /flex:\s*0 0 6px/);
    assert.match(toggleRule, /position:\s*sticky/);
    assert.match(toggleRule, /height:\s*100vh/);
    assert.doesNotMatch(toggleRule, /left:\s*var\(--sidebar-width\)/);

    const contentRule = readCSSRule(css, '.content');
    assert.match(contentRule, /flex:\s*1 1 0/);
    assert.match(contentRule, /width:\s*100%/);
    assert.match(contentRule, /max-width:\s*1200px/);
    assert.match(contentRule, /margin:\s*0 auto/);
    assert.doesNotMatch(contentRule, /margin-left:\s*max\(/);

    const collapsedSidebarRule = readCSSRule(css, '.sidebar.sidebar-collapsed');
    assert.match(collapsedSidebarRule, /flex-basis:\s*60px/);
});

test('dashboard layout pins Chart.js to 4.5.0', () => {
    const template = readFixture('../../../templates/layout.html');

    assert.match(
        template,
        /<script src="https:\/\/cdn\.jsdelivr\.net\/npm\/chart\.js@4\.5\.0\/dist\/chart\.umd\.min\.js"><\/script>/
    );
});
