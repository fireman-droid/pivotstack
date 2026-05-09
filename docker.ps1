$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $scriptDir

if (-not (Test-Path ".env")) {
    Write-Host "[ERROR] .env file not found" -ForegroundColor Red
    Write-Host "Please copy .env.example to .env and configure it"
    Read-Host "Press Enter to exit"
    exit 1
}

docker info 2>&1 | Out-Null
if ($LASTEXITCODE -ne 0) {
    Write-Host "[ERROR] Docker is not running, please start Docker Desktop" -ForegroundColor Red
    Read-Host "Press Enter to exit"
    exit 1
}

function Show-Done {
    Write-Host ""
    Write-Host "============================================" -ForegroundColor Green
    Write-Host "  Started!" -ForegroundColor Green
    Write-Host ""
    Write-Host "  Admin:      http://127.0.0.1:8088/admin"
    Write-Host "  OpenAI API: http://127.0.0.1:8088/v1/chat/completions"
    Write-Host "  Claude API: http://127.0.0.1:8088/v1/messages"
    Write-Host "============================================" -ForegroundColor Green
    Write-Host ""
    docker compose ps
    Write-Host ""
    $vl = Read-Host "View logs? [y/N]"
    if ($vl -eq "y" -or $vl -eq "Y") { docker compose logs -f }
}

while ($true) {
    Clear-Host
    Write-Host "============================================" -ForegroundColor Cyan
    Write-Host "  PivotStack - Docker Manager" -ForegroundColor Cyan
    Write-Host "============================================" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "  1. Start          (no rebuild)"
    Write-Host "  2. Update+Restart (rebuild images)"
    Write-Host "  3. Stop"
    Write-Host "  4. Logs"
    Write-Host "  5. Status"
    Write-Host "  6. Exit"
    Write-Host ""
    $choice = Read-Host "Choose [1-6]"

    switch ($choice) {
        "1" {
            Write-Host "[START] Starting..." -ForegroundColor Green
            docker compose up -d
            if ($LASTEXITCODE -eq 0) { Show-Done }
            else { Write-Host "[ERROR] Start failed" -ForegroundColor Red; Read-Host "Press Enter" }
        }
        "2" {
            git rev-parse --git-dir 2>&1 | Out-Null
            if ($LASTEXITCODE -eq 0) {
                Write-Host "[GIT] Pulling latest code..." -ForegroundColor Yellow
                git pull
            } else {
                Write-Host "[SKIP] Not a git repo, skipping git pull"
            }
            Write-Host "[BUILD] Stopping old containers..." -ForegroundColor Yellow
            docker compose down
            Write-Host "[BUILD] Rebuilding and starting..." -ForegroundColor Yellow
            docker compose up -d --build
            if ($LASTEXITCODE -eq 0) { Show-Done }
            else { Write-Host "[ERROR] Build failed" -ForegroundColor Red; Read-Host "Press Enter" }
        }
        "3" {
            Write-Host "[STOP] Stopping services..." -ForegroundColor Red
            docker compose down
            Write-Host "[DONE] Stopped" -ForegroundColor Green
            Read-Host "Press Enter"
        }
        "4" {
            Write-Host "  1. kiro-go"
            Write-Host "  2. kiro-gateway"
            Write-Host "  3. All"
            $lc = Read-Host "Choose [1-3]"
            switch ($lc) {
                "1" { docker compose logs -f kiro-go }
                "2" { docker compose logs -f kiro-gateway }
                "3" { docker compose logs -f }
            }
        }
        "5" {
            docker compose ps
            Read-Host "Press Enter"
        }
        "6" { exit 0 }
    }
}
