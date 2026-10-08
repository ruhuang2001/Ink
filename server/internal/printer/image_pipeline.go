package printer

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	memobirdapi "github.com/ruhuang2001/memobird-go/memobird"
)

type imagePrintPipeline interface {
	PrintJob(ctx context.Context, client *memobirdapi.Client, job Job) (*memobirdapi.PrintResponse, error)
}

//go:embed assets/NotoSansSC-Regular.otf
var printerFontData []byte

type textImagePrintPipeline struct{}

func (p textImagePrintPipeline) PrintJob(ctx context.Context, client *memobirdapi.Client, job Job) (*memobirdapi.PrintResponse, error) {
	imageBase64, err := renderPrintImage(job.Title, job.Content)
	if err != nil {
		return nil, err
	}

	return client.PrintImage(ctx, imageBase64)
}

func renderPrintableText(title string, content string) string {
	parts := []string{
		normalizePrintableText(title),
		normalizePrintableText(content),
	}
	return strings.Join(parts, "\n\n")
}

func normalizePrintableText(input string) string {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "\r", "\n")

	lines := strings.Split(input, "\n")
	normalizedLines := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(strings.Map(normalizePrintableRune, line))
		line = strings.TrimPrefix(line, "• ")
		line = strings.TrimPrefix(line, "· ")
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")
		line = strings.TrimSpace(line)
		normalizedLines = append(normalizedLines, line)
	}

	return strings.TrimSpace(strings.Join(collapseBlankLines(normalizedLines), "\n"))
}

func collapseBlankLines(lines []string) []string {
	collapsed := make([]string, 0, len(lines))
	previousBlank := true
	for _, line := range lines {
		blank := strings.TrimSpace(line) == ""
		if blank && previousBlank {
			continue
		}
		collapsed = append(collapsed, line)
		previousBlank = blank
	}

	return collapsed
}

func normalizePrintableRune(r rune) rune {
	switch {
	case r == utf8.RuneError:
		return -1
	case r == '\t':
		return ' '
	case unicode.IsControl(r):
		return -1
	case unicode.In(r, unicode.So):
		return -1
	case r == '•', r == '·':
		return '-'
	case r == '\u00A0', r == '\u2007', r == '\u202F':
		return ' '
	case unicode.IsSpace(r):
		return ' '
	default:
		return r
	}
}

func primaryPrintResponse(responses []*memobirdapi.PrintResponse) (*memobirdapi.PrintResponse, error) {
	for _, response := range responses {
		if response != nil {
			return response, nil
		}
	}

	return nil, fmt.Errorf("print returned no responses")
}
