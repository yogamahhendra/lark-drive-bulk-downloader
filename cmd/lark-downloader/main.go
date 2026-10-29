package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lark-downloader/internal/cli"
	"lark-downloader/internal/downloader"
	"lark-downloader/internal/filesystem"
	"lark-downloader/internal/lark"
	"lark-downloader/internal/report"
)

func main() {
	cfg, err := cli.ParseArgs(os.Args[1:], os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		cli.PrintUsage(os.Stderr)
		os.Exit(cli.ExitInvalidArgs)
	}

	if cfg.Help {
		os.Exit(cli.ExitSuccess)
	}

	startTime := time.Now()

	fmt.Println("Lark Drive Bulk Downloader")
	fmt.Println("==========================")
	fmt.Printf("Share URL : %s\n", cfg.URL)
	fmt.Printf("Workers   : %d\n", cfg.Workers)
	fmt.Printf("Retries   : %d\n", cfg.Retries)
	fmt.Printf("Output    : %s\n\n", cfg.OutputDir)

	// Step 1: Create session output directory
	sessionDir, err := filesystem.CreateSessionDir(cfg.OutputDir, startTime)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to create session directory: %v\n", err)
		os.Exit(cli.ExitInvalidArgs)
	}
	fmt.Printf("Session Dir : %s\n\n", sessionDir)

	// Set up cancellation context
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Step 2: Inspect shared link and discover files
	fmt.Println("[1/4] Inspecting shared link...")
	fmt.Print("[2/4] Discovering files...")

	extractor := lark.NewBrowserExtractor()
	if !cfg.Quiet {
		extractor.OnProgress = func(found int) {
			displayCount := found - 1
			if displayCount < 0 {
				displayCount = 0
			}
			fmt.Printf("\r[2/4] Discovering files... (%d items loaded)", displayCount)
		}
	}

	files, session, err := extractor.ListFiles(ctx, cfg.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n[ERROR] Failed to discover files from Lark Drive:\n  %v\n\n", err)
		fmt.Fprintln(os.Stderr, "Diagnostic hints:")
		fmt.Fprintln(os.Stderr, "  1. Make sure Google Chrome or Microsoft Edge is installed.")
		fmt.Fprintln(os.Stderr, "  2. Verify that the public link is active and 'Anyone with link can view' is enabled.")
		fmt.Fprintln(os.Stderr, "  3. Check if the link requires tenant login or password.")
		os.Exit(cli.ExitSourceError)
	}
	fmt.Printf("\r[2/4] Discovering files... Done.                  \n")

	var totalExpectedBytes int64
	for _, f := range files {
		totalExpectedBytes += f.Size
	}

	fmt.Printf("Found %d file(s) (%s)\n\n", len(files), formatBytes(totalExpectedBytes))

	if len(files) == 0 {
		fmt.Println("No downloadable files found in this shared folder.")
		rep := &report.DownloadReport{
			StartTime:  startTime,
			EndTime:    time.Now(),
			SourceURL:  cfg.URL,
			TotalFiles: 0,
		}
		_, _ = report.WriteReport(sessionDir, rep)
		os.Exit(cli.ExitSuccess)
	}

	// Step 3: Bulk download engine
	fmt.Println("[3/4] Downloading files...")

	engineCfg := downloader.EngineConfig{
		Workers:   cfg.Workers,
		Retries:   cfg.Retries,
		Timeout:   cfg.Timeout,
		OutputDir: sessionDir,
		Quiet:     cfg.Quiet,
	}

	engine := downloader.NewEngine(engineCfg, session)
	fileResults := engine.DownloadAll(ctx, files)

	// Step 4: Write download report
	fmt.Println("\n[4/4] Writing report...")
	endTime := time.Now()

	var successCount, failedCount int
	var totalDownloadedBytes int64
	for _, item := range fileResults {
		if item.Status == "success" {
			successCount++
			totalDownloadedBytes += item.BytesDownloaded
		} else {
			failedCount++
		}
	}

	rep := &report.DownloadReport{
		StartTime:            startTime,
		EndTime:              endTime,
		SourceURL:            cfg.URL,
		TotalFiles:           len(files),
		SuccessCount:         successCount,
		FailedCount:          failedCount,
		TotalBytesDownloaded: totalDownloadedBytes,
		Files:                fileResults,
	}

	reportPath, err := report.WriteReport(sessionDir, rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] Failed to write download report: %v\n", err)
	}

	fmt.Println("\nDownload summary:")
	fmt.Printf("  Output  : %s\n", sessionDir)
	fmt.Printf("  Success : %d\n", successCount)
	fmt.Printf("  Failed  : %d\n", failedCount)
	fmt.Printf("  Total   : %s downloaded\n", formatBytes(totalDownloadedBytes))
	fmt.Printf("  Duration: %s\n", endTime.Sub(startTime).Round(time.Second))
	if reportPath != "" {
		fmt.Printf("  Report  : %s\n", reportPath)
	}

	if failedCount > 0 {
		os.Exit(cli.ExitDownloadError)
	}

	os.Exit(cli.ExitSuccess)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
