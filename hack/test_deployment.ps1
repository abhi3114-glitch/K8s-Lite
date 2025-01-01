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

# 1. Create Deployment v1
$depJson = '{
    "apiVersion": "apps/v1",
    "kind": "Deployment",
    "metadata": { "name": "web-dep" },
    "spec": {
        "replicas": 3,
        "selector": { "matchLabels": { "app": "web" } },
        "template": {
            "metadata": { "labels": { "app": "web" } },
            "spec": {
                "containers": [{ "name": "nginx", "image": "nginx:1.0" }]
            }
        }
    }
}'
Test-Endpoint -Name "Create Deployment v1" -Method Post -Url "http://localhost:8080/apis/apps/v1/deployments" -Body $depJson

Write-Host "Waiting for Deployment Rollout (v1)..."
Start-Sleep -Seconds 10

# Verify RS created
$rss = Test-Endpoint -Name "List ReplicaSets" -Method Get -Url "http://localhost:8080/apis/apps/v1/replicasets"
$v1RS = $rss.items | Where-Object { $_.metadata.name -like "web-dep-*" }
if (!$v1RS) { Write-Error "No RS found"; exit 1 }
Write-Host "Found RS: $($v1RS.metadata.name)"
if ($v1RS.spec.replicas -ne 3) { Write-Error "Expected 3 replicas"; exit 1 }

# 2. Update Deployment to v2 (Image Change)
$depJsonV2 = '{
    "apiVersion": "apps/v1",
    "kind": "Deployment",
    "metadata": { "name": "web-dep" },
    "spec": {
        "replicas": 3,
        "selector": { "matchLabels": { "app": "web" } },
        "template": {
            "metadata": { "labels": { "app": "web" } },
            "spec": {
                "containers": [{ "name": "nginx", "image": "nginx:2.0" }]
            }
        }
    }
}'

Write-Host "Updating Deployment to v2..."
Test-Endpoint -Name "Update Deployment v2" -Method Put -Url "http://localhost:8080/apis/apps/v1/deployments/web-dep" -Body $depJsonV2

Write-Host "Waiting for Rolling Update..."
Start-Sleep -Seconds 10

# 3. Verify New RS and Old RS
$rss = Test-Endpoint -Name "List ReplicaSets Again" -Method Get -Url "http://localhost:8080/apis/apps/v1/replicasets"
$allRS = $rss.items | Where-Object { $_.metadata.name -like "web-dep-*" }
Write-Host "Found $($allRS.Count) ReplicaSets"

if ($allRS.Count -lt 2) { Write-Error "Expected 2 RS (Old and New)"; exit 1 }

# Check for New RS (Replicas=3) and Old RS (Replicas=0)
$newRS = $allRS | Where-Object { $_.spec.template.spec.containers[0].image -eq "nginx:2.0" }
$oldRS = $allRS | Where-Object { $_.spec.template.spec.containers[0].image -eq "nginx:1.0" }

if (!$newRS) { Write-Error "New RS (nginx:2.0) not found"; exit 1 }
if ($newRS.spec.replicas -ne 3) { Write-Error "New RS should have 3 replicas, got $($newRS.spec.replicas)"; exit 1 }

if ($oldRS) {
    if ($oldRS.spec.replicas -ne 0) { Write-Error "Old RS should have 0 replicas, got $($oldRS.spec.replicas)"; exit 1 }
    Write-Host "Old RS scaled down correctly."
}

Write-Host "Rolling Update Verified! (v1 -> v2)" -ForegroundColor Cyan
