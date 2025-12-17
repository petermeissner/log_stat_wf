$VERSION = "$(git describe --tags  --dirty)" 
$GIT_COMMIT = "$(git rev-parse --short HEAD)"
$DATE = $(Get-Date -Format "yyyy-MM-dd_HH:mm:ss")




go build -ldflags "-X main.Version=$VERSION -X main.BuildTime='$DATE' -X main.GitCommit='$GIT_COMMIT'" -o "log_stat-dev.exe" ..\src\ 

Write-Host "Build completed: log_stat-dev.exe"
.\log_stat-dev.exe -version