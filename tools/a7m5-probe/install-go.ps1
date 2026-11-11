[CmdletBinding()]
param(
    [switch]$SkipBuild
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$GoVersion = "1.27.4"
$MinimumMajor = 1
$MinimumMinor = 27
$MinimumPatch = 4
$DownloadBase = "https://go.dev/dl"
$ExpectedHashes = @{
    "AMD64" = "178ab98950ac69ab25069307dc6ade79f3816e414cce95b4da166a115cde1e7e"
    "ARM64" = "9e7a72565792b8ab370b37ac9ac109799237abacb39ff2f541c438b963c0cc64"
}

function Get-CompatibleGoVersion {
    $goCommand = Get-Command go -ErrorAction SilentlyContinue
    if (-not $goCommand) {
        return $false
    }
    try {
        $versionText = (& go env GOVERSION 2>$null)
    }
    catch {
        return $false
    }
    if ($versionText -notmatch '^go(\d+)\.(\d+)\.(\d+)') {
        return $false
    }
    $major = [int]$Matches[1]
    $minor = [int]$Matches[2]
    $patch = [int]$Matches[3]
    return ($major -gt $MinimumMajor) -or (
        ($major -eq $MinimumMajor) -and (
            ($minor -gt $MinimumMinor) -or (($minor -eq $MinimumMinor) -and ($patch -ge $MinimumPatch))
        )
    )
}

function Add-UserPathEntry {
    param([Parameter(Mandatory = $true)][string]$Directory)

    $normalized = [System.IO.Path]::GetFullPath($Directory).TrimEnd('\')
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $entries = @($userPath -split ';' | Where-Object { $_ -ne '' })
    $alreadyPresent = $entries | Where-Object {
        [System.IO.Path]::GetFullPath($_.TrimEnd('\')) -ieq $normalized
    }
    if (-not $alreadyPresent) {
        $updatedPath = (@($entries) + $normalized) -join ';'
        [Environment]::SetEnvironmentVariable("Path", $updatedPath, "User")
    }
    if (($env:Path -split ';') -notcontains $normalized) {
        $env:Path = "$normalized;$env:Path"
    }
}

$architecture = $env:PROCESSOR_ARCHITECTURE.ToUpperInvariant()
if (-not $ExpectedHashes.ContainsKey($architecture)) {
    throw "Unsupported Windows architecture '$architecture'. Supported: AMD64, ARM64."
}

$localPrograms = Join-Path $env:LOCALAPPDATA "Programs\Go"
$installRoot = Join-Path $localPrograms $GoVersion
$installBin = Join-Path $installRoot "bin"
if (Test-Path -LiteralPath (Join-Path $installBin "go.exe")) {
    Add-UserPathEntry -Directory $installBin
}

if (Get-CompatibleGoVersion) {
    Write-Host "Compatible Go is already installed: $(& go version)" -ForegroundColor Green
}
else {
    $archiveName = "go$GoVersion.windows-$($architecture.ToLowerInvariant()).zip"
    $downloadUrl = "$DownloadBase/$archiveName"
    $workRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("sonyfastconnect-go-" + [guid]::NewGuid().ToString("N"))
    $archivePath = Join-Path $workRoot $archiveName
    $extractRoot = Join-Path $workRoot "extract"
    New-Item -ItemType Directory -Path $workRoot -Force | Out-Null
    try {
        Write-Host "Downloading official Go $GoVersion for Windows $architecture..."
        Invoke-WebRequest -Uri $downloadUrl -OutFile $archivePath -UseBasicParsing

        $actualHash = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($actualHash -ne $ExpectedHashes[$architecture]) {
            throw "Go archive checksum mismatch. Expected $($ExpectedHashes[$architecture]), got $actualHash."
        }

        Expand-Archive -LiteralPath $archivePath -DestinationPath $extractRoot -Force
        $extractedGo = Join-Path $extractRoot "go"
        if (-not (Test-Path -LiteralPath (Join-Path $extractedGo "bin\go.exe"))) {
            throw "Downloaded archive does not contain go\bin\go.exe."
        }
        if (Test-Path -LiteralPath $installRoot) {
            throw "Install directory already exists but is not a compatible Go installation: $installRoot"
        }

        New-Item -ItemType Directory -Path $localPrograms -Force | Out-Null
        Move-Item -LiteralPath $extractedGo -Destination $installRoot
    }
    finally {
        $tempBase = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath())
        $resolvedWork = [System.IO.Path]::GetFullPath($workRoot)
        if (
            $resolvedWork.StartsWith($tempBase, [System.StringComparison]::OrdinalIgnoreCase) -and
            (Split-Path -Leaf $resolvedWork).StartsWith("sonyfastconnect-go-", [System.StringComparison]::Ordinal)
        ) {
            Remove-Item -LiteralPath $resolvedWork -Recurse -Force -ErrorAction SilentlyContinue
        }
    }

    Add-UserPathEntry -Directory $installBin
}

if (-not (Get-CompatibleGoVersion)) {
    throw "Go installation completed but this process still cannot find a compatible go command. Open a new terminal and rerun the script."
}
Write-Host "Using: $(& go version)" -ForegroundColor Green

if ($SkipBuild) {
    Write-Host "Go installation complete. Probe build skipped." -ForegroundColor Green
    exit 0
}

Push-Location $PSScriptRoot
try {
    Write-Host "Downloading pinned Go module dependencies..."
    & go mod download
    if ($LASTEXITCODE -ne 0) {
        throw "go mod download failed."
    }
    Write-Host "Running probe tests..."
    & go test ./...
    if ($LASTEXITCODE -ne 0) {
        throw "Probe tests failed."
    }
    Write-Host "Building a7m5-probe.exe..."
    $env:GOOS = "windows"
    $env:GOARCH = $architecture.ToLowerInvariant()
    & go build -trimpath -o a7m5-probe.exe .
    if ($LASTEXITCODE -ne 0) {
        throw "Probe build failed."
    }
}
finally {
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    Pop-Location
}

Write-Host "Ready: $(Join-Path $PSScriptRoot 'a7m5-probe.exe')" -ForegroundColor Green
