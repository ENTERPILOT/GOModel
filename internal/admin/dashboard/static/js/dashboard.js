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

        // Data
        summary: { total_requests: 0, total_input_tokens: 0, total_output_tokens: 0, total_tokens: 0 },
        daily: [],
        models: [],

        // Filters
        modelFilter: '',

        // Chart
        chart: null,

        init() {
            this.apiKey = localStorage.getItem('gomodel_api_key') || '';
            this.theme = localStorage.getItem('gomodel_theme') || 'system';
            this.sidebarCollapsed = localStorage.getItem('gomodel_sidebar_collapsed') === 'true';
            this.applyTheme();

            // Parse initial page from URL path
            const path = window.location.pathname.replace(/\/$/, '');
            const slug = path.split('/').pop();
            this.page = (slug === 'models') ? 'models' : 'overview';

            // Handle browser back/forward
            window.addEventListener('popstate', () => {
                const p = window.location.pathname.replace(/\/$/, '');
                const s = p.split('/').pop();
                this.page = (s === 'models') ? 'models' : 'overview';
                if (this.page === 'overview') this.renderChart();
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

        navigate(page) {
            this.page = page;
            history.pushState(null, '', '/admin/dashboard/' + page);
            if (page === 'overview') this.renderChart();
        },

        setTheme(t) {
            this.theme = t;
            localStorage.setItem('gomodel_theme', t);
            this.applyTheme();
            this.renderChart();
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
            await Promise.all([this.fetchUsage(), this.fetchModels()]);
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

        async fetchUsage() {
            try {
                const [summaryRes, dailyRes] = await Promise.all([
                    fetch('/admin/api/v1/usage/summary?days=' + this.days, { headers: this.headers() }),
                    fetch('/admin/api/v1/usage/daily?days=' + this.days, { headers: this.headers() })
                ]);

                if (!this.handleFetchResponse(summaryRes, 'usage summary') ||
                    !this.handleFetchResponse(dailyRes, 'usage daily')) {
                    return;
                }

                this.summary = await summaryRes.json();
                this.daily = await dailyRes.json();
                this.renderChart();
            } catch (e) {
                console.error('Failed to fetch usage:', e);
            }
        },

        async fetchModels() {
            try {
                const res = await fetch('/admin/api/v1/models', { headers: this.headers() });
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

        fillMissingDays(daily) {
            const byDate = {};
            daily.forEach(d => { byDate[d.date] = d; });
            const end = new Date();
            end.setHours(0, 0, 0, 0);
            const start = new Date(end);
            start.setDate(start.getDate() - (parseInt(this.days, 10) - 1));
            const result = [];
            for (let d = new Date(start); d <= end; d.setDate(d.getDate() + 1)) {
                const key = d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0');
                result.push(byDate[key] || { date: key, input_tokens: 0, output_tokens: 0, total_tokens: 0, requests: 0 });
            }
            return result;
        },

        renderChart() {
            this.$nextTick(() => {
                if (this.chart) {
                    this.chart.destroy();
                    this.chart = null;
                }

                if (this.daily.length === 0) return;

                const canvas = document.getElementById('usageChart');
                if (!canvas || canvas.offsetWidth === 0) return;

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
                                borderColor: '#b8956e',
                                backgroundColor: 'rgba(184, 149, 110, 0.1)',
                                fill: true,
                                tension: 0.3,
                                pointRadius: 3,
                                pointHoverRadius: 5
                            },
                            {
                                label: 'Output Tokens',
                                data: outputData,
                                borderColor: '#34d399',
                                backgroundColor: 'rgba(52, 211, 153, 0.1)',
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
                (m.model?.owned_by ?? '').toLowerCase().includes(f)
            );
        },

        formatNumber(n) {
            if (n == null || n === undefined) return '-';
            return n.toLocaleString();
        }
    };
}
