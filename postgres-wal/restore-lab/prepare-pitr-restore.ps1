param(
  [Parameter(Mandatory = $true)]
  [string]$BackupName,

  [string]$WalSource = "",

  [string]$RecoveryTargetTime = "",

  [switch]$KeepWorkDir
)

$ErrorActionPreference = "Stop"

function Resolve-SafeChildPath {
  param(
    [Parameter(Mandatory = $true)][string]$BasePath,
    [Parameter(Mandatory = $true)][string]$ChildPath
  )

  $base = [System.IO.Path]::GetFullPath($BasePath)
  $child = [System.IO.Path]::GetFullPath((Join-Path $base $ChildPath))
  if (-not $child.StartsWith($base, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Unsafe path outside base path: $child"
  }
  return $child
}

function Reset-Directory {
  param([Parameter(Mandatory = $true)][string]$Path)

  if (Test-Path -LiteralPath $Path) {
    Remove-Item -LiteralPath $Path -Recurse -Force
  }
  New-Item -ItemType Directory -Path $Path | Out-Null
}

function Test-WalArchiveName {
  param([Parameter(Mandatory = $true)][string]$Name)

  if ($Name.StartsWith(".")) {
    return $false
  }
  if ($Name.EndsWith(".backup")) {
    return $true
  }
  return $Name -match "^[0-9A-F]{24}$"
}

function Get-WalComparableName {
  param([Parameter(Mandatory = $true)][string]$Name)

  if ($Name.EndsWith(".backup")) {
    return ($Name -split "\.")[0]
  }
  return $Name
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$stackDir = [System.IO.Path]::GetFullPath((Join-Path $scriptDir ".."))
$backupDir = Join-Path $stackDir "backups"
$backupFileName = Split-Path -Leaf $BackupName
$backupPath = Resolve-SafeChildPath -BasePath $backupDir -ChildPath $backupFileName

if (-not (Test-Path -LiteralPath $backupPath)) {
  throw "Backup ZIP not found: $backupPath"
}

if ([string]::IsNullOrWhiteSpace($WalSource)) {
  $WalSource = Join-Path $stackDir "wal-archive"
}
$walSourcePath = [System.IO.Path]::GetFullPath($WalSource)
if (-not (Test-Path -LiteralPath $walSourcePath)) {
  throw "WAL source folder not found: $walSourcePath"
}

$dataDir = Join-Path $scriptDir "data"
$walDir = Join-Path $scriptDir "wal"
$workDir = Join-Path $scriptDir "work"

Reset-Directory -Path $dataDir
Reset-Directory -Path $walDir
Reset-Directory -Path $workDir

Write-Host "Extracting backup ZIP: $backupFileName"
Expand-Archive -Path $backupPath -DestinationPath $workDir

$baseTar = Join-Path $workDir "base.tar.gz"
$pgWalTar = Join-Path $workDir "pg_wal.tar.gz"
$manifestPath = Join-Path $workDir "backup-manifest.json"

if (-not (Test-Path -LiteralPath $baseTar)) {
  throw "base.tar.gz not found inside $backupFileName"
}
if (-not (Test-Path -LiteralPath $pgWalTar)) {
  throw "pg_wal.tar.gz not found inside $backupFileName"
}

$startWal = ""
if (Test-Path -LiteralPath $manifestPath) {
  $manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
  if ($manifest.start_wal) {
    $startWal = [string]$manifest.start_wal
  }
}

Write-Host "Extracting base.tar.gz to restore-lab/data"
tar -xzf $baseTar -C $dataDir

$pgWalDir = Join-Path $dataDir "pg_wal"
if (-not (Test-Path -LiteralPath $pgWalDir)) {
  New-Item -ItemType Directory -Path $pgWalDir | Out-Null
}

Write-Host "Extracting pg_wal.tar.gz into restore-lab/data/pg_wal"
tar -xzf $pgWalTar -C $pgWalDir

Write-Host "Copying WAL archive files from $walSourcePath"
$walCopied = 0
Get-ChildItem -LiteralPath $walSourcePath -File | ForEach-Object {
  if (-not (Test-WalArchiveName -Name $_.Name)) {
    return
  }

  $comparable = Get-WalComparableName -Name $_.Name
  if ($startWal -ne "" -and $comparable.Length -eq 24 -and $comparable -lt $startWal) {
    return
  }

  Copy-Item -LiteralPath $_.FullName -Destination (Join-Path $walDir $_.Name) -Force
  $script:walCopied++
}

Remove-Item -LiteralPath (Join-Path $dataDir "postmaster.pid") -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath (Join-Path $dataDir "standby.signal") -Force -ErrorAction SilentlyContinue
New-Item -ItemType File -Path (Join-Path $dataDir "recovery.signal") -Force | Out-Null

$postgresqlConf = @"
listen_addresses = '*'
port = 5432
max_connections = 100
shared_buffers = '256MB'
dynamic_shared_memory_type = 'posix'
wal_level = 'replica'
hot_standby = off
archive_mode = off
restore_command = 'cp /restore-wal/%f %p'
recovery_target_action = 'promote'
"@

if (-not [string]::IsNullOrWhiteSpace($RecoveryTargetTime)) {
  $safeTargetTime = $RecoveryTargetTime.Replace("'", "''")
  $postgresqlConf += "`nrecovery_target_time = '$safeTargetTime'`n"
}

$pgHbaConf = @"
local all all trust
host all all 0.0.0.0/0 md5
host all all ::/0 md5
"@

Set-Content -LiteralPath (Join-Path $dataDir "postgresql.conf") -Value $postgresqlConf -Encoding ASCII
Set-Content -LiteralPath (Join-Path $dataDir "pg_hba.conf") -Value $pgHbaConf -Encoding ASCII

if (-not $KeepWorkDir) {
  Remove-Item -LiteralPath $workDir -Recurse -Force
}

Write-Host ""
Write-Host "PITR restore lab prepared."
Write-Host "Backup: $backupFileName"
Write-Host "Start WAL: $startWal"
Write-Host "WAL copied: $walCopied"
Write-Host "Data dir: $dataDir"
Write-Host "WAL dir: $walDir"
Write-Host ""
Write-Host "Next:"
Write-Host "docker compose -f docker-compose.restore.yml --env-file ..\postgres-wal.env down -v"
Write-Host "docker compose -f docker-compose.restore.yml --env-file ..\postgres-wal.env up -d"
