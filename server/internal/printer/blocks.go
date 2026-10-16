package printer

import (
	"fmt"
	"strings"

	"github.com/ruhuang/ink/server/internal/content"
)

// RenderBlocksToText flattens a list of ContentBlocks into the plain-text
// representation consumed by the existing text→image pipeline.
//
// The renderer is intentionally minimal: the 5 block types are projected to
// the subset of formatting the thermal printer can actually render. Headings
// are decorated with markdown-style hashes (#, ##, ###) so the printer driver
// can upscale them; paragraphs and link labels become regular lines; image
// and link URLs are printed below a short caption; dividers become a line of
// dashes.
func RenderBlocksToText(blocks []content.ContentBlock) (string, error) {
	if err := content.ValidateBlocks(blocks); err != nil {
		return "", err
	}

	sections := make([]string, 0, len(blocks))
	for _, block := range blocks {
		section, err := renderBlock(block)
		if err != nil {
			return "", err
		}
		if section == "" {
			continue
		}
		sections = append(sections, section)
	}
	return strings.Join(sections, "\n\n"), nil
}

func renderBlock(block content.ContentBlock) (string, error) {
	switch block.Type {
	case content.BlockHeading:
		return strings.TrimSpace(block.Text), nil
	case content.BlockParagraph:
		return strings.TrimSpace(block.Text), nil
	case content.BlockImage:
		alt := strings.TrimSpace(block.Alt)
		if alt == "" {
			alt = "图片"
		}
		return fmt.Sprintf("[%s] %s", alt, strings.TrimSpace(block.URL)), nil
	case content.BlockLink:
		label := strings.TrimSpace(block.Text)
		url := strings.TrimSpace(block.URL)
		if label == "" {
			return url, nil
		}
		return fmt.Sprintf("%s\n%s", label, url), nil
	case content.BlockDivider:
		return strings.Repeat("-", 16), nil
	case content.BlockList:
		lines := make([]string, 0, len(block.Items))
		for index, item := range block.Items {
			prefix := "- "
			if block.Style == "ordered" {
				prefix = fmt.Sprintf("%d. ", index+1)
			}
			lines = append(lines, prefix+strings.TrimSpace(item))
		}
		return strings.Join(lines, "\n"), nil
	case content.BlockQuote:
		return "“" + strings.TrimSpace(block.Text) + "”", nil
	default:
		return "", fmt.Errorf("unsupported block type %q", block.Type)
	}
}
