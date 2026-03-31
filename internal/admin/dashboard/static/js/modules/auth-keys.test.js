const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

function loadAuthKeysModuleFactory(overrides = {}) {
    const source = fs.readFileSync(path.join(__dirname, 'auth-keys.js'), 'utf8');
    const window = {
        ...(overrides.window || {})
    };
    const context = {
        console,
        setTimeout,
        clearTimeout,
        ...overrides,
        window
    };
    vm.createContext(context);
    vm.runInContext(source, context);
    return context.window.dashboardAuthKeysModule;
}

function createAuthKeysModule(overrides) {
    const factory = loadAuthKeysModuleFactory(overrides);
    return factory();
}

test('submitAuthKeyForm serializes date-only expirations to the end of the selected UTC day', async () => {
    const requests = [];
    const module = createAuthKeysModule({
        fetch: async (url, options) => {
            requests.push({ url, options });
            return {
                status: 201,
                async json() {
                    return { value: 'sk_gom_test' };
                }
            };
        }
    });

    module.headers = () => ({ 'Content-Type': 'application/json' });
    module.fetchAuthKeys = async () => {};
    module.authKeyForm = {
        name: 'ci-deploy',
        description: '',
        expires_at: '2026-04-01'
    };

    await module.submitAuthKeyForm();

    assert.equal(requests.length, 1);
    assert.equal(requests[0].url, '/admin/api/v1/auth-keys');
    assert.equal(
        JSON.parse(requests[0].options.body).expires_at,
        '2026-04-01T23:59:59Z'
    );
});
