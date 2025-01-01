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

$dbFile = "test-data.json"
if (Test-Path $dbFile) { Remove-Item $dbFile }

# 1. Start Server
Write-Host "Starting API Server (Round 1)..."
$p1 = Start-Process -FilePath ".\bin\apiserver.exe" -ArgumentList "-data-file=$dbFile" -PassThru -NoNewWindow
Start-Sleep -Seconds 2

# 2. Create Object
$podJson = '{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": { "name": "persist-pod", "labels": {"test": "true"} },
    "spec": { "containers": [{ "name": "c1", "image": "busybox" }] }
}'
try {
    Test-Endpoint -Name "Create Pod" -Method Post -Url "http://localhost:8080/api/v1/pods" -Body $podJson
}
catch {
    Stop-Process -Id $p1.Id
    exit 1
}

# 3. Kill Server
Write-Host "Stopping API Server..."
Stop-Process -Id $p1.Id
Start-Sleep -Seconds 2

if (!(Test-Path $dbFile)) {
    Write-Error "Database file $dbFile was not created!"
    exit 1
}
Write-Host "Database file exists." -ForegroundColor Cyan

# 4. Restart Server
Write-Host "Restarting API Server (Round 2)..."
$p2 = Start-Process -FilePath ".\bin\apiserver.exe" -ArgumentList "-data-file=$dbFile" -PassThru -NoNewWindow
Start-Sleep -Seconds 2

# 5. Verify Object Exists
try {
    $pod = Test-Endpoint -Name "Get Preserved Pod" -Method Get -Url "http://localhost:8080/api/v1/pods/persist-pod"
    if ($pod.metadata.name -ne "persist-pod") {
        Write-Error "Pod name mismatch"
        exit 1
    }
    Write-Host "Persistence Verified!" -ForegroundColor Cyan
}
finally {
    Stop-Process -Id $p2.Id
    Remove-Item $dbFile -ErrorAction SilentlyContinue
}
