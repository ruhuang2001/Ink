package printer

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"

	memobirdtextrender "github.com/ruhuang2001/memobird-go/textrender"
)

const (
	maxPrintTitleBytes   = 512
	maxPrintContentBytes = 64 << 10
	maxPrintImageHeight  = 8192
	maxPrintTextRunes    = 8000
	maxMeasuredRunes     = 512
	printWidth           = 384
	printPadding         = 18
	printFontSize        = 22
	printLineHeight      = 1.55
)

var parsedPrinterFont = sync.OnceValues(func() (*opentype.Font, error) {
	return opentype.Parse(printerFontData)
})

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

// Count the same wrapped lines as textrender before it allocates the image.
// MaxLines in the renderer silently truncates, which is unsuitable for printing.
func validatePrintImageHeight(text string) error {
	parsed, err := parsedPrinterFont()
	if err != nil {
		return fmt.Errorf("parse printer font: %w", err)
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: printFontSize, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return fmt.Errorf("create printer font face: %w", err)
	}
	defer func() { _ = face.Close() }()
	metrics := face.Metrics()
	lineStep := max(1, int(math.Ceil(float64(metrics.Height.Ceil())*printLineHeight)))
	height := 2*printPadding + metrics.Ascent.Ceil() + metrics.Descent.Ceil()
	maxLines := 1 + (maxPrintImageHeight-height)/lineStep
	lines := 0
	drawer := font.Drawer{Face: face}
	for paragraph := range strings.SplitSeq(text, "\n") {
		if paragraph == "" {
			lines++
		} else {
			remaining := []rune(paragraph)
			for len(remaining) > 0 {
				lastFit, lastSpace := 0, 0
				for i := 1; i <= len(remaining); i++ {
					if i > maxMeasuredRunes {
						return ErrContentTooLarge
					}
					if drawer.MeasureString(string(remaining[:i])).Ceil() > printWidth-2*printPadding {
						break
					}
					lastFit = i
					if unicode.IsSpace(remaining[i-1]) {
						lastSpace = i
					}
				}
				cut := max(1, lastFit)
				if lastFit > 0 && lastSpace > 0 && lastSpace < len(remaining) {
					cut = lastSpace
				}
				lines++
				if lines > maxLines {
					return ErrContentTooLarge
				}
				remaining = remaining[cut:]
				for len(remaining) > 0 && unicode.IsSpace(remaining[0]) {
					remaining = remaining[1:]
				}
			}
		}
		if lines > maxLines {
			return ErrContentTooLarge
		}
	}
	return nil
}

func renderPrintImage(title, content string) (string, error) {
	text, err := printableTextForRender(title, content)
	if err != nil {
		return "", err
	}
	return memobirdtextrender.RenderBase64PNG(text, memobirdtextrender.Options{
		Width: printWidth, Padding: printPadding, FontSize: printFontSize,
		LineHeight: printLineHeight, FontData: printerFontData,
	})
}
