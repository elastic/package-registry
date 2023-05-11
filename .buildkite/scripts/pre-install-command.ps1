Write-Host "Install golang --"
choco install -y golang --version $SETUP_GOLANG_VERSION
$env:ChocolateyInstall = Convert-Path "$((Get-Command choco).Path)\..\.."
Import-Module "$env:ChocolateyInstall\helpers\chocolateyProfile.psm1"
refreshenv

go version
