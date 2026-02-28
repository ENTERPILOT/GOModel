(function(global) {
    function dashboardAuditListModule() {
        return {
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
                if (statusCode === null || statusCode === undefined || statusCode === '') return 'status-unknown';
                const parsedStatus = Number(statusCode);
                if (!Number.isFinite(parsedStatus)) return 'status-unknown';
                if (parsedStatus >= 500) return 'status-error';
                if (parsedStatus >= 400) return 'status-warning';
                if (parsedStatus >= 300) return 'status-neutral';
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
            }
        };
    }

    global.dashboardAuditListModule = dashboardAuditListModule;
})(window);
