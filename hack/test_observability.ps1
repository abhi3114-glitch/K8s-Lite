$ErrorActionPreference = "Stop"

function Test-Endpoint {
    param($Name, $Method, $Url)
    Write-Host "Testing $Name..." -NoNewline
    try {
        $response = Invoke-RestMethod -Method $Method -Uri $Url -ErrorAction Stop
        Write-Host " OK" -ForegroundColor Green
        return $response
    }
    catch {
        Write-Host " FAILED" -ForegroundColor Red
        Write-Host $_
        exit 1
    }
}

# 1. Start Server
Write-Host "Starting API Server..."
$p = Start-Process -FilePath ".\bin\apiserver.exe" -PassThru -NoNewWindow
Start-Sleep -Seconds 2

try {
    # 2. Generate Traffic
    Test-Endpoint -Name "Get Nodes" -Method Get -Url "http://localhost:8080/api/v1/nodes"
    Test-Endpoint -Name "Get Nodes Again" -Method Get -Url "http://localhost:8080/api/v1/nodes"

    # 3. Fetch Metrics
    Write-Host "Fetching Metrics..." -NoNewline
    $metrics = Invoke-RestMethod -Method Get -Uri "http://localhost:8080/metrics"
    Write-Host " OK" -ForegroundColor Green

    # 4. Verify Content
    if ($metrics -match 'http_requests_total') {
        Write-Host "Found 'http_requests_total' metric." -ForegroundColor Cyan
    }
    else {
        Write-Error "Metric 'http_requests_total' not found!"
        exit 1
    }

    if ($metrics -match 'path="/api/v1/nodes"') {
        Write-Host "Found specific path metric." -ForegroundColor Cyan
    }
    else {
        Write-Error "Path label not found in metrics!"
        exit 1
    }

    Write-Host "Observability Verified!" -ForegroundColor Cyan

}
finally {
    Stop-Process -Id $p.Id
}
