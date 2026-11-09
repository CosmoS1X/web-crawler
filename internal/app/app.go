package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/CosmoS1X/web-crawler/crawler"
)

const (
	appName          = "web-crawler"
	appUsage         = "analyze a website structure"
	argsUsage        = "<url>"
	defaultDepth     = 10
	defaultRetries   = 1
	defaultDelay     = 0 * time.Second
	defaultTimeout   = 15 * time.Second
	defaultRPS       = 0.0
	defaultUserAgent = ""
)

// New builds the CLI command. The HTTP client and version are injected by the caller.
func New(client *http.Client, version string) *cli.Command {
	return &cli.Command{
		Name:      appName,
		Usage:     appUsage,
		Version:   version,
		ArgsUsage: argsUsage,
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "url", UsageText: argsUsage, Required: true},
		},
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "depth",
				Usage: "crawl depth",
				Value: defaultDepth,
			},
			&cli.IntFlag{
				Name:  "retries",
				Usage: "number of retries for failed requests",
				Value: defaultRetries,
			},
			&cli.DurationFlag{
				Name:  "delay",
				Usage: "delay between requests (example: 200ms, 1s)",
				Value: defaultDelay,
			},
			&cli.DurationFlag{
				Name:  "timeout",
				Usage: "per-request timeout",
				Value: defaultTimeout,
			},
			&cli.FloatFlag{
				Name:  "rps",
				Usage: "limit requests per second (overrides delay)",
				Value: defaultRPS,
			},
			&cli.StringFlag{
				Name:  "user-agent",
				Usage: "custom user agent",
				Value: defaultUserAgent,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			report, err := crawler.Analyze(ctx, optionsFromCommand(cmd, client))
			if err != nil {
				return fmt.Errorf("analyze: %w", err)
			}

			report = append(report, '\n')
			if _, err := cmd.Root().Writer.Write(report); err != nil {
				return fmt.Errorf("write report: %w", err)
			}
			return nil
		},
	}
}

func optionsFromCommand(cmd *cli.Command, client *http.Client) crawler.Options {
	return crawler.Options{
		URL:        cmd.StringArg("url"),
		Depth:      cmd.Int("depth"),
		Retries:    cmd.Int("retries"),
		Delay:      cmd.Duration("delay"),
		Timeout:    cmd.Duration("timeout"),
		RPS:        cmd.Float("rps"),
		UserAgent:  cmd.String("user-agent"),
		IndentJSON: true,
		HTTPClient: client,
	}
}
