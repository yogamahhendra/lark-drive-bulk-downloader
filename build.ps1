# Build script for Lark Drive Bulk Downloader (Windows PowerShell)
$ErrorActionPreference = "Stop"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host " Building Lark Drive Bulk Downloader    " -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

Write-Host "`n[1/3] Running go mod tidy..." -ForegroundColor Yellow
go mod tidy

Write-Host "`n[2/3] Running tests..." -ForegroundColor Yellow
go test ./...

Write-Host "`n[3/3] Compiling lark-downloader.exe..." -ForegroundColor Yellow
go build -o lark-downloader.exe ./cmd/lark-downloader

if (Test-Path "lark-downloader.exe") {
    $size = (Get-Item "lark-downloader.exe").Length / 1MB
    Write-Host "`n[SUCCESS] Build completed successfully: lark-downloader.exe ($("{0:N2}" -f $size) MB)" -ForegroundColor Green
    Write-Host "Example usage: .\lark-downloader.exe `"https://example.larksuite.com/drive/folder/...`"" -ForegroundColor Cyan
} else {
    Write-Host "`n[ERROR] Build failed." -ForegroundColor Red
    exit 1
}
