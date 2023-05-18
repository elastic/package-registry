Write-Host "-- Fixing CRLF in git checkout --"
# Forcing to checkout again all the files with a correct autocrlf.
# Doing this here because we cannot set git clone options before.
git config core.autocrlf input
git rm --quiet --cached -r .
git reset --quiet --hard

Write-Host "-- Install golang --"
choco install -y golang --version $env:SETUP_GOLANG_VERSION
$env:ChocolateyInstall = Convert-Path "$((Get-Command choco).Path)\..\.."
Import-Module "$env:ChocolateyInstall\helpers\chocolateyProfile.psm1"
refreshenv
go version
go env

Write-Host "-- Install go-junit-report --"
go install github.com/jstemmer/go-junit-report/v2@latest

Write-Host "-- Run test, prepare junit-xml --"
go mod download -x
go test -v 2>&1 ./... | go-junit-report > "tests-report-win-unicode.xml"
# Junit-Report will be used in linux enviroment.
Get-Content tests-report-win-unicode.xml -Encoding Unicode | Set-Content -Encoding UTF8 tests-report-win.xml
Remove-Item tests-report-win-unicode.xml
ls
