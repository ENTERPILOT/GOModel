const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

function loadGuardrailsModuleFactory() {
    const source = fs.readFileSync(path.join(__dirname, 'guardrails.js'), 'utf8');
    const context = {
        window: {},
        console
    };
    vm.createContext(context);
    vm.runInContext(source, context);
    return context.window.dashboardGuardrailsModule;
}

function createGuardrailsModule() {
    const factory = loadGuardrailsModuleFactory();
    return factory();
}

test('defaultGuardrailForm uses the first available type defaults', () => {
    const module = createGuardrailsModule();
    module.guardrailTypes = [
        {
            type: 'system_prompt',
            defaults: { mode: 'inject', content: '' },
            fields: []
        }
    ];

    const form = module.defaultGuardrailForm();

    assert.equal(form.type, 'system_prompt');
    assert.equal(form.user_path, '');
    assert.equal(JSON.stringify(form.config), JSON.stringify({ mode: 'inject', content: '' }));
});

test('normalizeGuardrailConfig merges stored config over type defaults', () => {
    const module = createGuardrailsModule();
    module.guardrailTypes = [
        {
            type: 'system_prompt',
            defaults: { mode: 'inject', content: '' },
            fields: []
        }
    ];

    const config = module.normalizeGuardrailConfig({ content: 'be careful' }, 'system_prompt');

    assert.equal(JSON.stringify(config), JSON.stringify({ mode: 'inject', content: 'be careful' }));
});

test('normalizeGuardrailConfig returns the input config for unknown types', () => {
    const module = createGuardrailsModule();
    module.guardrailTypes = [
        {
            type: 'system_prompt',
            defaults: { mode: 'inject', content: '' },
            fields: []
        }
    ];

    const config = module.normalizeGuardrailConfig({ content: 'test' }, 'unknown_type');

    assert.equal(JSON.stringify(config), JSON.stringify({ content: 'test' }));
});

test('filteredGuardrails matches user_path values', () => {
    const module = createGuardrailsModule();
    module.guardrails = [
        { name: 'policy', type: 'system_prompt', user_path: '/team/alpha', summary: 'be careful' }
    ];
    module.guardrailFilter = 'alpha';

    assert.equal(module.filteredGuardrails.length, 1);
    assert.equal(module.filteredGuardrails[0].name, 'policy');
});
