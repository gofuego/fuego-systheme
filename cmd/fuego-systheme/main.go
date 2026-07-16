package main

import (
	"flag"
	"fmt"
	"os"

	systheme "github.com/gofuego/fuego-systheme"
)

func main() {
	fs := flag.NewFlagSet("fuego-systheme", flag.ContinueOnError)
	siteName := fs.String("site-name", "", "site title (default: \"System Docs\")")
	baseURL := fs.String("base-url", "", "base URL for the generated site")
	output := fs.String("output", "", "output directory (default: \"build\")")
	strictLinks := fs.Bool("strict-links", false, "fail the build on a broken internal link")

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: fuego-systheme [flags] <repo-path> [build|serve|validate]")
		fmt.Fprintln(os.Stderr, "\nFlags:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(1)
	}

	args := fs.Args()
	if len(args) < 1 {
		fs.Usage()
		os.Exit(1)
	}

	repoPath := args[0]
	command := "serve"
	if len(args) > 1 {
		command = args[1]
	}

	err := systheme.Run(repoPath, systheme.Options{
		SiteName:    *siteName,
		BaseURL:     *baseURL,
		Output:      *output,
		Command:     command,
		StrictLinks: *strictLinks,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "fuego-systheme: %v\n", err)
		os.Exit(1)
	}
}
