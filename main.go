package main

import (
	_ "embed"
	"os"

	"github.com/fernandoaleman/dots/internal/cli"
)

//go:embed LICENSE
var license string

func main() {
	os.Exit(cli.RunWithLicense(os.Args[1:], os.Stdout, os.Stderr, license))
}
