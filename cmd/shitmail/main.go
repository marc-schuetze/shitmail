// shitmail — self-hosted disposable email service.
// Copyright (c) 2026 DML Labs. Apache 2.0 License.
//
// Usage:
//
//      shitmail                       # start the server (default)
//      shitmail serve [flags]         # start the server explicitly
//      shitmail new [flags]           # create a mailbox and print its address
//      shitmail list <address> [flags]# list emails in a mailbox
//      shitmail read <address> <id>   # display a single email
//      shitmail watch <address> [flags]# stream new emails in real time
//      shitmail send <to> [flags]     # send a test email via SMTP
//      shitmail version               # print version and runtime info
//      shitmail help                  # print this help

//go:generate goversioninfo -icon=../../assets/brand/shitmail-logo-512.ico -manifest=shitmail.exe.manifest
package main

import (
	"fmt"
	"os"
	"runtime"
)

// version, commit, and date are set at build time via ldflags:
//
//	go build -ldflags "-X main.version=v1.0.0 -X main.commit=abc1234 -X main.date=2025-01-01" ./cmd/shitmail
//
// Default to "dev" / "unknown" when built without release tags.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// subcommand is a function that receives the remaining args after the verb.
type subcommand func(args []string)

var commands = map[string]subcommand{
	"serve":     runServe,
	"server":    runServe,
	"new":       runNew,
	"list":      runList,
	"ls":        runList,
	"read":      runRead,
	"cat":       runRead,
	"watch":     runWatch,
	"send":      runSend,
	"version":   runVersion,
	"health":    runHealth,
	"help":      runHelp,
	"-h":        runHelp,
	"--help":    runHelp,
	"-v":        runVersion,
	"--version": runVersion,
}

func main() {
	if len(os.Args) < 2 {
		// No subcommand — start the server (backward-compatible default).
		runServe(nil)
		return
	}

	verb := os.Args[1]
	if fn, ok := commands[verb]; ok {
		fn(os.Args[2:])
		return
	}

	// Unknown subcommand — check whether it looks like a flag meant for serve.
	if len(verb) > 0 && verb[0] == '-' {
		runServe(os.Args[1:])
		return
	}

	fmt.Fprintf(os.Stderr, "shitmail: unknown command %q\n\n", verb)
	printUsage()
	os.Exit(1)
}

func runVersion(_ []string) {
	fmt.Printf("shitmail %s\n", version)
	fmt.Printf("Go:       %s\n", runtime.Version())
	fmt.Printf("OS/Arch:  %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Commit:   %s\n", commit)
	fmt.Printf("Built:    %s\n", date)
	fmt.Printf("Source:   https://github.com/marc-schuetze/shitmail\n")
}

func runHelp(_ []string) {
	printUsage()
}

func printUsage() {
	fmt.Print(`shitmail — self-hosted disposable email (https://github.com/marc-schuetze/shitmail)

USAGE
  shitmail [command] [flags]

COMMANDS
  serve       Start the shitmail server (HTTP + SMTP + WebSocket)
  new         Create a temporary mailbox and print its address
  list        List emails in a mailbox
  read        Display a single email
  watch       Stream new emails in real time (WebSocket)
  send        Send a test email via SMTP
  version     Print version and runtime information
  help        Print this help

FLAGS (serve)
  --debug     Enable debug-level logging
  --verbose   Alias for --debug
  --config    Path to a .env file (default: looks for .env in CWD)

Run "shitmail <command> -h" for command-specific flags.

With no command given, "serve" is the default.

Environment: https://github.com/marc-schuetze/shitmail/blob/main/docs/configuration.md
`)
}
