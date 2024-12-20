# npm-pkg

**npm-pkg** is a command-line tool designed to download npm tarballs exhaustively. Its primary goal is to enable offline development of Node.js servers or JavaScript frontends by pre-downloading all required packages and their dependencies.

## Overview
**npm-pkg** fetches packages and their associated tarballs by supporting multiple input sources. It can read package names directly from command-line arguments, a text file, or even parse a `package.json` file to extract dependencies. The tool leverages parallel processing for efficient downloading and metadata fetching, ensuring a robust solution for environments with limited or no internet access.

## Features

* Multiple Input Sources:
  * Direct CLI arguments
  * A file containing a list of packages (one per line)
  * A `package.json` file to extract dependencies (including dev and peer dependencies)
* Parallel Downloads:
  * Configurable parallelism for both metadata retrieval and tarball downloads using the `--metadata-workers` and `--download-workers` flags.
* Offline Development Support:
  * Pre-downloads all necessary packages, making it easier to set up an offline development environment for Node.js servers or JavaScript frontends.
* State Management:
  * Maintains a state file to keep track of already downloaded packages, ensuring efficient incremental updates.

## Prerequisites
* [Go](https://go.dev) (for building and running the project)
* Internet access for the initial download phase (subsequent offline development does not require connectivity)
* The npm registry is used as the source for packages: `https://registry.npmjs.org`

## Installation
1. Clone the repository:

```bash
git clone https://github.com/your-repo/npm-pkg.git
cd npm-pkg
```

2. Build the project:

```bash
go build -o npm-pkg .
```

## Usage
The tool provides several ways to specify the packages you wish to download. Here are some examples:

* **Download packages by specifying them as arguments:**
```bash
./npm-pkg download express left-pad
```

* **Download packages from a file:**
```bash
./npm-pkg download --file=my_packages.txt
```

* **Download packages using a package.json file:**
```bash
./npm-pkg download --package-json=./package.json
```

* **Specify a custom state file:**
```bash
./npm-pkg download --state-file=/path/to/my_state.json
```

* **Control parallelism:**
```bash
./npm-pkg download express left-pad --metadata-workers=10 --download-workers=200
```

## Running Tests
To run tests, simply use:

```bash
go test ./...
```

## License
This project is licensed under the MIT License.