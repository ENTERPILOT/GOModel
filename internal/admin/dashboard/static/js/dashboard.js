// GOModel Dashboard — Alpine.js + Chart.js logic

function dashboard(flowEnabled) {
    const base = {
        // State
        flowEnabled: !!flowEnabled,
        page: 'overview',
        days: '30',
        loading: false,
        authError: false,
        needsAuth: false,
        apiKey: '',
        theme: 'system',
        sidebarCollapsed: false,

        // Date picker
        datePickerOpen: false,
        selectedPreset: '30',
        customStartDate: null,
        customEndDate: null,
        selectingDate: 'start',
        calendarMonth: new Date(),
        cursorHint: { show: false, x: 0, y: 0 },

        // Interval
        interval: 'daily',

        // Data
        summary: { total_requests: 0, total_input_tokens: 0, total_output_tokens: 0, total_tokens: 0, total_input_cost: null, total_output_cost: null, total_cost: null },
        daily: [],
        models: [],
        categories: [],
        activeCategory: 'all',

        // Filters
        modelFilter: '',

        // Chart
        chart: null,

        // Usage page state
        usageMode: 'tokens',
        modelUsage: [],
        usageLog: { entries: [], total: 0, limit: 50, offset: 0 },
        usageLogSearch: '',
        usageLogModel: '',
        usageLogProvider: '',
        usageBarChart: null,

        // Audit page state
        auditLog: { entries: [], total: 0, limit: 25, offset: 0 },
        auditSearch: '',
        auditModel: '',
        auditProvider: '',
        auditMethod: '',
        auditPath: '',
        auditStatusCode: '',
        auditStream: '',
        auditFetchToken: 0,

        // Conversation drawer state
        conversationOpen: false,
        conversationLoading: false,
        conversationError: '',
        conversationAnchorID: '',
        conversationEntries: [],
        conversationMessages: [],
        conversationRequestToken: 0,
        conversationReturnFocusEl: null,
        bodyPointerStart: null,

        // Flow page state
        flowPlans: { plans: [], writable: false, source_mode: 'yaml' },
        flowExecutions: { entries: [], total: 0, limit: 25, offset: 0 },
        flowSearch: '',
        flowEditor: null,

        _parseRoute(pathname) {
            const path = pathname.replace(/\/$/, '');
            const rest = path.replace('/admin/dashboard', '').replace(/^\//, '');
            const parts = rest.split('/');
            const page = (['overview', 'usage', 'models', 'audit', 'flows'].includes(parts[0])) ? parts[0] : 'overview';
            const sub = parts[1] || null;
            return { page, sub };
        },

        init() {
            this.apiKey = localStorage.getItem('gomodel_api_key') || '';
            this.theme = localStorage.getItem('gomodel_theme') || 'system';
            this.sidebarCollapsed = localStorage.getItem('gomodel_sidebar_collapsed') === 'true';
            this.applyTheme();

            const { page, sub } = this._parseRoute(window.location.pathname);
            this.page = page;
            this.flowEditor = this.emptyFlowPlan();
            if (page === 'usage' && sub === 'costs') this.usageMode = 'costs';
            if (page === 'audit') this.fetchAuditLog(true);
            if (page === 'flows' && this.flowEnabled) this.fetchFlowPage();

            window.addEventListener('popstate', () => {
                const { page: p, sub: s } = this._parseRoute(window.location.pathname);
                this.page = p;
                if (p === 'usage') {
                    this.usageMode = s === 'costs' ? 'costs' : 'tokens';
                    this.fetchUsagePage();
                }
                if (p === 'overview') this.renderChart();
                if (p === 'audit') this.fetchAuditLog(true);
                if (p === 'flows' && this.flowEnabled) this.fetchFlowPage();
            });

            window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
                if (this.theme === 'system') {
                    this.renderChart();
                }
            });

            this.fetchAll();
        },

        toggleSidebar() {
            this.sidebarCollapsed = !this.sidebarCollapsed;
            localStorage.setItem('gomodel_sidebar_collapsed', this.sidebarCollapsed);
            setTimeout(() => this.renderChart(), 220);
        },

        navigate(page) {
            this.page = page;
            if (page === 'usage') this.usageMode = 'tokens';
            history.pushState(null, '', '/admin/dashboard/' + page);
            if (page === 'overview') this.renderChart();
            if (page === 'usage') this.fetchUsagePage();
            if (page === 'audit') this.fetchAuditLog(true);
            if (page === 'flows' && this.flowEnabled) this.fetchFlowPage();
        },

        setTheme(t) {
            this.theme = t;
            localStorage.setItem('gomodel_theme', t);
            this.applyTheme();
            this.renderChart();
            this.renderBarChart();
        },

        toggleTheme() {
            const order = ['light', 'system', 'dark'];
            this.setTheme(order[(order.indexOf(this.theme) + 1) % order.length]);
        },

        applyTheme() {
            const root = document.documentElement;
            if (this.theme === 'system') {
                root.removeAttribute('data-theme');
            } else {
                root.setAttribute('data-theme', this.theme);
            }
        },

        cssVar(name) {
            return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
        },

        chartColors() {
            return {
                grid: this.cssVar('--chart-grid'),
                text: this.cssVar('--chart-text'),
                tooltipBg: this.cssVar('--chart-tooltip-bg'),
                tooltipBorder: this.cssVar('--chart-tooltip-border'),
                tooltipText: this.cssVar('--chart-tooltip-text')
            };
        },

        saveApiKey() {
            if (this.apiKey) {
                localStorage.setItem('gomodel_api_key', this.apiKey);
            } else {
                localStorage.removeItem('gomodel_api_key');
            }
        },

        headers() {
            const h = { 'Content-Type': 'application/json' };
            if (this.apiKey) {
                h.Authorization = 'Bearer ' + this.apiKey;
            }
            return h;
        },

        async fetchAll() {
            this.loading = true;
            this.authError = false;
            this.needsAuth = false;
            await Promise.all([this.fetchUsage(), this.fetchModels(), this.fetchCategories()]);
            this.loading = false;
        },

        handleFetchResponse(res, label) {
            if (res.status === 401) {
                this.authError = true;
                this.needsAuth = true;
                return false;
            }
            if (!res.ok) {
                console.error(`Failed to fetch ${label}: ${res.status} ${res.statusText}`);
                return false;
            }
            return true;
        },

        _formatDate(date) {
            return date.getFullYear() + '-' +
                String(date.getMonth() + 1).padStart(2, '0') + '-' +
                String(date.getDate()).padStart(2, '0');
        },

        async fetchModels() {
            try {
                let url = '/admin/api/v1/models';
                if (this.activeCategory && this.activeCategory !== 'all') {
                    url += '?category=' + encodeURIComponent(this.activeCategory);
                }
                const res = await fetch(url, { headers: this.headers() });
                if (!this.handleFetchResponse(res, 'models')) {
                    this.models = [];
                    return;
                }
                this.models = await res.json();
            } catch (e) {
                console.error('Failed to fetch models:', e);
                this.models = [];
            }
        },

        async fetchCategories() {
            try {
                const res = await fetch('/admin/api/v1/models/categories', { headers: this.headers() });
                if (!this.handleFetchResponse(res, 'categories')) {
                    this.categories = [];
                    return;
                }
                this.categories = await res.json();
            } catch (e) {
                console.error('Failed to fetch categories:', e);
                this.categories = [];
            }
        },

        selectCategory(cat) {
            this.activeCategory = cat;
            this.modelFilter = '';
            this.fetchModels();
        },

        get filteredModels() {
            if (!this.modelFilter) return this.models;
            const f = this.modelFilter.toLowerCase();
            return this.models.filter((m) =>
                (m.model?.id ?? '').toLowerCase().includes(f) ||
                (m.provider_type ?? '').toLowerCase().includes(f) ||
                (m.model?.owned_by ?? '').toLowerCase().includes(f) ||
                (m.model?.metadata?.modes ?? []).join(',').toLowerCase().includes(f) ||
                (m.model?.metadata?.categories ?? []).join(',').toLowerCase().includes(f)
            );
        },

        formatNumber(n) {
            if (n == null || n === undefined) return '-';
            return n.toLocaleString();
        },

        formatCost(v) {
            if (v == null || v === undefined) return 'N/A';
            return '$' + v.toFixed(4);
        },

        formatCostTooltip(entry) {
            const lines = [];
            lines.push('Input: ' + this.formatCost(entry.input_cost));
            lines.push('Output: ' + this.formatCost(entry.output_cost));
            if (entry.raw_data) {
                lines.push('');
                for (const [key, value] of Object.entries(entry.raw_data)) {
                    const label = key.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
                    lines.push(label + ': ' + this.formatNumber(value));
                }
            }
            return lines.join('\n');
        },

        formatPrice(v) {
            if (v == null || v === undefined) return '\u2014';
            return '$' + v.toFixed(2);
        },

        formatPriceFine(v) {
            if (v == null || v === undefined) return '\u2014';
            if (v < 0.01) return '$' + v.toFixed(6);
            return '$' + v.toFixed(4);
        },

        categoryCount(cat) {
            const entry = this.categories.find((c) => c.category === cat);
            return entry ? entry.count : 0;
        },

        formatTokensShort(n) {
            if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M';
            if (n >= 1000) return (n / 1000).toFixed(1) + 'K';
            return String(n);
        },

        formatTimestamp(ts) {
            if (!ts) return '-';
            const d = new Date(ts);
            return d.getFullYear() + '-' +
                String(d.getMonth() + 1).padStart(2, '0') + '-' +
                String(d.getDate()).padStart(2, '0') + ' ' +
                String(d.getHours()).padStart(2, '0') + ':' +
                String(d.getMinutes()).padStart(2, '0') + ':' +
                String(d.getSeconds()).padStart(2, '0');
        },

        emptyFlowPlan() {
            return {
                id: '',
                name: '',
                description: '',
                enabled: true,
                priority: 100,
                match: { model: '' },
                spec: {
                    guardrails: { mode: 'append', rules: [] },
                    retry: { max_retries: 0, initial_backoff: '1s', max_backoff: '30s' },
                    failover: { enabled: false, strategy: 'same_model' }
                },
                source: 'db'
            };
        },

        flowMatchLabel(plan) {
            if (plan?.match?.model) return plan.match.model;
            return 'app-wide';
        },

        newFlowPlan() {
            this.flowEditor = this.emptyFlowPlan();
        },

        selectFlowPlan(plan) {
            this.flowEditor = JSON.parse(JSON.stringify(plan));
            if (!this.flowEditor.spec) this.flowEditor.spec = {};
            if (!this.flowEditor.spec.guardrails) this.flowEditor.spec.guardrails = { mode: 'append', rules: [] };
            if (!this.flowEditor.spec.guardrails.rules) this.flowEditor.spec.guardrails.rules = [];
            if (!this.flowEditor.spec.retry) this.flowEditor.spec.retry = {};
            if (!this.flowEditor.spec.failover) this.flowEditor.spec.failover = { enabled: false, strategy: 'same_model' };
            if (!this.flowEditor.match) this.flowEditor.match = { model: '' };
        },

        addFlowGuardrail() {
            if (!this.flowEditor?.spec?.guardrails?.rules) this.flowEditor.spec.guardrails.rules = [];
            this.flowEditor.spec.guardrails.rules.push({
                name: '',
                type: 'system_prompt',
                order: 0,
                system_prompt: { mode: 'inject', content: '' }
            });
        },

        removeFlowGuardrail(index) {
            this.flowEditor.spec.guardrails.rules.splice(index, 1);
        },

        normalizeFlowPayload() {
            const payload = JSON.parse(JSON.stringify(this.flowEditor || this.emptyFlowPlan()));
            if (!payload.match) payload.match = { model: '' };
            if (!payload.spec) payload.spec = {};
            if (!payload.spec.guardrails) payload.spec.guardrails = { mode: 'append', rules: [] };
            if (!payload.spec.retry) payload.spec.retry = {};
            if (!payload.spec.failover) payload.spec.failover = {};
            payload.name = (payload.name || '').trim();
            payload.description = (payload.description || '').trim();
            payload.match.model = (payload.match.model || '').trim();
            payload.spec.guardrails.mode = payload.spec.guardrails.mode || 'append';
            payload.spec.guardrails.rules = (payload.spec.guardrails.rules || []).map((rule) => ({
                name: (rule.name || '').trim(),
                type: rule.type || 'system_prompt',
                order: Number(rule.order || 0),
                system_prompt: {
                    mode: rule.system_prompt?.mode || 'inject',
                    content: rule.system_prompt?.content || ''
                }
            })).filter((rule) => rule.name || rule.system_prompt.content);
            if (payload.spec.retry.max_retries === '' || payload.spec.retry.max_retries == null) delete payload.spec.retry.max_retries;
            if (!payload.spec.retry.initial_backoff) delete payload.spec.retry.initial_backoff;
            if (!payload.spec.retry.max_backoff) delete payload.spec.retry.max_backoff;
            if (payload.spec.failover.enabled == null) payload.spec.failover.enabled = false;
            return payload;
        },

        async fetchFlowPlans() {
            if (!this.flowEnabled) return;
            try {
                const res = await fetch('/admin/api/v1/flows/plans', { headers: this.headers() });
                if (!this.handleFetchResponse(res, 'request flow plans')) {
                    this.flowPlans = { plans: [], writable: false, source_mode: 'yaml' };
                    return;
                }
                const result = await res.json();
                this.flowPlans = result;
                if (!this.flowEditor?.id && result.plans?.length > 0) {
                    this.selectFlowPlan(result.plans[0]);
                }
            } catch (e) {
                console.error('Failed to fetch request flow plans:', e);
                this.flowPlans = { plans: [], writable: false, source_mode: 'yaml' };
            }
        },

        async fetchFlowExecutions(reset = false) {
            if (!this.flowEnabled) return;
            if (reset) this.flowExecutions.offset = 0;
            const params = new URLSearchParams({
                limit: String(this.flowExecutions.limit),
                offset: String(this.flowExecutions.offset)
            });
            if (this.flowSearch) params.set('search', this.flowSearch);
            try {
                const res = await fetch('/admin/api/v1/flows/executions?' + params.toString(), { headers: this.headers() });
                if (!this.handleFetchResponse(res, 'request flow executions')) {
                    this.flowExecutions = { entries: [], total: 0, limit: 25, offset: 0 };
                    return;
                }
                this.flowExecutions = await res.json();
            } catch (e) {
                console.error('Failed to fetch request flow executions:', e);
                this.flowExecutions = { entries: [], total: 0, limit: 25, offset: 0 };
            }
        },

        async fetchFlowPage() {
            await Promise.all([this.fetchFlowPlans(), this.fetchFlowExecutions()]);
        },

        async saveFlowPlan() {
            if (!this.flowPlans.writable) return;
            const payload = this.normalizeFlowPayload();
            const id = payload.id || (payload.name || 'flow').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '') || 'flow-plan';
            payload.id = id;
            try {
                const res = await fetch('/admin/api/v1/flows/plans/' + encodeURIComponent(id), {
                    method: 'PUT',
                    headers: this.headers(),
                    body: JSON.stringify(payload)
                });
                if (!this.handleFetchResponse(res, 'save request flow plan')) return;
                const saved = await res.json();
                await this.fetchFlowPlans();
                this.selectFlowPlan(saved);
            } catch (e) {
                console.error('Failed to save request flow plan:', e);
            }
        },

        async deleteFlowPlan(id) {
            if (!this.flowPlans.writable || !id) return;
            try {
                const res = await fetch('/admin/api/v1/flows/plans/' + encodeURIComponent(id), {
                    method: 'DELETE',
                    headers: this.headers()
                });
                if (res.status !== 204 && !this.handleFetchResponse(res, 'delete request flow plan')) return;
                this.newFlowPlan();
                await this.fetchFlowPlans();
            } catch (e) {
                console.error('Failed to delete request flow plan:', e);
            }
        },

        flowExecutionsPrevPage() {
            if (this.flowExecutions.offset === 0) return;
            this.flowExecutions.offset = Math.max(0, this.flowExecutions.offset - this.flowExecutions.limit);
            this.fetchFlowExecutions();
        },

        flowExecutionsNextPage() {
            if (this.flowExecutions.offset + this.flowExecutions.limit >= this.flowExecutions.total) return;
            this.flowExecutions.offset += this.flowExecutions.limit;
            this.fetchFlowExecutions();
        }
    };

    const moduleFactories = [
        typeof dashboardDatePickerModule === 'function' ? dashboardDatePickerModule : null,
        typeof dashboardUsageModule === 'function' ? dashboardUsageModule : null,
        typeof dashboardAuditListModule === 'function' ? dashboardAuditListModule : null,
        typeof dashboardConversationDrawerModule === 'function' ? dashboardConversationDrawerModule : null,
        typeof dashboardChartsModule === 'function' ? dashboardChartsModule : null
    ];

    return moduleFactories.reduce((app, factory) => {
        if (!factory) return app;
        return Object.assign(app, factory());
    }, base);
}
