// Copyright (c) 2025, soup and the SRTran contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package translate

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

func (s *Service) translateWithLMStudio(ctx context.Context, text string, sourceLang, targetLang string) ([][]string, error) {
	if s.config.Model == "" {
		return nil, fmt.Errorf("model must be specified for LM Studio backend")
	}

	if s.config.BaseURL == "" {
		return nil, fmt.Errorf("base URL must be specified for LM Studio backend")
	}

	if err := s.waitForRateLimit(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait interrupted: %w", err)
	}

	resp, err := s.openaiClient.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: s.config.Model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: fmt.Sprintf(translationPrompt, sourceLang, targetLang, text),
				},
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to translate batch: %w", err)
	}

	// split response by subtitle separator
	translations := strings.Split(resp.Choices[0].Message.Content, "===SUBTITLE===")

	// Count how many translations we expect by counting "[N]" in the input text
	expectedCount := strings.Count(text, "[")

	var cleanTranslations [][]string
	for i, t := range translations {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}

		// Skip if it's the format instruction from the system prompt
		if i == 0 && strings.Contains(t, "(subtitle number)") {
			continue
		}

		// remove the [N] prefix
		if idx := strings.Index(t, "]\n"); idx != -1 {
			t = strings.TrimSpace(t[idx+2:])
		}

		// split by natural line breaks
		lines := strings.Split(t, "\n")
		if len(lines) > 0 {
			cleanTranslations = append(cleanTranslations, lines)
			// Stop if we have enough translations
			if len(cleanTranslations) >= expectedCount {
				break
			}
		}
	}

	// Ensure we have exactly the expected number of translations
	if len(cleanTranslations) != expectedCount {
		return nil, fmt.Errorf("expected %d translations but got %d", expectedCount, len(cleanTranslations))
	}

	return cleanTranslations, nil
}
