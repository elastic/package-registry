# Forcing to checkout again all the files with a correct autocrlf.
# Doing this here because we cannot set git clone options before.
function fixCRLF {
    Write-Host "-- Fixing CRLF in git checkout --"
    git config core.autocrlf input
    git rm --quiet --cached -r .
    git reset --quiet --hard
}

function withGolang($version) {
    Write-Host "-- Install golang --"
    choco install -y golang --version $version
    $env:ChocolateyInstall = Convert-Path "$((Get-Command choco).Path)\..\.."
    Import-Module "$env:ChocolateyInstall\helpers\chocolateyProfile.psm1"
    refreshenv
    go version
    go env
}

function withGoJUnitReport {
    Write-Host "-- Install go-junit-report --"
    go install github.com/jstemmer/go-junit-report/v2@latest
}

function withMage($version) {
    Write-Host "-- Install Mage --"
    go mod download -x
    go install github.com/magefile/mage@v$version
}

# Run test, prepare junit-xml by gotestsum
function goTestJUnit($output_file, $options) {
    Write-Host "-- Run test, prepare junit-xml --"
    go mod download -x
    go install gotest.tools/gotestsum@latest
    gotestsum --format testname --junitfile $output_file -- $options
}

fixCRLF
withGolang $env:SETUP_GOLANG_VERSION
withMage $env:SETUP_MAGE_VERSION
withGoJUnitReport

mage -debug test

# | go-junit-report > "tests-report-win-unicode.xml"
Get-Content tests-report-win-unicode.xml -Encoding Unicode | Set-Content -Encoding UTF8 tests-report-win.xml
Remove-Item tests-report-win-unicode.xml
