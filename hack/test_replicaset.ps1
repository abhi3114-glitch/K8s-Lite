$ErrorActionPreference = "Stop"

function Test-Endpoint {
    param($Name, $Method, $Url, $Body)
    Write-Host "Testing $Name..." -NoNewline
    try {
        if ($Body) {
            $response = Invoke-RestMethod -Method $Method -Uri $Url -Body $Body -ContentType "application/json"
        }
        else {
            $response = Invoke-RestMethod -Method $Method -Uri $Url
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

Start-Sleep -Seconds 2

# 1. Create ReplicaSet (Replicas=3)
$rsJson = '{
    "apiVersion": "apps/v1",
    "kind": "ReplicaSet",
    "metadata": { "name": "web-rs" },
    "spec": {
        "replicas": 3,
        "selector": { "matchLabels": { "app": "web" } },
        "template": {
            "metadata": { "labels": { "app": "web" } },
            "spec": {
                "containers": [{ "name": "nginx", "image": "nginx:alpine" }]
            }
        }
    }
}'
Test-Endpoint -Name "Create ReplicaSet (3)" -Method Post -Url "http://localhost:8080/apis/apps/v1/replicasets" -Body $rsJson

Write-Host "Waiting for Controller..."
Start-Sleep -Seconds 10

# 2. Verify Pod Creation
$pods = Test-Endpoint -Name "List Pods" -Method Get -Url "http://localhost:8080/api/v1/pods"
$webPods = $pods.items | Where-Object { $_.metadata.labels.app -eq "web" }
Write-Host "Found $($webPods.Count) web pods."
if ($webPods.Count -ne 3) {
    Write-Error "Expected 3 pods, got $($webPods.Count)"
    exit 1
}

# 3. Simulate Failure (Delete one pod)
$victim = $webPods[0].metadata.name
Test-Endpoint -Name "Delete Victim Pod" -Method Delete -Url "http://localhost:8080/api/v1/pods/$victim"

Write-Host "Waiting for Reconciliation (Self-Healing)..."
Start-Sleep -Seconds 10

$pods = Test-Endpoint -Name "List Pods again" -Method Get -Url "http://localhost:8080/api/v1/pods"
$webPods = $pods.items | Where-Object { $_.metadata.labels.app -eq "web" }
Write-Host "Found $($webPods.Count) web pods."
if ($webPods.Count -ne 3) {
    Write-Error "Expected 3 pods after healing, got $($webPods.Count)"
    exit 1
}

Write-Host "ReplicaSet Self-Healing verified!" -ForegroundColor Cyan
