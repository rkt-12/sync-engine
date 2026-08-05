// This file exists ONLY to mark a Go module boundary. Go's tooling does
// not descend into a directory that declares its own module when
// resolving a parent module's `./...` package pattern -- so this file's
// entire purpose is to stop `go build ./...` / `go test ./...` run from
// the repo root from wandering into web/node_modules, where npm
// packages occasionally ship stray .go files (e.g. reference
// implementations, benchmarks) that have nothing to do with this
// project. There is no actual Go code in this directory.
module sync-engine/web-placeholder-do-not-use

go 1.22