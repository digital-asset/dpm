# Extract the downloaded zip ($ZIP_PATH) to temp directory ($EXTRACTED)
# Avoid using the system's temp directory as the user may not have rights to it
New-Item -ItemType Directory -Path $EXTRACTED | Out-Null
Expand-Archive -Path $ZIP_PATH -DestinationPath $EXTRACTED

# Optionally, override the TMP and DPM_HOME environment variable to point to directories other than the default,
# as the user might not have rights to the default directories.
# (You might also want to persist these variables as DPM uses them on every invocation)
$env:TMP = "<user-owned temporary directory>"
$env:DPM_HOME = "<user-owned directory>"

& "$EXTRACTED\windows-amd64\bin\dpm.exe" bootstrap $EXTRACTED\windows-amd64