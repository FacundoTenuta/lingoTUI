#!/usr/bin/env sh

set -eu

MODULE_PATH="github.com/FacundoTenuta/lingoTUI/cmd/lingotui"
VERSION="${LINGOTUI_VERSION:-latest}"
PACKAGE="${MODULE_PATH}@${VERSION}"
DEFAULT_WHISPER_MODEL="ggml-base.bin"
DEFAULT_WHISPER_MODEL_URL="https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin"

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

is_interactive() {
  [ -t 0 ] && [ -t 1 ]
}

prompt_yes_no() {
  prompt="$1"
  printf '%s [y/N] ' "$prompt"
  read -r answer || answer=""
  case "$answer" in
    y|Y|yes|YES|Yes)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

maybe_install_whisper_cpp() {
  if command_exists whisper-cli; then
    info "whisper-cli is already installed: $(command -v whisper-cli)"
    return 0
  fi

  warn "whisper-cli was not found. It is optional, but required for local Whisper transcription."
  if ! command_exists brew; then
    warn "Homebrew was not found. To use localwhisper, install whisper.cpp manually and make whisper-cli available on PATH."
    return 0
  fi

  if ! is_interactive; then
    warn "Non-interactive shell detected; skipping optional whisper-cpp install. Run: brew install whisper-cpp"
    return 0
  fi

  if prompt_yes_no "Install whisper-cpp with Homebrew now?"; then
    info "Installing whisper-cpp"
    brew install whisper-cpp
    if command_exists whisper-cli; then
      info "whisper-cli installed: $(command -v whisper-cli)"
    else
      warn "whisper-cpp finished installing, but whisper-cli was not found on PATH. Restart your terminal or check Homebrew's output."
    fi
  else
    warn "Skipping whisper-cpp install. You can install it later with: brew install whisper-cpp"
  fi
}

maybe_install_ffmpeg() {
  if command_exists ffmpeg; then
    info "ffmpeg is already installed: $(command -v ffmpeg)"
    return 0
  fi

  warn "ffmpeg was not found. It is optional for installation, but required for /record mic."
  if ! command_exists brew; then
    warn "Homebrew was not found. To use /record mic, install ffmpeg manually and make it available on PATH."
    return 0
  fi

  if ! is_interactive; then
    warn "Non-interactive shell detected; skipping optional ffmpeg install. Run: brew install ffmpeg"
    return 0
  fi

  if prompt_yes_no "Install ffmpeg with Homebrew now?"; then
    info "Installing ffmpeg"
    brew install ffmpeg
    if command_exists ffmpeg; then
      info "ffmpeg installed: $(command -v ffmpeg)"
    else
      warn "ffmpeg finished installing, but ffmpeg was not found on PATH. Restart your terminal or check Homebrew's output."
    fi
  else
    warn "Skipping ffmpeg install. You can install it later with: brew install ffmpeg"
  fi
}

lingotui_config_dir() {
  if [ "$(uname -s 2>/dev/null || printf unknown)" = "Darwin" ]; then
    printf '%s\n' "$HOME/Library/Application Support/lingotui"
    return 0
  fi

  if [ -n "${XDG_CONFIG_HOME:-}" ]; then
    printf '%s\n' "$XDG_CONFIG_HOME/lingotui"
    return 0
  fi

  printf '%s\n' "$HOME/.config/lingotui"
}

download_whisper_model() {
  model_path="$1"
  tmp_path="$model_path.tmp.$$"

  if [ -f "$model_path" ]; then
    info "Whisper model already exists: $model_path"
    return 0
  fi

  if command_exists curl; then
    info "Downloading $DEFAULT_WHISPER_MODEL"
    if curl -fL --retry 3 -o "$tmp_path" "$DEFAULT_WHISPER_MODEL_URL"; then
      mv "$tmp_path" "$model_path"
      return 0
    fi
    rm -f "$tmp_path"
    warn "curl download failed; trying wget if available."
  fi

  if command_exists wget; then
    info "Downloading $DEFAULT_WHISPER_MODEL"
    if wget -O "$tmp_path" "$DEFAULT_WHISPER_MODEL_URL"; then
      mv "$tmp_path" "$model_path"
      return 0
    fi
  fi

  if ! command_exists curl && ! command_exists wget; then
    warn "Neither curl nor wget was found; cannot download the Whisper model automatically."
    warn "Download it manually: $DEFAULT_WHISPER_MODEL_URL"
    warn "Expected path: $model_path"
    return 1
  fi

  rm -f "$tmp_path"
  warn "Failed to download $DEFAULT_WHISPER_MODEL; leaving config unchanged."
  warn "Download it manually: $DEFAULT_WHISPER_MODEL_URL"
  return 1
}

