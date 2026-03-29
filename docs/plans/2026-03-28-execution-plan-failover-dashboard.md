# Execution Plan Failover Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose the existing execution-plan failover flag in workflow authoring, hide it when global fallback is disabled, and provide a small allowlisted admin runtime-config endpoint for dashboard feature gating.

**Architecture:** Add one admin runtime-config endpoint that returns a narrow, env-style public config surface, initially only `FEATURE_FALLBACK_MODE`. Keep workflow persistence on the existing `plan_payload.features.fallback` field, default older plans to `true`, and let the dashboard hide all failover UI when the global mode is `off`.

**Tech Stack:** Go, Echo, Alpine.js, embedded dashboard templates, Node test runner, Go test

---

### Task 1: Admin runtime-config endpoint

**Files:**
- Modify: `internal/admin/handler.go`
- Modify: `internal/admin/handler_test.go`
- Modify: `internal/server/http.go`
- Modify: `internal/server/http_test.go`

- [ ] **Step 1: Write the failing admin handler tests**

Add coverage for:
- `GET /admin/api/v1/dashboard/config` returning `{"FEATURE_FALLBACK_MODE":"auto"}` when configured
- the same endpoint returning `"off"` when fallback is globally disabled

- [ ] **Step 2: Run the targeted Go tests to verify they fail**

Run: `go test ./internal/admin ./internal/server -run 'TestDashboardConfig|TestAdminDashboardConfig' -v`
Expected: FAIL because the handler/route does not exist yet.

- [ ] **Step 3: Implement the minimal backend support**

Add:
- an allowlisted runtime-config field on the admin handler
- an option/helper for injecting `FEATURE_FALLBACK_MODE`
- the new handler method and route registration in `internal/server/http.go`

- [ ] **Step 4: Run the targeted Go tests to verify they pass**

Run: `go test ./internal/admin ./internal/server -run 'TestDashboardConfig|TestAdminDashboardConfig' -v`
Expected: PASS

### Task 2: Workflow failover persistence and admin coverage

**Files:**
- Modify: `internal/admin/handler_executionplans_test.go`

- [ ] **Step 1: Write the failing execution-plan tests**

Add coverage for:
- listing workflows preserves explicit `features.fallback=false`
- creating workflows accepts and persists explicit `features.fallback=false`

- [ ] **Step 2: Run the targeted Go tests to verify they fail**

Run: `go test ./internal/admin ./internal/executionplans -run 'TestListExecutionPlans|TestCreateExecutionPlan' -v`
Expected: FAIL because the assertions for fallback visibility/persistence are not yet satisfied.

- [ ] **Step 3: Implement the minimal backend changes if needed**

Only touch production code if the tests reveal missing serialization or plumbing. Prefer leaving storage/compiler behavior unchanged if JSON binding already supports the field.

- [ ] **Step 4: Run the targeted Go tests to verify they pass**

Run: `go test ./internal/admin ./internal/executionplans -run 'TestListExecutionPlans|TestCreateExecutionPlan' -v`
Expected: PASS

### Task 3: Dashboard runtime-config gating and failover toggle

**Files:**
- Modify: `internal/admin/dashboard/static/js/dashboard.js`
- Modify: `internal/admin/dashboard/static/js/modules/execution-plans.js`
- Modify: `internal/admin/dashboard/static/js/modules/execution-plans.test.js`
- Modify: `internal/admin/dashboard/static/js/modules/execution-plans-layout.test.js`
- Modify: `internal/admin/dashboard/templates/index.html`

- [ ] **Step 1: Write the failing dashboard tests**

Add coverage for:
- loading and storing `FEATURE_FALLBACK_MODE`
- hiding workflow failover controls when the mode is `"off"`
- defaulting workflow fallback to `true` when omitted
- submitting explicit `fallback` booleans when the mode is not `"off"`
- rendering a failover indicator only when the mode is not `"off"`

- [ ] **Step 2: Run the targeted JS tests to verify they fail**

Run: `node --test internal/admin/dashboard/static/js/modules/execution-plans.test.js internal/admin/dashboard/static/js/modules/execution-plans-layout.test.js`
Expected: FAIL because the runtime-config state and failover UI do not exist yet.

- [ ] **Step 3: Implement the minimal dashboard changes**

Add:
- runtime-config state and fetch logic in `dashboard.js`
- `FEATURE_FALLBACK_MODE` helpers in the workflows module
- the two-state failover toggle and read-only card indicator in the template

- [ ] **Step 4: Run the targeted JS tests to verify they pass**

Run: `node --test internal/admin/dashboard/static/js/modules/execution-plans.test.js internal/admin/dashboard/static/js/modules/execution-plans-layout.test.js`
Expected: PASS

### Task 4: Final verification

**Files:**
- Verify only

- [ ] **Step 1: Run focused Go verification**

Run: `go test ./internal/app ./internal/admin ./internal/server ./internal/executionplans -v`
Expected: PASS

- [ ] **Step 2: Run focused dashboard JS verification**

Run: `node --test internal/admin/dashboard/static/js/modules/execution-plans.test.js internal/admin/dashboard/static/js/modules/execution-plans-layout.test.js`
Expected: PASS
