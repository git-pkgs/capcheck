# capcheck

Fail CI when your Go code or its dependencies gain access to new privileged operations: spawning processes, opening sockets, calling cgo, touching the filesystem. Built on [google/capslock](https://github.com/google/capslock).

govulncheck tells you when a dependency has a CVE. capcheck tells you when a dependency (or your own code) gains the ability to do something it couldn't before: spawn a process, open a socket, write a file, call cgo. You check a baseline into your repo and CI fails when the capability set drifts.

## Install

```
go install github.com/git-pkgs/capcheck/cmd/capcheck@latest
```

Binaries for linux, darwin, and windows are attached to each [release](https://github.com/git-pkgs/capcheck/releases).

## Quick start

From the root of a Go module:

```
capcheck init ./...
```

This analyses your packages, writes a default `capcheck.json`, and writes the current capability set to `capcheck.lock.json`. Commit both.

Thereafter, in CI or locally:

```
capcheck ./...
```

Exit code 0 means no new capabilities. Exit code 1 means something changed and you'll see the offending call path:

```
capcheck: 1 new capability since baseline

  github.com/you/app gained EXEC
    cmd/server/main.go:42:3   main.run
    handler/upload.go:88:5    handler.processImage
                              github.com/disintegration/imaging.Resize
                              os/exec.Command

Run `capcheck update` to accept, or remove the offending call path.
```

If the change is expected, run `capcheck update ./...` and commit the new lock file.

## Commands

```
capcheck init   [packages]   analyse and write capcheck.json + capcheck.lock.json
capcheck check  [packages]   compare against the lock file (default command)
capcheck update [packages]   re-analyse and overwrite the lock file
capcheck list   [packages]   print current capabilities, no baseline involved
```

`packages` defaults to `./...`.

Flags accepted by all commands:

```
-c, --config string       path to capcheck.json (default "capcheck.json")
    --baseline string     path to capcheck.lock.json (overrides config)
-C, --dir string          run as if in this directory
-f, --format string       text, json, or github (default "text")
    --granularity string  package or function (overrides config)
    --timeout duration    analysis timeout (overrides config)
    --strict              fail on removed capabilities as well as added
    --omit-paths          do not record call paths (smaller lock file)
    --ignore strings      capabilities to ignore (repeatable, stacks with config)
```

## Configuration

`capcheck.json` is optional; missing keys take defaults.

```json
{
  "granularity": "package",
  "timeout": "5m",
  "baseline": "capcheck.lock.json",
  "ignore": [
    "FILES",
    "NETWORK",
    "REFLECT",
    "RUNTIME"
  ]
}
```

`ignore` matches hierarchically: an entry of `MODIFY_SYSTEM_STATE` also ignores `MODIFY_SYSTEM_STATE/ENV`. Most projects will want to ignore the noisy capabilities and watch only for `EXEC`, `CGO`, `ARBITRARY_EXECUTION`, `UNSAFE_POINTER`, and `MODIFY_SYSTEM_STATE`.

The full capability list is documented in [capslock's docs](https://github.com/google/capslock/blob/main/docs/capabilities.md).

`granularity` controls what counts as a change. At `package` (the default) you're told when a package gains a capability it didn't have. At `function` you're told when a new entry point reaches an existing capability, which is more precise but produces a much larger and more volatile lock file.

Other keys: `build_tags`, `goos`, `goarch` are passed through to package loading; `capability_map` points to a custom capslock classifier file if you want to extend the stdlib mappings; `omit_paths` drops the example call path from each lock entry, which shrinks `capcheck.lock.json` considerably at the cost of less helpful failure output.

## GitHub Action

```yaml
- uses: git-pkgs/capcheck@v1
  with:
    packages: ./...
```

The action installs capcheck, runs `check --format github`, and turns each new capability into an error annotation anchored at the first line of user code in the call path. On failure it also writes the human-readable diff to the job summary.

Inputs: `go-version` (default `stable`), `capcheck-version` (default `latest`), `working-directory`, `packages`, `strict`, `ignore`.

## Exit codes

```
0  no new capabilities (or only removed ones, when not --strict)
1  capability set changed
2  analysis or configuration error
```

## How it relates to capslock

capcheck does not reimplement any analysis. It calls `github.com/google/capslock/analyzer` directly and the lock file is the same protojson that `capslock -output=json` produces, so you can inspect or regenerate it with the upstream tool. What capcheck adds is the config file, the diff, the exit codes, and the CI ergonomics.

## License

MIT
