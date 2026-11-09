package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"

	"github.com/CosmoS1X/web-crawler/internal/app"
	"github.com/CosmoS1X/web-crawler/internal/versioninfo"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cmd := app.New(&http.Client{}, versioninfo.Resolve(version, versioninfo.ModuleVersion()))
	if err := cmd.Run(ctx, os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
