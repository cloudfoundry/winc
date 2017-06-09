$CommitSha = (git rev-parse HEAD)
if ($LASTEXITCODE -ne 0) {
  $CommitSha = ""
}
$Commit = "$CommitSha-dirty"
$changes = (git status --porcelain --untracked-files=no)
if ($LASTEXITCODE -ne 0) {
  Write-Host "Command 'git status --porcelain --untracked-files=no' failed with exit code: $LASTEXITCODE"
  Exit 1
}
if ($changes -eq "") {
  $Commit = $CommitSha
}

$Version = (Get-Content VERSION)

go build -ldflags "-X main.gitCommit=$Commit -X main.version=$Version" ./cmd/winc
Exit $LASTEXITCODE