# 本地 8089 测试服务重启脚本
# 必带 VPN_PROXY_URL — 直连美东 Kiro 上游会被伪装成 INVALID_MODEL_ID 拒掉。
# 用法：.\restart-local.ps1   （从仓库根 kiro-stack/kiro-go 跑）

$ErrorActionPreference = "Stop"

# 1. kill 当前 8089
$p = Get-NetTCPConnection -LocalPort 8089 -ErrorAction SilentlyContinue | Select-Object -First 1
if ($p) {
    Stop-Process -Id $p.OwningProcess -Force
    Write-Host "Killed PID $($p.OwningProcess) on :8089"
    Start-Sleep 2
}

# 2. 探测代理（默认 clash 7897；如改了端口在这里改）
$proxyPort = 7897
$proxyUrl  = "http://127.0.0.1:$proxyPort"
$proxyAlive = (Get-NetTCPConnection -LocalPort $proxyPort -State Listen -ErrorAction SilentlyContinue) -ne $null
if (-not $proxyAlive) {
    Write-Host "WARNING: $proxyUrl 没监听，Kiro 直连大概率会被 AWS 拒（INVALID_MODEL_ID 伪装）" -ForegroundColor Yellow
} else {
    Write-Host "Proxy alive on $proxyUrl" -ForegroundColor Green
}

# 3. 启服务
$env:PORT          = "8089"
$env:CONFIG_PATH   = "data_local/config.json"
$env:VPN_PROXY_URL = $proxyUrl
$proc = Start-Process -FilePath ".\kiro-go-test.exe" `
    -RedirectStandardOutput "kiro-test.stdout.log" `
    -RedirectStandardError  "kiro-test.stderr.log" `
    -WindowStyle Hidden -PassThru
Write-Host "Started PID $($proc.Id) on :8089 with VPN_PROXY_URL=$proxyUrl"

# 4. 健康检查
Start-Sleep 3
try {
    $code = (Invoke-WebRequest http://127.0.0.1:8089/health -UseBasicParsing).StatusCode
    Write-Host "/health → $code" -ForegroundColor Green
} catch {
    Write-Host "/health failed: $($_.Exception.Message)" -ForegroundColor Red
}
