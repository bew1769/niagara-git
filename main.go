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
	case "clean", "smudge":
		return filterStdio(rest[0], rest[1:])
	case "install":
		repo, storeXML, err := parseInstallArgs(rest[1:])
		if err != nil {
			usage(os.Stderr)
			return err
		}
		return install(repo, storeXML)
	case "snapshot":
		opts, err := parseSnapshotArgs(rest[1:])
		if err != nil {
			usage(os.Stderr)
			return err
		}
		return snapshot(opts)
	default:
		usage(os.Stderr)
		return errUsage
	}
}

func filterStdio(mode string, args []string) error {
	if len(args) > 1 {
		usage(os.Stderr)
		return errUsage
	}
	if len(args) == 1 && strings.HasSuffix(strings.ToLower(args[0]), ".dist") {
		return fmt.Errorf("niagara-git: %s does not store .dist archives; they stay zipped and are diffed with textconv\n", mode)
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	var out []byte
	if mode == "clean" {
		out, err = clean(data)
	} else {
		out, err = smudge(data)
	}
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(out)
	return err
}

func parseInstallArgs(args []string) (string, *bool, error) {
	var repo string
	var storeXML *bool
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--store-xml":
			if storeXML != nil && !*storeXML {
				return "", nil, errUsage
			}
			v := true
			storeXML = &v
		case "--no-store-xml":
			if storeXML != nil && *storeXML {
				return "", nil, errUsage
			}
			v := false
			storeXML = &v
		default:
			if strings.HasPrefix(args[i], "-") || repo != "" {
				return "", nil, errUsage
			}
			repo = args[i]
		}
	}
	return repo, storeXML, nil
}

func parseSnapshotArgs(args []string) (snapshotOpts, error) {
	var opts snapshotOpts
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--shared":
			opts.shared = true
		case "--push":
			opts.push = true
		case "--source", "--repo":
			if i+1 >= len(args) {
				return opts, errUsage
			}
			i++
			if args[i-1] == "--source" {
				opts.source = args[i]
			} else {
				opts.repo = args[i]
			}
		default:
			return opts, errUsage
		}
	}
	if opts.source == "" || opts.repo == "" {
		return opts, errUsage
	}
	return opts, nil
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
  niagara-git clean
  niagara-git smudge
  niagara-git install [--store-xml | --no-store-xml] [REPO]
  niagara-git snapshot --source PATH --repo PATH [--shared] [--push]

bog and dist write a deterministic text view to stdout. install registers
those commands in a repository's local git config and merges the starter
.gitattributes and .gitignore templates.

--store-xml is opt-in. It stores each .bog as text (every zip entry, in
order, with its original bytes) and rebuilds a zip on checkout. .dist files
stay zipped. snapshot copies config.bog, and with --shared the shared
folder, into a git repo and commits only when something changed.
`)
}
