$ErrorActionPreference = "Stop"

function Test-Endpoint {
    param($Name, $Method, $Url, $Cert, $Key, $CA, $ShouldFail = $false)
    Write-Host "Testing $Name..." -NoNewline
    try {
        if ($Cert) {
            # Powershell Invoke-RestMethod with certs is tricky. Using curl.exe if available or custom .NET approach.
            # Easier: use curl.exe provided by Windows/Git.
            $args = @("-s", "-k", "--cert", $Cert, "--key", $Key, "--cacert", $CA, "-X", $Method, $Url)
            $output = & curl.exe $args
            if ($LASTEXITCODE -ne 0) { throw "Curl failed" }
            Write-Host " OK" -ForegroundColor Green
            return $output
        }
        else {
            # Insecure approach
            try {
                Invoke-RestMethod -Method $Method -Uri $Url -ErrorAction Stop
                if ($ShouldFail) { Write-Error "Expected failure but succeeded" }
                Write-Host " OK" -ForegroundColor Green
            }
            catch {
                if ($ShouldFail) { Write-Host " OK (Failed as expected)" -ForegroundColor Green }
                else { throw $_ }
            }
        }
    }
    catch {
        Write-Host " FAILED" -ForegroundColor Red
        Write-Host $_
        exit 1
    }
}

# 1. Start Cluster (Secure)
Write-Host "Starting Secure Cluster..."
$pApi = Start-Process -FilePath ".\bin\apiserver.exe" -ArgumentList "-tls-cert=server.pem", "-tls-key=server.key", "-tls-ca=ca.pem" -PassThru -NoNewWindow
Start-Sleep -Seconds 2

try {
    # 2. Test Insecure Access (Should Fail or be rejected)
    # The Go server with ClientAuth=RequireAndVerifyClientCert will close connection or return error during handshake.
    # Invoke-RestMethod might throw "The underlying connection was closed".
    Write-Host "Testing Insecure Access (Should Fail)..." -NoNewline
    try {
        Invoke-RestMethod -Uri "https://localhost:8080/api/v1/pods" -ErrorAction Stop -SkipCertificateCheck
        Write-Error "Should have failed authentication"
    }
    catch {
        Write-Host " OK (Connection Rejected)" -ForegroundColor Green
    }

    # 3. Test Secure Access with Admin Cert (Should Succeed)
    # Using curl for mTLS simplicity
    Write-Host "Testing Secure Access (Admin)..." -NoNewline
    $out = & curl.exe -s -k --cert client-admin.pem --key client-admin.key --cacert ca.pem https://localhost:8080/api/v1/pods
    if ($LASTEXITCODE -eq 0) {
        Write-Host " OK" -ForegroundColor Green
    }
    else {
        Write-Error "Failed to connect with admin cert"
    }

    # 4. Start Components with Certs
    $logPath = $PWD.Path
    $pCm = Start-Process -FilePath ".\bin\controller-manager.exe" -ArgumentList "-api-url=https://localhost:8080", "-tls-cert=client-cm.pem", "-tls-key=client-cm.key", "-tls-ca=ca.pem" -PassThru -RedirectStandardOutput "$logPath\cm_secure.log" -RedirectStandardError "$logPath\cm_secure.err.log"
    $pKube = Start-Process -FilePath ".\bin\kubelet.exe" -ArgumentList "--node-name=node1", "-api-url=https://localhost:8080", "-tls-cert=client-kubelet.pem", "-tls-key=client-kubelet.key", "-tls-ca=ca.pem" -PassThru -RedirectStandardOutput "$logPath\kubelet_secure.log" -RedirectStandardError "$logPath\kubelet_secure.err.log"
    $pProxy = Start-Process -FilePath ".\bin\proxy.exe" -ArgumentList "-api-url=https://localhost:8080", "-tls-cert=client-kubelet.pem", "-tls-key=client-kubelet.key", "-tls-ca=ca.pem" -PassThru -RedirectStandardOutput "$logPath\proxy_secure.log" -RedirectStandardError "$logPath\proxy_secure.err.log"

    Start-Sleep -Seconds 5
    
    # Check if they crashed (process exited)
    if ($pCm.HasExited) { 
        Write-Error "Controller Manager exited (failed to connect?)" 
        Get-Content "cm_secure.err.log"
    }
    if ($pKube.HasExited) { 
        Write-Error "Kubelet exited (failed to connect?)" 
        Get-Content "kubelet_secure.err.log"
    }
    if ($pProxy.HasExited) { 
        Write-Error "Proxy exited (failed to connect?)" 
        Get-Content "proxy_secure.err.log"
    }
    
    Write-Host "All components running securely!" -ForegroundColor Green

}
finally {
    if ($pApi) { Stop-Process -Id $pApi.Id -ErrorAction SilentlyContinue }
    if ($pCm) { Stop-Process -Id $pCm.Id -ErrorAction SilentlyContinue }
    if ($pKube) { Stop-Process -Id $pKube.Id -ErrorAction SilentlyContinue }
    if ($pProxy) { Stop-Process -Id $pProxy.Id -ErrorAction SilentlyContinue }
}
