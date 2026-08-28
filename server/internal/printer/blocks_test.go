package printer

import (
	"strings"
	"testing"

	"github.com/ruhuang/ink/server/internal/plugins"
)

func TestRenderBlocksToTextAllTypes(t *testing.T) {
	t.Parallel()

	blocks := []plugins.ContentBlock{
		{Type: plugins.BlockHeading, Level: 1, Text: "今日要闻"},
		{Type: plugins.BlockParagraph, Text: "早上好,这是今天的推送。"},
		{Type: plugins.BlockImage, URL: "https://example.com/a.png", Alt: "封面"},
		{Type: plugins.BlockLink, URL: "https://example.com/article", Text: "阅读全文"},
		{Type: plugins.BlockDivider},
	}

	rendered, err := RenderBlocksToText(blocks)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	expectedSubstrings := []string{
		"今日要闻",
		"早上好",
		"[封面] https://example.com/a.png",
		"阅读全文\nhttps://example.com/article",
		strings.Repeat("-", 16),
	}
	for _, snippet := range expectedSubstrings {
		if !strings.Contains(rendered, snippet) {
			t.Fatalf("expected rendered output to contain %q, got:\n%s", snippet, rendered)
		}
	}
}

func TestRenderBlocksToTextEmpty(t *testing.T) {
	t.Parallel()

	if _, err := RenderBlocksToText(nil); err == nil {
		t.Fatalf("expected error for empty blocks")
	}
}

func TestRenderBlocksToTextInvalidURL(t *testing.T) {
	t.Parallel()

	blocks := []plugins.ContentBlock{
		{Type: plugins.BlockImage, URL: "not-a-url"},
	}
	if _, err := RenderBlocksToText(blocks); err == nil {
		t.Fatalf("expected error for invalid image url")
	}
}

func TestRenderBlocksToTextLinkWithoutLabel(t *testing.T) {
	t.Parallel()

	blocks := []plugins.ContentBlock{
		{Type: plugins.BlockLink, URL: "https://example.com"},
	}

	rendered, err := RenderBlocksToText(blocks)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if rendered != "https://example.com" {
		t.Fatalf("unexpected rendered output: %q", rendered)
	}
}

func TestRenderBlocksToTextHeadingLevels(t *testing.T) {
	t.Parallel()

	for _, level := range []int{1, 2, 3} {
		blocks := []plugins.ContentBlock{{Type: plugins.BlockHeading, Level: level, Text: "标题"}}
		rendered, err := RenderBlocksToText(blocks)
		if err != nil {
			t.Fatalf("render level %d: %v", level, err)
		}
		if rendered != "标题" {
			t.Fatalf("expected plain heading text, got %q", rendered)
		}
	}
}
