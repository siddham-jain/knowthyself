# knowthyself installer for Windows — https://github.com/siddham-jain/knowthyself
#
# Usage:  irm https://<domain>/install.ps1 | iex
#
# Downloads the prebuilt binary for your architecture from GitHub Releases, verifies
# its SHA-256 against the published checksums, installs it to a per-user tools dir,
# adds that dir to PATH, and offers to run it.

$ErrorActionPreference = 'Stop'
$Repo = 'siddham-jain/knowthyself'
$Bin  = 'knowthyself'

function Write-Amber($t) { Write-Host $t -ForegroundColor Yellow }
function Write-Dim($t)   { Write-Host $t -ForegroundColor DarkGray }

function Show-Banner {
	"", `
	"   ┌────────────────────────────────┐", `
	"   │                                │", `
	"   │     K N O W  T H Y S E L F     │", `
	"   │                                │" | ForEach-Object { Write-Amber $_ }
	Write-Dim "   │         ΓΝΩΘΙ  ΣΕΑΥΤΟΝ         │"
	"   │                                │", `
	"   └────────────────────────────────┘" | ForEach-Object { Write-Amber $_ }
	Write-Dim "        know how you build with AI`n"
}

Show-Banner

# --- detect architecture ---------------------------------------------------------
$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
	'AMD64' { 'amd64' }
	'ARM64' { 'arm64' }
	default { throw "unsupported architecture: $($env:PROCESSOR_ARCHITECTURE)" }
}

# --- resolve version -------------------------------------------------------------
$version = $env:KNOWTHYSELF_VERSION
if (-not $version) {
	$rel = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
	$version = $rel.tag_name
}
if (-not $version) { throw "could not find a published release — see https://github.com/$Repo/releases" }
$num = $version.TrimStart('v')

$asset = "${Bin}_${num}_windows_${arch}.zip"
$base  = "https://github.com/$Repo/releases/download/$version"
Write-Host "  downloading $asset ($version)"

$tmp = Join-Path $env:TEMP ("knowthyself-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
	Invoke-WebRequest "$base/$asset" -OutFile "$tmp\$asset"
	Invoke-WebRequest "$base/checksums.txt" -OutFile "$tmp\checksums.txt"

	# --- verify checksum ---------------------------------------------------------
	$want = (Select-String -Path "$tmp\checksums.txt" -Pattern ([regex]::Escape($asset)) |
		Select-Object -First 1).Line.Split(' ')[0]
	$got = (Get-FileHash "$tmp\$asset" -Algorithm SHA256).Hash.ToLower()
	if ($want -ne $got) { throw "checksum verification FAILED — aborting" }
	Write-Amber "  checksum ok"

	# --- install -----------------------------------------------------------------
	$dest = Join-Path $env:LOCALAPPDATA "Programs\knowthyself"
	New-Item -ItemType Directory -Force -Path $dest | Out-Null
	Expand-Archive "$tmp\$asset" -DestinationPath $dest -Force
	Write-Host "  installed to $dest\$Bin.exe"

	# --- add to user PATH --------------------------------------------------------
	$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
	if ($userPath -notlike "*$dest*") {
		[Environment]::SetEnvironmentVariable('Path', "$userPath;$dest", 'User')
		$env:Path = "$env:Path;$dest"
		Write-Dim "  added $dest to your PATH (restart terminals to pick it up)"
	}
}
finally {
	Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

# --- offer to run it now ---------------------------------------------------------
$ans = Read-Host "`n  Meet yourself now? [Y/n]"
if ($ans -match '^(n|no)$') {
	Write-Host "`n  whenever you are ready:  " -NoNewline; Write-Amber "knowthyself"
}
else {
	Write-Host ""
	& "$dest\$Bin.exe"
}
