// Command termaid renders Mermaid diagram syntax as terminal art.
//
// Usage:
//
//	termaid [flags] [file]
//
// If no file is given and stdin is a pipe, input is read from stdin.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	termaid "github.com/termaid/termaid-go"
)

const version = "0.1.0"

func main() {
	ascii := flag.Bool("ascii", false, "Use ASCII characters instead of Unicode")
	paddingX := flag.Int("padding-x", 4, "Horizontal padding inside node boxes")
	paddingY := flag.Int("padding-y", 2, "Vertical padding inside node boxes")
	sharpEdges := flag.Bool("sharp-edges", false, "Use sharp corners on edge turns")
	showVersion := flag.Bool("version", false, "Print version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: termaid [flags] [file]\n\nRender Mermaid diagrams as terminal art.\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Printf("termaid %s\n", version)
		os.Exit(0)
	}

	input, err := readInput(flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "termaid: %v\n", err)
		os.Exit(1)
	}

	var opts []termaid.Option
	if *ascii {
		opts = append(opts, termaid.WithASCII())
	}
	if *paddingX != 4 || *paddingY != 2 {
		opts = append(opts, termaid.WithPadding(*paddingX, *paddingY))
	}
	if *sharpEdges {
		opts = append(opts, termaid.WithSharpEdges())
	}

	result := termaid.Render(input, opts...)
	fmt.Print(result)
}

// readInput returns the mermaid source from a file argument or stdin.
func readInput(args []string) (string, error) {
	if len(args) > 0 {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return "", fmt.Errorf("reading %s: %w", args[0], err)
		}
		return string(data), nil
	}

	// Check if stdin has data (piped input).
	info, err := os.Stdin.Stat()
	if err != nil {
		return "", fmt.Errorf("stat stdin: %w", err)
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		// stdin is a terminal, not a pipe — no input available.
		flag.Usage()
		os.Exit(1)
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("reading stdin: %w", err)
	}
	return string(data), nil
}
