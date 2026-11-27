# Fixtures

`check-result-*.json` and every `expected.json` are `CheckResult` documents, validated against `../check-result.schema.json` by `make lint-content`.

## `cargo-*` folders

Real `cargo test` output of a two-test Rust crate (`add(a, b)` with `adds_two_numbers` and `adds_negative_numbers`),
recorded with cargo 1.93.0 and rustc 1.93.0 (stable, aarch64-apple-darwin). Home and project paths are replaced by `~` and `/work/demo`.

| Folder | Case |
| ------ | ---- |
| `cargo-pass` | both tests pass |
| `cargo-fail` | `assert_eq!(add(-1, -1), -3)` fails: left (expected) `-2`, right (got) `-3` |
| `cargo-build-error` | `a + "b"` does not compile |
| `cargo-stable-no-json` | the libtest JSON flags on a stable toolchain: rejected, no stdout |

Per folder, `libtest.*` is the output of `cargo test --quiet --no-fail-fast -- -Z unstable-options --format json --report-time`
and `text.*` the output of `cargo test --no-fail-fast`. Stable rejects `-Z`, so the `libtest.*` files were recorded with `RUSTC_BOOTSTRAP=1`,
which makes a stable toolchain behave like nightly for this flag. `*.exit` is the exit code and `expected.json` the normalized result
(without `durationMs` for the text output, which has no per-test times).
