package lex

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strings"
	"unicode"
	"unicode/utf8"
)

type TokenType int

const (
	TokenAtom = TokenType(iota)
	TokenEqual
	TokenError
	TokenNewLine
	TokenNumber
	TokenQuotedString
	TokenWhiteSpace
	TokenUnidentified
)

func (t TokenType) String() string {
	m := map[TokenType]string{
		TokenAtom:         "ATOM",
		TokenEqual:        "EQUAL",
		TokenError:        "ERROR",
		TokenNewLine:      "NEWLINE",
		TokenNumber:       "NUMBER",
		TokenQuotedString: "QUOTED-STRING",
		TokenUnidentified: "UNIDENTIFIED",
		TokenWhiteSpace:   "WHITE-SPACE",
	}
	if s, ok := m[t]; ok {
		return s
	}
	return "UNKNOWN"
}

type Token struct {
	Type  TokenType
	Value interface{}
}

type Lexer struct {
	rs  io.RuneScanner
	log *log.Logger
}

func WithLogger(lggr *log.Logger) func(*Lexer) error {
	return func(l *Lexer) error {
		l.log = lggr
		return nil
	}
}

func runeScanner(r io.Reader) (io.RuneScanner, error) {
	if rs, isRuneScanner := r.(io.RuneScanner); isRuneScanner {
		return rs, nil
	}
	return bufio.NewReader(r), nil
}

func WithReader(r io.Reader) func(*Lexer) error {
	return func(l *Lexer) error {
		rs, err := runeScanner(r)
		if err != nil {
			return err
		}
		l.rs = rs
		return nil
	}
}

func NewLexer(opts ...func(*Lexer) error) (*Lexer, error) {
	lexer := Lexer{
		log: log.New(ioutil.Discard, "", 0),
	}
	for _, opt := range opts {
		if err := opt(&lexer); err != nil {
			return nil, err
		}
	}
	return &lexer, nil
}

func (l *Lexer) peek() (rune, error) {
	r, _, err := l.rs.ReadRune()
	l.rs.UnreadRune()
	if err != nil {
		return utf8.RuneError, err
	}
	return r, nil
}

// match() scans an io.RuneScanner and calls matchFunc() for every rune read.
// this is the core of this package.
//
// matchFunc() returns two bools:
//
//  accept - the rune scanned should be placed in the resulting output string
//  as part of the value associated with the token we're scanning.
//
//  continue - match() should continue scanning input.  another word for it is
//  "halt" -- that is, there are times when we'd like to continue scanning for
//  runes even if a particular rune isn't accepeted.  the best example is a
//  quoted string where we acknowledge that we want to continue scanning if we
//  encounter the first quote (therefore continue is true) but we don't want to
//  accept the quote rune as part of the value of the token (so accept will be
//  false).
//
//  error - indicates that we should break out of the loop AND place the most
//  recently scanned rune back into the stream.  this is distinct from
//  !continue since since it is possible to ask to not continue and not to
//  accept a rune without Unread()ing a rune.  The end quote character in a
//  quoted string is an example case.
//
// typically it is easiest to supply matchFunc() as a function literal so we
// can maintain state outside of the matchFunc().
//
func (l *Lexer) match(rs io.RuneScanner, matchFunc func(rune) (bool, bool, error)) (string, error) {
	lexeme := &strings.Builder{}
	var matchErr error
	for {
		r, _, err := rs.ReadRune()
		if err != nil {
			matchErr = err
			break
		}

		accept, cont, err := matchFunc(r)
		if accept {
			lexeme.WriteRune(r)
		}

		if err != nil {
			rs.UnreadRune()
			break
		}

		if !cont {
			break
		}
	}
	return lexeme.String(), matchErr
}

func (l *Lexer) matchToken(t TokenType, rs io.RuneScanner, matchFunc func(rune) (bool, bool, error)) (TokenType, string, error) {
	s, err := l.match(rs, matchFunc)
	return t, s, err
}

