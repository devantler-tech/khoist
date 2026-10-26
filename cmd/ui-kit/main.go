// The ui-kit command retains the publisher-review entrypoint for existing deployments.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/devantler-tech/data-product-controller/internal/portablehost"
	"github.com/devantler-tech/data-product-controller/pkg/featureflag"
	"github.com/devantler-tech/data-product-controller/web"
)

// main starts the independently deployed host and reports configuration failures before listening.
func main() {
	if err := run(os.Args[1:], os.Getenv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		slog.Error("UI compatibility kit", "error", err)
		os.Exit(1)
	}
}

// run binds only the operator-selected listener; it never fetches or proxies a product.
func run(args []string, getenv func(string) string) error {
	options, err := portablehost.ParseOptions("ui-kit", args)
	if err != nil {
		return err
	}
	publisherSetting := getenv("PUBLISHER_PREFLIGHT_ENABLED")
	if publisherSetting != "" && publisherSetting != "true" && publisherSetting != "false" {
		return errors.New("PUBLISHER_PREFLIGHT_ENABLED must be true or false")
	}
	client, err := featureflag.NewClient(
		"ui-kit",
		featureflag.NewProvider(
			map[string]bool{
				"publisher-preflight": publisherSetting == "true",
			},
		),
	)
	if err != nil {
		return fmt.Errorf("configure publisher flag: %w", err)
	}
	return portablehost.Run(options, web.PublisherHandler(func() bool {
		return featureflag.Enabled(context.Background(), client, "publisher-preflight")
	}))
}
