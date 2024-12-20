// cmd/root.go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "npm-pkg",
	Short: "A CLI tool for local npm package management",
	Long: `npm-pkg is a command-line tool to manage and download npm packages locally.
It helps you fetch npm packages, store them offline, and manage dependencies.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// init is where you can define persistent flags or global config for rootCmd
func init() {
	// Example of persistent flag on the root command:
	// rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
}
