# Kerio Control log exporter (Windows / PowerShell).
# Fetches all log categories via Logs.get and saves each to a file. The last-
# 2-weeks date filter + Loki import happen afterward on the collector host.
#
# Run in PowerShell:
#   $env:KERIO_URL='https://10.10.0.2:4081'
#   $env:KERIO_USERNAME='admin'
#   $env:KERIO_PASSWORD='yourpass'
#   powershell -ExecutionPolicy Bypass -File .\kerio-export.ps1
#
# Optional: $env:KERIO_COUNTLINES='500000'  (how many recent lines per log)

$ErrorActionPreference = 'Stop'

$KerioUrl = $env:KERIO_URL;      if (-not $KerioUrl) { $KerioUrl = Read-Host 'KERIO_URL (e.g. https://10.10.0.2:4081)' }
$User     = $env:KERIO_USERNAME; if (-not $User)     { $User     = Read-Host 'Username' }
$Pass     = $env:KERIO_PASSWORD; if (-not $Pass)     { $Pass     = Read-Host 'Password' }
$Count    = $env:KERIO_COUNTLINES; if (-not $Count)  { $Count    = 500000 }

$endpoint = ($KerioUrl.TrimEnd('/')) + '/admin/api/jsonrpc/'
$outDir   = Join-Path ([Environment]::GetFolderPath('Desktop')) 'kerio-logs'
New-Item -ItemType Directory -Force -Path $outDir | Out-Null

# Kerio log categories (logName values). Adjust if any errors.
$logs = @('alert','config','connection','debug','dial','error','filter','http','security','sslvpn','warning','web')

Write-Host "Endpoint: $endpoint"
Write-Host "Output:   $outDir"
Write-Host "Lines/log: $Count"
Write-Host '-------------------------------------------------------------'

# --- accept the self-signed Kerio certificate ---
$common = @{ Method = 'Post'; ContentType = 'application/json' }
if ($PSVersionTable.PSVersion.Major -ge 6) {
    $common['SkipCertificateCheck'] = $true
} else {
    Add-Type @"
using System.Net; using System.Security.Cryptography.X509Certificates;
public class TrustAllCerts : ICertificatePolicy {
  public bool CheckValidationResult(ServicePoint s, X509Certificate c, WebRequest r, int p) { return true; }
}
"@
    [System.Net.ServicePointManager]::CertificatePolicy = New-Object TrustAllCerts
    [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.SecurityProtocolType]::Tls12
}

# --- login ---
$loginBody = @{
    jsonrpc = '2.0'; id = 1; method = 'Session.login'
    params  = @{ userName = $User; password = $Pass; application = @{ name = 'log-export'; vendor = 'murad'; version = '1.0' } }
} | ConvertTo-Json -Depth 6
$loginResp = Invoke-WebRequest @common -Uri $endpoint -Body $loginBody -SessionVariable sess
$token = ($loginResp.Content | ConvertFrom-Json).result.token
if (-not $token) { Write-Host 'LOGIN FAILED:'; Write-Host $loginResp.Content; exit 1 }
Write-Host 'Login OK.'
Write-Host '-------------------------------------------------------------'

function Get-Log([string]$name) {
    $body = @{ jsonrpc='2.0'; id=2; method='Logs.get'; params=@{ logName=$name; fromLine=-1; countLines=[int]$Count } } | ConvertTo-Json -Depth 6
    return Invoke-WebRequest @common -Uri $endpoint -Body $body -WebSession $sess -Headers @{ 'X-Token' = $token }
}

foreach ($name in $logs) {
    Write-Host -NoNewline "Fetching $name ... "
    try {
        $r = Get-Log $name
        $file = Join-Path $outDir "$name.json"
        [IO.File]::WriteAllText($file, $r.Content)
        $kb = [math]::Round(($r.Content.Length/1KB),1)
        Write-Host "OK ($kb KB) -> $file"
    } catch {
        Write-Host "SKIPPED ($_)"
    }
}

Write-Host '-------------------------------------------------------------'
Write-Host "Done. Files are in: $outDir"
Write-Host 'Send me ONE small file (e.g. alert.json) so I can confirm the line'
Write-Host 'format, then finalize the 2-week filter + Loki import.'
