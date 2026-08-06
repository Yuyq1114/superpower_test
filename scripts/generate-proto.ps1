$ErrorActionPreference = "Stop"
$missing = @()
foreach ($tool in @("protoc", "protoc-gen-go", "protoc-gen-go-grpc")) {
    if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) { $missing += $tool }
}
if ($missing.Count -gt 0) {
    Write-Error ("Missing protobuf tools: {0}. Install protoc from https://github.com/protocolbuffers/protobuf/releases and plugins with: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest; go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest" -f ($missing -join ", "))
    exit 1
}
$protoFiles = Get-ChildItem -Path proto -Filter *.proto -Recurse
foreach ($file in $protoFiles) {
    & protoc --proto_path=. --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative $file.FullName
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
Write-Host "Generated protobuf files. Move generated service code into each service internal/gen directory before implementation."
