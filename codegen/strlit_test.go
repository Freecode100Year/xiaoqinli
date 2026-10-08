package codegen

import "testing"

func TestQuoting(t *testing.T) {
	cases := []struct{ got, want string }{
		{quoteRuby(`#{x}`), `"\#{x}"`},
		{quoteDollar(`${x}$y`), `"\${x}\$y"`},
		{quotePerl(`$x @y`), `"\$x \@y"`},
		{quoteTcl(`[exec id] $x`), `"\[exec id\] \$x"`},
		{quoteShell("$(id) `id` 'q'\n"), `$'$(id) ` + "`id`" + ` \'q\'\n'`},
		{quotePowerShell(`$(id) it's`), `'$(id) it''s'`},
		{quotePascal("a'b\nc"), `'a''b'#10'c'`},
		{quotePascal(""), `''`},
		{quoteFortran("a\nb"), `'a' // achar(10) // 'b'`},
		{commentLine("x\nimport os"), "x import os"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %s, want %s", c.got, c.want)
		}
	}
	if batchSafe("100%") == nil || batchSafe("a & b") == nil || batchSafe("plain text") != nil {
		t.Error("batchSafe")
	}
}
