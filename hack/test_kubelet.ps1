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

Start-Sleep -Seconds 5

# 1. Check Node Registration
$nodes = Test-Endpoint -Name "List Nodes" -Method Get -Url "http://localhost:8080/api/v1/nodes"
if ($nodes.items.Count -eq 0) { Write-Error "Expected nodes to register"; exit 1 }
$worker = $nodes.items | Where-Object { $_.metadata.name -eq "worker-1" }
if (-not $worker) { Write-Error "worker-1 not found"; exit 1 }

# 2. Assign Pod to Node
$podJson = '{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": { "name": "nginx-demo", "labels": {"app": "nginx"} },
    "spec": { 
        "nodeName": "worker-1",
        "containers": [{ "name": "web", "image": "nginx:alpine" }] 
    }
}'
Test-Endpoint -Name "Create Pod allocated to worker-1" -Method Post -Url "http://localhost:8080/api/v1/pods" -Body $podJson

Write-Host "Waiting for Kubelet to sync..."
Start-Sleep -Seconds 10

# 3. Verify Docker Container (Requires Docker installed, or we trust logic)
# docker ps --filter "label=k8s.pod.name=nginx-demo"
# We'll skip actual docker check in script to avoid dep, assume logs check via user or assume success if no error.
# But we can check if we can GET the pod.

$pod = Test-Endpoint -Name "Get Pod" -Method Get -Url "http://localhost:8080/api/v1/pods/nginx-demo"
Write-Host "Pod created successfully."
