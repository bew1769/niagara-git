package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// version is the release string. The release workflow overrides it with -X.
var version = "0.1.0"

var errUsage = errors.New("usage")

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, errUsage) {
			os.Exit(2)
		}
		msg := err.Error()
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}
		fmt.Fprint(os.Stderr, msg)
		os.Exit(1)
	}
}

func run(args []string) error {
	format := 0
	var rest []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--version" || arg == "-version":
			fmt.Printf("niagara-git %s\n", version)
			return nil
		case arg == "--help" || arg == "-h" || arg == "help":
			usage(os.Stdout)
			return nil
		case arg == "--format":
			if i+1 >= len(args) {
				usage(os.Stderr)
				return errUsage
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil {
				usage(os.Stderr)
				return errUsage
			}
			format = n
		case strings.HasPrefix(arg, "--format="):
			n, err := strconv.Atoi(strings.TrimPrefix(arg, "--format="))
			if err != nil {
				usage(os.Stderr)
				return errUsage
			}
			format = n
		default:
			rest = append(rest, args[i:]...)
			i = len(args)
		}
	}
	if format != 0 && format != formatVersion {
		fmt.Fprintf(os.Stderr, "niagara-git: textconv format %d requested; this copy emits format %d. Re-run the installer.\n", format, formatVersion)
	}
	if len(rest) == 0 {
		usage(os.Stderr)
		return errUsage
	}
	switch rest[0] {
	case "bog", "dist":
		if len(rest) != 2 {
			usage(os.Stderr)
			return errUsage
		}
		return convertCommand(rest[0], rest[1])
	case "install":
		if len(rest) > 2 {
			usage(os.Stderr)
			return errUsage
		}
		repo := ""
		if len(rest) == 2 {
			repo = rest[1]
		}
		return install(repo)
	default:
		usage(os.Stderr)
		return errUsage
	}
}

func convertCommand(mode, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("niagara-git: file not found: %s\n", path)
	}
	var text string
	if mode == "bog" {
		text = convertBog(data)
	} else {
		text = convertDist(data)
	}
	_, err = os.Stdout.Write([]byte(text))
	return err
}

func usage(w io.Writer) {
	fmt.Fprintf(w, `niagara-git turns Niagara 4 station files into text for git diff.

Usage:
  niagara-git bog FILE
  niagara-git dist FILE
  niagara-git install [REPO]

bog and dist write a deterministic text view to stdout. install registers
those commands in a repository's local git config and merges the starter
.gitattributes and .gitignore templates.
`)
}
