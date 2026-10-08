package cli

import (
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const envJSONOut = "SHOULD_I_READ_JSON"

// wantJSON reports machine-readable stdout for the current command.
func wantJSON(cmd *cobra.Command) bool {
	if v := strings.ToLower(strings.TrimSpace(os.Getenv(envJSONOut))); v == "1" || v == "true" || v == "yes" {
		return true
	}
	if cmd == nil {
		return false
	}
	root := cmd.Root()
	if root == nil {
		return false
	}
	f := root.PersistentFlags().Lookup("json")
	if f == nil {
		return false
	}
	val, err := root.PersistentFlags().GetBool("json")
	return err == nil && val
}

func printJSON(w io.Writer, v any) error {
	const op = "cli.printJSON"
	if w == nil {
		w = os.Stdout
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "encode json")
	}
	return nil
}

func printPimalayaResult(cmd *cobra.Command, res *pimalaya.Result, accountHint string) error {
	if res == nil {
		return nil
	}
	w := cmd.OutOrStdout()
	if wantJSON(cmd) {
		return printPimalayaJSON(w, res)
	}
	text := formatPimalayaHuman(res, accountHint)
	_, err := io.WriteString(w, text)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, "cli.printPimalayaResult", "write human summary")
	}
	if !strings.HasSuffix(text, "\n") {
		_, _ = io.WriteString(w, "\n")
	}
	return nil
}

func printPimalayaJSON(w io.Writer, res *pimalaya.Result) error {
	if w == nil {
		w = os.Stdout
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if len(res.Raw) > 0 {
		return enc.Encode(json.RawMessage(res.Raw))
	}
	return enc.Encode(map[string]any{
		"ok":     res.OK,
		"exit":   res.Exit,
		"stderr": res.Stderr,
	})
}

func printPimalayaFailure(cmd *cobra.Command, opts *rootOptions, res *pimalaya.Result, err error, accountHint string) {
	if res == nil || wantJSON(cmd) {
		return
	}
	w := cmd.ErrOrStderr()
	text := formatPimalayaFailureHuman(res, err, accountHint)
	_, _ = io.WriteString(w, text)
	if !strings.HasSuffix(text, "\n") {
		_, _ = io.WriteString(w, "\n")
	}
	if opts != nil {
		opts.humanErr = true
	}
}
