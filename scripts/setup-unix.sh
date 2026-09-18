#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
os_name=$(uname -s)
arch_name=$(uname -m)

case "$os_name:$arch_name" in
  Darwin:x86_64) binary="$script_dir/../bin/salcara-image-darwin-amd64" ;;
  Darwin:arm64) binary="$script_dir/../bin/salcara-image-darwin-arm64" ;;
  Linux:x86_64|Linux:amd64) binary="$script_dir/../bin/salcara-image-linux-amd64" ;;
  Linux:aarch64|Linux:arm64) binary="$script_dir/../bin/salcara-image-linux-arm64" ;;
  *) echo "Unsupported platform: $os_name $arch_name" >&2; exit 1 ;;
esac

chmod u+x "$binary"
printf 'Salcara API URL [https://salcara.top]: '
IFS= read -r api_url
api_url=${api_url:-https://salcara.top}

printf 'Salcara API Key (input hidden): '
stty -echo
IFS= read -r api_key
stty echo
printf '\n'
trap 'api_key=' EXIT HUP INT TERM
printf '%s\n' "$api_key" | "$binary" configure --api-url "$api_url" --key-stdin --default-model gpt-image-2.5-sunburst --default-quality max
api_key=

models_json=$($binary models)
printf '%s\n' "$models_json"
printf '\nChoose a default model:\n'
printf '1. gpt-image-2.5-sunburst — best fidelity and precise edits (default)\n'
printf '2. gpt-image-2.5-flare — fast everyday work\n'
printf '3. gpt-image-2 — legacy compatibility and lower official token rates\n'
printf '4. gpt-image-2.5 — compatibility alias\n'
printf 'Selection [1]: '
IFS= read -r choice
case "${choice:-1}" in
  1) selected=gpt-image-2.5-sunburst ;;
  2) selected=gpt-image-2.5-flare ;;
  3) selected=gpt-image-2 ;;
  4) selected=gpt-image-2.5 ;;
  *) echo "Invalid selection" >&2; exit 1 ;;
esac

case "$models_json" in
  *\"$selected\"*) ;;
  *) echo "The selected model is not exposed by this API." >&2; exit 1 ;;
esac

printf '\nChoose a default quality:\n'
if [ "$selected" = "gpt-image-2" ]; then
  printf '1. high (default)\n2. medium\n3. low\n4. auto\nSelection [1]: '
  IFS= read -r quality_choice
  case "${quality_choice:-1}" in
    1) selected_quality=high ;;
    2) selected_quality=medium ;;
    3) selected_quality=low ;;
    4) selected_quality=auto ;;
    *) echo "Invalid quality selection" >&2; exit 1 ;;
  esac
else
  printf '1. max (default)\n2. xhigh\n3. high\n4. medium\n5. low\n6. auto\nSelection [1]: '
  IFS= read -r quality_choice
  case "${quality_choice:-1}" in
    1) selected_quality=max ;;
    2) selected_quality=xhigh ;;
    3) selected_quality=high ;;
    4) selected_quality=medium ;;
    5) selected_quality=low ;;
    6) selected_quality=auto ;;
    *) echo "Invalid quality selection" >&2; exit 1 ;;
  esac
fi

"$binary" configure --api-url "$api_url" --default-model "$selected" --default-quality "$selected_quality"
printf '\nConfiguration complete. Default model: %s; default quality: %s\n' "$selected" "$selected_quality"
