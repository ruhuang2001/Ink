package plugins

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidateBlocks returns the first semantic error found in blocks, or nil if
// every block is well-formed. It is intentionally strict: malformed blocks are
// rejected at ingest time so the dispatcher never has to guess.
func ValidateBlocks(blocks []ContentBlock) error {
	if len(blocks) == 0 {
		return fmt.Errorf("item must contain at least one block")
	}

	for index, block := range blocks {
		if err := validateBlock(block); err != nil {
			return fmt.Errorf("block[%d]: %w", index, err)
		}
	}
	return nil
}

var blockValidators = map[BlockType]func(ContentBlock) error{
	BlockHeading:   validateHeadingBlock,
	BlockParagraph: validateParagraphBlock,
	BlockImage:     validateImageBlock,
	BlockLink:      validateLinkBlock,
	BlockDivider:   func(ContentBlock) error { return nil },
	BlockList:      validateListBlock,
	BlockQuote:     validateQuoteBlock,
}

func validateBlock(block ContentBlock) error {
	fn, ok := blockValidators[block.Type]
	if !ok {
		return fmt.Errorf("unsupported block type %q", block.Type)
	}
	return fn(block)
}

func validateHeadingBlock(block ContentBlock) error {
	if block.Level < 1 || block.Level > 3 {
		return fmt.Errorf("heading.level must be 1..3, got %d", block.Level)
	}
	if strings.TrimSpace(block.Text) == "" {
		return fmt.Errorf("heading.text is required")
	}
	return nil
}

func validateParagraphBlock(block ContentBlock) error {
	if strings.TrimSpace(block.Text) == "" {
		return fmt.Errorf("paragraph.text is required")
	}
	return nil
}

func validateImageBlock(block ContentBlock) error {
	if err := validateHTTPURL(block.URL); err != nil {
		return fmt.Errorf("image.url: %w", err)
	}
	return nil
}

func validateLinkBlock(block ContentBlock) error {
	if err := validateHTTPURL(block.URL); err != nil {
		return fmt.Errorf("link.url: %w", err)
	}
	return nil
}

func validateListBlock(block ContentBlock) error {
	if block.Style != "" && block.Style != "bullet" && block.Style != "ordered" {
		return fmt.Errorf("list.style must be bullet or ordered")
	}
	if len(block.Items) == 0 {
		return fmt.Errorf("list.items must not be empty")
	}
	for _, item := range block.Items {
		if strings.TrimSpace(item) == "" {
			return fmt.Errorf("list.items must not contain empty values")
		}
	}
	return nil
}

func validateQuoteBlock(block ContentBlock) error {
	if strings.TrimSpace(block.Text) == "" {
		return fmt.Errorf("quote.text is required")
	}
	return nil
}

func validateHTTPURL(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("url is required")
	}
	parsed, err := url.ParseRequestURI(trimmed)
	if err != nil {
		return fmt.Errorf("invalid url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("url must use http or https scheme")
	}
	if parsed.Host == "" {
		return fmt.Errorf("url must include a host")
	}
	return nil
}
