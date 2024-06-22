#!/usr/bin/env bash

script_dir="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
cd "$script_dir" || exit 1
repo_dir="$(git rev-parse --show-toplevel)"
next_version="$("${script_dir}/get_next_version.sh")"

sed -i "s|ghcr.io/erik142/vault-op-autounseal:.*|ghcr.io/erik142/vault-op-autounseal:$next_version|" "${repo_dir}/examples/deployment.yaml"
