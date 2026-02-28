// GOModel Dashboard — Alpine.js + Chart.js logic

function dashboard() {
    return {
        // State
        page: 'overview',
        days: '30',
        loading: false,
        authError: false,
        needsAuth: false,
        apiKey: '',
        theme: 'system', // 'system', 'light', 'dark'
        sidebarCollapsed: false,

        // Date picker
        datePickerOpen: false,
        selectedPreset: '30',
        customStartDate: null,
        customEndDate: null,
        selectingDate: 'start', // 'start' or 'end'
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
        conversationOpen: false,
        conversationLoading: false,
        conversationError: '',
        conversationAnchorID: '',
        conversationEntries: [],
        conversationMessages: [],
        bodyPointerStart: null,

        _parseRoute(pathname) {
            const path = pathname.replace(/\/$/, '');
            const rest = path.replace('/admin/dashboard', '').replace(/^\//, '');
            const parts = rest.split('/');
            const page = (['overview', 'usage', 'models', 'audit'].includes(parts[0])) ? parts[0] : 'overview';
            const sub = parts[1] || null;
            return { page, sub };
        },

        init() {
            this.apiKey = localStorage.getItem('gomodel_api_key') || '';
            this.theme = localStorage.getItem('gomodel_theme') || 'system';
            this.sidebarCollapsed = localStorage.getItem('gomodel_sidebar_collapsed') === 'true';
            this.applyTheme();

            // Parse initial page and sub-path from URL
            const { page, sub } = this._parseRoute(window.location.pathname);
            this.page = page;
            if (page === 'usage' && sub === 'costs') this.usageMode = 'costs';

            // Handle browser back/forward
            window.addEventListener('popstate', () => {
                const { page: p, sub: s } = this._parseRoute(window.location.pathname);
                this.page = p;
                if (p === 'usage') {
                    this.usageMode = s === 'costs' ? 'costs' : 'tokens';
                    this.fetchUsagePage();
                }
                if (p === 'overview') this.renderChart();
                if (p === 'audit') this.fetchAuditLog(true);
            });

            // Re-render chart when system theme changes (only matters in 'system' mode)
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
            // Re-render chart after CSS transition so Chart.js picks up the new width
            setTimeout(() => this.renderChart(), 220);
        },

        // Date picker methods
        toggleDatePicker() {
            this.datePickerOpen = !this.datePickerOpen;
            if (this.datePickerOpen) {
                this.calendarMonth = new Date();
                this.selectingDate = 'start';
            }
        },

        closeDatePicker() {
            this.datePickerOpen = false;
            this.cursorHint.show = false;
        },

        onCalendarMouseMove(e) {
            this.cursorHint = { show: true, x: e.clientX, y: e.clientY };
        },

        onCalendarMouseLeave() {
            this.cursorHint.show = false;
        },

        selectPreset(days) {
            this.selectedPreset = days;
            this.customStartDate = null;
            this.customEndDate = null;
            this.selectingDate = 'start';
            this.days = days;
            this.fetchUsage();
            this.closeDatePicker();
        },

        selectionHint() {
            return this.selectingDate === 'end' ? 'Select end date' : 'Select start date';
        },

        dateRangeLabel() {
            if (this.selectedPreset) return 'Last ' + this.selectedPreset + ' days';
            if (this.customStartDate && this.customEndDate) {
                return this.formatDateShort(this.customStartDate) + ' \u2013 ' + this.formatDateShort(this.customEndDate);
            }
            if (this.customStartDate) {
                return this.formatDateShort(this.customStartDate) + ' \u2013 ...';
            }
            return 'Last 30 days';
        },

        formatDateShort(date) {
            const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
            return months[date.getMonth()] + ' ' + date.getDate() + ', ' + date.getFullYear();
        },

        calendarTitle(offset) {
            const d = new Date(this.calendarMonth.getFullYear(), this.calendarMonth.getMonth() + offset, 1);
            const months = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];
            return months[d.getMonth()] + ' ' + d.getFullYear();
        },

        calendarDays(offset) {
            const year = this.calendarMonth.getFullYear();
            const month = this.calendarMonth.getMonth() + offset;
            const first = new Date(year, month, 1);
            const last = new Date(year, month + 1, 0);
            // Monday = 0, Sunday = 6
            let startDay = (first.getDay() + 6) % 7;
            const days = [];

            // Padding days from previous month
            const prevLast = new Date(year, month, 0);
            for (let i = startDay - 1; i >= 0; i--) {
                const d = prevLast.getDate() - i;
                days.push({ day: d, date: new Date(year, month - 1, d), current: false, key: 'p' + d });
            }

            // Current month days
            for (let d = 1; d <= last.getDate(); d++) {
                days.push({ day: d, date: new Date(year, month, d), current: true, key: 'c' + d });
            }

            // Padding days from next month
            const remaining = 42 - days.length;
            for (let d = 1; d <= remaining; d++) {
                days.push({ day: d, date: new Date(year, month + 1, d), current: false, key: 'n' + d });
            }

            return days;
        },

        prevMonth() {
            this.calendarMonth = new Date(this.calendarMonth.getFullYear(), this.calendarMonth.getMonth() - 1, 1);
        },

        nextMonth() {
            const next = new Date(this.calendarMonth.getFullYear(), this.calendarMonth.getMonth() + 1, 1);
            const today = new Date();
            // Don't navigate past current month
            if (next.getFullYear() < today.getFullYear() ||
                (next.getFullYear() === today.getFullYear() && next.getMonth() <= today.getMonth())) {
                this.calendarMonth = next;
            }
        },

        isCurrentMonth() {
            const today = new Date();
            return this.calendarMonth.getFullYear() === today.getFullYear()
                && this.calendarMonth.getMonth() === today.getMonth();
        },

        selectCalendarDay(day) {
            if (!day.current || this.isFutureDay(day)) return;
            const clicked = new Date(day.date);
            clicked.setHours(0, 0, 0, 0);
            this.selectedPreset = null;

            if (this.selectingDate === 'start') {
                this.customStartDate = clicked;
                // Keep existing end date; if it's now before start, move it to start
                if (this.customEndDate && this.customEndDate < clicked) {
                    this.customEndDate = clicked;
                }
                // If no end date yet, default to today
                if (!this.customEndDate) {
                    const today = new Date();
                    today.setHours(0, 0, 0, 0);
                    this.customEndDate = today;
                }
                this.selectingDate = 'end';
                this.fetchUsage();
            } else {
                // Selecting end date
                if (clicked < this.customStartDate) {
                    // Swap: treat click as new start, old start becomes end
                    this.customEndDate = this.customStartDate;
                    this.customStartDate = clicked;
                } else {
                    this.customEndDate = clicked;
                }
                this.selectingDate = 'start';
                this.fetchUsage();
                this.closeDatePicker();
            }
        },

        isToday(day) {
            if (!day.current) return false;
            const today = new Date();
            return day.date.getFullYear() === today.getFullYear() &&
                   day.date.getMonth() === today.getMonth() &&
                   day.date.getDate() === today.getDate();
        },

        isFutureDay(day) {
            const today = new Date();
            today.setHours(23, 59, 59, 999);
            return day.date > today;
        },

        isRangeStart(day) {
            if (!day.current) return false;
            const start = this._rangeStart();
            if (!start) return false;
            return day.date.getFullYear() === start.getFullYear() &&
                   day.date.getMonth() === start.getMonth() &&
                   day.date.getDate() === start.getDate();
        },

        isRangeEnd(day) {
            if (!day.current) return false;
            const end = this._rangeEnd();
            if (!end) return false;
            return day.date.getFullYear() === end.getFullYear() &&
                   day.date.getMonth() === end.getMonth() &&
                   day.date.getDate() === end.getDate();
        },

        isInRange(day) {
            if (!day.current) return false;
            const start = this._rangeStart();
            const end = this._rangeEnd();
            if (!start || !end) return false;
            const dayDate = new Date(day.date);
            dayDate.setHours(0, 0, 0, 0);
            return dayDate >= start && dayDate <= end;
        },

        _rangeStart() {
            if (this.customStartDate) return this.customStartDate;
            if (this.selectedPreset) {
                const s = new Date();
                s.setHours(0, 0, 0, 0);
                s.setDate(s.getDate() - (parseInt(this.selectedPreset, 10) - 1));
                return s;
            }
            return null;
        },

        _rangeEnd() {
            if (this.customEndDate) return this.customEndDate;
            if (this.customStartDate || this.selectedPreset) {
                const t = new Date();
                t.setHours(0, 0, 0, 0);
                return t;
            }
            return null;
        },

        // Interval methods
        setInterval(val) {
            this.interval = val;
            this.fetchUsage();
        },

        chartTitle() {
            const titles = { daily: 'Daily', weekly: 'Weekly', monthly: 'Monthly', yearly: 'Yearly' };
            return (titles[this.interval] || 'Daily') + ' Token Usage';
        },

        navigate(page) {
            this.page = page;
            if (page === 'usage') this.usageMode = 'tokens';
            history.pushState(null, '', '/admin/dashboard/' + page);
            if (page === 'overview') this.renderChart();
            if (page === 'usage') { this.fetchUsagePage(); }
            if (page === 'audit') { this.fetchAuditLog(true); }
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
                tooltipText: this.cssVar('--chart-tooltip-text'),
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
                h['Authorization'] = 'Bearer ' + this.apiKey;
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

        async fetchUsage() {
            try {
                var queryStr;
                if (this.customStartDate && this.customEndDate) {
                    queryStr = 'start_date=' + this._formatDate(this.customStartDate) +
                               '&end_date=' + this._formatDate(this.customEndDate);
                } else {
                    queryStr = 'days=' + this.days;
                }
                queryStr += '&interval=' + this.interval;

                const [summaryRes, dailyRes] = await Promise.all([
                    fetch('/admin/api/v1/usage/summary?' + queryStr, { headers: this.headers() }),
                    fetch('/admin/api/v1/usage/daily?' + queryStr, { headers: this.headers() })
                ]);

                if (!this.handleFetchResponse(summaryRes, 'usage summary') ||
                    !this.handleFetchResponse(dailyRes, 'usage daily')) {
                    return;
                }

                this.summary = await summaryRes.json();
                this.daily = await dailyRes.json();
                this.renderChart();
                if (this.page === 'usage') this.fetchUsagePage();
                if (this.page === 'audit') this.fetchAuditLog(true);
            } catch (e) {
                console.error('Failed to fetch usage:', e);
            }
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

        fillMissingDays(daily) {
            // For non-daily intervals, return data as-is (no gap filling)
            if (this.interval !== 'daily') {
                return daily;
            }

            const byDate = {};
            daily.forEach(d => { byDate[d.date] = d; });
            const end = this.customEndDate ? new Date(this.customEndDate) : new Date();
            end.setHours(0, 0, 0, 0);
            const start = this.customStartDate ? new Date(this.customStartDate) : new Date(end);
            if (!this.customStartDate) {
                start.setDate(start.getDate() - (parseInt(this.days, 10) - 1));
            }
            const result = [];
            for (let d = new Date(start); d <= end; d.setDate(d.getDate() + 1)) {
                const key = d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0');
                result.push(byDate[key] || { date: key, input_tokens: 0, output_tokens: 0, total_tokens: 0, requests: 0 });
            }
            return result;
        },

        renderChart(retries) {
            if (retries === undefined) retries = 3;
            this.$nextTick(() => {
                if (this.chart) {
                    this.chart.destroy();
                    this.chart = null;
                }

                if (this.daily.length === 0) return;
                if (this.page !== 'overview') return;

                const canvas = document.getElementById('usageChart');
                if (!canvas || canvas.offsetWidth === 0) {
                    if (retries > 0) {
                        setTimeout(() => this.renderChart(retries - 1), 100);
                    }
                    return;
                }

                const colors = this.chartColors();
                const filled = this.fillMissingDays(this.daily);
                const labels = filled.map(d => d.date);
                const inputData = filled.map(d => d.input_tokens);
                const outputData = filled.map(d => d.output_tokens);

                this.chart = new Chart(canvas, {
                    type: 'line',
                    data: {
                        labels: labels,
                        datasets: [
                            {
                                label: 'Input Tokens',
                                data: inputData,
                                borderColor: '#c2845a',
                                backgroundColor: 'rgba(194, 132, 90, 0.1)',
                                fill: true,
                                tension: 0.3,
                                pointRadius: 3,
                                pointHoverRadius: 5
                            },
                            {
                                label: 'Output Tokens',
                                data: outputData,
                                borderColor: '#7a9e7e',
                                backgroundColor: 'rgba(122, 158, 126, 0.1)',
                                fill: true,
                                tension: 0.3,
                                pointRadius: 3,
                                pointHoverRadius: 5
                            }
                        ]
                    },
                    options: {
                        responsive: true,
                        maintainAspectRatio: false,
                        animation: { duration: 0 },
                        interaction: { mode: 'index', intersect: false },
                        plugins: {
                            legend: {
                                labels: { color: colors.text, font: { size: 12 } }
                            },
                            tooltip: {
                                backgroundColor: colors.tooltipBg,
                                borderColor: colors.tooltipBorder,
                                borderWidth: 1,
                                titleColor: colors.tooltipText,
                                bodyColor: colors.tooltipText,
                                callbacks: {
                                    label: function(c) {
                                        return c.dataset.label + ': ' + c.parsed.y.toLocaleString();
                                    }
                                }
                            }
                        },
                        scales: {
                            x: {
                                grid: { color: colors.grid },
                                ticks: { color: colors.text, font: { size: 11 }, maxTicksLimit: 10 }
                            },
                            y: {
                                beginAtZero: true,
                                grid: { color: colors.grid },
                                ticks: {
                                    color: colors.text,
                                    font: { size: 11 },
                                    callback: function(value) {
                                        if (value >= 1000000) return (value / 1000000).toFixed(1) + 'M';
                                        if (value >= 1000) return (value / 1000).toFixed(1) + 'K';
                                        return value;
                                    }
                                }
                            }
                        }
                    }
                });
            });
        },

        get filteredModels() {
            if (!this.modelFilter) return this.models;
            const f = this.modelFilter.toLowerCase();
            return this.models.filter(m =>
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
            let lines = [];
            lines.push('Input: ' + this.formatCost(entry.input_cost));
            lines.push('Output: ' + this.formatCost(entry.output_cost));
            if (entry.raw_data) {
                lines.push('');
                for (const [key, value] of Object.entries(entry.raw_data)) {
                    const label = key.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase());
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
            const entry = this.categories.find(c => c.category === cat);
            return entry ? entry.count : 0;
        },

        // Usage page methods
        _usageQueryStr() {
            if (this.customStartDate && this.customEndDate) {
                return 'start_date=' + this._formatDate(this.customStartDate) +
                       '&end_date=' + this._formatDate(this.customEndDate);
            }
            return 'days=' + this.days;
        },

        async fetchUsagePage() {
            await Promise.all([this.fetchModelUsage(), this.fetchUsageLog(true)]);
            this.renderBarChart();
        },

        async fetchModelUsage() {
            try {
                const res = await fetch('/admin/api/v1/usage/models?' + this._usageQueryStr(), { headers: this.headers() });
                if (!this.handleFetchResponse(res, 'usage models')) {
                    this.modelUsage = [];
                    return;
                }
                this.modelUsage = await res.json();
            } catch (e) {
                console.error('Failed to fetch model usage:', e);
                this.modelUsage = [];
            }
        },

        async fetchUsageLog(resetOffset) {
            try {
                if (resetOffset) this.usageLog.offset = 0;
                let qs = this._usageQueryStr();
                qs += '&limit=' + this.usageLog.limit + '&offset=' + this.usageLog.offset;
                if (this.usageLogSearch) qs += '&search=' + encodeURIComponent(this.usageLogSearch);
                if (this.usageLogModel) qs += '&model=' + encodeURIComponent(this.usageLogModel);
                if (this.usageLogProvider) qs += '&provider=' + encodeURIComponent(this.usageLogProvider);

                const res = await fetch('/admin/api/v1/usage/log?' + qs, { headers: this.headers() });
                if (!this.handleFetchResponse(res, 'usage log')) {
                    this.usageLog = { entries: [], total: 0, limit: 50, offset: 0 };
                    return;
                }
                this.usageLog = await res.json();
                if (!this.usageLog.entries) this.usageLog.entries = [];
            } catch (e) {
                console.error('Failed to fetch usage log:', e);
                this.usageLog = { entries: [], total: 0, limit: 50, offset: 0 };
            }
        },

        toggleUsageMode(mode) {
            this.usageMode = mode;
            const url = mode === 'costs' ? '/admin/dashboard/usage/costs' : '/admin/dashboard/usage';
            history.pushState(null, '', url);
            this.renderBarChart();
        },

        usageLogNextPage() {
            if (this.usageLog.offset + this.usageLog.limit < this.usageLog.total) {
                this.usageLog.offset += this.usageLog.limit;
                this.fetchUsageLog(false);
            }
        },

        usageLogPrevPage() {
            if (this.usageLog.offset > 0) {
                this.usageLog.offset = Math.max(0, this.usageLog.offset - this.usageLog.limit);
                this.fetchUsageLog(false);
            }
        },

        usageLogModelOptions() {
            const set = new Set();
            this.modelUsage.forEach(m => { set.add(m.model); });
            return [...set].sort();
        },

        usageLogProviderOptions() {
            const set = new Set();
            this.modelUsage.forEach(m => { set.add(m.provider); });
            return [...set].sort();
        },

        // Audit page methods
        _auditQueryStr() {
            if (this.customStartDate && this.customEndDate) {
                return 'start_date=' + this._formatDate(this.customStartDate) +
                    '&end_date=' + this._formatDate(this.customEndDate);
            }
            return 'days=' + this.days;
        },

        async fetchAuditLog(resetOffset) {
            try {
                if (resetOffset) this.auditLog.offset = 0;
                let qs = this._auditQueryStr();
                qs += '&limit=' + this.auditLog.limit + '&offset=' + this.auditLog.offset;
                if (this.auditSearch) qs += '&search=' + encodeURIComponent(this.auditSearch);
                if (this.auditModel) qs += '&model=' + encodeURIComponent(this.auditModel);
                if (this.auditProvider) qs += '&provider=' + encodeURIComponent(this.auditProvider);
                if (this.auditMethod) qs += '&method=' + encodeURIComponent(this.auditMethod);
                if (this.auditPath) qs += '&path=' + encodeURIComponent(this.auditPath);
                if (this.auditStatusCode) qs += '&status_code=' + encodeURIComponent(this.auditStatusCode);
                if (this.auditStream) qs += '&stream=' + encodeURIComponent(this.auditStream);

                const res = await fetch('/admin/api/v1/audit/log?' + qs, { headers: this.headers() });
                if (!this.handleFetchResponse(res, 'audit log')) {
                    this.auditLog = { entries: [], total: 0, limit: 25, offset: 0 };
                    return;
                }
                this.auditLog = await res.json();
                if (!this.auditLog.entries) this.auditLog.entries = [];
            } catch (e) {
                console.error('Failed to fetch audit log:', e);
                this.auditLog = { entries: [], total: 0, limit: 25, offset: 0 };
            }
        },

        clearAuditFilters() {
            this.auditSearch = '';
            this.auditModel = '';
            this.auditProvider = '';
            this.auditMethod = '';
            this.auditPath = '';
            this.auditStatusCode = '';
            this.auditStream = '';
            this.fetchAuditLog(true);
        },

        auditLogNextPage() {
            if (this.auditLog.offset + this.auditLog.limit < this.auditLog.total) {
                this.auditLog.offset += this.auditLog.limit;
                this.fetchAuditLog(false);
            }
        },

        auditLogPrevPage() {
            if (this.auditLog.offset > 0) {
                this.auditLog.offset = Math.max(0, this.auditLog.offset - this.auditLog.limit);
                this.fetchAuditLog(false);
            }
        },

        formatDurationNs(ns) {
            if (ns == null || ns === undefined) return '-';
            if (ns < 1000000) return Math.round(ns / 1000) + ' \u00b5s';
            if (ns < 1000000000) return (ns / 1000000).toFixed(2) + ' ms';
            return (ns / 1000000000).toFixed(2) + ' s';
        },

        statusCodeClass(statusCode) {
            if (statusCode >= 500) return 'status-error';
            if (statusCode >= 400) return 'status-warning';
            if (statusCode >= 300) return 'status-neutral';
            return 'status-success';
        },

        formatJSON(v) {
            if (v == null || v === undefined || v === '') return 'Not captured';

            if (typeof v === 'string') {
                const trimmed = v.trim();
                if ((trimmed.startsWith('{') && trimmed.endsWith('}')) || (trimmed.startsWith('[') && trimmed.endsWith(']'))) {
                    try {
                        return JSON.stringify(JSON.parse(trimmed), null, 2);
                    } catch (_) {
                        return v;
                    }
                }
                return v;
            }

            try {
                return JSON.stringify(v, null, 2);
            } catch (_) {
                return String(v);
            }
        },

        async copyAuditJSON(v) {
            if (v == null || v === undefined || v === '') return;
            try {
                const payload = this.formatJSON(v);
                await navigator.clipboard.writeText(payload);
            } catch (e) {
                console.error('Failed to copy audit payload:', e);
            }
        },

        canShowConversation(entry) {
            if (!entry) return false;
            if (this._isConversationExcludedPath(entry.path)) return false;
            return this._isConversationalPath(entry.path) || this._hasConversationPayload(entry);
        },

        _isConversationExcludedPath(path) {
            if (!path) return false;
            const p = String(path).toLowerCase();
            return p === '/v1/embeddings' ||
                p === '/v1/embeddings/' ||
                p.startsWith('/v1/embeddings?') ||
                p.startsWith('/v1/embeddings/');
        },

        _isConversationalPath(path) {
            if (!path) return false;
            const p = String(path).toLowerCase();
            return p === '/v1/chat/completions' ||
                p === '/v1/chat/completions/' ||
                p.startsWith('/v1/chat/completions?') ||
                p.startsWith('/v1/chat/completions/') ||
                p === '/v1/responses' ||
                p === '/v1/responses/' ||
                p.startsWith('/v1/responses?') ||
                p.startsWith('/v1/responses/');
        },

        _hasConversationPayload(entry) {
            const requestBody = entry && entry.data ? entry.data.request_body : null;
            const responseBody = entry && entry.data ? entry.data.response_body : null;

            const reqHas = requestBody && (
                Array.isArray(requestBody.messages) ||
                requestBody.input !== undefined ||
                typeof requestBody.instructions === 'string' ||
                typeof requestBody.previous_response_id === 'string'
            );
            const respHas = responseBody && (
                Array.isArray(responseBody.choices) ||
                this._looksLikeResponsesOutput(responseBody.output)
            );

            return !!(reqHas || respHas);
        },

        startBodyInteraction(event) {
            this.bodyPointerStart = {
                x: event.clientX,
                y: event.clientY
            };
        },

        _isBodyDrag(event) {
            if (!this.bodyPointerStart) return false;
            const dx = Math.abs(event.clientX - this.bodyPointerStart.x);
            const dy = Math.abs(event.clientY - this.bodyPointerStart.y);
            return dx > 4 || dy > 4;
        },

        _hasActiveSelection() {
            const selection = window.getSelection ? window.getSelection() : null;
            if (!selection) return false;
            if (selection.isCollapsed) return false;
            return String(selection.toString() || '').trim().length > 0;
        },

        _looksLikeResponsesOutput(output) {
            if (!Array.isArray(output)) return false;
            return output.some((item) => {
                if (!item || typeof item !== 'object') return false;
                if (item.type === 'message' || item.role === 'assistant' || item.role === 'user' || item.role === 'system') return true;
                if (!Array.isArray(item.content)) return false;
                return item.content.some((part) => {
                    if (!part || typeof part !== 'object') return false;
                    return typeof part.text === 'string' || part.type === 'output_text' || part.type === 'input_text';
                });
            });
        },

        handleBodyConversationClick(event, entry) {
            const wasDrag = this._isBodyDrag(event);
            this.bodyPointerStart = null;
            if (wasDrag) return;
            if (this._hasActiveSelection()) return;
            if (!this.canShowConversation(entry)) return;
            const el = event.target && event.target.closest ? event.target.closest('[data-conversation-trigger="1"]') : null;
            if (!el) return;
            event.preventDefault();
            event.stopPropagation();
            this.openConversation(entry);
        },

        renderBodyWithConversationHighlights(entry, value) {
            const raw = this.formatJSON(value);
            const escaped = this._escapeHTML(raw);
            if (!this.canShowConversation(entry) || !raw || raw === 'Not captured') {
                return escaped;
            }

            const lines = raw.split('\n');
            const sectionKeys = new Set(['instructions', 'messages', 'input', 'previous_response_id', 'choices', 'output']);
            const rendered = [];

            let i = 0;
            while (i < lines.length) {
                const line = lines[i];
                const match = line.match(/^(\s*)"([^"]+)"\s*:\s*(.*)$/);
                if (match && sectionKeys.has(match[2])) {
                    const key = match[2];
                    const valuePart = match[3] || '';
                    const end = this._findConversationSectionEnd(lines, i, valuePart);
                    const roleClass = this._conversationHighlightRoleClass(key);
                    const block = lines.slice(i, end + 1).map((l) => this._escapeHTML(l)).join('\n');
                    rendered.push('<span class="conversation-body-highlight ' + roleClass + '" data-conversation-trigger="1">' + block + '</span>');
                    i = end + 1;
                    continue;
                }
                rendered.push(this._escapeHTML(line));
                i++;
            }

            return rendered.join('\n');
        },

        _findConversationSectionEnd(lines, startIdx, valuePart) {
            const value = String(valuePart || '').trim();
            if (!(value.startsWith('{') || value.startsWith('['))) {
                return startIdx;
            }

            let depth = this._jsonBracketDelta(valuePart);
            let idx = startIdx;
            while (depth > 0 && idx + 1 < lines.length) {
                idx++;
                depth += this._jsonBracketDelta(lines[idx]);
            }
            return idx;
        },

        _jsonBracketDelta(text) {
            let depth = 0;
            let inString = false;
            let escaped = false;
            const src = String(text || '');

            for (let i = 0; i < src.length; i++) {
                const ch = src[i];
                if (inString) {
                    if (escaped) {
                        escaped = false;
                        continue;
                    }
                    if (ch === '\\') {
                        escaped = true;
                        continue;
                    }
                    if (ch === '"') {
                        inString = false;
                    }
                    continue;
                }

                if (ch === '"') {
                    inString = true;
                    continue;
                }
                if (ch === '{' || ch === '[') {
                    depth++;
                    continue;
                }
                if (ch === '}' || ch === ']') {
                    depth--;
                }
            }

            return depth;
        },

        _conversationHighlightRoleClass(key) {
            if (key === 'instructions') return 'conversation-system';
            if (key === 'messages' || key === 'input' || key === 'previous_response_id') return 'conversation-user';
            return 'conversation-assistant';
        },

        _escapeHTML(value) {
            return String(value == null ? '' : value)
                .replaceAll('&', '&amp;')
                .replaceAll('<', '&lt;')
                .replaceAll('>', '&gt;')
                .replaceAll('"', '&quot;')
                .replaceAll("'", '&#39;');
        },

        async openConversation(entry, detailsEl, expandEntry) {
            if (!entry || !entry.id || !this.canShowConversation(entry)) return;
            if (expandEntry && detailsEl && !detailsEl.open) {
                detailsEl.open = true;
            }
            this.conversationOpen = true;
            this.conversationLoading = true;
            this.conversationError = '';
            this.conversationAnchorID = entry.id;
            this.conversationEntries = [];
            this.conversationMessages = [];
            await this.fetchConversation(entry.id);
        },

        closeConversation() {
            this.conversationOpen = false;
        },

        async fetchConversation(logID) {
            try {
                const qs = 'log_id=' + encodeURIComponent(logID) + '&limit=120';
                const res = await fetch('/admin/api/v1/audit/conversation?' + qs, { headers: this.headers() });
                if (!this.handleFetchResponse(res, 'audit conversation')) {
                    this.conversationError = 'Unable to load conversation.';
                    this.conversationEntries = [];
                    this.conversationMessages = [];
                    return;
                }

                const result = await res.json();
                this.conversationAnchorID = result.anchor_id || logID;
                this.conversationEntries = Array.isArray(result.entries) ? result.entries : [];
                this.conversationMessages = this.buildConversationMessages(this.conversationEntries, this.conversationAnchorID);
            } catch (e) {
                console.error('Failed to fetch audit conversation:', e);
                this.conversationError = 'Failed to load conversation.';
                this.conversationEntries = [];
                this.conversationMessages = [];
            } finally {
                this.conversationLoading = false;
            }
        },

        buildConversationMessages(entries, anchorID) {
            if (!Array.isArray(entries) || entries.length === 0) return [];

            const sorted = [...entries].sort((a, b) => new Date(a.timestamp) - new Date(b.timestamp));
            const messages = [];
            let idx = 0;

            sorted.forEach((entry) => {
                const isAnchor = entry.id === anchorID;
                const ts = entry.timestamp;
                const requestBody = entry.data && entry.data.request_body ? entry.data.request_body : null;
                const responseBody = entry.data && entry.data.response_body ? entry.data.response_body : null;

                // System prompt in request body (chat + responses APIs)
                if (requestBody && typeof requestBody.instructions === 'string' && requestBody.instructions.trim()) {
                    messages.push(this._conversationMessage('system', requestBody.instructions, ts, entry.id, isAnchor, ++idx));
                }

                if (requestBody && Array.isArray(requestBody.messages)) {
                    requestBody.messages.forEach((m) => {
                        if (!m) return;
                        const role = (m.role || 'user').toLowerCase();
                        const text = this._extractText(m.content);
                        if (text) messages.push(this._conversationMessage(role, text, ts, entry.id, isAnchor, ++idx));
                    });
                }

                if (requestBody && requestBody.input !== undefined) {
                    this._extractResponsesInputMessages(requestBody.input).forEach((m) => {
                        if (m.text) messages.push(this._conversationMessage(m.role, m.text, ts, entry.id, isAnchor, ++idx));
                    });
                }

                if (responseBody && Array.isArray(responseBody.choices)) {
                    const first = responseBody.choices[0];
                    if (first && first.message) {
                        const role = (first.message.role || 'assistant').toLowerCase();
                        const text = this._extractText(first.message.content);
                        if (text) messages.push(this._conversationMessage(role, text, ts, entry.id, isAnchor, ++idx));
                    }
                }

                if (responseBody && Array.isArray(responseBody.output)) {
                    responseBody.output.forEach((item) => {
                        if (!item) return;
                        const role = (item.role || 'assistant').toLowerCase();
                        const text = this._extractResponsesOutputText(item);
                        if (text) messages.push(this._conversationMessage(role, text, ts, entry.id, isAnchor, ++idx));
                    });
                }

                const errMsg = this._extractConversationErrorMessage(entry);
                if (errMsg) {
                    messages.push(this._conversationMessage('error', errMsg, ts, entry.id, isAnchor, ++idx));
                }
            });

            return messages;
        },

        _conversationMessage(role, text, timestamp, entryID, isAnchor, idx) {
            const normalized = this._roleMeta(role);
            return {
                uid: entryID + '-' + idx,
                entryID,
                timestamp,
                text,
                role: normalized.role,
                roleLabel: normalized.label,
                roleClass: normalized.className,
                isAnchor
            };
        },

        _roleMeta(role) {
            const normalized = String(role || '').toLowerCase();
            if (normalized === 'system' || normalized === 'developer') {
                return { role: 'system', label: 'System Prompt', className: 'role-system' };
            }
            if (normalized === 'assistant') {
                return { role: 'assistant', label: 'Agent', className: 'role-assistant' };
            }
            if (normalized === 'error') {
                return { role: 'error', label: 'Error', className: 'role-error' };
            }
            return { role: 'user', label: 'User', className: 'role-user' };
        },

        _extractText(content) {
            if (content == null) return '';
            if (typeof content === 'string') return content.trim();

            if (Array.isArray(content)) {
                const parts = content.map((part) => {
                    if (typeof part === 'string') return part;
                    if (!part || typeof part !== 'object') return '';
                    if (typeof part.text === 'string') return part.text;
                    if (typeof part.output_text === 'string') return part.output_text;
                    return '';
                }).filter(Boolean);
                return parts.join('\n').trim();
            }

            if (typeof content === 'object') {
                if (typeof content.text === 'string') return content.text.trim();
                try {
                    return JSON.stringify(content, null, 2);
                } catch (_) {
                    return '';
                }
            }

            return String(content).trim();
        },

        _extractResponsesInputMessages(input) {
            if (input == null) return [];
            if (typeof input === 'string') {
                const text = input.trim();
                return text ? [{ role: 'user', text }] : [];
            }

            if (!Array.isArray(input)) {
                const text = this._extractText(input);
                return text ? [{ role: 'user', text }] : [];
            }

            return input.map((item) => {
                if (!item || typeof item !== 'object') return null;
                const role = String(item.role || 'user').toLowerCase();
                const text = this._extractText(item.content);
                if (!text) return null;
                return { role, text };
            }).filter(Boolean);
        },

        _extractResponsesOutputText(item) {
            if (!item || typeof item !== 'object') return '';
            if (!Array.isArray(item.content)) return this._extractText(item.content);

            const parts = item.content.map((part) => {
                if (!part) return '';
                if (typeof part.text === 'string') return part.text;
                return '';
            }).filter(Boolean);

            return parts.join('\n').trim();
        },

        _extractConversationErrorMessage(entry) {
            if (!entry || !entry.data) return '';

            const responseBodyMessage = this._findNestedErrorMessage(entry.data.response_body);
            if (responseBodyMessage) return responseBodyMessage;

            const rawError = entry.data.error_message;
            if (rawError == null) return '';

            if (typeof rawError === 'string') {
                const trimmed = rawError.trim();
                if (!trimmed) return '';

                const parsed = this._tryParseJSON(trimmed);
                const parsedMessage = this._findNestedErrorMessage(parsed);
                if (parsedMessage) return parsedMessage;
                return trimmed;
            }

            const structuredMessage = this._findNestedErrorMessage(rawError);
            if (structuredMessage) return structuredMessage;
            return this._extractText(rawError);
        },

        _tryParseJSON(value) {
            if (typeof value !== 'string') return null;
            try {
                return JSON.parse(value);
            } catch (_) {
                return null;
            }
        },

        _normalizeErrorMessageText(text, depth) {
            const trimmed = String(text || '').trim();
            if (!trimmed) return '';
            if (depth >= 4) return trimmed;

            const parsed = this._tryParseJSON(trimmed);
            if (!parsed || typeof parsed !== 'object') {
                return trimmed;
            }

            const nested = this._findNestedErrorMessage(parsed, depth + 1);
            if (nested) return nested;

            const fallback = this._extractText(parsed);
            return fallback || trimmed;
        },

        _findNestedErrorMessage(value, depth = 0) {
            const visited = new Set();
            const stack = [value];

            while (stack.length > 0) {
                const current = stack.shift();
                if (!current || typeof current !== 'object') continue;
                if (visited.has(current)) continue;
                visited.add(current);

                if (Array.isArray(current)) {
                    for (let i = 0; i < current.length; i++) {
                        stack.push(current[i]);
                    }
                    continue;
                }

                const error = current.error;
                if (typeof error === 'string' && error.trim()) {
                    return this._normalizeErrorMessageText(error, depth);
                }
                if (error && typeof error === 'object') {
                    if (typeof error.message === 'string' && error.message.trim()) {
                        return this._normalizeErrorMessageText(error.message, depth);
                    }
                    stack.push(error);
                }

                if (typeof current.message === 'string' && current.message.trim()) {
                    if (current.error !== undefined || current.code !== undefined || current.status !== undefined || current.type !== undefined) {
                        return this._normalizeErrorMessageText(current.message, depth);
                    }
                }

                const keys = Object.keys(current);
                for (let i = 0; i < keys.length; i++) {
                    const key = keys[i];
                    if (key === 'error') continue;
                    stack.push(current[key]);
                }
            }

            return '';
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

        _barColors() {
            return [
                '#c2845a', '#7a9e7e', '#d4a574', '#b8a98e', '#8b9e6b',
                '#7d8a97', '#c47a5a', '#6b8e6b', '#a09486', '#9b7ea4',
                '#c49a6c'
            ];
        },

        _barData() {
            const sorted = [...this.modelUsage].sort((a, b) => {
                if (this.usageMode === 'costs') {
                    return ((b.total_cost || 0) - (a.total_cost || 0));
                }
                return ((b.input_tokens + b.output_tokens) - (a.input_tokens + a.output_tokens));
            });

            const top = sorted.slice(0, 10);
            const rest = sorted.slice(10);

            const labels = top.map(m => m.model);
            const values = top.map(m => {
                if (this.usageMode === 'costs') return m.total_cost || 0;
                return m.input_tokens + m.output_tokens;
            });

            if (rest.length > 0) {
                labels.push('Other');
                let otherVal = 0;
                rest.forEach(m => {
                    if (this.usageMode === 'costs') otherVal += (m.total_cost || 0);
                    else otherVal += m.input_tokens + m.output_tokens;
                });
                values.push(otherVal);
            }

            return { labels, values };
        },

        barLegendItems() {
            const { labels, values } = this._barData();
            const colors = this._barColors();
            return labels.map((label, i) => ({
                label,
                color: colors[i % colors.length],
                value: this.usageMode === 'costs' ? '$' + values[i].toFixed(4) : this.formatTokensShort(values[i])
            }));
        },

        renderBarChart(retries) {
            if (retries === undefined) retries = 3;
            this.$nextTick(() => {
                if (this.usageBarChart) {
                    this.usageBarChart.destroy();
                    this.usageBarChart = null;
                }

                if (this.modelUsage.length === 0) return;
                if (this.page !== 'usage') return;

                const canvas = document.getElementById('usageBarChart');
                if (!canvas || canvas.offsetWidth === 0) {
                    if (retries > 0) {
                        setTimeout(() => this.renderBarChart(retries - 1), 100);
                    }
                    return;
                }

                const colors = this.chartColors();
                const { labels, values } = this._barData();
                const palette = this._barColors();

                this.usageBarChart = new Chart(canvas, {
                    type: 'bar',
                    data: {
                        labels: labels,
                        datasets: [{
                            data: values,
                            backgroundColor: labels.map((_, i) => palette[i % palette.length]),
                            borderColor: 'transparent',
                            borderWidth: 0,
                            borderRadius: 4
                        }]
                    },
                    options: {
                        responsive: true,
                        maintainAspectRatio: false,
                        animation: { duration: 0 },
                        layout: { padding: { top: 8 } },
                        scales: {
                            x: {
                                grid: { display: false },
                                ticks: {
                                    color: colors.text,
                                    font: { size: 11, family: "'SF Mono', Menlo, Consolas, monospace" },
                                    maxRotation: 45,
                                    minRotation: 0
                                }
                            },
                            y: {
                                grid: { color: colors.grid },
                                border: { display: false },
                                ticks: {
                                    color: colors.text,
                                    font: { size: 11, family: "'SF Mono', Menlo, Consolas, monospace" },
                                    callback: (v) => {
                                        if (this.usageMode === 'costs') return '$' + v.toFixed(2);
                                        return this.formatTokensShort(v);
                                    }
                                }
                            }
                        },
                        plugins: {
                            legend: { display: false },
                            tooltip: {
                                backgroundColor: colors.tooltipBg,
                                borderColor: colors.tooltipBorder,
                                borderWidth: 1,
                                titleColor: colors.tooltipText,
                                bodyColor: colors.tooltipText,
                                callbacks: {
                                    label: (c) => {
                                        const val = c.parsed.y;
                                        if (this.usageMode === 'costs') return '$' + val.toFixed(4);
                                        return this.formatTokensShort(val);
                                    }
                                }
                            }
                        }
                    }
                });
            });
        }
    };
}
