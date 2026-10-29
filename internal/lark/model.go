package lark

import "context"

// RemoteFile represents a downloadable file discovered from a Lark Drive share link.
type RemoteFile struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	Type        int    `json:"type"`
	DownloadURL string `json:"download_url"`
}

// SessionInfo holds the necessary authenticated context from the visitor session.
type SessionInfo struct {
	ShareURL  string `json:"share_url"`
	Host      string `json:"host"`
	Cookies   string `json:"cookies"`
	UserAgent string `json:"user_agent"`
}

// LarkProvider defines the interface for discovering files from a Lark public share URL.
type LarkProvider interface {
	ListFiles(ctx context.Context, shareURL string) ([]RemoteFile, *SessionInfo, error)
}
