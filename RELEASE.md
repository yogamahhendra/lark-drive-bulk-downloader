# Release Notes: v0.0.1 (Initial Release)

**Release Date:** October 29, 2026  
**Status:** General Availability (GA)  
**Binary:** `lark-downloader.exe` / `lark-downloader`  

---

## Overview

**Lark Drive Bulk Downloader v0.0.1** is the inaugural general availability release of an enterprise-grade command-line utility engineered to download files in bulk from publicly shared **Lark Drive** and **Feishu Drive** folders without requiring account authentication or manual one-by-one file saving.

This release addresses Lark's inherent browser session protections, virtualized DOM constraints, and the lack of native archive/zip export buttons for unauthenticated public visitors.

---

## Key Features in v0.0.1

### 1. Hybrid Architecture (CDP + Native Go HTTP Engine)
- **Transient Headless Browser Discovery**: Automatically provisions a lightweight, headless Google Chrome or Microsoft Edge instance via the Chrome DevTools Protocol (`chromedp`) to bypass regional anti-bot verification and extract ephemeral visitor session cookies (`passport_web_did`, `t_beda37`, `QXV0aHpDb250ZXh0`, etc.).
- **Zero-Footprint Browser Teardown**: The browser session runs only for the duration of folder metadata extraction (typically 2–15 seconds) and is immediately terminated, ensuring system RAM is conserved.
- **Pure Go Parallel Streaming Engine**: Direct payload downloads bypass browser rendering entirely, leveraging a high-performance Go `net/http` worker pool.

### 2. Virtualized Scroller Auto-Unrolling
- **Bypasses the 50-Item Threshold**: Lark Drive's web application limits the initial render to 50 items. v0.0.1 implements dynamic container targeting (`[class*="VirtualizerScroller"]`, `.explorer-file-list-virtualized__container`) with synthesized scroll and mouse-wheel events to continuously hydrate the client-side Redux store (`entities.objs`) until all files are loaded.
- **Steady-State Detection**: Automatically detects when the folder has been fully scrolled and all remote items are indexed before commencing downloads.
- **Real-Time Discovery Feedback**: Displays a live counter in the terminal during folder discovery (`Discovering files... (X items loaded)`).

### 3. High-Concurrency Download Engine
- **Configurable Worker Pool**: Parallelize downloads across 1 to 64 concurrent workers (`--workers`, default: `4`).
- **Zero-RAM Disk Streaming**: Data streams directly from HTTP response bodies to destination files on disk using efficient buffer allocation, supporting arbitrarily large files without memory exhaustion.
- **Persistent HTTP Connection Pooling**: Reuses TCP/TLS connections across requests (`MaxIdleConns: 100`, `IdleConnTimeout: 90s`) to maximize throughput.

### 4. Enterprise-Grade Resilience & Fault Tolerance
- **Exponential Backoff with Full Jitter**: Automatically retries transient network interruptions, dropped sockets, and HTTP `429 Too Many Requests` or `5xx Server Error` responses.
- **Fast-Fail on Permanent Errors**: Immediately halts retry loops on fatal status codes (HTTP `401 Unauthorized`, `403 Forbidden`, `404 Not Found`).
- **MIME & Response Validation**: Validates `Content-Type` headers to prevent saving HTML error pages as downloaded files.

### 5. Path Security & Filesystem Safety
- **Path Traversal Protection**: Sanitizes remote file names, stripping relative traversal vectors (`../`, `..\`) and directory prefixes.
- **Windows & POSIX Normalization**: Replaces illegal filesystem characters (`<>:"/\|?*` and ASCII control codes) with safe underscores and guards against reserved Windows names (`CON`, `PRN`, `AUX`, `NUL`, `COM1-9`, `LPT1-9`).
- **Collision Avoidance**: Automatically appends sequential numeric suffixes (`filename (1).ext`) if a file with the same name already exists.
- **Hierarchical Path Readiness**: Fully prepared for nested subdirectories with parent directory auto-creation (`os.MkdirAll`).

### 6. Session Isolation & Machine-Readable Audit Reports
- **Timestamped Session Folders**: Organizes each download run into an isolated directory formatted as `YYYYMMDD_HHMMSS`.
- **Comprehensive Audit Log (`download-report.json`)**: Emits structured telemetry including start/end timestamps, elapsed duration, source URL, total byte count, and per-file transfer statuses (downloaded bytes, attempt count, error messages).
- **Zero Credential Leakage**: Visitor session cookies and internal auth headers are maintained strictly in volatile memory and omitted from audit logs.

### 7. Multi-Region Compatibility
- **Regional Domain Routing**: Automatically resolves and maps regional download streaming endpoints for:
  - Japan (`*.jp.larksuite.com` &rarr; `internal-api-drive-stream-jp.larksuite.com`)
  - Singapore (`*.sg.larksuite.com` &rarr; `internal-api-drive-stream-sg.larksuite.com`)
  - Global / North America (`*.larksuite.com` &rarr; `internal-api-drive-stream.larksuite.com`)
  - Mainland China / Feishu (`*.feishu.cn` &rarr; `internal-api-drive-stream.feishu.cn`)

---

## Command-Line Usage

```bash
# Basic usage
lark-downloader "https://company.larksuite.com/drive/folder/fldcnExampleFolderToken123"

# Advanced configuration
lark-downloader "https://company.larksuite.com/drive/folder/fldcnExampleFolderToken123" \
  --output "./downloads" \
  --workers 8 \
  --retries 5 \
  --timeout 20m \
  --quiet
```

### Process Exit Codes

| Code | Constant | Meaning |
|:---:|---|---|
| `0` | `ExitSuccess` | All files discovered and downloaded successfully. |
| `1` | `ExitDownloadError` | One or more files failed to download after all retry attempts. |
| `2` | `ExitInvalidArgs` | Invalid command-line arguments or invalid share URL format. |
| `3` | `ExitSourceError` | Link unreachable, browser unavailable, or access denied by host. |

---

## Verification & Validation

Release v0.0.1 has undergone rigorous end-to-end testing:
- **Test Suite**: 100% pass across all unit and integration test packages:
  - `internal/cli`
  - `internal/downloader`
  - `internal/filesystem`
  - `internal/lark`
  - `internal/report`
- **Real-World Live Benchmark**:
  - **Dataset**: Public Lark Drive folder containing 196 high-resolution media files.
  - **Result**: 196 / 196 files successfully discovered and downloaded (909.12 MB total) with 0 failures in 3 minutes 21 seconds using 6 concurrent workers.

---

## Known Scope & Future Roadmap

- **v0.0.1 Scope**: Focuses on complete, unrolled flat-file discovery within the targeted shared folder. Non-file folder entities (`type: 0`) are safely excluded from file streaming to avoid HTTP 400 errors.
- **v0.0.2 Planned**:
  - Automated recursive subfolder crawler queue to traverse nested directories and mirror the remote folder hierarchy locally.
  - Checksum validation (MD5/SHA256) where supported by remote metadata headers.
  - Docker container image for headless server deployments without host browser installation.
