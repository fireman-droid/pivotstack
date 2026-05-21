param([string]$KeyVal = 'sk-c9dc408fd6fca083cc705cadbdc040e8')

$cfgPath = 'data/config.json'
$logPath = 'data/call_logs.jsonl'

$cfg = Get-Content $cfgPath -Raw -Encoding UTF8 | ConvertFrom-Json
$k = $cfg.apiKeys | Where-Object { $_.key -eq $KeyVal }
if (-not $k) {
    Write-Host "本地 config 找不到 key $KeyVal" -ForegroundColor Red
    return
}

$CnyPerDollar = 0.05  # 1 虚拟$ = 0.05 ¥ (因为 1¥ = 20 虚拟$)

Write-Host "=== Key 基本信息 ===" -ForegroundColor Cyan
Write-Host ("  ID            : " + $k.id)
Write-Host ("  Note          : " + $k.note)
Write-Host ("  Plan          : " + $k.plan)
Write-Host ("  Enabled       : " + $k.enabled)
Write-Host ("  Balance       : `$" + $k.balance + "  (¥" + [math]::Round($k.balance * $CnyPerDollar, 4) + ")")
Write-Host ("  GiftBalance   : `$" + $k.giftBalance + "  (¥" + [math]::Round($k.giftBalance * $CnyPerDollar, 4) + ")")
Write-Host ("  TotalRecharged: `$" + $k.totalRecharged + "  (¥" + [math]::Round($k.totalRecharged * $CnyPerDollar, 2) + ")")
Write-Host ("  TotalGifted   : `$" + $k.totalGifted + "  (¥" + [math]::Round($k.totalGifted * $CnyPerDollar, 2) + ")")
Write-Host ("  Requests      : " + $k.requests)
Write-Host ("  Errors        : " + $k.errors)
Write-Host ("  Tokens(累计)  : " + $k.tokens)
Write-Host ("  Credits(累计) : " + $k.credits + "  (¥" + [math]::Round($k.credits * $CnyPerDollar, 2) + ")")
$epoch = Get-Date '1970-01-01'
if ($k.createdAt) { Write-Host ("  CreatedAt     : " + $epoch.AddSeconds($k.createdAt).ToLocalTime().ToString('yyyy-MM-dd HH:mm')) }
if ($k.lastUsed)  { Write-Host ("  LastUsed      : " + $epoch.AddSeconds($k.lastUsed).ToLocalTime().ToString('yyyy-MM-dd HH:mm')) }

if (-not (Test-Path $logPath)) {
    Write-Host ""
    Write-Host "本地 call_logs.jsonl 不存在，跳过流水分析" -ForegroundColor Yellow
    return
}

Write-Host ""
Write-Host "=== 今天的调用流水 (本地 data/call_logs.jsonl) ===" -ForegroundColor Cyan
$today = (Get-Date).ToString('yyyy-MM-dd')
$todayStart = [int][double]::Parse((Get-Date $today).ToString('yyyyMMddHHmmss').Substring(0,4))  # only to scope; better: epoch
$todayEpoch = [int][double](Get-Date $today -UFormat %s)

$keyID = $k.id
$todayLogs = @()
$totalLogs = 0
Get-Content $logPath -Encoding UTF8 | ForEach-Object {
    $totalLogs++
    try {
        $row = $_ | ConvertFrom-Json
        if ($row.api_key_id -eq $keyID -and $row.timestamp -ge $todayEpoch) {
            $todayLogs += $row
        }
    } catch {}
}

Write-Host ("  总行数 : " + $totalLogs)
Write-Host ("  今日   : " + $todayLogs.Count + " 条")

if ($todayLogs.Count -eq 0) {
    Write-Host "  今日这个 key 没有调用记录" -ForegroundColor Yellow
    return
}

$sumCost = ($todayLogs | Measure-Object cost_usd -Sum).Sum
$sumPaid = ($todayLogs | Measure-Object paid_credits -Sum).Sum
$sumGift = ($todayLogs | Measure-Object gifted_credits -Sum).Sum
$sumIn   = ($todayLogs | Measure-Object input_tokens -Sum).Sum
$sumOut  = ($todayLogs | Measure-Object output_tokens -Sum).Sum
$success = ($todayLogs | Where-Object { $_.status -eq 'success' }).Count
$errCnt  = ($todayLogs | Where-Object { $_.status -eq 'error' }).Count

Write-Host ""
Write-Host "=== 今日合计 ===" -ForegroundColor Green
Write-Host ("  success / error : " + $success + " / " + $errCnt)
Write-Host ("  input tokens    : " + $sumIn)
Write-Host ("  output tokens   : " + $sumOut)
Write-Host ("  paid_credits    : `$" + [math]::Round($sumPaid, 6) + "  (¥" + [math]::Round($sumPaid * $CnyPerDollar, 4) + ")")
Write-Host ("  gifted_credits  : `$" + [math]::Round($sumGift, 6) + "  (¥" + [math]::Round($sumGift * $CnyPerDollar, 4) + ")")
Write-Host ("  cost_usd        : `$" + [math]::Round($sumCost, 6) + "  (¥" + [math]::Round($sumCost * $CnyPerDollar, 4) + ")")

Write-Host ""
Write-Host "=== 按模型聚合 ===" -ForegroundColor Green
$todayLogs | Group-Object original_model | ForEach-Object {
    $g = $_
    $gCost = ($g.Group | Measure-Object cost_usd -Sum).Sum
    $gIn = ($g.Group | Measure-Object input_tokens -Sum).Sum
    $gOut = ($g.Group | Measure-Object output_tokens -Sum).Sum
    Write-Host ("  " + $g.Name + " : " + $g.Count + " 次 | in=" + $gIn + " out=" + $gOut + " | `$" + [math]::Round($gCost, 4) + " (¥" + [math]::Round($gCost * $CnyPerDollar, 4) + ")")
}

Write-Host ""
Write-Host "=== 最近 10 条 ===" -ForegroundColor Green
$todayLogs | Sort-Object timestamp -Descending | Select-Object -First 10 | ForEach-Object {
    $r = $_
    $statusIcon = if ($r.status -eq 'success') { 'OK' } else { 'X ' }
    Write-Host ("  [" + $statusIcon + "] " + $r.time + " " + $r.original_model + " in=" + $r.input_tokens + " out=" + $r.output_tokens + " cost=`$" + ([math]::Round($r.cost_usd, 4)) + " ch=" + $r.channel_id)
}
