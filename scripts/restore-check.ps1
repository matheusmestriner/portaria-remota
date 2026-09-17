param([Parameter(Mandatory=$true)][string]$BackupFile,[Parameter(Mandatory=$true)][string]$TestDatabaseUrl)
$ErrorActionPreference='Stop'
$connection=[System.Uri]$TestDatabaseUrl
$databaseName=$connection.AbsolutePath.TrimStart('/')
if ($databaseName -notmatch '^restore_test_[a-z0-9_]+$') { throw 'Use um banco vazio separado com prefixo restore_test_. Nunca restaure sobre produção.' }
$credentials=$connection.UserInfo.Split(':',2)
$env:PGPASSWORD=[System.Uri]::UnescapeDataString($credentials[1])
try {
 $arguments=@('-h',$connection.Host,'-p',$connection.Port,'-U',[System.Uri]::UnescapeDataString($credentials[0]),'-d',$databaseName)
 $tableCount=& psql @arguments -At -c "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'"
 if($LASTEXITCODE -ne 0 -or [int]$tableCount -ne 0){throw 'Banco de teste não está vazio ou não foi possível conectar.'}
 & pg_restore @arguments --exit-on-error --no-owner $BackupFile
 if($LASTEXITCODE -ne 0){throw 'Restauração falhou.'}
 & psql @arguments -c "SELECT 'companies' AS entity,count(*) FROM companies UNION ALL SELECT 'invitations',count(*) FROM invitations UNION ALL SELECT 'events',count(*) FROM events;"
 if($LASTEXITCODE -ne 0){throw 'Validação de restauração falhou.'}
} finally {Remove-Item Env:PGPASSWORD -ErrorAction SilentlyContinue}
