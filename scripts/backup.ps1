param([string]$Destination = "backups")
$ErrorActionPreference = 'Stop'
if (-not $env:MIGRATION_DATABASE_URL) { throw 'Defina MIGRATION_DATABASE_URL na sessão.' }
New-Item -ItemType Directory -Force -Path $Destination | Out-Null
$backupFile = Join-Path $Destination ("portaria-" + (Get-Date -Format 'yyyyMMdd-HHmmss') + '.dump')
# Keep credentials out of process arguments.
$connection = [System.Uri]$env:MIGRATION_DATABASE_URL
$credentials = $connection.UserInfo.Split(':',2)
$env:PGPASSWORD = [System.Uri]::UnescapeDataString($credentials[1])
try {
 & pg_dump -h $connection.Host -p $connection.Port -U ([System.Uri]::UnescapeDataString($credentials[0])) -d $connection.AbsolutePath.TrimStart('/') --format=custom --file=$backupFile
 if ($LASTEXITCODE -ne 0) { throw 'Falha no backup.' }
 Get-FileHash -Algorithm SHA256 -LiteralPath $backupFile | Select-Object Hash,Path
} finally { Remove-Item Env:PGPASSWORD -ErrorAction SilentlyContinue }
