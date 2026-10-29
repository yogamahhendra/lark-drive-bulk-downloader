package lark

import (
	"testing"
)

func TestParseShareURL(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		wantHost     string
		wantDomain   string
		wantToken    string
		wantFolder   bool
		wantFile     bool
		expectErr    bool
	}{
		{
			name:         "JP Lark Folder",
			url:          "https://kjpflil5k85o.jp.larksuite.com/drive/folder/XP0cfwQQclSLZsds27Cjuknxppg",
			wantHost:     "kjpflil5k85o.jp.larksuite.com",
			wantDomain:   "internal-api-drive-stream-jp.larksuite.com",
			wantToken:    "XP0cfwQQclSLZsds27Cjuknxppg",
			wantFolder:   true,
			wantFile:     false,
			expectErr:    false,
		},
		{
			name:         "Feishu File",
			url:          "https://mycompany.feishu.cn/file/boxcn12345678",
			wantHost:     "mycompany.feishu.cn",
			wantDomain:   "internal-api-drive-stream.feishu.cn",
			wantToken:    "boxcn12345678",
			wantFolder:   false,
			wantFile:     true,
			expectErr:    false,
		},
		{
			name:         "SG Lark Folder",
			url:          "https://myorg.sg.larksuite.com/drive/folder/fldcn123?from=copy",
			wantHost:     "myorg.sg.larksuite.com",
			wantDomain:   "internal-api-drive-stream-sg.larksuite.com",
			wantToken:    "fldcn123",
			wantFolder:   true,
			wantFile:     false,
			expectErr:    false,
		},
		{
			name:         "Invalid host",
			url:          "https://google.com/drive/folder/123",
			expectErr:    true,
		},
		{
			name:         "Missing scheme",
			url:          "larksuite.com/drive/folder/123",
			expectErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := ParseShareURL(tc.url)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error for %s, got nil", tc.url)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Host != tc.wantHost {
				t.Errorf("Host: got %s, want %s", res.Host, tc.wantHost)
			}
			if res.RegionDomain != tc.wantDomain {
				t.Errorf("RegionDomain: got %s, want %s", res.RegionDomain, tc.wantDomain)
			}
			if res.Token != tc.wantToken {
				t.Errorf("Token: got %s, want %s", res.Token, tc.wantToken)
			}
			if res.IsFolder != tc.wantFolder {
				t.Errorf("IsFolder: got %v, want %v", res.IsFolder, tc.wantFolder)
			}
			if res.IsFile != tc.wantFile {
				t.Errorf("IsFile: got %v, want %v", res.IsFile, tc.wantFile)
			}
		})
	}
}

func TestBuildDownloadURL(t *testing.T) {
	got := BuildDownloadURL("internal-api-drive-stream-jp.larksuite.com", "token123")
	want := "https://internal-api-drive-stream-jp.larksuite.com/space/api/box/stream/download/all/token123/?mount_point=explorer"
	if got != want {
		t.Errorf("BuildDownloadURL() = %s, want %s", got, want)
	}
}
