package codegen

import (
	"fmt"
	"strings"
)

// A string literal is the one place user text reaches the output, and Go's %q
// only knows Go's escapes. In a language that interpolates inside double
// quotes, "$(id)", "#{`id`}" or "[exec id]" written through %q is not a string
// any more but a command, in a program the capability checker had just called
// pure. These quote for the language that will read them.

// quoteEscaping is %q with a backslash added before each rune in special.
// Every language using it reads `\c` as a literal c for those runes.
func quoteEscaping(s, special string) string {
	q := fmt.Sprintf("%q", s)
	var b strings.Builder
	for _, r := range q[1 : len(q)-1] {
		if strings.ContainsRune(special, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return `"` + b.String() + `"`
}

// Per-language quoting for double-quoted strings that interpolate.
func quoteRuby(s string) string   { return quoteEscaping(s, "#") }  // also Crystal, Elixir
func quoteDollar(s string) string { return quoteEscaping(s, "$") }  // PHP, Kotlin, Groovy, Dart, Julia
func quotePerl(s string) string   { return quoteEscaping(s, "$@") } //
func quoteTcl(s string) string    { return quoteEscaping(s, "$[]") }

// quoteShell renders s as a bash ANSI-C string, $'...', in which nothing is
// expanded and every byte can be written.
func quoteShell(s string) string {
	var b strings.Builder
	b.WriteString("$'")
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\' || c == '\'':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\t':
			b.WriteString(`\t`)
		case c < 0x20 || c == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteString("'")
	return b.String()
}

// quotePowerShell renders s as a verbatim single-quoted string. PowerShell
// also treats the typographic single quotes as quote characters, so they are
// doubled along with the ASCII one.
func quotePowerShell(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		switch r {
		case '\'', '‘', '’', '‚', '‛':
			b.WriteRune(r)
		}
		b.WriteRune(r)
	}
	b.WriteByte('\'')
	return b.String()
}

// quotePascal renders s as a Pascal string, writing control characters as
// #nn character constants between the quoted runs: 'a'#10'b'.
func quotePascal(s string) string {
	var b strings.Builder
	open := false
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			if open {
				b.WriteByte('\'')
				open = false
			}
			fmt.Fprintf(&b, "#%d", r)
			continue
		}
		if !open {
			b.WriteByte('\'')
			open = true
		}
		if r == '\'' {
			b.WriteByte('\'')
		}
		b.WriteRune(r)
	}
	if open || b.Len() == 0 {
		if !open {
			b.WriteByte('\'')
		}
		b.WriteByte('\'')
	}
	return b.String()
}

// quoteFortran renders s as Fortran character data, concatenating achar()
// for control characters, which a quoted Fortran string cannot hold.
func quoteFortran(s string) string {
	var parts []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			parts = append(parts, "'"+cur.String()+"'")
			cur.Reset()
		}
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			flush()
			parts = append(parts, fmt.Sprintf("achar(%d)", r))
			continue
		}
		if r == '\'' {
			cur.WriteByte('\'')
		}
		cur.WriteRune(r)
	}
	flush()
	if len(parts) == 0 {
		return "''"
	}
	return strings.Join(parts, " // ")
}

// batchSafe reports whether s can be written into a batch file as-is. cmd
// expands %VAR% and, under EnableDelayedExpansion, !VAR!, and & | < > ^ end or
// redirect a command; quoting rules differ between `set` and `echo`, so text
// that needs them is refused rather than escaped for one context and run in
// the other.
func batchSafe(s string) error {
	for _, r := range s {
		if r < 0x20 || r == 0x7f || strings.ContainsRune("%!&|<>^\"", r) {
			return fmt.Errorf("XQL_E402: Batch cannot safely hold the character %q in a string literal", r)
		}
	}
	return nil
}

// commentLine makes s safe to end a line comment with: a newline in a
// strategy tag set over the network would otherwise end the comment and put
// the rest of the tag into every program compiled afterwards.
func commentLine(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == '\u2028' || r == '\u2029' || r == '\u0085' {
			return ' '
		}
		return r
	}, s)
}
