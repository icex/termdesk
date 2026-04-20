package launcher

import (
	"reflect"
	"testing"
)

func TestTokenizeCommand(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"  ", nil},
		{"hello", []string{"hello"}},
		{"hello world", []string{"hello", "world"}},
		{"  hello   world  ", []string{"hello", "world"}},
		{`python -c "print('hello world')"`, []string{"python", "-c", "print('hello world')"}},
		{`echo 'single quoted'`, []string{"echo", "single quoted"}},
		{`ls -la /tmp`, []string{"ls", "-la", "/tmp"}},
		{`git commit -m "fix bug"`, []string{"git", "commit", "-m", "fix bug"}},
		// Escaped quote inside double quotes
		{`echo "say \"hi\""`, []string{"echo", `say "hi"`}},
		// Unmatched quote treats rest as token
		{`echo "unterminated`, []string{"echo", "unterminated"}},
		// Mixed quotes
		{`cmd "arg one" 'arg two' three`, []string{"cmd", "arg one", "arg two", "three"}},
		// Tab separator
		{"a\tb", []string{"a", "b"}},
	}
	for _, tt := range tests {
		got := TokenizeCommand(tt.input)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("TokenizeCommand(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
