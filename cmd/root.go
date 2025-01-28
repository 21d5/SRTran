package cmd

import (
	"github.com/spf13/cobra"
)

var (
	// Flags
	inputFile      string
	outputFile     string
	targetLanguage string
	sourceLanguage string
	verbose        bool

	// Root command
	rootCmd = &cobra.Command{
		Use:   "srtran",
		Short: "SRTran - Subtitle Translation Tool",
		Long: `SRTran is a command-line tool for translating subtitle files (.srt)
from one language to another using OpenAI's translation capabilities.

Example:
  srtran translate -i input.srt -o output.srt -s en -t es`,
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}
