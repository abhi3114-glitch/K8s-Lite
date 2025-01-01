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

# 1. Start Cluster (API, CM, Kubelet)
# Assuming they are running or specific start script.
# For this test script, we assume they are running.
# User should have started them or we can try to start them here?
# Let's try to start them fresh.

Write-Host "Killing old processes..."
Stop-Process -Name "apiserver", "controller-manager", "kubelet" -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1

Write-Host "Starting API Server..."
$pApi = Start-Process -FilePath ".\bin\apiserver.exe" -ArgumentList "-data-file=test-net.db" -PassThru -NoNewWindow
Start-Sleep -Seconds 2

Write-Host "Starting Controller Manager..."
$pCm = Start-Process -FilePath ".\bin\controller-manager.exe" -ArgumentList "--api-url=http://localhost:8080" -PassThru -NoNewWindow
Start-Sleep -Seconds 1

Write-Host "Starting Kubelet..."
$pKube = Start-Process -FilePath ".\bin\kubelet.exe" -ArgumentList "--node-name=node1", "--api-url=http://localhost:8080" -PassThru -NoNewWindow
Start-Sleep -Seconds 3

try {
    # 2. Create Deployment
    $depJson = '{
        "apiVersion": "apps/v1",
        "kind": "Deployment",
        "metadata": { "name": "net-dep" },
        "spec": {
            "replicas": 3,
            "selector": { "matchLabels": { "app": "net-test" } },
            "template": {
                "metadata": { "labels": { "app": "net-test" } },
                "spec": {
                    "nodeName": "node1", 
                    "containers": [{ "name": "nginx", "image": "nginx:alpine" }]
                }
            }
        }
    }'
    # Direct nodeName to skip scheduler requirement for this test

    Test-Endpoint -Name "Create Deployment" -Method Post -Url "http://localhost:8080/apis/apps/v1/deployments" -Body $depJson

    # 3. Wait for Pods
    Write-Host "Waiting for Pods to be Running and have IPs..."
    for ($i = 0; $i -lt 30; $i++) {
        $pods = (Test-Endpoint -Name "List Pods" -Method Get -Url "http://localhost:8080/api/v1/pods").items
        $running = $pods | Where-Object { $_.status.phase -eq "Running" -and $_.status.podIP -ne "" }
        $count = $running.Count
        Write-Host "Running Pods with IPs: $count / 3" -ForegroundColor Yellow
        if ($count -ge 3) {
            break
        }
        Start-Sleep -Seconds 2
    }

    if ($count -lt 3) {
        Write-Error "Timeout waiting for pods"
    }

    # 4. Check IPs Unique
    $ips = $running | ForEach-Object { $_.status.podIP }
    $unique = $ips | Select-Object -Unique
    Write-Host "Pod IPs: $($ips -join ', ')" -ForegroundColor Cyan
    
    if ($unique.Count -eq 3) {
        Write-Host "SUCCESS: 3 Unique Pod IPs detected!" -ForegroundColor Green
    }
    else {
        Write-Error "Duplicate IPs or missing IPs detected"
    }

}
finally {
    Stop-Process -Id $pApi.Id -ErrorAction SilentlyContinue
    Stop-Process -Id $pCm.Id -ErrorAction SilentlyContinue
    Stop-Process -Id $pKube.Id -ErrorAction SilentlyContinue
    Remove-Item "test-net.db" -ErrorAction SilentlyContinue
}
