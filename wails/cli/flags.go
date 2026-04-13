package cli

import (
	"errors"
	"flag"
	"io"
)

type flags struct {
	Capture         bool
	Base64Stdout    bool
	InjectSVG       string
	OutputFormat    string
	CopyToClipboard bool
	Help            bool
	Version         bool
}

func parseFlags(args []string) (flags, error) {
	var f flags

	fs := flag.NewFlagSet("snapvector", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&f.Capture, "capture", false, "")
	fs.BoolVar(&f.Base64Stdout, "base64-stdout", false, "")
	fs.StringVar(&f.InjectSVG, "inject-svg", "", "")
	fs.StringVar(&f.OutputFormat, "output-format", "svg", "")
	fs.BoolVar(&f.CopyToClipboard, "copy-to-clipboard", false, "")
	fs.BoolVar(&f.Help, "help", false, "")
	fs.BoolVar(&f.Version, "version", false, "")

	if err := fs.Parse(args); err != nil {
		return f, err
	}

	switch {
	case f.Help, f.Version:
		return f, nil
	case !validOutputFormat(f.OutputFormat):
		return f, errors.New("--output-format must be one of svg, png, jpg, pdf")
	case f.Capture && f.InjectSVG != "":
		return f, errors.New("--capture and --inject-svg cannot be used together")
	case f.Capture && !f.Base64Stdout:
		return f, errors.New("--capture requires --base64-stdout")
	case !f.Capture && f.Base64Stdout:
		return f, errors.New("--base64-stdout requires --capture")
	case f.InjectSVG == "" && f.OutputFormat != "svg":
		return f, errors.New("--output-format is only valid with --inject-svg")
	case f.InjectSVG == "" && f.CopyToClipboard:
		return f, errors.New("--copy-to-clipboard is only valid with --inject-svg")
	}

	return f, nil
}

func validOutputFormat(format string) bool {
	switch format {
	case "svg", "png", "jpg", "pdf":
		return true
	default:
		return false
	}
}
