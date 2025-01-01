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

# 1. Register 2 Nodes (Simulating Kubelets)
$node1 = '{
    "metadata": { "name": "worker-1" },
    "status": { 
        "conditions": [{"type": "Ready", "status": "True"}],
        "capacity": { "cpu": "1000m", "memory": "1Gi" }
    }
}'
Test-Endpoint -Name "Register Worker 1" -Method Post -Url "http://localhost:8080/api/v1/nodes" -Body $node1

$node2 = '{
    "metadata": { "name": "worker-2" },
    "status": { 
        "conditions": [{"type": "Ready", "status": "True"}],
        "capacity": { "cpu": "1000m", "memory": "1Gi" }
    }
}'
Test-Endpoint -Name "Register Worker 2" -Method Post -Url "http://localhost:8080/api/v1/nodes" -Body $node2

# 2. Create Unscheduled Pod
$podJson = '{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": { "name": "scheduler-test", "labels": {"app": "test"} },
    "spec": { 
        "containers": [{ "name": "pause", "image": "k8s.gcr.io/pause", "resources": {"requests": {"cpu": "500m"}} }] 
    }
}'
Test-Endpoint -Name "Create Unscheduled Pod" -Method Post -Url "http://localhost:8080/api/v1/pods" -Body $podJson

Write-Host "Waiting for Scheduler..."
Start-Sleep -Seconds 5

# 3. Check Pod Assignment
$pod = Test-Endpoint -Name "Get Pod" -Method Get -Url "http://localhost:8080/api/v1/pods/scheduler-test"
if ([string]::IsNullOrEmpty($pod.spec.nodeName)) {
    Write-Error "Pod was not scheduled!"
    exit 1
}
Write-Host "Pod scheduled to: $($pod.spec.nodeName)" -ForegroundColor Cyan
