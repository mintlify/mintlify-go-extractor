# mintlify-go-extractor

Extracts the public API of a Go module into a JSON artifact that Mintlify turns into SDK reference pages.

It reads only the module's own source files with the Go standard library (`go/parser`, `go/doc`, `go/doc/comment`). It never downloads dependencies, type-checks, or runs code, so it works for private and vendored modules without credentials.

## Usage

Run it from anywhere inside the module:

```bash
go run github.com/mintlify/mintlify-go-extractor@latest -o go.json
```

With Go 1.24 or later, pin it in the module's `go.mod` instead:

```bash
go get -tool github.com/mintlify/mintlify-go-extractor@latest
go tool mintlify-go-extractor -o go.json
```

```text
mintlify-go-extractor [flags] [packages]

packages               Package patterns relative to the module root (default "./...")
-o file                Write the artifact to file instead of stdout
-exclude list          Comma-separated package patterns to skip
-exclude-symbols list  Comma-separated symbol patterns to skip
-comments syntax       Doc comment syntax: markdown (default) or godoc
-goos, -goarch         Build context used to select files (default linux/amd64)
-tags list             Comma-separated build tags used to select files
-internal              Include internal packages
-no-parent             Require go.mod in the target directory instead of searching parent directories
-version               Print the version and exit
```

Package patterns follow `go` command syntax: `./...`, `./option`, `./option/...`, or full import paths.

### Excluding symbols

`-exclude-symbols` leaves individual types, functions, methods, fields, constants, and variables out of the artifact, for exported symbols that should not appear in the reference. Each pattern is matched against a symbol's name (`Client`, `NewClient`), a method or field as `Type.Member` (`Client.Debug`), or either form after the package's directory (`option.WithDebugLog`, `option.Config.Debug`). Patterns use the wildcards of Go's `path.Match` (`*`, `?`, `[...]`): `Legacy*` matches every name that starts with `Legacy` in every package. A constant or variable declared in a group is excluded only when every name in its group matches.

```bash
mintlify-go-extractor -o go.json -exclude-symbols 'Client.Debug,NewLegacyClient,Legacy*'
```

The module root is found the way the `go` command finds it, by walking up from the target directory to the nearest `go.mod`. Pass `-no-parent` when the extractor runs over checkouts that sit side by side, so a `go.mod` above the checkout can never pull its siblings into the artifact.

### Comment syntax

Generated SDKs (Stainless, Speakeasy, Fern) write Markdown in doc comments, so `markdown` is the default: comment text is kept as Markdown and Go doc links such as `[Client]`, `[Client.Do]`, and `[io.Reader]` are resolved. Use `-comments=godoc` for SDKs that follow Go's [doc comment syntax](https://go.dev/doc/comment), which is rendered to Markdown the same way pkg.go.dev reads it.

### Build context

Files are selected for one build context: the given GOOS and GOARCH, the given `-tags`, cgo off, and a fixed set of `go1.N` release tags, so the same module yields the same artifact on any machine and with any Go that built the extractor. Declarations that exist only on another platform (`*_windows.go`, `//go:build darwin`) are not in the artifact, where pkg.go.dev shows the union of several platforms. Run once per platform with `-goos`/`-goarch` to document platform-specific APIs separately; combining them is not supported yet.

### What downstream clients must escape

The artifact is CommonMark plus Go source text, with no MDX-specific escaping in either comment mode: braces, angle brackets, lines starting with `import` or `export`, newlines inside struct tags and multi-line signatures all reach the artifact as written. `godoc` mode escapes `<` as `\<` because that is how `go/doc/comment` prints Markdown; treat that as incidental. Downstream clients own MDX safety, and the `hostile` golden fixture holds the cases they must handle.

### Private dependencies

References to modules matching `GOPRIVATE` are marked as private so they are not linked to pkg.go.dev.

## What is extracted

- Every package in the module except `internal/` packages, commands (`package main`), `testdata/`, `vendor/`, and nested modules
- Exported constants, variables, functions, and types, with constructors and methods grouped under their type
- Struct fields, interface methods, type parameters, parameters, and results
- Signatures as tokens, where identifiers that refer to a declaration carry a reference to it
- Doc comments as Markdown, `Deprecated:` notices, and `Example` functions from test files
- A `README.md` (or `README.markdown`, `README`) from the package's own directory, as pkg.go.dev shows it; sub-packages without one get none. Only plain files are read (READMEs up to 1 MiB, `.go` files up to 64 MiB), through one open handle so the checks and the read see the same file; symlinks and hard links with more than one name are skipped, so a repository cannot pull text from outside itself into the artifact. The same check applies to `go.mod` (capped at 1 MiB), except that an unusable `go.mod` is an error rather than skipped, so the search never walks past it to a `go.mod` above the checkout

## Development

```bash
go test ./...
go test ./internal/extract -update
```

The second command regenerates the golden artifacts in `internal/extract/testdata/golden` from the modules in `internal/extract/testdata/modules`.

## License

[MIT](LICENSE).
