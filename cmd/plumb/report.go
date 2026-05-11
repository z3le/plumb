package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/z3le/plumb/internal/profile"
	"golang.org/x/tools/cover"
)

type FuncStats struct {
	Name  string
	Line  int
	Count int // call count
}

func reportCmd(args []string) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)

	open := fs.Bool("open", false, "open report in browser after writing")
	out := fs.String("out", "coverage.html", "output HTML file")
	title := fs.String("title", "", "report title")

	if err := fs.Parse(args); err != nil {
		return err
	}

	profilePath := ".plumb/coverage.out" // default
	if len(fs.Args()) > 0 {
		profilePath = fs.Args()[0]
	}

	fmt.Println("profile:", profilePath)
	fmt.Println("open:", *open)
	fmt.Println("out:", *out)
	_ = title
	profiles, err := cover.ParseProfiles(profilePath)

	if err != nil {
		panic(err)
	}

	for _, p := range profiles {
		if strings.HasSuffix(p.FileName, "_test.go") {
			continue
		}

		diskPath, err := profile.ResolveFilePath(p.FileName)
		if err != nil {
			return fmt.Errorf("resolving file path: %w", err)
		}

		profile.Annotate(p, diskPath)
	}
	fmt.Println(profiles)
	return nil
}
