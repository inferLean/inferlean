param(
  [string]$Version = "latest",
  [string]$InstallDir = "$env:LOCALAPPDATA\InferLean\bin",
  [string]$Repo = "inferLean/inferlean"
)

$ErrorActionPreference = "Stop"

function Get-LatestVersion {
  param([string]$Repository)
  $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repository/releases/latest" -Headers @{ "User-Agent" = "inferlean-installer" }
  return $release.tag_name
}

if ($Version -eq "latest") {
  $Version = Get-LatestVersion -Repository $Repo
}

$os = "windows"
$arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
switch ($arch) {
  "x64" { $arch = "amd64" }
  "arm64" { $arch = "arm64" }
  default { throw "unsupported architecture: $arch" }
}

$archive = "inferlean_{0}_{1}_{2}.zip" -f $Version.TrimStart("v"), $os, $arch
$url = "https://github.com/$Repo/releases/download/$Version/$archive"
$tmpDir = Join-Path $env:TEMP ("inferlean-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmpDir | Out-Null
New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null

try {
  $archivePath = Join-Path $tmpDir $archive
  Invoke-WebRequest -Uri $url -OutFile $archivePath
  Expand-Archive -Path $archivePath -DestinationPath $tmpDir -Force

  $binary = Get-ChildItem -Path $tmpDir -Recurse -Filter inferlean.exe | Select-Object -First 1
  if (-not $binary) {
    throw "release archive did not contain inferlean.exe"
  }

  Copy-Item $binary.FullName -Destination (Join-Path $InstallDir "inferlean.exe") -Force
  $toolsDir = Get-ChildItem -Path $tmpDir -Recurse -Directory -Filter tools | Select-Object -First 1
  if ($toolsDir) {
    $destinationToolsDir = Join-Path $InstallDir "tools"
    New-Item -ItemType Directory -Path $destinationToolsDir -Force | Out-Null
    Copy-Item (Join-Path $toolsDir.FullName "*") -Destination $destinationToolsDir -Recurse -Force
  }
  Write-Host "installed inferlean $Version to $(Join-Path $InstallDir 'inferlean.exe')"
}
finally {
  Remove-Item -Path $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
}
