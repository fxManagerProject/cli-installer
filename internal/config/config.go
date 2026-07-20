// Package config declares command parameters and turns command-line flags
// into (a) already-resolved values and (b) the parameters that still need an
// interactive choice from the user.
package config

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// Option is one selectable value for a parameter.
type Option struct {
	Value string // the value your logic will read (values[Key])
	Title string // label shown in the list (falls back to Value if empty)
	Desc  string // optional one-line description shown under the label
}

// Param describes a single, always-optional command parameter.
//
// Resolution order at runtime:
//
//	1. --<Flag> passed on the command line -> used as-is (validated against Options)
//	2. else if Default != ""                -> used silently
//	3. else if Prompt == true               -> the user is asked (list selector)
//	4. else                                 -> left as an empty string
type Param struct {
	Key     string   // key used to read the resolved value later (values[Key])
	Flag    string   // command-line flag name, without dashes
	Usage   string   // help text (-h) and the prompt title
	Default string   // default value; empty means "no default"
	Prompt  bool     // ask the user (list) when not provided and no default
	Options []Option // choices for the selector / accepted flag values
}

// Result is everything the UI needs after flags are parsed.
type Result struct {
	Values  map[string]string // resolved values (from flags + defaults)
	Prompts []Param           // parameters still needing an interactive choice
}

// Parse registers a flag per parameter, parses os.Args, and returns the split
// into resolved values and pending prompts.
func Parse(params []Param) (Result, error) {
	return ParseArgs(params, os.Args[1:])
}

// ParseArgs is Parse with an explicit argument slice (useful for tests).
func ParseArgs(params []Param, args []string) (Result, error) {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)

	raw := make(map[string]*string, len(params))
	for _, p := range params {
		help := p.Usage
		if len(p.Options) > 0 {
			help += " (" + joinValues(p.Options) + ")"
		}
		raw[p.Key] = fs.String(p.Flag, p.Default, help)
	}

	if err := fs.Parse(args); err != nil {
		return Result{}, err // includes flag.ErrHelp on -h
	}

	// Record which flags were explicitly set (vs. left at their default).
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	res := Result{Values: map[string]string{}}
	for _, p := range params {
		switch {
		case set[p.Flag]:
			v := *raw[p.Key]
			if err := validate(p, v); err != nil {
				return Result{}, err
			}
			res.Values[p.Key] = v
		case p.Default != "":
			res.Values[p.Key] = p.Default
		case p.Prompt:
			res.Prompts = append(res.Prompts, p)
		default:
			res.Values[p.Key] = ""
		}
	}
	return res, nil
}

func validate(p Param, v string) error {
	if len(p.Options) == 0 {
		return nil
	}
	for _, o := range p.Options {
		if o.Value == v {
			return nil
		}
	}
	return fmt.Errorf("invalid value %q for --%s (allowed: %s)", v, p.Flag, joinValues(p.Options))
}

func joinValues(opts []Option) string {
	vs := make([]string, len(opts))
	for i, o := range opts {
		vs[i] = o.Value
	}
	return strings.Join(vs, ", ")
}
