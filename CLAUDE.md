# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go (Golang) project licensed under Mozilla Public License 2.0. The repository is currently in its initial state with no source code committed.

## Development Commands

When source code is added, common commands will include:

- `go mod init <module>` - Initialize Go module (if not already done)
- `go build ./...` - Build all packages
- `go test ./...` - Run all tests
- `go test -v -run <TestName>` - Run a specific test
- `go vet ./...` - Run Go static analysis
- `go fmt` - Format code

## Project Structure

Once code is added, expected structure:
- `cmd/` - Application entry points
- `internal/` - Private application code
- `pkg/` - Reusable libraries
- `api/` - API definitions/schemas
- `configs/` - Configuration files
- `test/` - Additional test utilities/data

## Architecture

This is a blank project - no architecture decisions have been made yet. When implementing features:
- Follow standard Go project layout conventions
- Use idiomatic Go patterns
- Place main packages in `cmd/`, internal packages in `internal/`
