package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/sqlc-dev/plugin-sdk-go/codegen"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("sqlc-bulk-plugin " + buildVersion())
		os.Exit(0)
	}
	codegen.Run(generate)
}

func buildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "dev"
}
