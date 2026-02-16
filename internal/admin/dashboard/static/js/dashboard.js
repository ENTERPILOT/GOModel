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
            this.applyTheme();

            // Re-render chart when system theme changes (only matters in 'system' mode)
            window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
                if (this.theme === 'system') {
                    this.renderChart();
                }
            });

            this.fetchAll();
        },

        setTheme(t) {
            this.theme = t;
            localStorage.setItem('gomodel_theme', t);
            this.applyTheme();
            this.renderChart();
        },

        applyTheme() {
            const root = document.documentElement;
            if (this.theme === 'system') {
                root.removeAttribute('data-theme');
            } else {
                root.setAttribute('data-theme', this.theme);
            }
        },

        isDark() {
            if (this.theme === 'dark') return true;
            if (this.theme === 'light') return false;
            return window.matchMedia('(prefers-color-scheme: dark)').matches;
        },

        chartColors() {
            const dark = this.isDark();
            return {
                grid: dark ? '#2a2d3e' : '#e5e7eb',
                text: dark ? '#8b8fa3' : '#6b7085',
                tooltipBg: dark ? '#161923' : '#ffffff',
                tooltipBorder: dark ? '#2a2d3e' : '#d8dbe3',
                tooltipText: dark ? '#e4e6f0' : '#1a1d2b',
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

        async fetchUsage() {
            try {
                const [summaryRes, dailyRes] = await Promise.all([
                    fetch('/admin/api/v1/usage/summary?days=' + this.days, { headers: this.headers() }),
                    fetch('/admin/api/v1/usage/daily?days=' + this.days, { headers: this.headers() })
                ]);

                if (summaryRes.status === 401 || dailyRes.status === 401) {
                    this.authError = true;
                    this.needsAuth = true;
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
                if (res.status === 401) {
                    this.authError = true;
                    this.needsAuth = true;
                    return;
                }
                this.models = await res.json();
            } catch (e) {
                console.error('Failed to fetch models:', e);
            }
        },

        renderChart() {
            const ctx = document.getElementById('usageChart');
            if (!ctx) return;

            if (this.chart) {
                this.chart.destroy();
            }

            if (this.daily.length === 0) return;

            const colors = this.chartColors();
            const labels = this.daily.map(d => d.date);
            const inputData = this.daily.map(d => d.input_tokens);
            const outputData = this.daily.map(d => d.output_tokens);

            this.chart = new Chart(ctx, {
                type: 'line',
                data: {
                    labels: labels,
                    datasets: [
                        {
                            label: 'Input Tokens',
                            data: inputData,
                            borderColor: '#6366f1',
                            backgroundColor: 'rgba(99, 102, 241, 0.1)',
                            fill: true,
                            tension: 0.3,
                            pointRadius: 3,
                            pointHoverRadius: 5
                        },
                        {
                            label: 'Output Tokens',
                            data: outputData,
                            borderColor: '#22c55e',
                            backgroundColor: 'rgba(34, 197, 94, 0.1)',
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
                                label: function(ctx) {
                                    return ctx.dataset.label + ': ' + ctx.parsed.y.toLocaleString();
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
        },

        get filteredModels() {
            if (!this.modelFilter) return this.models;
            const f = this.modelFilter.toLowerCase();
            return this.models.filter(m =>
                m.model.id.toLowerCase().includes(f) ||
                m.provider_type.toLowerCase().includes(f) ||
                (m.model.owned_by && m.model.owned_by.toLowerCase().includes(f))
            );
        },

        formatNumber(n) {
            if (n == null || n === undefined) return '-';
            return n.toLocaleString();
        }
    };
}
