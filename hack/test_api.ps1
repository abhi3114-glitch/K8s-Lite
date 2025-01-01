$ErrorActionPreference = "Stop"

function Test-Endpoint {
    param($Name, $Method, $Url, $Body)
    Write-Host "Testing $Name..." -NoNewline
    try {
        if ($Body) {
            $response = Invoke-RestMethod -Method $Method -Uri $Url -Body $Body -ContentType "application/json"
        } else {
            $response = Invoke-RestMethod -Method $Method -Uri $Url
        }
        Write-Host " OK" -ForegroundColor Green
        return $response
    } catch {
        Write-Host " FAILED" -ForegroundColor Red
        Write-Host $_
        exit 1
    }
}

Start-Sleep -Seconds 2

# 1. List Pods (Empty)
$pods = Test-Endpoint -Name "List Pods (Empty)" -Method Get -Url "http://localhost:8080/api/v1/pods"
if ($pods.items.Count -ne 0) { Write-Error "Expected 0 pods"; exit 1 }

# 2. Create Pod
$podJson = '{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": { "name": "test-pod", "labels": {"app": "foo"} },
    "spec": { "containers": [{ "name": "c1", "image": "nginx" }] }
}'
$created = Test-Endpoint -Name "Create Pod" -Method Post -Url "http://localhost:8080/api/v1/pods" -Body $podJson

# 3. Get Pod
$got = Test-Endpoint -Name "Get Pod" -Method Get -Url "http://localhost:8080/api/v1/pods/test-pod"
if ($got.metadata.name -ne "test-pod") { Write-Error "Name mismatch"; exit 1 }

# 4. List Pods (1)
$pods = Test-Endpoint -Name "List Pods (One)" -Method Get -Url "http://localhost:8080/api/v1/pods"
if ($pods.items.Count -ne 1) { Write-Error "Expected 1 pod"; exit 1 }

# 5. Delete Pod
Test-Endpoint -Name "Delete Pod" -Method Delete -Url "http://localhost:8080/api/v1/pods/test-pod"

# 6. Get Pod (404)
Write-Host "Testing Get Deleted Pod..." -NoNewline
try {
    Invoke-RestMethod -Method Get -Uri "http://localhost:8080/api/v1/pods/test-pod" | Out-Null
    Write-Host " FAILED (Should be 404)" -ForegroundColor Red
    exit 1
} catch {
    if ($_.Exception.Response.StatusCode -eq [System.Net.HttpStatusCode]::NotFound) {
        Write-Host " OK (404)" -ForegroundColor Green
    } else {
        Write-Host " FAILED (Wrong status)" -ForegroundColor Red
        Write-Host $_
        exit 1
    }
}
