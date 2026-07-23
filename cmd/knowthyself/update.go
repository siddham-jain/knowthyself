package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/siddham-jain/knowthyself/internal/update"
)

func runUpdate(args []string) error {
	fs := flag.NewFlagSet("knowthyself update", flag.ContinueOnError)
	checkOnly := fs.Bool("check", false, "report whether an update is available, then exit")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: knowthyself update [--check]\n\nUpdates knowthyself to the latest release. Binaries installed by a package\nmanager are reported, not replaced.\n\nflags:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	latest, err := update.Latest(ctx)
	if err != nil {
		return err
	}

	if version == "dev" {
		fmt.Printf("this is a development build; the latest release is %s\n", latest.Version)
		fmt.Println("build from source with `make build`, or install a release from https://github.com/siddham-jain/knowthyself/releases")
		return nil
	}
	if update.Compare(version, latest.Version) >= 0 {
		fmt.Printf("knowthyself %s is the latest release\n", version)
		return nil
	}

	fmt.Printf("knowthyself %s → %s available\n", version, latest.Version)
	if *checkOnly {
		return nil
	}

	method := update.Detect()
	if cmd := method.Command(); cmd != "" {
		fmt.Printf("\ninstalled with %s, so knowthyself will not replace the binary itself.\nupgrade with:\n\n  %s\n\n", method, cmd)
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate the running binary: %w", err)
	}
	fmt.Printf("\n  downloading %s\n", latest.AssetName())
	if err := update.Apply(ctx, latest, exe); err != nil {
		return err
	}
	fmt.Printf("  checksum ok\n  installed to %s\n\nnow on %s\n", exe, latest.Version)
	return nil
}
