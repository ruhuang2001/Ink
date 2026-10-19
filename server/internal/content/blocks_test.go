package content

import (
	"strings"
	"testing"
)

func TestValidateBlocksRejectsEmpty(t *testing.T) {
	t.Parallel()

	if err := ValidateBlocks(nil); err == nil {
		t.Fatalf("expected error for nil blocks")
	}
	if err := ValidateBlocks([]ContentBlock{}); err == nil {
		t.Fatalf("expected error for empty slice")
	}
}

func TestValidateBlocksHeading(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		block   ContentBlock
		wantErr bool
	}{
		{name: "valid level 1", block: ContentBlock{Type: BlockHeading, Level: 1, Text: "hi"}},
		{name: "valid level 3", block: ContentBlock{Type: BlockHeading, Level: 3, Text: "hi"}},
		{name: "missing level", block: ContentBlock{Type: BlockHeading, Text: "hi"}, wantErr: true},
		{name: "level too high", block: ContentBlock{Type: BlockHeading, Level: 4, Text: "hi"}, wantErr: true},
		{name: "missing text", block: ContentBlock{Type: BlockHeading, Level: 1}, wantErr: true},
		{name: "whitespace text", block: ContentBlock{Type: BlockHeading, Level: 1, Text: "   "}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateBlocks([]ContentBlock{tc.block})
			if tc.wantErr && err == nil {
				t.Fatalf("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateBlocksParagraph(t *testing.T) {
	t.Parallel()

	if err := ValidateBlocks([]ContentBlock{{Type: BlockParagraph, Text: "hello"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ValidateBlocks([]ContentBlock{{Type: BlockParagraph}}); err == nil {
		t.Fatalf("expected error for empty paragraph")
	}
}

func TestValidateBlocksImage(t *testing.T) {
	t.Parallel()

	if err := ValidateBlocks([]ContentBlock{{Type: BlockImage, URL: "https://example.com/a.png"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cases := []string{"", "  ", "example.com", "ftp://example.com/a.png", "https://"}
	for _, raw := range cases {
		if err := ValidateBlocks([]ContentBlock{{Type: BlockImage, URL: raw}}); err == nil {
			t.Fatalf("expected error for url %q", raw)
		}
	}
}

func TestValidateBlocksLink(t *testing.T) {
	t.Parallel()

	if err := ValidateBlocks([]ContentBlock{{Type: BlockLink, URL: "http://example.com", Text: "click"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ValidateBlocks([]ContentBlock{{Type: BlockLink, URL: "javascript:alert(1)"}}); err == nil {
		t.Fatalf("expected error for non-http scheme")
	}
}

func TestValidateBlocksDivider(t *testing.T) {
	t.Parallel()

	if err := ValidateBlocks([]ContentBlock{{Type: BlockDivider}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateBlocksUnknownType(t *testing.T) {
	t.Parallel()

	err := ValidateBlocks([]ContentBlock{{Type: "carousel"}})
	if err == nil {
		t.Fatalf("expected error for unknown type")
	}
	if !strings.Contains(err.Error(), "unsupported block type") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestValidateBlocksMixedOrderErrorIndex(t *testing.T) {
	t.Parallel()

	blocks := []ContentBlock{
		{Type: BlockHeading, Level: 1, Text: "ok"},
		{Type: BlockParagraph, Text: ""},
	}
	err := ValidateBlocks(blocks)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "block[1]") {
		t.Fatalf("expected error to mention block[1]: %v", err)
	}
}
