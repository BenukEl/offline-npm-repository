
# Offline NPM Repository

## Overview

**Offline NPM Repository** provides a way to fetch and store all production-ready versions (in `x.y.z` format) of public NPM packages into a local, fully offline repository. This setup is useful in environments with restricted internet access, or when you want to create a private NPM mirror for your organization. By doing so, you ensure that all required versions of dependencies are available locally, avoiding issues during dependency resolution in isolated environments.

**New Features and Changes:**
- **Multiple Input Sources**: The script can now read the list of dependencies either from a simple text file (one dependency per line) or directly from the `dependencies`, `devDependencies`, and `peerDependencies` sections of a `package.json`.
  
- **Incremental Fetching**: The script keeps track of the last downloaded version of each dependency. On subsequent runs, it only fetches versions newer than those already downloaded, saving time and bandwidth.

- **Configurable Source**: Use the `--source` option to specify whether dependencies come from a file or a `package.json` file.

- **Detailed Logging**: The script updates the `last_downloaded_versions.txt` file after each successful fetch, providing a clear record of the latest installed versions.

---

## Features

- **Fetch Public NPM Dependencies**: Download all production-ready versions (`x.y.z`) of the specified packages, ensuring comprehensive local availability.

- **Flexible Input Sources**:
  - **File mode**: Provide a plaintext file listing dependencies line-by-line.
  - **Package mode**: Automatically extract all dependencies from `dependencies`, `devDependencies`, and `peerDependencies` in a `package.json` file.

- **Incremental Updates**: Only fetch new versions that haven't been downloaded yet, enabling efficient updates as packages evolve.

- **Local NPM Registry**: All retrieved packages are stored in a local registry (e.g., via [Verdaccio](https://verdaccio.org/)), ensuring a fully offline, private NPM mirror.

- **Optimized and Stable**: Store only stable, production-ready versions, excluding pre-release or experimental versions, to ensure consistent and reliable dependency resolution.

---

## How It Works

1. **Define Dependencies**:  
   Depending on your preference, you can:
   - Create a text file with one dependency per line (e.g., `dependencies.txt`), or
   - Use an existing `package.json` from which the script will extract all dependencies, devDependencies, and peerDependencies.

2. **Select the Source**:  
   Run the script with `--source file` for line-based input or `--source package` to use a `package.json`. For example:
   ```bash
   ./get_npm.sh --source file dependencies.txt
   ```
   or
   ```bash
   ./get_npm.sh --source package package.json
   ```

3. **Fetch Dependencies**:  
   The script retrieves all stable versions (`x.y.z`) of each dependency. If the script has run before, it will only fetch versions newer than those previously downloaded (as recorded in `last_downloaded_versions.txt`).

4. **Store Locally**:  
   The fetched packages are stored in a local Verdaccio registry, making them immediately available offline without further network access.

5. **Track Progress**:  
   After each execution, the `last_downloaded_versions.txt` file is updated, ensuring that future runs only retrieve missing versions.

---

## Prerequisites

- **Node.js** and **npm** installed on your system.
- **Docker** (for running the Verdaccio local registry).
- **jq** (for JSON processing in Bash scripts).
- **curl** (for fetching registry metadata).

---

## Setup Instructions

### 1. Clone the Repository
```bash
git clone git@github.com:BenukEl/offline-npm-repository.git
cd offline-npm-repository
```

### 2. Prepare Your Dependencies

**Option A: Using a File**  
Create or modify `dependencies.txt`:
```plaintext
react
lodash
express
```

**Option B: Using `package.json`**  
Ensure `package.json` has the desired dependencies listed under `dependencies`, `devDependencies`, or `peerDependencies`.

### 3. Start Verdaccio (Local NPM Registry)
Launch Verdaccio using Docker:
```bash
docker-compose up -d
```

### 4. Fetch and Store Dependencies

**If using a file:**
```bash
./get_npm.sh --source file dependencies.txt
```

**If using `package.json`:**
```bash
./get_npm.sh --source package package.json
```

This will:
- Temporarily set the NPM registry to the local Verdaccio instance.
- Fetch all stable versions of the specified packages not previously downloaded.
- Store them in the Verdaccio registry.
- Update `last_downloaded_versions.txt` to record the latest downloaded versions.

---

## Future Enhancements

- Add support for custom filtering rules to limit downloaded versions further.
- Automated syncing of new versions, triggered by CI/CD pipelines.
- Enhanced error handling and reporting for more seamless integrations.

---

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

---

By leveraging this updated script, you can maintain a comprehensive, offline, and private NPM repository tailored to your needs. It streamlines the dependency management process by only fetching new versions when needed, all while ensuring that your environment remains stable and fully offline.
