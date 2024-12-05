# Offline NPM Repository

## Overview

The **Offline NPM Repository** project is designed to fetch and store public NPM dependencies in a local, fully offline repository. This is particularly useful for environments where internet access is restricted or for creating a private NPM mirror for your organization.

Due to the nature of NPM's dependency resolution mechanism, this project ensures that all necessary versions of packages are stored locally to avoid any resolution issues.

---

## Features

- **Fetch Public NPM Dependencies**: Retrieve public NPM packages based on a predefined list (without specifying versions).
- **Store Locally**: All retrieved dependencies and their required versions are stored locally in a custom NPM registry.
- **Fully Offline Repository**: Create a completely offline mirror of selected NPM packages for reuse in isolated environments.
- **Optimized Retrieval**: Only production-ready versions (`x.y.z`) of dependencies are downloaded, avoiding unnecessary overhead from pre-release or experimental versions.

---

## How It Works

1. **Define Dependencies**: Create a list of public NPM dependencies (names only, without versions) in a configuration file.
2. **Fetch Dependencies**:
   - The script fetches all production-ready versions (`x.y.z`) of the specified dependencies.
   - Pre-release or experimental versions (e.g., `x.y.z-alpha` or `x.y.z-beta`) are excluded.
   - It resolves all transitive dependencies by downloading their respective versions.
3. **Local Registry Setup**:
   - Dependencies are stored in a local NPM registry (e.g., using [Verdaccio](https://verdaccio.org/)).
   - The local registry is configured to serve packages offline, ensuring stability and reliability in isolated environments.
4. **Exclude Unnecessary Files**:
   - By default, only selected branches and versions are retained to minimize storage usage.
   - The script ensures the completeness of dependencies to satisfy NPM's resolution mechanism.

---

## Prerequisites

- **Node.js** and **npm** installed on your system.
- **Docker** (to run the Verdaccio local registry).
- **jq** (for JSON processing in Bash scripts).

---

## Setup Instructions

### 1. Clone the Repository
```bash
git clone git@github.com:BenukEl/offline-npm-repository.git
cd offline-npm-repository
```

### 2. Prepare the Dependencies File
Define the dependencies to fetch in the `dependencies.txt` file. Example:
```plaintext
react
lodash
express
```

### 3. Start Verdaccio (Local NPM Registry)
Launch Verdaccio using Docker:

```bash
docker-compose up -d
```

### 4. Fetch and Store Dependencies
Run the script to fetch and store dependencies:

```bash
./get_npm.sh dependencies.txt
```

The script will:
* Set the NPM registry to point to the local Verdaccio registry.
* Fetch all production-ready versions (`x.y.z`) of the specified packages.
* Store them in the Verdaccio registry.
* List newly added files in `verdaccio/storage` and log them in timestamped files under `deps_list`.

---

## Future Enhancements
* Add support for dependency filtering based on custom rules.
* Include a mechanism for syncing updates in public dependencies.
* Integrate with CI/CD pipelines for automated package fetching and validation.

---

## License
This project is licensed under the MIT License. See the LICENSE file for details.

---

With this repository, you can create a robust, offline NPM registry tailored to your project's needs, minimizing unnecessary downloads by only fetching production-ready versions and ensuring reliability in restricted environments. 🚀