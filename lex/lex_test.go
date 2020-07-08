package lex

import (
	"io"
	"strings"
	"testing"
)

func TestLex(t *testing.T) {
	input := `key="value"`

	lexer, err := NewLexer(WithReader(strings.NewReader(input)))
	if err != nil {
		t.Fatal(err)
	}

	for tok := range lexer.Lex() {
		t.Logf("%v", tok)
	}
}

func TestScanEqual(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected string
	}{
		"Single Equal": {input: `=`, expected: `=`},
		"Multi Equal":  {input: `=====================`, expected: `=`},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			lexer, err := NewLexer(WithReader(strings.NewReader(test.input)))
			if err != nil {
				t.Fatal(err)
			}

			tt, s, err := lexer.ScanEqual()
			if err != nil && err != io.EOF {
				t.Fatal(err)
			}

			if tt != TokenEqual {
				t.Fatal("token type mismatch")
			}

			if got, expected := s, test.expected; got != expected {
				t.Fatalf(".ScanEqual() yielded %q; expected %q", got, expected)
			} else {
				t.Logf("expected %q and got %q", expected, got)
			}
		})
	}
}

func TestScanQuotedString(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected string
	}{
		"Multi Word, Double Quote":   {input: `"this is a test"`, expected: "this is a test"},
		"Multi Word, Single Quote":   {input: `'this is a test'`, expected: "this is a test"},
		"Multi Word, Escaped Quotes": {input: `'this \'is\' a test'`, expected: "this 'is' a test"},
		"Escaped Quotes":             {input: `'\'\'\'\'\''`, expected: `'''''`},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			lexer, err := NewLexer(WithReader(strings.NewReader(test.input)))
			if err != nil {
				t.Fatal(err)
			}

			_, s, err := lexer.ScanQuotedString()
			if err != nil && err != io.EOF {
				t.Fatal(err)
			}

			if got, expected := s, test.expected; got != expected {
				t.Fatalf(".ScanQuotedString() yielded %q; expected %q", got, expected)
			} else {
				t.Logf("expected %q and got %q", expected, got)
			}
		})
	}
}

func TestScanAtom(t *testing.T) {
	tests := map[string]struct {
		input     string
		expected  string
		shouldErr bool
	}{
		"Multi Word":   {input: `this is a test`, expected: `this`},
		"Single":       {input: `hello`, expected: `hello`},
		"Empty String": {input: ``, expected: ``},
		"All Numbers":  {input: `1234`, expected: `1234`, shouldErr: true},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			lexer, err := NewLexer(WithReader(strings.NewReader(test.input)))
			if err != nil {
				t.Fatal(err)
			}

			_, s, err := lexer.ScanAtom()
			if err != nil && test.shouldErr {
				return
			}

			if err != nil && err != io.EOF {
				t.Fatal(err)
			}

			if got, expected := s, test.expected; got != expected {
				t.Fatalf(".ScanAtom() yielded %q; expected %q", got, expected)
			} else {
				t.Logf("expected %q and got %q", expected, got)
			}
		})
	}
}
