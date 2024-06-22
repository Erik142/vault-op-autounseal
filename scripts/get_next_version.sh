#!/usr/bin/env bash

script_dir="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
cd "$script_dir" || exit 1

latest_version_tag="$(git tag -l --sort=-v:refname | head -1)"
latest_version="${latest_version_tag//v/}"

major_version_prefixes=( "release" )
major_version_footers=( "BREAKING CHANGE" )
minor_version_prefixes=( "feat" )
minor_version_footers=( "MINOR VERSION")

# option --output/-o requires 1 argument
# LONGOPTS=debug,force,output:,verbose
# OPTIONS=dfo:v
export verbose=n

LONGOPTS=verbose
OPTIONS=v

export batch=n batch_args="" config_file="" debug=n verbose=n remove=n

function parse_args() {
  #set -o errexit -o pipefail -o noclobber -o nounset
  # -allow a command to fail with !’s side effect on errexit
  # -use return value from ${PIPESTATUS[0]}, because ! hosed $?
  ! getopt --test > /dev/null 
  if [[ ${PIPESTATUS[0]} -ne 4 ]]; then
    echo "I’m sorry, 'getopt --test' failed in this environment."
    exit 1
  fi

  # -regarding ! and PIPESTATUS see above
  # -temporarily store output to be able to check for errors
  # -activate quoting/enhanced mode (e.g. by writing out “--options”)
  # -pass arguments only via   -- "$@"   to separate them correctly
  ! PARSED=$(getopt --options=$OPTIONS --longoptions=$LONGOPTS --name "$0" -- "$@")
  if [[ ${PIPESTATUS[0]} -ne 0 ]]; then
      # e.g. return value is 1
      #  then getopt has complained about wrong arguments to stdout
      exit 2
  fi
  # read getopt’s output this way to handle the quoting right:
  eval set -- "$PARSED"

  # now enjoy the options in order and nicely split until we see --
  while true; do
      case "$1" in
          -v|--verbose)
              verbose=y
              shift
              ;;
          --)
              shift
              break
              ;;
          *)
              echo "Programming error"
              exit 3
              ;;
      esac
  done
}

function get_filtered_git_log() {
  grep=""
  declare -n prefixes="${1}_version_prefixes"
  declare -n footers="${1}_version_footers"

  for prefix in "${prefixes[@]}";
  do
    grep="${grep} --grep=\"^${prefix}\"" 
  done

  for footer in "${footers[@]}";
  do
    grep="${grep} --grep=\"^${footer}\$\"" 
  done

  if [[ "$verbose" == "y" ]]; then
    echo "grep=${grep}" 1>&2
  fi

  eval git log --oneline --no-decorate "${grep}" "${latest_version_tag}"..HEAD
}

parse_args "$@"

major_version="$(echo "$latest_version" | cut -d'.' -f1)"
minor_version="$(echo "$latest_version" | cut -d'.' -f2)"
patch_version="$(echo "$latest_version" | cut -d'.' -f3)"

if [ -n "$(get_filtered_git_log "major")" ]; then
  major_version=$((major_version + 1))
  minor_version=0
  patch_version=0
elif [ -n "$(get_filtered_git_log "minor")" ]; then
  minor_version=$((minor_version + 1))
  patch_version=0
else
  patch_version=$((patch_version + 1))
fi

if [[ "$verbose" == "y" ]]; then
  echo "Before bump: $latest_version" 1>&2
fi

echo "${major_version}.${minor_version}.${patch_version}"
