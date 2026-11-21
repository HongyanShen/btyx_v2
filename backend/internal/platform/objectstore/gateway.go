package objectstore

import (
	"context"
	"time"

	"baituber.cn/yanxuan/v2/internal/domain/enum"
)

type Reference struct {
	Bucket    string
	Key       string
	VersionID string
}

type Metadata struct {
	Size      int64
	MediaType string
	ETag      string
	VersionID string
}

type UploadRequest struct {
	Object    Reference
	Size      int64
	MediaType enum.FileMediaType
	Expires   time.Duration
}

type UploadFields struct {
	Key             string `json:"key"`
	Policy          string `json:"policy"`
	Algorithm       string `json:"x-tos-algorithm"`
	Credential      string `json:"x-tos-credential"`
	Date            string `json:"x-tos-date"`
	Signature       string `json:"x-tos-signature"`
	ACL             string `json:"acl"`
	ContentType     string `json:"Content-Type"`
	SuccessStatus   string `json:"success_action_status"`
	ForbidOverwrite string `json:"x-tos-forbid-overwrite"`
}

type UploadForm struct {
	URL    string
	Fields UploadFields
}

type CopyRequest struct {
	Source      Reference
	SourceETag  string
	Destination Reference
	MediaType   enum.FileMediaType
	FileName    string
}

// Gateway is the storage protocol boundary. File contents upload directly to
// TOS; the backend only reads a short prefix and promotes verified objects.
type Gateway interface {
	SignUpload(context.Context, UploadRequest) (UploadForm, error)
	Inspect(context.Context, Reference) (Metadata, error)
	ReadPrefix(context.Context, Reference, string) ([]byte, error)
	Copy(context.Context, CopyRequest) (Metadata, error)
	Delete(context.Context, Reference) error
	SignDownload(context.Context, Reference, time.Duration) (string, error)
}
