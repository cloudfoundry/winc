$CommitSha = (git rev-parse HEAD)
if ($LASTEXITCODE -ne 0) {
  Write-Host "Command 'git rev-parse HEAD' failed with exit code: $LASTEXITCODE"
  Exit 1
}
$Commit = $CommitSha
git diff --exit-code --quiet
if ($LASTEXITCODE -ne 0) {
  $Commit = "$CommitSha-dirty"
}

go build -ldflags "-X main.gitCommit=$Commit" ./cmd/winc
Exit $LASTEXITCODE
