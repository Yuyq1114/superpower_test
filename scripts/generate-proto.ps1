$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot ".."))
$protoRoot = Join-Path $repoRoot "proto"
$outputRoot = Join-Path $protoRoot "gen"
$missing = @()
foreach ($tool in @("protoc", "protoc-gen-go", "protoc-gen-go-grpc")) {
    if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) { $missing += $tool }
}
if ($missing.Count -gt 0) {
    Write-Error ("Missing protobuf tools: {0}. Install protoc from https://github.com/protocolbuffers/protobuf/releases and plugins with: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest; go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest" -f ($missing -join ", "))
    exit 1
}
New-Item -ItemType Directory -Force -Path $outputRoot | Out-Null
$protoFiles = Get-ChildItem -Path $protoRoot -Filter *.proto -Recurse
foreach ($file in $protoFiles) {
    $relative = $file.FullName.Substring($protoRoot.Length).TrimStart([char[]]"\/").Replace("\", "/")
    & protoc "--proto_path=$protoRoot" "--go_out=$outputRoot" "--go_opt=paths=source_relative" "--go-grpc_out=$outputRoot" "--go-grpc_opt=paths=source_relative" $relative
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
Write-Host "Generated protobuf files in $outputRoot"
