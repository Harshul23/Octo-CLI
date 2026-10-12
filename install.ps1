# Octo CLI - Universal Install Script for Windows PowerShell
# Usage:
#   irm https://raw.githubusercontent.com/Octo-sh/Octo-CLI/main/install.ps1 | iex
#
# Options (via environment variables):
#   $env:OCTO_VERSION     Specific version tag to install (e.g. v1.2.2). Defaults to latest.
#   $env:OCTO_INSTALL_DIR Custom installation directory (default: %LOCALAPPDATA%\Programs\octo).
#   $env:OCTO_REPO        GitHub repository (default: Octo-sh/Octo-CLI).

$ErrorActionPreference = 'Stop'

function Write-Info {
    param([string]$Message)
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Write-Success {
    param([string]$Message)
    Write-Host "==> $Message" -ForegroundColor Green
}

function Write-Warn {
    param([string]$Message)
    Write-Host "Warning: $Message" -ForegroundColor Yellow
}

function Write-Failure {
    param([string]$Message)
    Write-Host "Error: $Message" -ForegroundColor Red
}

try {
    Write-Info "Installing Octo CLI for Windows..."

    # 1. Detect Architecture
    $rawArch = $env:PROCESSOR_ARCHITECTURE
    if ($env:PROCESSOR_ARCHITEW6432) {
        $rawArch = $env:PROCESSOR_ARCHITEW6432
    }

    $arch = "amd64"
    if ($rawArch -eq "ARM64") {
        $arch = "arm64"
    } elseif ($rawArch -ne "AMD64" -and $rawArch -ne "x86_64") {
        Write-Failure "Unsupported CPU architecture '$rawArch'. Octo CLI requires 64-bit Windows (x64 or ARM64)."
        exit 1
    }

    $repo = if ($env:OCTO_REPO) { $env:OCTO_REPO } else { "Octo-sh/Octo-CLI" }

    # 2. Resolve Version Tag
    Write-Info "Resolving latest release for $repo..."
    $tag = $env:OCTO_VERSION
    if (-not $tag -or $tag -eq "latest") {
        try {
            [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12 -bor [Net.SecurityProtocolType]::Tls13
            $headers = @{ "User-Agent" = "Octo-CLI-Installer" }
            $releaseInfo = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -Headers $headers -ErrorAction Stop
            $tag = $releaseInfo.tag_name
        } catch {
            # Fallback: resolve HTTP redirect
            try {
                $req = [System.Net.HttpWebRequest]::Create("https://github.com/$repo/releases/latest")
                $req.AllowAutoRedirect = $false
                $resp = $req.GetResponse()
                $loc = $resp.GetResponseHeader("Location")
                $resp.Close()
                if ($loc) {
                    $tag = $loc.Substring($loc.LastIndexOf('/') + 1)
                }
            } catch {
                # Fallback handled below
            }
        }
    }

    if (-not $tag) {
        Write-Failure "Unable to determine release version tag for $repo."
        exit 1
    }

    if (-not $tag.StartsWith("v")) {
        $tag = "v$tag"
    }

    # 3. Determine Archive URLs
    $archiveName = "octo_${tag}_windows_${arch}.zip"
    $downloadUrl = "https://github.com/$repo/releases/download/$tag/$archiveName"
    $checksumUrl = "https://github.com/$repo/releases/download/$tag/checksums.txt"

    # Fallback to amd64 if native arm64 release asset does not exist (Windows 11 runs amd64 via x64 emulation)
    if ($arch -eq "arm64") {
        try {
            $checkReq = [System.Net.HttpWebRequest]::Create($downloadUrl)
            $checkReq.Method = "HEAD"
            $checkResp = $checkReq.GetResponse()
            $checkResp.Close()
        } catch {
            Write-Warn "Native Windows ARM64 build not found; falling back to AMD64 (emulated)."
            $arch = "amd64"
            $archiveName = "octo_${tag}_windows_${arch}.zip"
            $downloadUrl = "https://github.com/$repo/releases/download/$tag/$archiveName"
        }
    }

    # 4. Create Temporary Directory
    $tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("octo_install_" + [System.Guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
    $zipPath = Join-Path $tempDir $archiveName
    $checksumPath = Join-Path $tempDir "checksums.txt"

    try {
        # 5. Download Release Archive
        Write-Info "Downloading Octo CLI $tag for windows/$arch..."
        Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath -UseBasicParsing

        # 6. Verify Checksum
        Write-Info "Verifying release checksum..."
        $verified = $false
        try {
            Invoke-WebRequest -Uri $checksumUrl -OutFile $checksumPath -UseBasicParsing -ErrorAction SilentlyContinue
            if (Test-Path $checksumPath) {
                $checksumLines = Get-Content $checksumPath
                $matchingLine = $checksumLines | Where-Object { $_ -match [regex]::Escape($archiveName) }
                if ($matchingLine) {
                    $expectedHash = ($matchingLine -split '\s+')[0].Trim().ToLower()
                    $actualHash = (Get-FileHash -Path $zipPath -Algorithm SHA256).Hash.ToLower()

                    if ($expectedHash -ne $actualHash) {
                        Write-Failure "SHA-256 checksum mismatch for $archiveName!"
                        Write-Failure "  Expected: $expectedHash"
                        Write-Failure "  Actual:   $actualHash"
                        exit 1
                    }
                    Write-Info "Checksum verified successfully ($actualHash)."
                    $verified = $true
                }
            }
        } catch {
            # Network issue retrieving checksums.txt
        }

        if (-not $verified) {
            Write-Warn "No matching checksum entry found in release; continuing."
        }

        # 7. Extract Archive
        Write-Info "Extracting archive..."
        $extractDir = Join-Path $tempDir "extracted"
        Expand-Archive -Path $zipPath -DestinationPath $extractDir -Force

        $octoExe = Join-Path $extractDir "octo.exe"
        if (-not (Test-Path $octoExe)) {
            Write-Failure "Release archive did not contain 'octo.exe'."
            exit 1
        }

        # 8. Install to Target Directory
        $installDir = if ($env:OCTO_INSTALL_DIR) {
            $env:OCTO_INSTALL_DIR
        } else {
            Join-Path $env:LOCALAPPDATA "Programs\octo"
        }

        if (-not (Test-Path $installDir)) {
            New-Item -ItemType Directory -Path $installDir -Force | Out-Null
        }

        $targetExe = Join-Path $installDir "octo.exe"
        Copy-Item -Path $octoExe -Destination $targetExe -Force

        # 9. Add to User and Process PATH
        $currentPath = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::User)
        $pathParts = if ($currentPath) { $currentPath -split ';' } else { @() }
        if ($pathParts -notcontains $installDir) {
            $newPath = if ([string]::IsNullOrEmpty($currentPath)) { $installDir } else { "$currentPath;$installDir" }
            [Environment]::SetEnvironmentVariable("Path", $newPath, [EnvironmentVariableTarget]::User)
            Write-Info "Added $installDir to User PATH."
        }

        $procPath = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::Process)
        $procPathParts = if ($procPath) { $procPath -split ';' } else { @() }
        if ($procPathParts -notcontains $installDir) {
            $env:Path = "$installDir;$env:Path"
        }

        Write-Success "Successfully installed Octo CLI $tag to $targetExe"
        Write-Host "Run 'octo --help' to get started." -ForegroundColor Green
        Write-Host "Note: Restart your terminal if 'octo' is not recognized in other open windows." -ForegroundColor Gray
    } finally {
        # Cleanup
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
} catch {
    Write-Failure "Installation failed: $_"
    exit 1
}
