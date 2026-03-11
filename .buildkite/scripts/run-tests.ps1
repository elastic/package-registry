$ErrorActionPreference = "Stop" # set -e
# Forcing to checkout again all the files with a correct autocrlf.
# Doing this here because we cannot set git clone options before.
function fixCRLF {
    Write-Host "--- Fixing CRLF in git checkout"
    git config core.autocrlf input
    git rm --quiet --cached -r .
    git reset --quiet --hard
}

function withGolang($version) {
    Write-Host "--- Install golang (GVM)"
    $workDir = if ($env:WORKSPACE) { $env:WORKSPACE } else { $PWD.Path }
    $binDir = Join-Path $workDir "bin"
    if (-not (Test-Path $binDir)) { New-Item -ItemType Directory -Path $binDir | Out-Null }
    $gvmExe = Join-Path $binDir "gvm-windows-amd64.exe"
    $gvmUrl = "https://github.com/andrewkroh/gvm/releases/download/$env:SETUP_GVM_VERSION/gvm-windows-amd64.exe"

    $maxTries = 5
    for ($i = 1; $i -le $maxTries; $i++) {
        try {
            Invoke-WebRequest -Uri $gvmUrl -OutFile $gvmExe -UseBasicParsing
            break
        } catch {
            if ($i -eq $maxTries) { throw }
            Start-Sleep -Seconds 3
        }
    }

    & $gvmExe --format=powershell $version | Invoke-Expression
    $env:PATH = "$(go env GOPATH)\bin;$env:PATH"
    go version
    go env
}

function withGoJUnitReport {
    Write-Host "--- Install go-junit-report"
    go install github.com/jstemmer/go-junit-report/v2@latest
}

function withMage($version) {
    Write-Host "--- Install Mage"
    go mod download -x
    go install github.com/magefile/mage@v$version
}

fixCRLF
withGolang $env:SETUP_GOLANG_VERSION
withMage $env:SETUP_MAGE_VERSION
withGoJUnitReport

Write-Host "--- Run Unit tests"
$ErrorActionPreference = "Continue" # set +e
mage -debug test > test-report.txt
$EXITCODE=$LASTEXITCODE
$ErrorActionPreference = "Stop"

# Buildkite collapse logs under --- symbols
# need to change --- to anything else or switch off collapsing (note: not available at the moment of this commit)
$contest = Get-Content test-report.txt
foreach ($line in $contest) {
    $changed = $line -replace '---', '----'
    Write-Host $changed
}

Write-Host "--- Create Junit report for junit annotation plugin"
Get-Content test-report.txt | go-junit-report > "unicode-tests-report-win.xml"
Get-Content unicode-tests-report-win.xml -Encoding Unicode | Set-Content -Encoding UTF8 tests-report-win.xml
Remove-Item unicode-tests-report-win.xml, test-report.txt

Exit $EXITCODE
