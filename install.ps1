$ErrorActionPreference = 'Stop'
if ($env:PROCESSOR_ARCHITECTURE -ne 'AMD64') {
    throw 'Only 64-bit Windows (amd64) is supported.'
}

$installDir = Join-Path $env:LOCALAPPDATA 'Programs\cmg'
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
$binary = Join-Path $installDir 'cmg.exe'
$download = Join-Path $installDir 'cmg.download'
try {
    Invoke-WebRequest 'https://github.com/riskibarqy/diffscribe/releases/latest/download/diffscribe-windows-amd64.exe' -OutFile $download
    Move-Item -Force $download $binary
} finally {
    Remove-Item $download -ErrorAction SilentlyContinue
}

$userPath = [string][Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $installDir) {
    [Environment]::SetEnvironmentVariable('Path', ($userPath.TrimEnd(';') + ';' + $installDir).TrimStart(';'), 'User')
}
Write-Output "Installed $binary. Open a new terminal, then run cmg --help."
