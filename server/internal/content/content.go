package content

// BlockType identifies the printable meaning of a content block.
type BlockType string

const (
	BlockHeading   BlockType = "heading"
	BlockParagraph BlockType = "paragraph"
	BlockImage     BlockType = "image"
	BlockLink      BlockType = "link"
	BlockDivider   BlockType = "divider"
	BlockList      BlockType = "list"
	BlockQuote     BlockType = "quote"
)

// ContentBlock is one semantic unit rendered by the shared print pipeline.
type ContentBlock struct {
	Type  BlockType `json:"type"`
	Level int       `json:"level,omitempty"` // heading: 1..3
	Text  string    `json:"text,omitempty"`  // heading / paragraph / link
	URL   string    `json:"url,omitempty"`   // image / link
	Alt   string    `json:"alt,omitempty"`   // image
	Items []string  `json:"items,omitempty"` // list
	Style string    `json:"style,omitempty"` // list: bullet / ordered
}
