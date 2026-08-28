package printer

import (
	"fmt"
	"strings"

	"github.com/ruhuang/ink/server/internal/plugins"
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
func RenderBlocksToText(blocks []plugins.ContentBlock) (string, error) {
	if err := plugins.ValidateBlocks(blocks); err != nil {
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

func renderBlock(block plugins.ContentBlock) (string, error) {
	switch block.Type {
	case plugins.BlockHeading:
		return strings.TrimSpace(block.Text), nil
	case plugins.BlockParagraph:
		return strings.TrimSpace(block.Text), nil
	case plugins.BlockImage:
		alt := strings.TrimSpace(block.Alt)
		if alt == "" {
			alt = "图片"
		}
		return fmt.Sprintf("[%s] %s", alt, strings.TrimSpace(block.URL)), nil
	case plugins.BlockLink:
		label := strings.TrimSpace(block.Text)
		url := strings.TrimSpace(block.URL)
		if label == "" {
			return url, nil
		}
		return fmt.Sprintf("%s\n%s", label, url), nil
	case plugins.BlockDivider:
		return strings.Repeat("-", 16), nil
	case plugins.BlockList:
		lines := make([]string, 0, len(block.Items))
		for index, item := range block.Items {
			prefix := "- "
			if block.Style == "ordered" {
				prefix = fmt.Sprintf("%d. ", index+1)
			}
			lines = append(lines, prefix+strings.TrimSpace(item))
		}
		return strings.Join(lines, "\n"), nil
	case plugins.BlockQuote:
		return "“" + strings.TrimSpace(block.Text) + "”", nil
	default:
		return "", fmt.Errorf("unsupported block type %q", block.Type)
	}
}
