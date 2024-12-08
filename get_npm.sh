#!/usr/bin/env bash

# Exit immediately if a command exits with a non-zero status,
# treat unset variables as errors, and propagate errors in pipelines.
set -euo pipefail

# Trap to always restore original npm registry even if an error occurs.
trap 'restore_registry' EXIT

# Print usage message and exit.
usage() {
  echo "Usage: $0 dependencies_file"
  exit 1
}

# Check if a given command is available.
check_command() {
  local cmd="$1"
  if ! command -v "$cmd" &>/dev/null; then
    echo "Error: '$cmd' is required but not installed or not in PATH." >&2
    exit 1
  fi
}

# Check the input file.
validate_input_file() {
  local file="$1"
  if [[ ! -f "$file" ]]; then
    echo "Error: The file '$file' does not exist." >&2
    exit 1
  fi
}

# Initialize the output file that stores the latest downloaded versions.
initialize_output_file() {
  local file="$1"
  if [[ ! -f "$file" ]]; then
    touch "$file"
  fi
}

# Encode a string for use in a URL.
url_encode() {
  local raw="$1"
  echo -n "$raw" | jq -sRr @uri
}

# Fetch package metadata from the npm registry.
fetch_package_metadata() {
  local package_name="$1"
  curl -s "https://registry.npmjs.org/$(url_encode "$package_name")"
}

# Extract valid versions (in x.y.z format) from package metadata.
extract_valid_versions() {
  local metadata="$1"
  echo "$metadata" | jq -r '.versions | keys[]' | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' | sort -V
}

# Get the last downloaded version of a given package.
get_last_downloaded_version() {
  local package_name="$1"
  local output_file="$2"
  grep "^$package_name@" "$output_file" | awk -F'@' '{print $NF}' || true
}

# Install a specific version of a given package.
install_package_version() {
  local package_name="$1"
  local version="$2"
  
  echo "Installing $package_name@$version..."
  npm install "$package_name@$version" --silent
  echo "Installation successful: $package_name@$version"
}

# Update the output file with the latest downloaded version.
update_downloaded_version() {
  local package_name="$1"
  local version="$2"
  local output_file="$3"
  
  # Remove any existing entry for this package
  sed -i "\|^$package_name@|d" "$output_file"
  # Add the new version
  echo "$package_name@$version" >> "$output_file"
}

# Process a single package by installing all newer versions.
process_package_versions() {
  local package_name="$1"
  local output_file="$2"

  echo "======================================="
  echo "Processing package: $package_name"
  echo "======================================="

  local metadata
  metadata=$(fetch_package_metadata "$package_name")
  if [[ -z "$metadata" ]]; then
    echo "Error: Unable to retrieve metadata for $package_name" >&2
    return 1
  fi

  local versions
  versions=$(extract_valid_versions "$metadata")
  if [[ -z "$versions" ]]; then
    echo "No valid versions found for $package_name."
    return 1
  fi

  local last_downloaded_version
  last_downloaded_version=$(get_last_downloaded_version "$package_name" "$output_file")

  local versions_to_download
  if [[ -z "$last_downloaded_version" ]]; then
    # If no version has been downloaded before, download all versions.
    versions_to_download="$versions"
  else
    # Download only versions greater than the last downloaded version.
    versions_to_download=$(echo "$versions" | awk -v last="$last_downloaded_version" '{
      if ($0 ~ last) { seen=1; next }
      if (seen) print $0
    }')
  fi

  if [[ -z "$versions_to_download" ]]; then
    echo "No new versions to download for $package_name."
    return 0
  fi

  echo "Versions to download for $package_name:"
  echo "$versions_to_download"

  # Clean npm cache to ensure fresh installs
  npm cache clean --force

  local latest_version=""
  for version in $versions_to_download; do
    # Remove node_modules to ensure a clean environment
    rm -rf node_modules
    if install_package_version "$package_name" "$version"; then
      latest_version="$version"
    else
      echo "Error installing $package_name@$version" >&2
    fi
  done

  if [[ -n "$latest_version" ]]; then
    update_downloaded_version "$package_name" "$latest_version" "$output_file"
  fi
}

# Restore the old npm registry.
restore_registry() {
  if [[ -n "${OLD_REGISTRY:-}" ]]; then
    npm set registry "$OLD_REGISTRY"
  fi
}

main() {
  if [[ $# -lt 1 ]]; then
    usage
  fi

  local dependencies_file=$1
  local output_file="last_downloaded_versions.txt"

  # Check required commands
  check_command "jq"
  check_command "curl"
  check_command "npm"

  validate_input_file "$dependencies_file"
  initialize_output_file "$output_file"

  # Store the old registry and set to local one
  OLD_REGISTRY=$(npm get registry)
  npm set registry http://localhost:4873

  # Read dependencies line by line, ignoring empty or commented lines.
  while IFS= read -r dependency; do
    if [[ -z "$dependency" || "$dependency" == \#* ]]; then
      continue
    fi
    process_package_versions "$dependency" "$output_file"
  done < "$dependencies_file"

  echo "Processing completed for all dependencies."
  echo "The last downloaded versions have been updated in $output_file"
}

main "$@"
