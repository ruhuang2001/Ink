package printer

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/ruhuang/ink/server/internal/plugins"
	"github.com/ruhuang/ink/server/internal/workspace"
)

func TestRenderLimitsRejectOversizedContentWithoutTruncation(t *testing.T) {
	service := NewService(nil, nil, nil, nil, "", "", time.Second)
	for _, test := range []struct{ name, title, content string }{
		{"title bytes", strings.Repeat("a", maxPrintTitleBytes+1), "text"},
		{"content bytes", "title", strings.Repeat("a", maxPrintContentBytes+1)},
		{"wrapped image height", "title", strings.Repeat("长", 4000)},
		{"explicit lines", "title", strings.Repeat("text\n", 500)},
		{"zero advance characters", "title", strings.Repeat("\u0301", 7000)},
	} {
		t.Run(test.name, func(t *testing.T) {
			image, err := service.RenderPreview(t.Context(), test.title, test.content)
			if !errors.Is(err, ErrContentTooLarge) || image != "" {
				t.Fatalf("expected rejected content with no truncated PNG, got image length %d, error %v", len(image), err)
			}
		})
	}
}

func TestRenderLimitsApplyToBlocksAndJobCreation(t *testing.T) {
	service := NewService(nil, nil, nil, nil, "", "", time.Second)
	_, err := service.RenderBlocksPreview(t.Context(), "title", []plugins.ContentBlock{{Type: plugins.BlockParagraph, Text: strings.Repeat("长", 4000)}})
	if !errors.Is(err, ErrContentTooLarge) {
		t.Fatalf("blocks bypassed limit: %v", err)
	}
	repo := newFakePrinterRepo()
	repo.bindings["device-1"] = Binding{ID: "device-1", UserID: "user-1", Status: workspace.DeviceStatusConnected}
	service = NewService(repo, fakeAuthenticator{}, fakeIDGenerator{}, fakeClock{now: time.Now()}, "", "", time.Second)
	_, err = service.CreatePrintJob(t.Context(), "token", CreateJobInput{Title: "title", Content: strings.Repeat("长", 4000), PrinterBindingID: "device-1"})
	if !errors.Is(err, ErrContentTooLarge) || len(repo.jobs) != 0 {
		t.Fatalf("oversized print was persisted: %v", err)
	}
}

func TestRenderLimitsPreserveNormalPNGAndBoundOutput(t *testing.T) {
	for _, content := range []string{"你好，世界\n\n打印清单", strings.Repeat("A shopping list: eggs, bread and milk. ", 20), strings.Repeat("中文排版", 300)} {
		image, err := renderPrintImage("title", content)
		if err != nil {
			t.Fatalf("ordinary content rejected: %v", err)
		}
		payload, err := base64.StdEncoding.DecodeString(image)
		if err != nil {
			t.Fatal(err)
		}
		config, err := png.DecodeConfig(bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		if config.Width != printWidth || config.Height > maxPrintImageHeight {
			t.Fatalf("unexpected image size: %+v", config)
		}
	}
}

func TestRenderLimitsRejectLegacyPendingJobBeforeClaim(t *testing.T) {
	repo := newFakePrinterRepo()
	repo.bindings["device-1"] = Binding{ID: "device-1", UserID: "user-1", Status: workspace.DeviceStatusConnected}
	repo.jobs["legacy"] = Job{ID: "legacy", UserID: "user-1", PrinterBindingID: "device-1", Title: "title", Content: strings.Repeat("长", 4000), Status: workspace.PrintStatusPending, UpdatedAt: time.Now()}
	service := NewService(repo, fakeAuthenticator{}, fakeIDGenerator{}, fakeClock{now: time.Now()}, "key", "", time.Second)
	_, err := service.SubmitPrintJob(t.Context(), "token", "legacy")
	if !errors.Is(err, ErrContentTooLarge) || repo.jobs["legacy"].Status != workspace.PrintStatusPending {
		t.Fatalf("oversized legacy task must remain pending: %v, %+v", err, repo.jobs["legacy"])
	}
}
