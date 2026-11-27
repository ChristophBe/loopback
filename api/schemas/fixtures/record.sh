#!/usr/bin/env bash
# Re-records the go test and cargo test fixtures for every supported toolchain version, each in a Docker container.
# Usage: record.sh [go|cargo]   (no argument: both). See README.md.
set -euo pipefail

# <fixture folder>:<Docker image tag>
GO_VERSIONS="1.26:1.26.5 1.25:1.25.14"
RUST_VERSIONS="1.93:1.93.0 1.85:1.85.0"

here="$(cd "$(dirname "$0")" && pwd)"

go_sources() { # <variant: pass|fail|build-error>
  printf 'module example.com/parser\n\ngo 1.25\n' >go.mod
  case "$1" in
    pass) printf 'package parser\n\nfunc Parse(in string) string { return in }\n' >parse.go ;;
    fail) printf 'package parser\n\nimport "strings"\n\nfunc Parse(in string) string { return strings.TrimLeft(in, "+-") }\n' >parse.go ;;
    build-error) printf 'package parser\n\nfunc Parse(in string) string {return undefinedVar }\n' >parse.go ;;
  esac
  cat >parse_test.go <<'GO'
package parser

import "testing"

func TestParseSimpleString(t *testing.T) {
	if got := Parse("+OK\r\n"); got != "+OK\r\n" {
		t.Errorf("expected %q, got %q", "+OK\r\n", got)
	}
}

func TestParseError(t *testing.T) {
	if got := Parse("-ERR"); got != "-ERR" {
		t.Errorf("expected %q, got %q", "-ERR", got)
	}
}
GO
}

rust_sources() { # <variant: pass|fail|build-error>
  mkdir -p src
  printf '[package]\nname = "demo"\nversion = "0.1.0"\nedition = "2021"\n' >Cargo.toml
  local sum='a + b' expected='-2'
  [ "$1" = build-error ] && sum='a + "b"'
  [ "$1" = fail ] && expected='-3'
  cat >src/lib.rs <<RS
pub fn add(a: i32, b: i32) -> i32 {
    $sum
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn adds_two_numbers() {
        assert_eq!(add(2, 3), 5);
    }

    #[test]
    fn adds_negative_numbers() {
        assert_eq!(add(-1, -1), $expected);
    }
}
RS
}

# Runs inside the container: writes the recordings into <out>.
inner_go() {
  local out=$1
  export GOCACHE=/tmp/gocache HOME=/tmp
  go version >"$out/toolchain.txt"
  for variant in pass fail build-error; do
    rm -rf /work && mkdir -p /work/demo && cd /work/demo
    go_sources "$variant"
    go test -json ./... >"$out/$variant.jsonl" 2>/dev/null || true
  done
}

inner_cargo() {
  local out=$1 libtest=(cargo test --quiet --no-fail-fast -- -Z unstable-options --format json --report-time)
  export CARGO_TARGET_DIR=/tmp/target
  { cargo --version; rustc --version; } >"$out/toolchain.txt"
  record() { # <case dir> <prefix> <command...>
    local dir=$1 prefix=$2 code=0
    shift 2
    "$@" >"$dir/$prefix.stdout.$([ "$prefix" = libtest ] && echo jsonl || echo txt)" 2>"$dir/$prefix.stderr.txt" || code=$?
    echo "$code" >"$dir/$prefix.exit"
  }
  for variant in pass fail build-error stable-no-json; do
    rm -rf /work "$CARGO_TARGET_DIR" && mkdir -p /work/demo "$out/$variant" && cd /work/demo
    if [ "$variant" = stable-no-json ]; then
      rust_sources pass
      record "$out/$variant" libtest "${libtest[@]}" # stable rejects -Z
      continue
    fi
    rust_sources "$variant"
    RUSTC_BOOTSTRAP=1 record "$out/$variant" libtest "${libtest[@]}" # makes stable accept -Z
    rm -rf "$CARGO_TARGET_DIR"
    record "$out/$variant" text cargo test --no-fail-fast
  done
}

# Replaces home and project paths and the volatile thread id, so that re-recording gives small diffs.
normalize() { # <dir>
  find "$1" -type f ! -name '*.exit' ! -name expected.json | while read -r f; do
    sed -e 's#/usr/local/rustup#~/.rustup#g' -e 's#/usr/local/cargo#~/.cargo#g' \
      -e "s#thread '\(.*\)' ([0-9]*)#thread '\1' (1)#" "$f" >"$f.tmp" && mv "$f.tmp" "$f"
  done
}

# Writes build-error/expected.json: the compiler message (from the first `error[` up to `error: could not compile`) is the one
# version-specific part of the expected results. The pass and fail results are the same for every version and kept by hand.
write_build_error_expected() { # <dir>
  local message
  message="$(awk '/^error\[/ { on = 1 } /^error: could not compile/ { exit } on' "$1/build-error/libtest.stderr.txt" |
    sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' | awk '{ printf "%s%s", sep, $0; sep = "\\n" }' | sed -e 's/\(\\n\)*$//')"
  printf '{\n  "passed": false,\n  "tests": [\n    { "name": "cargo build", "status": "fail", "got": "%s" }\n  ]\n}\n' "$message" \
    >"$1/build-error/expected.json"
}

record_all() { # <go|cargo> <versions>
  local kind=$1 pair dir image
  for pair in $2; do
    dir=$here/$kind/${pair%%:*}
    image=golang:${pair#*:}
    [ "$kind" = cargo ] && image=rust:${pair#*:}
    mkdir -p "$dir"
    echo "recording $kind with $image into $dir"
    docker run --rm -v "$here/record.sh:/record.sh:ro" -v "$dir:/out" -e HOST_UID="$(id -u)" -e HOST_GID="$(id -g)" \
      "$image" bash -c "source /record.sh --functions && inner_$kind /out; rc=\$?; chown -R \$HOST_UID:\$HOST_GID /out; exit \$rc"
    normalize "$dir"
    if [ "$kind" = cargo ]; then write_build_error_expected "$dir"; fi
  done
}

if [ "${1:-}" = --functions ]; then return 0; fi # sourced inside the container
case "${1:-all}" in
  go) record_all go "$GO_VERSIONS" ;;
  cargo) record_all cargo "$RUST_VERSIONS" ;;
  all) record_all go "$GO_VERSIONS"; record_all cargo "$RUST_VERSIONS" ;;
  *) echo "usage: record.sh [go|cargo]" >&2; exit 2 ;;
esac