json_string() {
  if printf '%s' "$1" | LC_ALL=C grep '[[:cntrl:]]' >/dev/null 2>&1; then
    fail "Cannot write JSON string containing control characters: $1"
  fi

  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

write_mixed_runtime_config() {
  config_path="$1"
  model_path="$2"
  model_path_json="$(json_string "$model_path")"

  if [ -f "$config_path" ]; then
    backup_base="$config_path.bak.$(date +%Y%m%d%H%M%S).$$"
    backup_path="$backup_base"
    backup_index=0
    while [ -e "$backup_path" ]; do
      backup_index=$((backup_index + 1))
      backup_path="$backup_base.$backup_index"
    done
    if cp "$config_path" "$backup_path"; then
      info "Backed up existing config to $backup_path"
    else
      warn "Could not back up existing config; leaving config unchanged."
      return 1
    fi
  fi

  umask 077
  tmp_config="$config_path.tmp.$$"
  if {
    printf '%s\n' '{'
    printf '%s\n' '  "provider": "openai",'
    printf '%s\n' '  "transcription_model": {'
    printf '%s\n' '    "provider": "localwhisper",'
    printf '%s\n' "    \"name\": \"$DEFAULT_WHISPER_MODEL\","
    printf '%s\n' '    "purpose": "transcription"'
    printf '%s\n' '  },'
    printf '%s\n' '  "chat_model": {'
    printf '%s\n' '    "provider": "chatgpt",'
    printf '%s\n' '    "name": "codex-mini",'
    printf '%s\n' '    "purpose": "chat"'
    printf '%s\n' '  },'
    printf '%s\n' '  "credential_storage": "file",'
    printf '%s\n' '  "local_whisper": {'
    printf '%s\n' '    "binary_path": "whisper-cli",'
    printf '%s\n' "    \"model_path\": \"$model_path_json\","
    printf '%s\n' '    "language": "auto"'
    printf '%s\n' '  }'
    printf '%s\n' '}'
  } >"$tmp_config"; then
    mv "$tmp_config" "$config_path"
    info "Wrote localwhisper + ChatGPT config: $config_path"
    return 0
  fi

  rm -f "$tmp_config"
  warn "Could not write config; leaving existing config unchanged."
  return 1
}

maybe_configure_localwhisper_chatgpt() {
  if ! command_exists whisper-cli; then
    warn "Skipping localwhisper + ChatGPT setup because whisper-cli is not available on PATH."
    warn "Install whisper-cpp first, then configure lingoTUI manually or rerun this installer."
    return 0
  fi

  if ! is_interactive; then
    warn "Non-interactive shell detected; skipping optional localwhisper + ChatGPT setup."
    warn "Manual setup: download $DEFAULT_WHISPER_MODEL_URL, write config.json, then run: lingotui login chatgpt"
    return 0
  fi

  if ! prompt_yes_no "Configure localwhisper transcription + ChatGPT/Codex chat now?"; then
    info "Skipping optional localwhisper + ChatGPT setup"
    return 0
  fi

  config_dir="$(lingotui_config_dir)"
  model_dir="$config_dir/models"
  config_path="$config_dir/config.json"
  model_path="$model_dir/$DEFAULT_WHISPER_MODEL"

  mkdir -p "$model_dir"
  if ! download_whisper_model "$model_path"; then
    return 0
  fi

  if write_mixed_runtime_config "$config_path" "$model_path"; then
    info "Run next: lingotui login chatgpt"
  fi
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

print_lingotui_launch_guidance() {
  binary="$1"
  go_bin="$2"
  profile="$3"

  if command_exists lingotui; then
    info "Run: lingotui"
    return 0
  fi

  warn "lingotui is installed, but this shell session cannot find it on PATH yet."
  warn "Run it now with: $binary"

  if [ -n "$profile" ]; then
    if [ "$(basename "${SHELL:-}")" = "fish" ]; then
      warn "Or restart your terminal / run: source \"$profile\""
      warn "For this session, run: fish_add_path $(shell_quote "$go_bin")"
    else
      warn "Or restart your terminal / run: . \"$profile\""
      warn "For this session, run: export PATH=$(shell_quote "$go_bin"):\$PATH"
    fi
  fi
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

profile=""
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
maybe_install_ffmpeg
maybe_install_whisper_cpp
maybe_configure_localwhisper_chatgpt
print_lingotui_launch_guidance "$binary" "$go_bin" "$profile"