func (l *Lexer) ScanUnidentified() (TokenType, string, error) {
	return l.matchToken(TokenUnidentified, l.rs, func(r rune) (bool, bool, error) {
		v := r != '\n' && !unicode.IsSpace(r)
		if !v {
			return v, v, fmt.Errorf("%c is not part of an unidentified", r)
		}
		return v, v, nil
	})
}

func (l *Lexer) ScanQuotedString() (TokenType, string, error) {
	count := 0
	var endQuote rune
	var escaped bool
	return l.matchToken(TokenAtom, l.rs, func(r rune) (bool, bool, error) {
		count++
		if escaped {
			escaped = false
			return true, true, nil
		}

		if count == 1 && (r == '"' || r == '\'') {
			if r == '"' {
				l.log.Printf("HANDLING DOUBLE QUOTED STRING")
			} else {
				l.log.Printf("HANDLING SINGLE QUOTED STRING")
			}
			endQuote = r
			// don't accept this rune but continue without error
			return false, true, nil
		}

		// if it is an escape sentinel, continue but don't accept it
		if r == '\\' && !escaped {
			escaped = true
			return false, true, nil
		}

		if r == endQuote {
			l.log.Printf("GOT ENDING QUOTE RUNE (%c)", r)
			return false, false, nil
		}
		return true, true, nil
	})
}

func (l *Lexer) ScanNewLine() (TokenType, string, error) {
	return l.matchToken(TokenNewLine, l.rs, func(r rune) (bool, bool, error) {
		v := r == '\n'
		if !v {
			return v, false, fmt.Errorf("did not scan a newline")
		}
		return v, v, nil
	})
}

func (l *Lexer) ScanEqual() (TokenType, string, error) {
	return l.matchToken(TokenEqual, l.rs, func(r rune) (bool, bool, error) {
		v := r == '='
		if !v {
			return v, v, fmt.Errorf("did not scan an equal sign")
		}
		return v, v, nil

	})
}

func (l *Lexer) ScanWhiteSpace() (TokenType, string, error) {
	return l.matchToken(TokenWhiteSpace, l.rs, func(r rune) (bool, bool, error) {
		v := r != '\n' && unicode.IsSpace(r)
		if !v {
			return v, v, fmt.Errorf("%c is not a whitespace charater", r)
		}
		return v, v, nil
	})
}

func atomClass(r rune) bool {
	return r != '\n' && r != '=' && unicode.IsPrint(r) && !unicode.IsSpace(r)
}

func (l *Lexer) ScanAtom() (TokenType, string, error) {
	return l.matchToken(TokenAtom, l.rs, func(r rune) (bool, bool, error) {
		v := atomClass(r)
		if !v {
			return v, v, fmt.Errorf("%c is not in the atom class", r)
		}
		return v, v, nil
	})
}

func (l *Lexer) scan() (*Token, error) {
	classify := func() (TokenType, string, error) {
		r, err := l.peek()
		l.log.Printf("PEEKED AT %[1]c (%[1]d)", r)
		if err != nil {
			return TokenError, err.Error(), err
		}
		switch {
		case unicode.IsSpace(r) && r != '\n':
			return l.ScanWhiteSpace()
		case r == '\n':
			return l.ScanNewLine()
		case r == '\'' || r == '"':
			return l.ScanQuotedString()
		case r == '=':
			return l.ScanEqual()
		case atomClass(r):
			return l.ScanAtom()
		default:
			return l.ScanUnidentified()
		}
	}

	tokenType, value, err := classify()
	if err != nil {
		return &Token{Type: TokenError, Value: err}, err
	}
	return &Token{Type: tokenType, Value: value}, nil
}

func (l *Lexer) lex(tch chan<- Token) {
	for {
		val, err := l.scan()
		if err != nil {
			break
		}
		l.log.Printf("val: %q", val)
		tch <- *val
	}
}

func (l *Lexer) Lex() <-chan Token {
	tch := make(chan Token)
	go func() { l.lex(tch); close(tch) }()
	return tch
}
