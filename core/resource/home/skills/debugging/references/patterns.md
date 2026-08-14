# Language-Specific Debugging Patterns

## Go

- `go vet ./...` — static analysis
- `go test -v -run TestName` — run specific test
- `go test -race ./...` — race condition detection
- Check `go.mod` for dependency version mismatches
- Use `fmt.Printf("%+v", err)` for detailed error output

## TypeScript / JavaScript

- `npx tsc --noEmit` — type checking without emitting
- `console.log(JSON.stringify(obj, null, 2))` — inspect objects
- Check `node_modules` for version conflicts
- Use debugger breakpoints in browser DevTools or VS Code

## Python

- `import pdb; pdb.set_trace()` — interactive debugger
- `print(type(x), repr(x))` — inspect type and value
- `pip list` — verify installed packages and versions
- Check virtual environment activation

## Rust

- `cargo check` — fast compile check without binary
- `cargo test -- --nocapture` — show test output
- `RUST_BACKTRACE=1 cargo run` — full backtrace
- Check `Cargo.toml` for feature flags

## Docker / CI

- `docker logs <container>` — container logs
- Check resource limits (memory, CPU)
- Verify network connectivity between containers
- Check CI configuration for missing secrets/environment variables
