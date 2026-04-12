const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

function loadDashboardApp() {
    const dashboardSource = fs.readFileSync(path.join(__dirname, '../dashboard.js'), 'utf8');
    const window = {
        localStorage: {
            getItem() {
                return null;
            },
            setItem() {},
            removeItem() {}
        },
        location: { pathname: '/admin/dashboard/usage' },
        matchMedia() {
            return { addEventListener() {} };
        },
        addEventListener() {}
    };
    const context = {
        console,
        Date,
        Intl,
        setTimeout,
        clearTimeout,
        requestAnimationFrame(callback) {
            callback();
        },
        history: { pushState() {} },
        document: {
            documentElement: {
                removeAttribute() {},
                setAttribute() {}
            },
            getElementById() {
                return null;
            }
        },
        getComputedStyle() {
            return {
                getPropertyValue() {
                    return '';
                }
            };
        },
        window
    };

    vm.createContext(context);
    vm.runInContext(dashboardSource, context);
    return context.dashboard();
}

test('qualifiedModelDisplay keeps provider identity for nested provider model IDs', () => {
    const app = loadDashboardApp();

    assert.equal(
        app.qualifiedModelDisplay({ provider: 'openrouter', model: 'openai/gpt-5-nano' }),
        'openrouter/openai/gpt-5-nano'
    );
    assert.equal(
        app.qualifiedModelDisplay({ provider: 'openai', model: 'gpt-5-nano' }),
        'openai/gpt-5-nano'
    );
});

test('qualifiedModelDisplay does not duplicate an existing exact provider prefix', () => {
    const app = loadDashboardApp();

    assert.equal(
        app.qualifiedModelDisplay({ provider: 'openai', model: 'openai/gpt-5-nano' }),
        'openai/gpt-5-nano'
    );
    assert.equal(
        app.qualifiedResolvedModelDisplay({ provider_name: 'primary-openai', resolved_model: 'gpt-5-nano' }),
        'primary-openai/gpt-5-nano'
    );
});
