(function(global) {
    function dashboardAuthKeysModule() {
        return {
            authKeys: [],
            authKeysAvailable: true,
            authKeysLoading: false,
            authKeyError: '',
            authKeyNotice: '',
            authKeyFormOpen: false,
            authKeyFormSubmitting: false,
            authKeyIssuedValue: '',
            authKeyDeactivatingID: '',
            authKeyCopied: false,
            authKeyForm: {
                name: '',
                description: '',
                expires_at: ''
            },

            defaultAuthKeyForm() {
                return { name: '', description: '', expires_at: '' };
            },

            async fetchAuthKeys() {
                this.authKeysLoading = true;
                this.authKeyError = '';
                try {
                    const res = await fetch('/admin/api/v1/auth-keys', { headers: this.headers() });
                    if (res.status === 503) {
                        this.authKeysAvailable = false;
                        this.authKeys = [];
                        return;
                    }
                    this.authKeysAvailable = true;
                    if (!this.handleFetchResponse(res, 'auth keys')) {
                        this.authKeys = [];
                        return;
                    }
                    const payload = await res.json();
                    this.authKeys = Array.isArray(payload) ? payload : [];
                } catch (e) {
                    console.error('Failed to fetch auth keys:', e);
                    this.authKeys = [];
                    this.authKeyError = 'Unable to load API keys.';
                } finally {
                    this.authKeysLoading = false;
                }
            },

            openAuthKeyForm() {
                this.authKeyFormOpen = true;
                this.authKeyError = '';
                this.authKeyNotice = '';
                this.authKeyIssuedValue = '';
                this.authKeyForm = this.defaultAuthKeyForm();
            },

            closeAuthKeyForm() {
                this.authKeyFormOpen = false;
                this.authKeyError = '';
                this.authKeyIssuedValue = '';
                this.authKeyCopied = false;
                this.authKeyForm = this.defaultAuthKeyForm();
            },

            copyAuthKeyValue() {
                navigator.clipboard.writeText(this.authKeyIssuedValue).then(() => {
                    this.authKeyCopied = true;
                    setTimeout(() => { this.authKeyCopied = false; }, 2000);
                });
            },

            dismissIssuedKey() {
                this.authKeyIssuedValue = '';
                this.authKeyCopied = false;
                this.authKeyForm = this.defaultAuthKeyForm();
            },

            async _authKeyResponseMessage(res, fallback) {
                try {
                    const payload = await res.json();
                    if (payload && payload.error && payload.error.message) {
                        return payload.error.message;
                    }
                } catch (_) {
                    // Ignore invalid or empty responses and return the fallback message.
                }
                return fallback;
            },

            async submitAuthKeyForm() {
                const name = String(this.authKeyForm.name || '').trim();
                if (!name) {
                    this.authKeyError = 'Name is required.';
                    return;
                }

                this.authKeyError = '';
                this.authKeyNotice = '';
                this.authKeyFormSubmitting = true;

                const payload = {
                    name,
                    description: String(this.authKeyForm.description || '').trim() || undefined
                };
                if (this.authKeyForm.expires_at) {
                    payload.expires_at = this.authKeyForm.expires_at + 'T23:59:59Z';
                }

                try {
                    const res = await fetch('/admin/api/v1/auth-keys', {
                        method: 'POST',
                        headers: this.headers(),
                        body: JSON.stringify(payload)
                    });
                    if (res.status === 503) {
                        this.authKeysAvailable = false;
                        this.authKeyError = 'Auth keys feature is unavailable.';
                        return;
                    }
                    if (res.status === 401) {
                        this.authError = true;
                        this.needsAuth = true;
                        this.authKeyError = 'Authentication required.';
                        return;
                    }
                    if (res.status !== 201) {
                        this.authKeyError = await this._authKeyResponseMessage(res, 'Failed to create API key.');
                        return;
                    }
                    const issued = await res.json();
                    this.authKeyIssuedValue = issued.value || '';
                    this.authKeyForm = this.defaultAuthKeyForm();
                    await this.fetchAuthKeys();
                } catch (e) {
                    console.error('Failed to issue auth key:', e);
                    this.authKeyError = 'Failed to create API key.';
                } finally {
                    this.authKeyFormSubmitting = false;
                }
            },

            async deactivateAuthKey(key) {
                if (!key || !key.active) {
                    return;
                }
                if (!window.confirm('Deactivate key "' + key.name + '"? This cannot be undone.')) {
                    return;
                }

                this.authKeyDeactivatingID = key.id;
                this.authKeyError = '';
                this.authKeyNotice = '';

                try {
                    const res = await fetch('/admin/api/v1/auth-keys/' + encodeURIComponent(key.id) + '/deactivate', {
                        method: 'POST',
                        headers: this.headers()
                    });
                    if (res.status === 503) {
                        this.authKeysAvailable = false;
                        this.authKeyError = 'Auth keys feature is unavailable.';
                        return;
                    }
                    if (res.status === 401) {
                        this.authError = true;
                        this.needsAuth = true;
                        this.authKeyError = 'Authentication required.';
                        return;
                    }
                    if (res.status !== 204) {
                        this.authKeyError = await this._authKeyResponseMessage(res, 'Failed to deactivate key.');
                        return;
                    }
                    await this.fetchAuthKeys();
                    this.authKeyNotice = 'Key "' + key.name + '" deactivated.';
                } catch (e) {
                    console.error('Failed to deactivate auth key:', e);
                    this.authKeyError = 'Failed to deactivate key.';
                } finally {
                    this.authKeyDeactivatingID = '';
                }
            }
        };
    }

    global.dashboardAuthKeysModule = dashboardAuthKeysModule;
})(window);
