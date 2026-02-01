# Knowlix Go Repo Doc Manager

Submit a GitHub Go repository, and this project will clone it, parse all public
APIs, invoke your local Claude Code to generate API documentation for each
public symbol, and manage the resulting docs.

## Features

- Clones a GitHub Go repository you provide
- Parses exported functions, methods, structs, and interfaces
- Iterates each public API with local Claude Code to generate descriptions
- Stores and manages generated API documentation

## Requirements

- Go 1.20+
- Git installed and available on `PATH`
- Local Claude Code available to run

## Usage

Generate docs for a GitHub Go repository (it will be cloned to a temporary folder):

```bash
go run ./cmd/knowlix https://github.com/<org>/<repo>
```

Override the Claude Code command:

```bash
KNOWLIX_CLAUDE_CMD="claude" go run ./cmd/knowlix https://github.com/<org>/<repo>
```

Specify an output directory:

```bash
go run ./cmd/knowlix https://github.com/<org>/<repo> --output docs/generated
```

Force regeneration if docs already exist:

```bash
go run ./cmd/knowlix https://github.com/<org>/<repo> --force
```

Generate docs for a specific tag/commit:

```bash
go run ./cmd/knowlix https://github.com/<org>/<repo> --ref v1.2.3
```

Dry run (only list APIs, no generation). Output columns:
`version<TAB>import_path<TAB>kind<TAB>package<TAB>signature`.

```bash
go run ./cmd/knowlix https://github.com/<org>/<repo> --dry-run
```

Limit the number of APIs processed:

```bash
go run ./cmd/knowlix https://github.com/<org>/<repo> --max-items 50
```

## What It Does

1. Clones the GitHub Go repository you provide.
2. Optionally checks out the requested git ref (`--ref`).
3. Uses `go list -json ./...` to enumerate packages.
4. Runs `go doc -all` and parses public APIs.
5. Calls local Claude Code for each public API to generate descriptions.
6. Stores and manages the generated documentation per version.

## Output Example

Generated docs are written to `docs/generated/<repo_slug>/<version>/...` with an
`index.json` per version for management and lookup. The version key is
`<tag>-<commit>` (e.g. `v1.2.3-<commit>`); if no tag exists, it uses
`untagged-<commit>`.

## Configuration

- `KNOWLIX_CLAUDE_CMD` or `CLAUDE_CODE_CMD`: command used to invoke Claude Code.
  The command receives the prompt via stdin and should print Markdown to stdout.

## Notes

- Only exported identifiers (capitalized names) are parsed.
- The parser relies on Go tool output and a regex-based parser, so results may
  vary for unusual formatting.

## Project Layout

```
cmd/
  knowlix/
    main.go
internal/
  claude/
  models/
  parser/
  repo/
  store/
```

## License

No license file is included. Add one if you plan to distribute this project.
