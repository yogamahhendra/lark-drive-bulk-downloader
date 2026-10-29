package lark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// BrowserExtractor implements LarkProvider using Chrome DevTools Protocol via chromedp.
type BrowserExtractor struct {
	CustomBrowserPath string
	Timeout           time.Duration
	OnProgress        func(found int)
}

// NewBrowserExtractor creates a new BrowserExtractor.
func NewBrowserExtractor() *BrowserExtractor {
	return &BrowserExtractor{
		Timeout: 90 * time.Second,
	}
}

// FindBrowserPath searches for an installed Chrome or Edge binary on the system.
func (e *BrowserExtractor) FindBrowserPath() (string, error) {
	if e.CustomBrowserPath != "" {
		if _, err := os.Stat(e.CustomBrowserPath); err == nil {
			return e.CustomBrowserPath, nil
		}
	}

	if envPath := os.Getenv("LARK_CHROME_PATH"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath, nil
		}
	}

	var candidates []string
	if runtime.GOOS == "windows" {
		candidates = []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
			filepath.Join(os.Getenv("LOCALAPPDATA"), `Google\Chrome\Application\chrome.exe`),
			filepath.Join(os.Getenv("LOCALAPPDATA"), `Microsoft\Edge\Application\msedge.exe`),
		}
		for _, name := range []string{"chrome.exe", "msedge.exe", "google-chrome.exe"} {
			if path, err := exec.LookPath(name); err == nil {
				candidates = append([]string{path}, candidates...)
			}
		}
	} else if runtime.GOOS == "darwin" {
		candidates = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	} else {
		candidates = []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/snap/bin/chromium",
		}
		for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"} {
			if path, err := exec.LookPath(name); err == nil {
				candidates = append([]string{path}, candidates...)
			}
		}
	}

	for _, path := range candidates {
		if path != "" {
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}

	return "", errors.New("no compatible browser (Google Chrome or Microsoft Edge) found on system. Please install Google Chrome or set LARK_CHROME_PATH")
}

type rawStoreObj struct {
	Name     string      `json:"name"`
	Token    string      `json:"token"`
	ObjToken string      `json:"obj_token"`
	Type     int         `json:"type"`
	Size     interface{} `json:"size"`
}

type evalResult struct {
	Title      string        `json:"title"`
	HasStore   bool          `json:"has_store"`
	NeedLogin  bool          `json:"need_login"`
	Empty      bool          `json:"empty"`
	FolderName string        `json:"folder_name"`
	Objs       []rawStoreObj `json:"objs"`
}

const defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// ListFiles extracts file list and visitor session credentials from a public Lark Drive URL.
func (e *BrowserExtractor) ListFiles(ctx context.Context, shareURL string) ([]RemoteFile, *SessionInfo, error) {
	parsedURL, err := ParseShareURL(shareURL)
	if err != nil {
		return nil, nil, err
	}

	browserPath, err := e.FindBrowserPath()
	if err != nil {
		return nil, nil, err
	}

	tempDir, err := os.MkdirTemp("", "lark_chrome_*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp chrome profile: %w", err)
	}
	defer os.RemoveAll(tempDir)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(browserPath),
		chromedp.UserDataDir(tempDir),
		chromedp.UserAgent(defaultUserAgent),
		chromedp.Flag("headless", "new"),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("mute-audio", true),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	taskCtx, cancelTask := chromedp.NewContext(allocCtx)
	defer cancelTask()

	timeoutCtx, cancelTimeout := context.WithTimeout(taskCtx, e.Timeout)
	defer cancelTimeout()

	var evalJSON string
	var cookies []*network.Cookie

	extractScript := `
	(() => {
		const title = document.title || '';
		const pageText = document.body ? document.body.innerText : '';
		const needLogin = pageText.includes("Log In") && !pageText.includes("Download") && !document.querySelector('.table-content, .tree-view, [data-token]');

		const s = window.store || window.__store__;
		if (!s) {
			return JSON.stringify({ title, needLogin, has_store: false, objs: [] });
		}

		const st = s.getState ? s.getState() : s.state;
		if (!st || !st.entities || !st.entities.objs) {
			return JSON.stringify({ title, needLogin, has_store: true, objs: [] });
		}

		const obs = st.entities.objs;
		const list = [];
		let folderName = '';

		for (const key of Object.keys(obs)) {
			const o = obs[key];
			const d = o.toJS ? o.toJS() : o;
			const type = typeof d.type === 'number' ? d.type : 0;
			const token = d.obj_token || d.token || key;
			const name = d.name || '';
			const size = d.size || 0;

			if (type === 0 && !folderName) {
				folderName = name;
			}
			list.push({ name, token, type, size });
		}

		return JSON.stringify({
			title,
			has_store: true,
			need_login: false,
			folder_name: folderName,
			objs: list
		});
	})()
	`

	// 1. Navigate to URL
	if err := chromedp.Do(timeoutCtx, chromedp.Navigate(parsedURL.RawURL)); err != nil {
		return nil, nil, fmt.Errorf("failed to navigate to Lark share page: %w", err)
	}

	// 2. Wait for initial page and store load
	initialDeadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(initialDeadline) {
		res, err := chromedp.Run(timeoutCtx, chromedp.Evaluate[string](extractScript))
		if err == nil && res != "" {
			var check evalResult
			if json.Unmarshal([]byte(res), &check) == nil && check.HasStore && len(check.Objs) > 1 {
				evalJSON = res
				if e.OnProgress != nil {
					e.OnProgress(len(check.Objs))
				}
				break
			}
		}
		time.Sleep(400 * time.Millisecond)
	}

	// 3. Focus list container and auto-scroll down to trigger infinite scroll for all items
	focusScript := `
	(() => {
		const target = document.querySelector('[class*="VirtualizerScroller"]') ||
		               document.querySelector('.explorer-file-list-virtualized__container') ||
		               document.querySelector('[class*="explorer-file-list-virtualized"]') ||
		               document.querySelector('div[scrollable="true"]') ||
		               document.querySelector('.table-content') ||
		               document.body;
		if (target) {
			target.focus();
			target.click();
			return "focused";
		}
		return "none";
	})()
	`
	_, _ = chromedp.Run(timeoutCtx, chromedp.Evaluate[string](focusScript))

	scrollScript := `
	(() => {
		const scroller = document.querySelector('[class*="VirtualizerScroller"]') ||
		                 document.querySelector('.explorer-file-list-virtualized__container') ||
		                 document.querySelector('[class*="explorer-file-list-virtualized"]') ||
		                 document.querySelector('div[scrollable="true"]') ||
		                 document.querySelector('.table-content') ||
		                 document.querySelector('.standard-scrollbar');
		if (scroller) {
			scroller.scrollTop = scroller.scrollHeight;
			scroller.dispatchEvent(new Event('scroll', { bubbles: true }));
			scroller.dispatchEvent(new WheelEvent('wheel', {
				deltaY: 1000,
				bubbles: true,
				cancelable: true
			}));
			return "scrolled_container";
		}
		window.scrollBy(0, 10000);
		window.dispatchEvent(new Event('scroll', { bubbles: true }));
		return "scrolled_window";
	})()
	`

	lastCount := 0
	unchangedRounds := 0

	for scrollStep := 0; scrollStep < 60; scrollStep++ {
		_, _ = chromedp.Run(timeoutCtx, chromedp.Evaluate[string](scrollScript))
		time.Sleep(600 * time.Millisecond)

		res, err := chromedp.Run(timeoutCtx, chromedp.Evaluate[string](extractScript))
		if err == nil && res != "" {
			var check evalResult
			if json.Unmarshal([]byte(res), &check) == nil && check.HasStore {
				evalJSON = res
				currentCount := len(check.Objs)
				if e.OnProgress != nil && currentCount > 0 {
					e.OnProgress(currentCount)
				}
				if currentCount == lastCount && currentCount > 1 {
					unchangedRounds++
					if unchangedRounds >= 5 {
						break // All files loaded!
					}
				} else {
					unchangedRounds = 0
					lastCount = currentCount
				}
			}
		}
	}

	if evalJSON == "" {
		res, err := chromedp.Run(timeoutCtx, chromedp.Evaluate[string](extractScript))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to evaluate page store: %w", err)
		}
		evalJSON = res
	}

	// 4. Retrieve all cookies from visitor session
	cookieRes, err := chromedp.Call(timeoutCtx, network.GetCookies, network.GetCookiesParams{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve visitor cookies: %w", err)
	}
	cookies = cookieRes.Cookies

	var parsedEval evalResult
	if err := json.Unmarshal([]byte(evalJSON), &parsedEval); err != nil {
		return nil, nil, fmt.Errorf("failed to parse page extraction result: %w", err)
	}

	if parsedEval.NeedLogin {
		return nil, nil, errors.New("access denied: this link requires account authentication or permission was revoked by the owner")
	}

	var cookiePairs []string
	for _, c := range cookies {
		cookiePairs = append(cookiePairs, fmt.Sprintf("%s=%s", c.Name, c.Value))
	}
	cookieHeader := strings.Join(cookiePairs, "; ")

	session := &SessionInfo{
		ShareURL:  shareURL,
		Host:      parsedURL.Host,
		Cookies:   cookieHeader,
		UserAgent: defaultUserAgent,
	}

	var remoteFiles []RemoteFile
	seen := make(map[string]bool)

	for _, item := range parsedEval.Objs {
		// Type 0 represents folder containers; non-zero types (e.g. 12, 11, 2) are downloadable files
		if item.Type == 0 {
			continue // Skip folder containers from direct file download
		}
		if item.Token == "" || seen[item.Token] {
			continue
		}
		seen[item.Token] = true

		var sizeInt int64
		switch v := item.Size.(type) {
		case float64:
			sizeInt = int64(v)
		case string:
			sizeInt, _ = strconv.ParseInt(v, 10, 64)
		case json.Number:
			sizeInt, _ = v.Int64()
		}

		name := item.Name
		if name == "" {
			name = item.Token
		}

		dlURL := BuildDownloadURL(parsedURL.RegionDomain, item.Token)

		remoteFiles = append(remoteFiles, RemoteFile{
			ID:          item.Token,
			Name:        name,
			Path:        name,
			Size:        sizeInt,
			Type:        item.Type,
			DownloadURL: dlURL,
		})
	}

	return remoteFiles, session, nil
}
