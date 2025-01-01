$ErrorActionPreference = "Stop"

function Start-Component {
    param($Name, $Bin, $Arguments, $Log)
    Write-Host "Starting $Name..." -NoNewline
    $p = Start-Process -FilePath $Bin -ArgumentList $Arguments -PassThru -RedirectStandardOutput "$Log.log" -RedirectStandardError "$Log.err.log" -WindowStyle Minimized
    if ($p.HasExited) {
        Write-Host " FAILED" -ForegroundColor Red
        Get-Content "$Log.err.log" -ErrorAction SilentlyContinue
        exit 1
    }
    Write-Host " OK ($($p.Id))" -ForegroundColor Green
    return $p
}

# Cleanup
Stop-Process -Name "apiserver", "controller-manager", "kubelet", "proxy" -ErrorAction SilentlyContinue

# 1. API Server
$pApi = Start-Component -Name "API Server" -Bin ".\bin\apiserver.exe" -Arguments @("-tls-cert=server.pem", "-tls-key=server.key", "-tls-ca=ca.pem") -Log "apiserver"
Start-Sleep -Seconds 5

# 2. Controller Manager
$pCm = Start-Component -Name "Controller Manager" -Bin ".\bin\controller-manager.exe" -Arguments @("-leader-elect=true", "-api-url=https://localhost:8080", "-tls-cert=client-cm.pem", "-tls-key=client-cm.key", "-tls-ca=ca.pem") -Log "controller-manager"

# 3. Kubelet
$pKube = Start-Component -Name "Kubelet" -Bin ".\bin\kubelet.exe" -Arguments @("--node-name=node1", "-api-url=https://localhost:8080", "-tls-cert=client-kubelet.pem", "-tls-key=client-kubelet.key", "-tls-ca=ca.pem") -Log "kubelet"

# 4. Proxy
$pProxy = Start-Component -Name "Proxy" -Bin ".\bin\proxy.exe" -Arguments @("-api-url=https://localhost:8080", "-tls-cert=client-kubelet.pem", "-tls-key=client-kubelet.key", "-tls-ca=ca.pem") -Log "proxy"

Write-Host "`nCluster started securely!" -ForegroundColor Cyan
Write-Host "Logs available in *.log and *.err.log"
Write-Host "Press Ctrl+C to stop..."

try {
    while ($true) { Start-Sleep -Seconds 1 }
}
finally {
    Write-Host "`nStopping cluster..."
    Stop-Process -Id $pApi.Id -ErrorAction SilentlyContinue
    Stop-Process -Id $pCm.Id -ErrorAction SilentlyContinue
    Stop-Process -Id $pKube.Id -ErrorAction SilentlyContinue
    Stop-Process -Id $pProxy.Id -ErrorAction SilentlyContinue
}
