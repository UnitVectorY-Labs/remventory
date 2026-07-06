
# Commands for remventory
default:
  @just --list
# Build remventory with Go
build:
  go build ./...

# Run tests for remventory with Go
test:
  go clean -testcache
  go test ./...