package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Exit codes according to specification
const (
	ExitSuccess       = 0
	ExitDownloadError = 1
	ExitInvalidArgs   = 2
	ExitSourceError   = 3
)

// Config stores parsed command-line parameters.
type Config struct {
	URL       string
	OutputDir string
	Workers   int
	Retries   int
	Timeout   time.Duration
	Quiet     bool
	Help      bool
}

// ParseArgs parses command line arguments and flags, supporting flags before or after positional URL.
func ParseArgs(args []string, out io.Writer) (*Config, error) {
	if out == nil {
		out = os.Stdout
	}

	cfg := &Config{}

	// Separate flags and positional arguments so flags after positional URL work seamlessly
	var flagArgs []string
	var positionalArgs []string

	valueFlags := map[string]bool{
		"-u": true, "--url": true,
		"-o": true, "--output": true,
		"-w": true, "--workers": true,
		"-r": true, "--retries": true,
		"-t": true, "--timeout": true,
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-h" || arg == "--help" || arg == "-help" {
			cfg.Help = true
			PrintUsage(out)
			return cfg, nil
		}
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			if !strings.Contains(arg, "=") && valueFlags[arg] {
				if i+1 < len(args) {
					i++
					flagArgs = append(flagArgs, args[i])
				}
			}
		} else {
			positionalArgs = append(positionalArgs, arg)
		}
	}

	fs := flag.NewFlagSet("lark-downloader", flag.ContinueOnError)
	fs.SetOutput(out)

	fs.StringVar(&cfg.URL, "url", "", "Public Lark Drive share URL")
	fs.StringVar(&cfg.URL, "u", "", "Public Lark Drive share URL (shorthand)")

	fs.StringVar(&cfg.OutputDir, "output", "./output", "Target output directory")
	fs.StringVar(&cfg.OutputDir, "o", "./output", "Target output directory (shorthand)")

	fs.IntVar(&cfg.Workers, "workers", 4, "Number of concurrent download workers (1-64)")
	fs.IntVar(&cfg.Workers, "w", 4, "Number of concurrent download workers (shorthand)")

	fs.IntVar(&cfg.Retries, "retries", 3, "Number of retries for temporary failures")
	fs.IntVar(&cfg.Retries, "r", 3, "Number of retries for temporary failures (shorthand)")

	fs.DurationVar(&cfg.Timeout, "timeout", 15*time.Minute, "HTTP timeout per file download")
	fs.DurationVar(&cfg.Timeout, "t", 15*time.Minute, "HTTP timeout per file download (shorthand)")

	fs.BoolVar(&cfg.Quiet, "quiet", false, "Quiet mode: suppress detailed progress logs")
	fs.BoolVar(&cfg.Quiet, "q", false, "Quiet mode (shorthand)")

	fs.BoolVar(&cfg.Help, "help", false, "Show help message")
	fs.BoolVar(&cfg.Help, "h", false, "Show help message (shorthand)")

	fs.Usage = func() {
		PrintUsage(out)
	}

	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			cfg.Help = true
			PrintUsage(out)
			return cfg, nil
		}
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if cfg.Help {
		PrintUsage(out)
		return cfg, nil
	}

	if cfg.URL == "" && len(positionalArgs) > 0 {
		cfg.URL = positionalArgs[0]
	}

	if cfg.URL == "" {
		return nil, errors.New("share URL is required. Provide it as a positional argument or with --url")
	}

	if cfg.Workers < 1 || cfg.Workers > 64 {
		return nil, fmt.Errorf("invalid workers count %d: must be between 1 and 64", cfg.Workers)
	}

	if cfg.Retries < 0 {
		return nil, fmt.Errorf("invalid retries count %d: cannot be negative", cfg.Retries)
	}

	return cfg, nil
}

// PrintUsage prints the CLI help and usage examples.
func PrintUsage(w io.Writer) {
	usage := `Lark Drive Bulk Downloader (Go CLI)
Usage:
  lark-downloader [OPTIONS] <URL>
  lark-downloader --url <URL> [OPTIONS]

Examples:
  lark-downloader "https://example.larksuite.com/drive/folder/fldcn123"
  lark-downloader "https://example.larksuite.com/drive/folder/fldcn123" --output "./downloads"
  lark-downloader "https://example.larksuite.com/drive/folder/fldcn123" --workers 8 --retries 5
  lark-downloader "https://example.larksuite.com/drive/folder/fldcn123" --quiet

Options:
  -u, --url string       Public Lark Drive share URL
  -o, --output string    Target output directory (default: "./output")
  -w, --workers int      Number of concurrent download workers (default: 4)
  -r, --retries int      Number of retry attempts on temporary errors (default: 3)
  -t, --timeout duration HTTP download timeout (default: 15m)
  -q, --quiet            Quiet mode: suppress detailed per-file output
  -h, --help             Show this help message

Exit Codes:
  0: All files downloaded successfully
  1: Download completed with errors or partial failures
  2: Invalid arguments or configuration
  3: Share link cannot be accessed or files cannot be retrieved
`
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprint(w, usage)
}
