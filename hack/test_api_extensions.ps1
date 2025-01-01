$ErrorActionPreference = "Stop"

function Test-Endpoint {
    param($Name, $Method, $Url, $Body)
    Write-Host "Testing $Name..." -NoNewline
    try {
        if ($Body) {
            $response = Invoke-RestMethod -Method $Method -Uri $Url -Body $Body -ContentType "application/json" -ErrorAction Stop
        }
        else {
            $response = Invoke-RestMethod -Method $Method -Uri $Url -ErrorAction Stop
        }
        Write-Host " OK" -ForegroundColor Green
        return $response
    }
    catch {
        Write-Host " FAILED" -ForegroundColor Red
        Write-Host $_
        exit 1
    }
}

# 1. Start API Server
Write-Host "Starting API Server..."
$p = Start-Process -FilePath ".\bin\apiserver.exe" -PassThru -NoNewWindow
Start-Sleep -Seconds 2

try {
    # 2. Create Service
    $svcJson = '{
        "apiVersion": "v1",
        "kind": "Service",
        "metadata": { "name": "nginx-svc" },
        "spec": {
            "selector": { "app": "nginx" },
            "ports": [{ "port": 80, "targetPort": 80, "nodePort": 30080 }]
        }
    }'
    Test-Endpoint -Name "Create Service" -Method Post -Url "http://localhost:8080/api/v1/services" -Body $svcJson

    # 3. Create Endpoints
    $epJson = '{
        "apiVersion": "v1",
        "kind": "Endpoints",
        "metadata": { "name": "nginx-svc" },
        "subsets": [{
            "addresses": [{ "ip": "1.2.3.4" }],
            "ports": [{ "port": 80 }]
        }]
    }'
    Test-Endpoint -Name "Create Endpoints" -Method Post -Url "http://localhost:8080/api/v1/endpoints" -Body $epJson

    # 4. Verify
    $svc = Test-Endpoint -Name "Get Service" -Method Get -Url "http://localhost:8080/api/v1/services/nginx-svc"
    if ($svc.spec.ports[0].nodePort -ne 30080) { Write-Error "Service Port Mismatch" }

    $ep = Test-Endpoint -Name "Get Endpoints" -Method Get -Url "http://localhost:8080/api/v1/endpoints/nginx-svc"
    if ($ep.subsets[0].addresses[0].ip -ne "1.2.3.4") { Write-Error "Endpoint IP Mismatch" }

    Write-Host "Service & Endpoints API Verified!" -ForegroundColor Cyan

}
finally {
    Stop-Process -Id $p.Id -ErrorAction SilentlyContinue
}
