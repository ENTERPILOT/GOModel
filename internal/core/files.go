package core

// FileCreateRequest represents an OpenAI-compatible file upload request.
// The actual request is multipart/form-data; Content is not serialized.
type FileCreateRequest struct {
	Purpose  string `json:"purpose"`
	Filename string `json:"filename,omitempty"`
	Content  []byte `json:"-"`
}

// FileObject represents an OpenAI-compatible file object.
type FileObject struct {
	ID            string  `json:"id"`
	Object        string  `json:"object"`
	Bytes         int64   `json:"bytes"`
	CreatedAt     int64   `json:"created_at"`
	Filename      string  `json:"filename"`
	Purpose       string  `json:"purpose"`
	Status        string  `json:"status,omitempty"`
	StatusDetails *string `json:"status_details,omitempty"`

	// Gateway enrichment for multi-provider deployments.
	Provider string `json:"provider,omitempty"`
}

// FileListResponse is returned by GET /v1/files.
type FileListResponse struct {
	Object  string       `json:"object"`
	Data    []FileObject `json:"data"`
	HasMore bool         `json:"has_more,omitempty"`
}

// FileDeleteResponse is returned by DELETE /v1/files/{id}.
type FileDeleteResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Deleted bool   `json:"deleted"`
}

// FileContentResponse wraps raw file bytes with response metadata.
type FileContentResponse struct {
	ID          string
	Filename    string
	ContentType string
	Data        []byte
}
