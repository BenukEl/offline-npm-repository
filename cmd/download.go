// cmd/download.go
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/npmoffline/internal/pkg/filesystem"
	"github.com/npmoffline/internal/pkg/httpclient"
	"github.com/npmoffline/internal/pkg/logger"
	"github.com/npmoffline/internal/repositories"
	"github.com/npmoffline/internal/services"
	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"
)

// Flags
var (
	downloadDest      string
	packageListFile   string
	packageJSONFile   string
	downloadStateFile string

	metadataWorkers int
	downloadWorkers int

	updateLocalRepository bool
)

// downloadCmd represents the "download" subcommand
var downloadCmd = &cobra.Command{
	Use:   "download [packages...]",
	Short: "Download one or more npm packages locally",
	Long: `download allows you to fetch npm packages by:
  1) Specifying them directly as arguments, e.g.:
        npm-pkg download express left-pad
  2) Providing a file listing packages (one per line), e.g.:
        npm-pkg download --file=my_packages.txt
  3) Providing a package.json file to parse dependencies from, e.g.:
        npm-pkg download --package-json=./package.json
  4) Specifying a custom state file for downloaded packages, e.g.:
        npm-pkg download --state-file=/path/to/my_state.json
You can combine these flags and arguments in a single command.`,

	// RunE is used instead of Run so that we can return an error if needed.
	RunE: func(cmd *cobra.Command, args []string) error {

		// 1) Aggregate all packages from CLI args, file, or package.json
		var pkgList []string

		// a) Direct CLI arguments
		if len(args) > 0 {
			pkgList = append(pkgList, args...)
		}

		// b) If a file with packages is specified
		if packageListFile != "" {
			filePkgs, err := parsePackageListFile(packageListFile)
			if err != nil {
				return fmt.Errorf("failed to parse package list file: %w", err)
			}
			pkgList = append(pkgList, filePkgs...)
		}

		// c) If a package.json file is specified
		if packageJSONFile != "" {
			pkgJSONPkgs, err := parsePackageJSON(packageJSONFile)
			if err != nil {
				return fmt.Errorf("failed to parse package.json file: %w", err)
			}
			pkgList = append(pkgList, pkgJSONPkgs...)
		}

		// If no package found at all, return an error
		if len(pkgList) == 0 {
			return fmt.Errorf("no packages were specified to download")
		}

		// 2) Create a context with a timeout (adjust time as you see fit)
		ctx, cancel := context.WithTimeout(context.Background(), time.Hour*24)
		defer cancel()

		// 3) Instantiate your service(s) and call the downloader
		//    Below is pseudo-code; adapt to your real code.
		//
		log := logger.NewLogger(zapcore.InfoLevel, true)
		httpCli := httpclient.NewHttpClient(http.DefaultClient)
		fs := filesystem.NewOsFileSystem()
		npmRepo := repositories.NewNpmRepository("https://registry.npmjs.org", httpCli, log)
		fileRepo := repositories.NewFileRepository(downloadDest, fs, downloadStateFile)

		serv := services.NewNpmDownloadService(npmRepo, fileRepo, log)

		// Pass the options for parallel workers and update local repository
		options := services.DownloadPackagesOptions{
			MetadataWorkers:       metadataWorkers,
			DownloadWorkers:       downloadWorkers,
			UpdateLocalRepository: updateLocalRepository,
		}

		serv.DownloadPackages(ctx, pkgList, options)

		// For demonstration, just print the result
		fmt.Println("Starting download...")
		// (simulate a call to the service)
		time.Sleep(time.Second * 2) // simulate some work

		// 4) Print a summary
		fmt.Println("Download summary:")
		fmt.Printf("  - Destination folder: %s\n", downloadDest)
		fmt.Printf("  - State file: %s\n", downloadStateFile)
		fmt.Printf("  - Total packages: %d\n", len(pkgList))
		fmt.Printf("  - Metadata workers: %d\n", metadataWorkers)
		fmt.Printf("  - Download workers: %d\n", downloadWorkers)
		fmt.Printf("  - Update local repository: %t\n", updateLocalRepository)
		for _, p := range pkgList {
			fmt.Printf("    * %s\n", p)
		}

		fmt.Println("All done. Have a great day!")
		return nil
	},
}

func init() {
	// Attach downloadCmd to the root command
	rootCmd.AddCommand(downloadCmd)

	// Define flags for the downloadCmd
	downloadCmd.Flags().StringVarP(&downloadDest, "dest", "d", ".",
		"Destination folder for downloaded packages")
	downloadCmd.Flags().StringVarP(&packageListFile, "file", "f", "",
		"Path to a file containing a list of packages (one package per line)")
	downloadCmd.Flags().StringVarP(&packageJSONFile, "package-json", "p", "",
		"Path to a package.json file from which dependencies will be extracted")
	downloadCmd.Flags().StringVarP(
		&downloadStateFile,
		"state-file", "s",
		"./download_state",
		"Path to the file storing the state of already-downloaded packages",
	)

	// Define flags for configuring the parallelism
	downloadCmd.Flags().IntVar(&metadataWorkers, "metadata-workers", 5,
		"Number of parallel workers for fetching metadata")
	downloadCmd.Flags().IntVar(&downloadWorkers, "download-workers", 100,
		"Number of parallel workers for downloading tarballs")

	// Define flag for updating local repository
	downloadCmd.Flags().BoolVar(&updateLocalRepository, "update-local-repository", true,
		"Check for package updates present in the local repository via the state file (default true)")
}

// parsePackageListFile reads a file line by line and returns a slice of package names.
// It trims whitespace and skips empty lines.
func parsePackageListFile(filePath string) ([]string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(content), "\n")

	var pkgs []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			pkgs = append(pkgs, line)
		}
	}
	return pkgs, nil
}

// PackageJSON represents the relevant fields we care about in a package.json
// You can add fields for devDependencies, peerDependencies, etc. as needed.
type PackageJSON struct {
	Dependencies     map[string]string `json:"dependencies,omitempty"`
	DevDependencies  map[string]string `json:"devDependencies,omitempty"`
	PeerDependencies map[string]string `json:"peerDependencies,omitempty"`
}

// parsePackageJSON reads a package.json file and collects package names from
// dependencies, devDependencies, and peerDependencies.
func parsePackageJSON(filePath string) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var pkg PackageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}

	var pkgs []string
	for name := range pkg.Dependencies {
		pkgs = append(pkgs, name)
	}
	for name := range pkg.DevDependencies {
		pkgs = append(pkgs, name)
	}
	for name := range pkg.PeerDependencies {
		pkgs = append(pkgs, name)
	}

	return pkgs, nil
}
