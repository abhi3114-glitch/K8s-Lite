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
Remove-Item "test-e2e.db" -ErrorAction SilentlyContinue
$pApi = Start-Process -FilePath ".\bin\apiserver.exe" -ArgumentList "-data-file=test-e2e.db" -PassThru -NoNewWindow
Start-Sleep -Seconds 2
$pCm = Start-Process -FilePath ".\bin\controller-manager.exe" -PassThru -NoNewWindow
$pKube = Start-Process -FilePath ".\bin\kubelet.exe" -ArgumentList "--node-name=node1" -PassThru -NoNewWindow
$pProxy = Start-Process -FilePath ".\bin\proxy.exe" -PassThru -NoNewWindow

Start-Sleep -Seconds 3

try {
    # 2. Create Deployment
    $depJson = '{
        "apiVersion": "apps/v1",
        "kind": "Deployment",
        "metadata": { "name": "nginx-demo" },
        "spec": {
            "replicas": 1,
            "selector": { "matchLabels": { "app": "demo" } },
            "template": {
                "metadata": { "labels": { "app": "demo" } },
                "spec": {
                    "nodeName": "node1", 
                    "containers": [{ "name": "nginx", "image": "nginx:alpine" }]
                }
            }
        }
    }'
    Test-Endpoint -Name "Create Deployment" -Method Post -Url "http://localhost:8080/apis/apps/v1/deployments" -Body $depJson

    # 3. Create Service
    $svcJson = '{
        "apiVersion": "v1",
        "kind": "Service",
        "metadata": { "name": "nginx-svc" },
        "spec": {
            "selector": { "app": "demo" },
            "ports": [{ "port": 80, "targetPort": 80, "nodePort": 30085 }]
        }
    }'
    Test-Endpoint -Name "Create Service" -Method Post -Url "http://localhost:8080/api/v1/services" -Body $svcJson

    # 4. Wait for Endpoints and Proxy Port
    Write-Host "Waiting for Pods & Endpoints..."
    for ($i = 0; $i -lt 30; $i++) {
        try {
            $ep = Invoke-RestMethod -Method Get -Uri "http://localhost:8080/api/v1/endpoints/nginx-svc" -ErrorAction Stop
            if ($ep -and $ep.subsets.Count -gt 0) {
                Write-Host "Endpoints ready." -ForegroundColor Green
                break
            }
        }
        catch {
            Write-Host "." -NoNewline
        }
        Start-Sleep -Seconds 2
    }

    # 5. Access NodePort
    Write-Host "Testing Access via NodePort 30085..."
    for ($i = 0; $i -lt 10; $i++) {
        try {
            $resp = Invoke-WebRequest -Uri "http://localhost:30085" -UseBasicParsing -ErrorAction Stop
            if ($resp.Content -match "Welcome to nginx") {
                Write-Host "SUCCESS: Access confirmed via Proxy!" -ForegroundColor Green
                exit 0
            }
        }
        catch {
            Write-Host "Retrying proxy access..."
        }
        Start-Sleep -Seconds 2
    }
    
    Write-Error "Failed to access service via proxy"

}
finally {
    Stop-Process -Id $pApi.Id -ErrorAction SilentlyContinue
    Stop-Process -Id $pCm.Id -ErrorAction SilentlyContinue
    Stop-Process -Id $pKube.Id -ErrorAction SilentlyContinue
    Stop-Process -Id $pProxy.Id -ErrorAction SilentlyContinue
    Remove-Item "test-e2e.db" -ErrorAction SilentlyContinue
}
