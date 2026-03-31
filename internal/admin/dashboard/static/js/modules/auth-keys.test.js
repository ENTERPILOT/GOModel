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

function createTimerHarness() {
    let nextID = 1;
    const timers = new Map();
    return {
        setTimeout(callback, _delay) {
            const id = nextID++;
            timers.set(id, callback);
            return id;
        },
        clearTimeout(id) {
            timers.delete(id);
        },
        runAll() {
            const callbacks = Array.from(timers.values());
            timers.clear();
            callbacks.forEach((callback) => callback());
        }
    };
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

test('copyAuthKeyValue uses navigator.clipboard when available and resets feedback', async () => {
    const timers = createTimerHarness();
    const writes = [];
    const module = createAuthKeysModule({
        setTimeout: timers.setTimeout,
        clearTimeout: timers.clearTimeout,
        window: {
            navigator: {
                clipboard: {
                    writeText(value) {
                        writes.push(value);
                        return Promise.resolve();
                    }
                }
            }
        }
    });

    module.authKeyIssuedValue = 'sk_gom_test';

    await module.copyAuthKeyValue();

    assert.deepEqual(writes, ['sk_gom_test']);
    assert.equal(module.authKeyCopied, true);
    assert.equal(module.authKeyCopyError, false);

    timers.runAll();

    assert.equal(module.authKeyCopied, false);
    assert.equal(module.authKeyCopyError, false);
});

test('copyAuthKeyValue sets an error flag when navigator.clipboard rejects', async () => {
    const timers = createTimerHarness();
    const module = createAuthKeysModule({
        console: {
            error() {}
        },
        setTimeout: timers.setTimeout,
        clearTimeout: timers.clearTimeout,
        window: {
            navigator: {
                clipboard: {
                    writeText() {
                        return Promise.reject(new Error('denied'));
                    }
                }
            }
        }
    });

    module.authKeyIssuedValue = 'sk_gom_test';

    await module.copyAuthKeyValue();

    assert.equal(module.authKeyCopied, false);
    assert.equal(module.authKeyCopyError, true);

    timers.runAll();

    assert.equal(module.authKeyCopied, false);
    assert.equal(module.authKeyCopyError, false);
});

test('copyAuthKeyValue falls back to document.execCommand when clipboard API is unavailable', async () => {
    const timers = createTimerHarness();
    const appended = [];
    const removed = [];
    const fakeBody = {
        appendChild(node) {
            node.parentNode = fakeBody;
            appended.push(node);
        },
        removeChild(node) {
            removed.push(node);
            node.parentNode = null;
        }
    };
    const fakeDocument = {
        body: fakeBody,
        createElement() {
            return {
                value: '',
                style: {},
                setAttribute() {},
                focus() {},
                select() {},
                setSelectionRange() {},
                parentNode: null
            };
        },
        execCommand(command) {
            assert.equal(command, 'copy');
            return true;
        }
    };
    const module = createAuthKeysModule({
        setTimeout: timers.setTimeout,
        clearTimeout: timers.clearTimeout,
        window: {
            document: fakeDocument
        }
    });

    module.authKeyIssuedValue = 'sk_gom_test';

    await module.copyAuthKeyValue();

    assert.equal(appended.length, 1);
    assert.equal(removed.length, 1);
    assert.equal(appended[0].value, 'sk_gom_test');
    assert.equal(module.authKeyCopied, true);
    assert.equal(module.authKeyCopyError, false);
});
