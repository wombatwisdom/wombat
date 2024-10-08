package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReplaceAsciidocLinksWithMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Single link",
			input:    `https://example.com[example]`,
			expected: `[example](https://example.com)`,
		},
		{
			name:     "Multiple links",
			input:    `https://example.com[example] and https://another.com[another]`,
			expected: `[example](https://example.com) and [another](https://another.com)`,
		},
		{
			name:     "No links",
			input:    `This is a text without links.`,
			expected: `This is a text without links.`,
		},
		{
			name:     "Mixed content",
			input:    `Check this https://example.com[example] and this text.`,
			expected: `Check this [example](https://example.com) and this text.`,
		},
		{
			name:     "Mixed content in new tab",
			input:    `Check this https://example.com[example^] and this text.`,
			expected: `Check this [example](https://example.com) and this text.`,
		},
		{
			name:     "Mixed xref content",
			input:    `Check this xref:configuration:secrets.adoc[secrets page for more info] and this text.`,
			expected: `Check this secrets page for more info and this text.`,
		},
		{
			name:     "Mixed angle bracket content",
			input:    `Check this <<configuration:secrets.adoc,secrets page for more info>> and this text.`,
			expected: `Check this secrets page for more info and this text.`,
		},
		{
			name:     "Mixed angle bracket content",
			input:    "Executes a topology of xref:components:processors/branch.adoc[`branch` processors], performing them in parallel where possible.",
			expected: "Executes a topology of `branch` processors, performing them in parallel where possible.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceAsciidocLinksWithMarkdown(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
