// Copyright (c) 2025, soup and the SRTran contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package srt

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Subtitle represents a single subtitle block in an SRT file
type Subtitle struct {
	Index      int
	Start      string
	End        string
	Text       []string
	Translated []string
}

// Parser handles SRT file parsing and writing
type Parser struct {
	Verbose bool
}

// NewParser creates a new SRT parser
func NewParser(verbose bool) *Parser {
	return &Parser{
		Verbose: verbose,
	}
}

// Parse reads an SRT file and returns a slice of Subtitle structs
func (p *Parser) Parse(filename string) ([]Subtitle, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var subtitles []Subtitle
	var current *Subtitle

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Check if this is a new subtitle index
		if index, err := strconv.Atoi(line); err == nil {
			// Save the previous subtitle if it exists
			if current != nil && current.Start != "" {
				subtitles = append(subtitles, *current)
			}
			// Start a new subtitle
			current = &Subtitle{Index: index}
			continue
		}

		// If we don't have a current subtitle, skip this line
		if current == nil {
			continue
		}

		// Try to parse as timestamp
		if current.Start == "" {
			if strings.Contains(line, " --> ") {
				times := strings.Split(line, " --> ")
				if len(times) == 2 {
					current.Start = times[0]
					current.End = times[1]
					continue
				}
			}
			continue
		}

		// If we get here, this must be subtitle text
		current.Text = append(current.Text, line)
	}

	// Don't forget the last subtitle
	if current != nil && current.Start != "" {
		subtitles = append(subtitles, *current)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	if len(subtitles) == 0 {
		return nil, fmt.Errorf("no valid subtitles found in file")
	}

	if p.Verbose {
		fmt.Printf("Parsed %d subtitles from %s\n", len(subtitles), filename)
	}

	return subtitles, nil
}

// Write saves the translated subtitles to a new SRT file
func (p *Parser) Write(filename string, subtitles []Subtitle) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for i, sub := range subtitles {
		// Write subtitle index
		if _, err := fmt.Fprintf(writer, "%d\n", sub.Index); err != nil {
			return fmt.Errorf("failed to write index: %w", err)
		}

		// Write timestamps
		if _, err := fmt.Fprintf(writer, "%s --> %s\n", sub.Start, sub.End); err != nil {
			return fmt.Errorf("failed to write timestamps: %w", err)
		}

		// Write translated text or original if translation is empty
		text := sub.Text
		if len(sub.Translated) > 0 {
			text = fmt.Sprintf("%s\n%s", sub.Text, sub.Translated)
		}
		for _, line := range text {
			if _, err := fmt.Fprintf(writer, "%s\n", line); err != nil {
				return fmt.Errorf("failed to write text: %w", err)
			}
		}

		// Add blank line between subtitles (except for last one)
		if i < len(subtitles)-1 {
			if _, err := fmt.Fprintf(writer, "\n"); err != nil {
				return fmt.Errorf("failed to write separator: %w", err)
			}
		}
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	if p.Verbose {
		fmt.Printf("Wrote %d subtitles to %s\n", len(subtitles), filename)
	}

	return nil
}
