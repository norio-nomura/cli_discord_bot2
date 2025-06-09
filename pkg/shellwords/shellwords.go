// Package shellwords provides utilities for splitting and escaping shell command strings.
package shellwords

import (
	"fmt"
	"regexp"
	"strings"
)

// Split splits a string into an array of tokens in the same way the UNIX Bourne shell does.
// It returns an error if quotes are unmatched.
func Split(input string) ([]string, error) {
	var words []string
	field := ""

	re := regexp.MustCompile(`\s*(?:([^\s\\\'\"]+)|'([^\']*)'|"((?:[^\"\\]|\\.)*)"|(\\.?)|(\S))(\s|$)?`)
	reDq := regexp.MustCompile("\\\\([$`\"\\\\\n])")
	reEsc := regexp.MustCompile(`\\(.)`)
	matches := re.FindAllStringSubmatchIndex(input, -1)
	for _, match := range matches {
		slice := make([]*string, len(match)/2)
		for j := range slice {
			if match[j*2] >= 0 {
				capture := input[match[2*j]:match[2*j+1]]
				slice[j] = &capture
			}
		}
		word, sq, dq, esc, garbage, sep := slice[1], slice[2], slice[3], slice[4], slice[5], slice[6]

		if garbage != nil {
			return nil, fmt.Errorf("unmatched quote: `%s`", input)
		}

		var token string
		switch {
		case word != nil:
			token = *word
		case sq != nil:
			token = *sq
		case dq != nil:
			token = reDq.ReplaceAllString(*dq, `$1`)
		case esc != nil:
			token = reEsc.ReplaceAllString(*esc, `$1`)
		}
		field += token

		if sep != nil {
			words = append(words, field)
			field = ""
		}
	}

	return words, nil
}

// Escape escapes a string so that it can be safely used in a Bourne shell command line.
func Escape(input string) string {
	if input == "" {
		return "''"
	}
	re := regexp.MustCompile("[^-A-Za-z0-9_.,:+/@\n]")
	escaped := re.ReplaceAllString(input, `\$0`)
	return strings.ReplaceAll(escaped, "\n", "'\n'")
}

// Join builds a command line string from an argument list.
func Join(inputs []string) string {
	escaped := make([]string, len(inputs))
	for i, input := range inputs {
		escaped[i] = Escape(input)
	}
	return strings.Join(escaped, " ")
}
