package main

import (
	"context"
	"fmt"

	"github.com/fxdv/patchlog/pkg/ai"
	"github.com/fxdv/patchlog/pkg/config"
	"github.com/fxdv/patchlog/pkg/i18n"
	"github.com/fxdv/patchlog/pkg/render"
)

type OutputRenderOptions struct {
	Format string
	Tone   ai.Tone
	Config config.Config
	Quiet  bool
	DryRun bool
	Lang   i18n.Lang
}

// ReportOutputRenderer isolates format selection from orchestration. Alternate
// renderers can be exercised without running release planning or Apply.
type ReportOutputRenderer interface {
	Render(context.Context, render.Report, OutputRenderOptions) ([]byte, error)
}

type DefaultReportOutputRenderer struct{}

func (DefaultReportOutputRenderer) Render(ctx context.Context, report render.Report, opts OutputRenderOptions) ([]byte, error) {
	switch opts.Format {
	case "markdown":
		return render.Markdown(report)
	case "json":
		return render.JSON(report)
	case "prose":
		if opts.DryRun {
			text, err := ai.GenerateProseLang(ctx, report, opts.Tone, nil, opts.Lang)
			return []byte(text), err
		}
		return renderProseLang(ctx, report, opts.Tone, opts.Config, opts.Quiet, opts.Lang)
	default:
		return nil, fmt.Errorf("unsupported output format %q", opts.Format)
	}
}
