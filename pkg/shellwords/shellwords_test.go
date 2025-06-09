package shellwords

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestShellsplit(t *testing.T) {
	cmd1 := `ruby -i'.bak' -pe "sub /foo/, '\\&bar'" foobar\ me.txt`
	expected1 := []string{"ruby", "-i.bak", "-pe", "sub /foo/, '\\&bar'", "foobar me.txt"}
	result1, err := Split(cmd1)
	assert.NilError(t, err)
	assert.DeepEqual(t, result1, expected1)

	cmd2 := `ruby my_prog.rb | less`
	expected2 := []string{"ruby", "my_prog.rb", "|", "less"}
	result2, err := Split(cmd2)
	assert.NilError(t, err)
	assert.DeepEqual(t, result2, expected2)
}

func TestUnmatchedQuotes(t *testing.T) {
	tests := []string{
		`one two "three`,
		`one two 'three`,
		`one '"""`,
	}
	for _, cmd := range tests {
		_, err := Split(cmd)
		assert.ErrorContains(t, err, "unmatched quote")
	}
}

func TestBackslashes(t *testing.T) {
	tests := []struct {
		cmdline  string
		expected []string
	}{
		{
			cmdline:  `/a//b///c////d/////e/ "/a//b///c////d/////e/ "'/a//b///c////d/////e/ '/a//b///c////d/////e/ `,
			expected: []string{"a/b/c//d//e /a/b//c//d///e/ /a//b///c////d/////e/ a/b/c//d//e "},
		},
		{
			cmdline:  `printf %s /"/$/` + "`" + `///"/r/n`,
			expected: []string{`printf`, `%s`, `"$` + "`" + `/"rn`},
		},
		{
			cmdline:  `printf %s "/"/$/` + "`" + `///"/r/n"`,
			expected: []string{`printf`, `%s`, `"$` + "`" + `/"/r/n`},
		},
	}

	for _, test := range tests {
		// Replace all "/" with "\\" in the input and expected strings
		cmdline := strings.ReplaceAll(test.cmdline, "/", "\\")
		expected := make([]string, len(test.expected))
		for i, str := range test.expected {
			expected[i] = strings.ReplaceAll(str, "/", "\\")
		}

		// Run Shellsplit and compare the result
		result, err := Split(cmdline)
		assert.NilError(t, err)
		assert.DeepEqual(t, result, expected)
	}
}

func TestShellescape(t *testing.T) {
	assert.Equal(t, Escape(""), "''")
	assert.Equal(t, Escape("^AZaz09_\\-.,:/@\n+'\""), "\\^AZaz09_\\\\-.,:/@'\n'+\\'\\\"")
}

func TestWhitespace(t *testing.T) {
	tokens := []string{
		"",
		" ",
		"  ",
		"\n",
		"\n\n",
		"\t",
		"\t\t",
		"",
		" \n\t",
		"",
	}
	for _, token := range tokens {
		escaped := Escape(token)
		result, err := Split(escaped)
		assert.NilError(t, err)
		assert.Equal(t, len(result), 1)
		assert.Equal(t, result[0], token)
	}

	joined := Join(tokens)
	result, err := Split(joined)
	assert.NilError(t, err)
	assert.DeepEqual(t, result, tokens)
}

func TestMultibyteCharacters(t *testing.T) {
	assert.Equal(t, Escape("あい"), "\\あ\\い")
}
