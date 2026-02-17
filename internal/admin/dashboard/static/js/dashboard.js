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
