// Mintlify-go-extractor extracts the public API of a Go module into a JSON
// artifact that Mintlify turns into SDK reference pages.
//
// It reads only the module's own source files with the Go standard library
// (go/parser, go/doc, go/doc/comment). It never downloads dependencies,
// type-checks, or runs code, so it works for private and vendored modules
// without credentials.
//
// Run it from anywhere inside the module:
//
//	go run github.com/mintlify/mintlify-go-extractor@latest -o go.json
//
// With Go 1.24 or later, pin it in the module's go.mod instead:
//
//	go get -tool github.com/mintlify/mintlify-go-extractor@latest
//	go tool mintlify-go-extractor -o go.json
//
// Then point a tab or group in docs.json at the artifact:
//
//	"sdk": { "format": "go", "source": "go.json" }
//
// # Usage
//
//	mintlify-go-extractor [flags] [packages]
//
// The flags are:
//
//	-o file
//		Write the artifact to file instead of stdout.
//	-exclude list
//		Comma-separated package patterns to skip.
//	-exclude-symbols list
//		Comma-separated symbol patterns to skip: a name (NewClient), a method
//		or field (Client.Debug), or either after its package directory
//		(option.WithDebugLog), with * and ? wildcards.
//	-comments syntax
//		Doc comment syntax: markdown (default) or godoc.
//	-goos, -goarch
//		Build context used to select files (default linux/amd64).
//	-tags list
//		Comma-separated build tags used to select files.
//	-internal
//		Include internal packages.
//	-no-parent
//		Require go.mod in the target directory instead of searching parent
//		directories.
//	-version
//		Print the version and exit.
//
// Package patterns accept ./..., ./option, ./option/..., or full import paths.
// Relative patterns start at the module root, not the current directory.
// Without patterns, every package in the module is extracted.
//
// # Comment syntax
//
// Generated SDKs (Stainless, Speakeasy, Fern) write Markdown in doc comments,
// so markdown is the default: comment text is kept as Markdown and Go doc
// links such as [io.Reader] are resolved. Use -comments=godoc for SDKs that
// follow Go's doc comment syntax, which is rendered to Markdown the same way
// pkg.go.dev reads it.
//
// See the [SDK reference setup guide] for configuring the generated pages.
//
// [SDK reference setup guide]: https://mintlify.com/docs/api-playground/sdk-reference-setup
package main
