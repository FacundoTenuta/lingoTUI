#!/usr/bin/env sh

set -eu

MODULE_PATH="github.com/FacundoTenuta/lingoTUI/cmd/lingotui"
VERSION="${LINGOTUI_VERSION:-latest}"
PACKAGE="${MODULE_PATH}@${VERSION}"

info() {
  printf '%s\n' "==> $*"
}

warn() {
  printf '%s\n' "WARN: $*" >&2
}

fail() {
  printf '%s\n' "ERROR: $*" >&2
  exit 1
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

shell_quote() {
  if printf '%s' "$1" | LC_ALL=C grep '[[:cntrl:]]' >/dev/null 2>&1; then
    fail "Go bin directory contains control characters, refusing to write it to a shell profile: $1"
  fi

  quoted="$(printf "%s" "$1" | sed "s/'/'\\''/g")"
  printf "'%s'" "$quoted"
}

detect_shell_profile() {
  shell_name="$(basename "${SHELL:-}")"

  case "$shell_name" in
    zsh)
      printf '%s\n' "${ZDOTDIR:-$HOME}/.zshrc"
      ;;
    bash)
      if [ "$(uname -s 2>/dev/null || printf unknown)" = "Darwin" ]; then
        printf '%s\n' "$HOME/.bash_profile"
      else
        printf '%s\n' "$HOME/.bashrc"
      fi
      ;;
    fish)
      printf '%s\n' "$HOME/.config/fish/config.fish"
      ;;
    *)
      printf '%s\n' "${PROFILE:-$HOME/.profile}"
      ;;
  esac
}

append_path_to_profile() {
  profile="$1"
  go_bin="$2"
  shell_name="$(basename "${SHELL:-}")"
  quoted_go_bin="$(shell_quote "$go_bin")"

  mkdir -p "$(dirname "$profile")"
  touch "$profile"

  case "$shell_name" in
    fish)
      line="fish_add_path $quoted_go_bin"
      ;;
    *)
      line="export PATH=$quoted_go_bin:\$PATH"
      ;;
  esac

  if grep -F "$go_bin" "$profile" >/dev/null 2>&1; then
    info "$go_bin is already present in $profile"
    return 0
  fi

  {
    printf '\n'
    printf '# Added by lingoTUI installer\n'
    printf '%s\n' "$line"
  } >>"$profile"

  info "Added Go bin directory to $profile"
}

if ! command_exists go; then
  fail "Go is required. Install it from https://go.dev/dl/ and rerun this script."
fi

info "Installing $PACKAGE"
go install "$PACKAGE"

go_bin="$(go env GOBIN)"
if [ -z "$go_bin" ]; then
  go_path="$(go env GOPATH)"
  go_bin="$go_path/bin"
fi

binary="$go_bin/lingotui"
if [ ! -x "$binary" ]; then
  fail "Installation finished, but $binary was not found or is not executable."
fi

case ":$PATH:" in
  *":$go_bin:"*)
    info "lingotui is installed and available on PATH"
    ;;
  *)
    profile="$(detect_shell_profile)"
    append_path_to_profile "$profile" "$go_bin"
    warn "$go_bin was not in PATH for this shell session."
    if [ "$(basename "${SHELL:-}")" = "fish" ]; then
      warn "Restart your terminal or run: source \"$profile\""
    else
      warn "Restart your terminal or run: . \"$profile\""
    fi
    ;;
esac

info "Installed: $binary"
info "Run: lingotui"
