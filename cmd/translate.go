// Copyright (c) 2025, soup and the SRTran contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/s0up4200/SRTran/internal/config"
	"github.com/s0up4200/SRTran/internal/srt"
	"github.com/s0up4200/SRTran/internal/translate"
	"github.com/spf13/cobra"
)

var translateCmd = &cobra.Command{
	Use:   "translate",
	Short: "Translate subtitle files",
	Long: `Translate subtitle files from one language to another using OpenAI.
	
Example:
  srtran translate -i input.srt -o output.srt -s english -t norwegian`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate flags
		if inputFile == "" {
			return fmt.Errorf("input file is required")
		}
		if outputFile == "" {
			return fmt.Errorf("output file is required")
		}
		if targetLanguage == "" {
			return fmt.Errorf("target language is required")
		}
		if sourceLanguage == "" {
			return fmt.Errorf("source language is required")
		}

		if verbose {
			fmt.Printf("Translating %s from %s to %s\n", inputFile, sourceLanguage, targetLanguage)
		}

		// Get configuration
		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Print configuration info
		log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
		log.Info().
			Str("backend", cfg.Backend).
			Str("model", cfg.Model).
			Msg("configuration loaded")

		if verbose {
			if configFile != "" {
				log.Debug().
					Str("config_file", configFile).
					Msg("using configuration file")
			} else {
				log.Debug().Msg("using environment variables")
			}
		}

		// Initialize the SRT parser
		parser := srt.NewParser(verbose)

		// Parse input file
		subtitles, err := parser.Parse(inputFile)
		if err != nil {
			return fmt.Errorf("failed to parse input file: %w", err)
		}

		// Configure translation service
		config := translate.ServiceConfig{
			APIKey:  cfg.APIKey,
			Model:   cfg.Model,
			Verbose: verbose,
			Backend: translate.Backend(cfg.Backend),
		}

		// If using OpenRouter, configure its specific settings
		if cfg.Backend == "openrouter" {
			config.BaseURL = "https://openrouter.ai/api/v1"
		}

		// Initialize translation service
		service, err := translate.NewService(config)
		if err != nil {
			return fmt.Errorf("failed to initialize translation service: %w", err)
		}

		// Translate subtitles
		translated, err := service.Translate(cmd.Context(), subtitles, sourceLanguage, targetLanguage)
		if err != nil {
			return fmt.Errorf("failed to translate subtitles: %w", err)
		}

		// Write output file
		if err := parser.Write(outputFile, translated); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}

		if verbose {
			fmt.Printf("Successfully translated %s to %s\n", inputFile, outputFile)
		}
		return nil
	},
}

func init() {
	translateCmd.Flags().StringVarP(&inputFile, "input", "i", "", "input subtitle file")
	translateCmd.Flags().StringVarP(&outputFile, "output", "o", "", "output subtitle file")
	translateCmd.Flags().StringVarP(&sourceLanguage, "source-language", "s", "", "source language (e.g., 'english', 'spanish')")
	translateCmd.Flags().StringVarP(&targetLanguage, "target-language", "t", "", "target language (e.g., 'norwegian', 'german')")
	translateCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(translateCmd)
}
