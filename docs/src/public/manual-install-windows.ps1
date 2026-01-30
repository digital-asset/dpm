echo "Determining latest SDK tarball link..."

# Determine latest SDK zip URL
$curlResult = Invoke-WebRequest -Uri "https://get.digitalasset.com/install/latest-windows-archive.html" -UseBasicParsing
$ZIP_URL = ($curlResult.Content -match 'https://[^ ]+\.zip') | Out-Null; $matches[0]
$ZIP_PATH = Join-Path $env:RUNNER_TEMP "dpm-windows-amd64.zip"
Invoke-WebRequest -Uri $ZIP_URL -OutFile $ZIP_PATH

# Extract the downloaded zip ($ZIP_PATH) to temp directory ($EXTRACTED)
# Avoid using the system's temp directory as the user may not have rights to it
$EXTRACTED = Join-Path $env:RUNNER_TEMP "extracted"
New-Item -ItemType Directory -Path $EXTRACTED | Out-Null
Expand-Archive -Path $ZIP_PATH -DestinationPath $EXTRACTED

# Path to dpm.exe
$exe = Join-Path $EXTRACTED "windows-amd64\bin\dpm.exe"

# Optionally, override the TMP and DPM_HOME environment variable to point to directories other than the default,
# as the user might not have rights to the default directories.
# (You might also want to persist these variables as DPM uses them on every invocation)
$env:TMP = "<user-owned temporary directory>"
$env:DPM_HOME = "<user-owned directory>"

& $exe bootstrap $EXTRACTED\windows-amd64

# Check that dpm.exe version works and does not error
try {
    & $exe version
} catch {
    Write-Error "dpm.exe version failed"
    exit 1
}
