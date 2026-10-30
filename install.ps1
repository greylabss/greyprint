param(
    [string]$InstallDir = $(if ($env:GREYPRINT_INSTALL_DIR) { $env:GREYPRINT_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "Programs\Greyprint" })
)

$ErrorActionPreference = "Stop"
$repo = "greylabss/greyprint"
$architecture = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
if ($architecture -eq "x64") { $arch = "amd64" }
elseif ($architecture -eq "arm64") { $arch = "arm64" }
else { throw "Unsupported architecture: $architecture" }

$asset = "greyprint_windows_$arch.zip"
$base = "https://github.com/$repo/releases/latest/download"
$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $tempDir | Out-Null

try {
    $archivePath = Join-Path $tempDir $asset
    $checksumsPath = Join-Path $tempDir "checksums.txt"
    Invoke-WebRequest -Uri "$base/$asset" -OutFile $archivePath
    Invoke-WebRequest -Uri "$base/checksums.txt" -OutFile $checksumsPath

    $checksumLine = Get-Content $checksumsPath | Where-Object { $_ -match "^[0-9a-fA-F]+\s+$([regex]::Escape($asset))$" } | Select-Object -First 1
    if (-not $checksumLine) { throw "No checksum found for $asset" }
    $expected = ($checksumLine -split "\s+")[0].ToLowerInvariant()
    $actual = (Get-FileHash -Path $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) { throw "Checksum verification failed for $asset" }

    Expand-Archive -Path $archivePath -DestinationPath $tempDir
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Copy-Item -Path (Join-Path $tempDir "greyprint.exe") -Destination (Join-Path $InstallDir "greyprint.exe") -Force

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $pathEntries = @($userPath -split ";" | Where-Object { $_ })
    if ($pathEntries -notcontains $InstallDir) {
        [Environment]::SetEnvironmentVariable("Path", (($pathEntries + $InstallDir) -join ";"), "User")
        $env:Path = "$env:Path;$InstallDir"
        Write-Output "Added $InstallDir to your user PATH. Open a new terminal to use it."
    }

    Write-Output "Installed Greyprint to $(Join-Path $InstallDir 'greyprint.exe')"
    & (Join-Path $InstallDir "greyprint.exe") version
}
finally {
    Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
}
