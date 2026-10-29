package lark

import (
	"fmt"
	"net/url"
	"strings"
)

// ParsedShareURL represents components extracted from a Lark/Feishu share URL.
type ParsedShareURL struct {
	RawURL       string
	Host         string
	RegionDomain string
	Token        string
	IsFolder     bool
	IsFile       bool
}

// ParseShareURL inspects and validates a Lark Drive public share URL.
func ParseShareURL(rawURL string) (*ParsedShareURL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("URL must include http or https scheme and host: %s", rawURL)
	}

	host := strings.ToLower(parsed.Host)
	if !strings.Contains(host, "larksuite.com") && !strings.Contains(host, "feishu.cn") && !strings.Contains(host, "larkoffice.com") {
		return nil, fmt.Errorf("unsupported host %q: must be a larksuite.com, feishu.cn, or larkoffice.com domain", host)
	}

	p := &ParsedShareURL{
		RawURL: rawURL,
		Host:   parsed.Host,
	}

	p.RegionDomain = ResolveStreamDomain(host)

	// Extract path token
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i, seg := range segments {
		if seg == "folder" && i+1 < len(segments) {
			p.IsFolder = true
			p.Token = segments[i+1]
			break
		}
		if seg == "file" && i+1 < len(segments) {
			p.IsFile = true
			p.Token = segments[i+1]
			break
		}
	}

	if p.Token == "" && len(segments) > 0 {
		p.Token = segments[len(segments)-1]
	}

	return p, nil
}

// ResolveStreamDomain returns the internal streaming domain for a given host.
func ResolveStreamDomain(host string) string {
	lower := strings.ToLower(host)

	if strings.Contains(lower, "feishu.cn") {
		return "internal-api-drive-stream.feishu.cn"
	}
	if strings.Contains(lower, ".jp.larksuite.com") {
		return "internal-api-drive-stream-jp.larksuite.com"
	}
	if strings.Contains(lower, ".sg.larksuite.com") {
		return "internal-api-drive-stream-sg.larksuite.com"
	}
	if strings.Contains(lower, ".us.larksuite.com") {
		return "internal-api-drive-stream-us.larksuite.com"
	}
	if strings.Contains(lower, "larkoffice.com") {
		return "internal-api-drive-stream.larkoffice.com"
	}

	return "internal-api-drive-stream.larksuite.com"
}

// BuildDownloadURL generates the stream download URL for a file token.
func BuildDownloadURL(streamDomain, fileToken string) string {
	return fmt.Sprintf("https://%s/space/api/box/stream/download/all/%s/?mount_point=explorer", streamDomain, fileToken)
}
