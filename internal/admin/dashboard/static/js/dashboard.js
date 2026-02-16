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

        // Data
        summary: { total_requests: 0, total_input_tokens: 0, total_output_tokens: 0, total_tokens: 0 },
        daily: [],
        models: [],

        // Filters
        modelFilter: '',
        providerFilter: '',

        // Chart
        chart: null,

        init() {
            this.apiKey = localStorage.getItem('gomodel_api_key') || '';
            this.fetchAll();
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
                            labels: { color: '#8b8fa3', font: { size: 12 } }
                        },
                        tooltip: {
                            backgroundColor: '#161923',
                            borderColor: '#2a2d3e',
                            borderWidth: 1,
                            titleColor: '#e4e6f0',
                            bodyColor: '#e4e6f0',
                            callbacks: {
                                label: function(ctx) {
                                    return ctx.dataset.label + ': ' + ctx.parsed.y.toLocaleString();
                                }
                            }
                        }
                    },
                    scales: {
                        x: {
                            grid: { color: '#2a2d3e' },
                            ticks: { color: '#8b8fa3', font: { size: 11 }, maxTicksLimit: 10 }
                        },
                        y: {
                            grid: { color: '#2a2d3e' },
                            ticks: {
                                color: '#8b8fa3',
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
            let result = this.models;
            if (this.modelFilter) {
                const f = this.modelFilter.toLowerCase();
                result = result.filter(m => m.model.id.toLowerCase().includes(f));
            }
            if (this.providerFilter) {
                result = result.filter(m => m.provider_type === this.providerFilter);
            }
            return result;
        },

        get providerList() {
            const providers = new Set(this.models.map(m => m.provider_type));
            return [...providers].sort();
        },

        formatNumber(n) {
            if (n == null || n === undefined) return '-';
            return n.toLocaleString();
        }
    };
}
