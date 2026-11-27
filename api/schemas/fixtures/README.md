# Fixtures

`check-result-*.json` and every `expected.json` are `CheckResult` documents, validated against `../check-result.schema.json` by `make lint-content`.
The runner tests in `plugin/src/test` read the recorded toolchain output below, so they need no local toolchain.

## Supported toolchain versions

The recordings are per toolchain version, one folder each, and every runner test runs against all of them.

| Toolchain | Versions | Why |
| --------- | -------- | --- |
| Go | `1.26` (go1.26.5), `1.25` (go1.25.14) | assumption: the two newest minor releases, which Go itself supports. `backend/go.mod` pins 1.26.5. |
| Rust | `1.93` (1.93.0), `1.85` (1.85.0) | assumption: the stable release the first recordings used (1.93.0) and 1.85.0 as the oldest supported one (first release of the 2024 edition). Neither `content/template-rust` nor CI pins a version. |

The folder name is the minor version. The exact toolchain is in `toolchain.txt` of each folder. These versions are an assumption, not a decision of the project.
To change the supported set, edit `GO_VERSIONS` or `RUST_VERSIONS` in `record.sh`, record, and delete the folders of dropped versions.

## Layout

- `go/<version>/{pass,fail,build-error}.jsonl`: `go test -json ./...` of a small `example.com/parser` package with the tests `TestParseSimpleString` and `TestParseError`.
  `build-error` does not compile.
- `cargo/<version>/<case>/`: `cargo test` output of a two-test Rust crate (`add(a, b)` with `adds_two_numbers` and `adds_negative_numbers`).

| Cargo case | Case |
| ---------- | ---- |
| `pass` | both tests pass |
| `fail` | `assert_eq!(add(-1, -1), -3)` fails: left (expected) `-2`, right (got) `-3` |
| `build-error` | `a + "b"` does not compile |
| `stable-no-json` | the libtest JSON flags on a stable toolchain: rejected, no stdout |

Per cargo case, `libtest.*` is the output of `cargo test --quiet --no-fail-fast -- -Z unstable-options --format json --report-time`
and `text.*` the output of `cargo test --no-fail-fast`. Stable rejects `-Z`, so the `libtest.*` files were recorded with `RUSTC_BOOTSTRAP=1`,
which makes a stable toolchain behave like nightly for this flag. `*.exit` is the exit code and `expected.json` the normalized result
(without `durationMs` for the text output, which has no per-test times, and with the tests in any order, as the test threads finish in any order).

## Re-recording

Run `make record-fixtures` (needs Docker; `./record.sh go` or `./record.sh cargo` records one toolchain).
It runs the demo projects in the official `golang:<version>` and `rust:<version>` images, so the recordings do not depend on the local toolchain, and writes
the folders above. It replaces `/usr/local/rustup` and `/usr/local/cargo` by `~/.rustup` and `~/.cargo`, and the thread id of a panic by `1`;
the project path is always `/work/demo`. It also regenerates `cargo/<version>/build-error/expected.json`, the only expected result that depends on the version.
Check `git diff` afterwards: durations, timestamps and compiler messages change, and the tests must still pass (`make test-plugin`).
When adding a version, copy `expected.json` of the cargo `pass` and `fail` cases from another version.
