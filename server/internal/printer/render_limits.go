package printer

import (
	"strings"
	"unicode/utf8"

	memobirdtextrender "github.com/ruhuang2001/memobird-go/textrender"
)

const (
	maxPrintTitleBytes   = 512
	maxPrintContentBytes = 64 << 10
	maxPrintImageHeight  = 8192
	maxPrintTextRunes    = 8000
	printWidth           = 384
	printPadding         = 18
	printFontSize        = 22
	printLineHeight      = 1.55
)

// printRenderOptions is the exact layout used for both preview and paper. Limits
// are checked against it through textrender.Measure, so a measured slip and a
// rendered slip cannot disagree.
var printRenderOptions = memobirdtextrender.Options{
	Width:      printWidth,
	Padding:    printPadding,
	FontSize:   printFontSize,
	LineHeight: printLineHeight,
	FontData:   printerFontData,
	// Ink prepares the text itself so every character the caller wrote survives
	// to the slip. The renderer's own cleanup would also rewrite spacing that the
	// rune limit below already bounds.
	SkipNormalize: true,
}

func printableTextForRender(title, content string) (string, error) {
	if len(title) > maxPrintTitleBytes || len(content) > maxPrintContentBytes || utf8.RuneCountInString(title)+utf8.RuneCountInString(content) > maxPrintTextRunes {
		return "", ErrContentTooLarge
	}
	text := renderPrintableText(title, content)
	if strings.TrimSpace(text) == "" {
		return "", ErrInvalidInput
	}
	if err := validatePrintImageHeight(text); err != nil {
		return "", err
	}
	return text, nil
}

// validatePrintImageHeight rejects text that would render taller than the paper
// allows. textrender.MaxLines is not used: it truncates silently, which would
// print a slip that stops mid-sentence.
func validatePrintImageHeight(text string) error {
	layout, err := memobirdtextrender.Measure(text, printRenderOptions)
	if err != nil {
		return ErrContentTooLarge
	}
	if layout.Height > maxPrintImageHeight {
		return ErrContentTooLarge
	}
	return nil
}

func renderPrintImage(title, content string) (string, error) {
	text, err := printableTextForRender(title, content)
	if err != nil {
		return "", err
	}
	return memobirdtextrender.RenderBase64PNG(text, printRenderOptions)
}
