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

# 1. Start Cluster
Write-Host "Starting Cluster..."
Remove-Item "test-svc.db" -ErrorAction SilentlyContinue
$pApi = Start-Process -FilePath ".\bin\apiserver.exe" -ArgumentList "-data-file=test-svc.db" -PassThru -NoNewWindow
Start-Sleep -Seconds 2
$pCm = Start-Process -FilePath ".\bin\controller-manager.exe" -PassThru -NoNewWindow
$pKube = Start-Process -FilePath ".\bin\kubelet.exe" -ArgumentList "--node-name=node1" -PassThru -NoNewWindow
Start-Sleep -Seconds 3

try {
    # 2. Create Deployment (to get running pods)
    $depJson = '{
        "apiVersion": "apps/v1",
        "kind": "Deployment",
        "metadata": { "name": "nginx-dep-svc" },
        "spec": {
            "replicas": 2,
            "selector": { "matchLabels": { "app": "nginx-svc" } },
            "template": {
                "metadata": { "labels": { "app": "nginx-svc" } },
                "spec": {
                    "nodeName": "node1", 
                    "containers": [{ "name": "nginx", "image": "nginx:alpine" }]
                }
            }
        }
    }'
    Test-Endpoint -Name "Create Deployment" -Method Post -Url "http://localhost:8080/apis/apps/v1/deployments" -Body $depJson

    # Wait for Pods to be Running
    Write-Host "Waiting for Pods..."
    for ($i = 0; $i -lt 30; $i++) {
        $pods = (Test-Endpoint -Name "List Pods" -Method Get -Url "http://localhost:8080/api/v1/pods").items
        $running = $pods | Where-Object { $_.metadata.labels.app -eq "nginx-svc" -and $_.status.phase -eq "Running" -and $_.status.podIP -ne "" }
        if ($running.Count -ge 2) { break }
        Start-Sleep -Seconds 2
    }

    # 3. Create Service
    $svcJson = '{
        "apiVersion": "v1",
        "kind": "Service",
        "metadata": { "name": "nginx-svc" },
        "spec": {
            "selector": { "app": "nginx-svc" },
            "ports": [{ "port": 80, "targetPort": 80 }]
        }
    }'
    # Note: Sending explicit targetPort struct to be safe/correct with our API
    
    Test-Endpoint -Name "Create Service" -Method Post -Url "http://localhost:8080/api/v1/services" -Body $svcJson

    # 4. Wait for Endpoints
    Write-Host "Waiting for Endpoints..."
    for ($i = 0; $i -lt 10; $i++) {
        try {
            $ep = Invoke-RestMethod -Method Get -Uri "http://localhost:8080/api/v1/endpoints/nginx-svc" -ErrorAction Stop
            if ($ep.subsets.Count -gt 0 -and $ep.subsets[0].addresses.Count -ge 2) {
                Write-Host "Endpoints Found: $($ep.subsets[0].addresses.ip -join ', ')" -ForegroundColor Green
                break
            }
        }
        catch {
            Write-Host "." -NoNewline
        }
        Start-Sleep -Seconds 2
    }
    
    if ($ep.subsets[0].addresses.Count -lt 2) {
        Write-Error "Endpoints not populated correctly"
    }

}
finally {
    Stop-Process -Id $pApi.Id -ErrorAction SilentlyContinue
    Stop-Process -Id $pCm.Id -ErrorAction SilentlyContinue
    Stop-Process -Id $pKube.Id -ErrorAction SilentlyContinue
    Remove-Item "test-svc.db" -ErrorAction SilentlyContinue
}
