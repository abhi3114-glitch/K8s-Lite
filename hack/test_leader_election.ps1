$ErrorActionPreference = "Stop"

function Start-BackgroundProcess {
    param([string]$FilePath, [string[]]$ArgumentList, [string]$LogFile)
    $pinfo = New-Object System.Diagnostics.ProcessStartInfo
    $pinfo.FileName = $FilePath
    $pinfo.Arguments = $ArgumentList -join " "
    $pinfo.RedirectStandardOutput = $true
    $pinfo.RedirectStandardError = $true
    $pinfo.UseShellExecute = $false
    $pinfo.CreateNoWindow = $true
    
    $p = New-Object System.Diagnostics.Process
    $p.StartInfo = $pinfo
    $p.Start() | Out-Null
    return $p
}

# Cleanup
Stop-Process -Name "apiserver", "controller-manager" -ErrorAction SilentlyContinue
rm *.log -ErrorAction SilentlyContinue

Write-Host "Starting API Server..."
$api = Start-Process -FilePath "./bin/apiserver.exe" -ArgumentList "-tls-cert=server.pem", "-tls-key=server.key", "-tls-ca=ca.pem" -Passthru -RedirectStandardOutput "apiserver.log" -RedirectStandardError "apiserver.err.log"
Start-Sleep -Seconds 2

Write-Host "Starting CM-1 (Leader Candidate)..."
$cm1 = Start-Process -FilePath "./bin/controller-manager.exe" -ArgumentList "-leader-elect=true", "-api-url=https://localhost:8080", "-tls-cert=client-cm.pem", "-tls-key=client-cm.key", "-tls-ca=ca.pem" -Passthru -RedirectStandardOutput "cm1.log" -RedirectStandardError "cm1.err.log"
Start-Sleep -Seconds 5

# Check if CM1 is leader (Check stderr log)
if (Select-String "Successfully acquired lease" "cm1.err.log") {
    Write-Host "PASS: CM-1 acquired leadership." -ForegroundColor Green
}
else {
    Write-Host "FAIL: CM-1 did not acquire leadership." -ForegroundColor Red
    cat cm1.err.log
    # exit 1
}

Write-Host "Starting CM-2 (Standby)..."
$cm2 = Start-Process -FilePath "./bin/controller-manager.exe" -ArgumentList "-leader-elect=true", "-api-url=https://localhost:8080", "-tls-cert=client-cm.pem", "-tls-key=client-cm.key", "-tls-ca=ca.pem" -Passthru -RedirectStandardOutput "cm2.log" -RedirectStandardError "cm2.err.log"
Start-Sleep -Seconds 5

# CM2 should be waiting
if (Select-String "Successfully acquired lease" "cm2.err.log") {
    Write-Host "FAIL: CM-2 acquired leadership improperly (Split Brain?)." -ForegroundColor Red
}
else {
    Write-Host "PASS: CM-2 is standby." -ForegroundColor Green
}

Write-Host "Killing CM-1..."
Stop-Process -Id $cm1.Id
Start-Sleep -Seconds 20 # Wait for lease duration (15s) + retry

# Check if CM2 became leader
if (Select-String "Successfully acquired lease" "cm2.err.log") {
    Write-Host "PASS: CM-2 acquired leadership after failover." -ForegroundColor Green
}
else {
    Write-Host "FAIL: CM-2 did not acquire leadership." -ForegroundColor Red
    Write-Host "CM2 Logs:"
    cat cm2.err.log
}

Stop-Process -Id $api.Id -ErrorAction SilentlyContinue
Stop-Process -Id $cm2.Id -ErrorAction SilentlyContinue
Write-Host "Done."
