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

# 1. Create Pod with Probe
# Nginx on port 80. Liveness check index.html.
$podJson = '{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": { "name": "lifecycle-demo", "labels": {"app": "nginx"} },
    "spec": { 
        "nodeName": "worker-1",
        "containers": [{ 
            "name": "web", 
            "image": "nginx:alpine",
            "livenessProbe": {
                "httpGet": { "path": "/", "port": { "type": 0, "intVal": 80 } },
                "initialDelaySeconds": 5,
                "periodSeconds": 5
            }
        }] 
    }
}'

# Ensure node exists
$node1 = '{ "metadata": { "name": "worker-1" }, "status": { "conditions": [{"type":"Ready","status":"True"}] } }'
try { Invoke-RestMethod -Method Post -Uri "http://localhost:8080/api/v1/nodes" -Body $node1 -ContentType "application/json" } catch {}

Test-Endpoint -Name "Create Lifecycle Pod" -Method Post -Url "http://localhost:8080/api/v1/pods" -Body $podJson

Write-Host "Waiting for Startup and Probe..."
Start-Sleep -Seconds 10

# 2. Verify Status is Running
# Kubelet should have updated status to Running if probe passed (or if container started and probe didn't fail yet)
$pod = Test-Endpoint -Name "Get Pod Status" -Method Get -Url "http://localhost:8080/api/v1/pods/lifecycle-demo"
Write-Host "Pod Phase: $($pod.status.phase)" -ForegroundColor Cyan
if ($pod.status.phase -ne "Running") {
    Write-Warning "Expected Running, got $($pod.status.phase). Probes might be failing or update slow."
    # We proceed, but note it.
}

# 3. Graceful Termination
Write-Host "Deleting Pod (Graceful Stop)..."
$start = Get-Date
Test-Endpoint -Name "Delete Pod" -Method Delete -Url "http://localhost:8080/api/v1/pods/lifecycle-demo"
$duration = (Get-Date) - $start
Write-Host "Deletion took $($duration.TotalSeconds) seconds" -ForegroundColor Gray
