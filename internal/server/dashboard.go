package server

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/labstack/echo/v4"
)

//go:embed dashboard_dist
var dashboardFS embed.FS

// RegisterDashboard mounts the embedded SPA under /admin/.
// It serves static files from the embedded filesystem and falls back
// to index.html for client-side routing.
func RegisterDashboard(e *echo.Echo) {
	// Strip the "dashboard_dist" prefix so files are served from root
	sub, err := fs.Sub(dashboardFS, "dashboard_dist")
	if err != nil {
		panic("failed to create sub filesystem for dashboard: " + err.Error())
	}

	handler := spaHandler(sub)
	e.GET("/admin/*", handler)
	e.GET("/admin", func(c echo.Context) error {
		return c.Redirect(http.StatusMovedPermanently, "/admin/")
	})
}

// spaHandler returns an Echo handler that serves files from the given FS.
// If the requested file doesn't exist, it falls back to index.html
// to support client-side routing.
func spaHandler(fsys fs.FS) echo.HandlerFunc {
	fileServer := http.FileServer(http.FS(fsys))

	return func(c echo.Context) error {
		// Get the path relative to /admin/
		reqPath := c.Param("*")
		if reqPath == "" {
			reqPath = "index.html"
		}

		// Clean the path to prevent directory traversal
		reqPath = path.Clean(reqPath)
		reqPath = strings.TrimPrefix(reqPath, "/")

		// Check if the file exists in the embedded FS
		f, err := fsys.Open(reqPath)
		if err != nil {
			// File not found — serve index.html for client-side routing
			reqPath = "index.html"
		} else {
			f.Close()
		}

		// Rewrite request path for the file server
		c.Request().URL.Path = "/" + reqPath
		fileServer.ServeHTTP(c.Response(), c.Request())
		return nil
	}
}
