# AGENTS Instructions

- When any API endpoint is added, removed, or changed (path, method, params, or request/response schema), update its Swagger annotations and run `make swagger`.
- Keep endpoint documentation concise and accurate in code comments (summary, params, success/error responses).
- Commit regenerated Swagger files (`cmd/gomodel/docs/docs.go` and `cmd/gomodel/docs/swagger.json`) together with the API change.
