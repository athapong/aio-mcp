build:
  CGO_ENABLED=0 go build -ldflags="-s -w" -o ./bin/aio-mcp ./main.go

docs:
  go run scripts/docs/update-doc.go

scan:
  trufflehog git file://. --only-verified

install:
  go install ./...
