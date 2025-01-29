// Copyright (c) 2025, soup and the SRTran contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package translate

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

func (s *Service) translateWithOpenAI(ctx context.Context, text string, sourceLang, targetLang string) ([][]string, error) {
	if s.config.Model == "" {
		return nil, fmt.Errorf("model must be specified for OpenAI/OpenRouter backend")
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

	var cleanTranslations [][]string
	for _, t := range translations {
		t = strings.TrimSpace(t)
		if t == "" {
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
		}
	}

	return cleanTranslations, nil
}
