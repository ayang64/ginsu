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
	switch t {
	case TokenAtom:
		return "ATOM"
	case TokenEqual:
		return "EQUAL"
	case TokenError:
		return "ERROR"
	case TokenNewLine:
		return "NEWLINE"
	case TokenNumber:
		return "NUMBER"
	case TokenQuotedString:
		return "QUOTED-STRING"
	case TokenUnidentified:
		return "UNIDENTIFIED"
	case TokenWhiteSpace:
		return "WHITE-SPACE"
	default:
		return fmt.Sprintf("UNKNOWN-%d", t)
	}
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
// typically it is easiest to supply matchFunc() as a function literal so we
// can maintain state outside of the matchFunc().
//
func match(rs io.RuneScanner, matchFunc func(rune) (bool, bool)) (string, error) {
	lexeme := &strings.Builder{}
	var matchErr error
	for {
		r, _, err := rs.ReadRune()
		if err != nil {
			matchErr = err
			break
		}

		accept, cont := matchFunc(r)
		if accept {
			lexeme.WriteRune(r)
		}

		if !cont {
			if !accept {
				rs.UnreadRune()
			}
			break
		}
	}
	return lexeme.String(), matchErr
}

func matchToken(t TokenType, rs io.RuneScanner, matchFunc func(rune) (bool, bool)) (TokenType, string, error) {
	s, err := match(rs, matchFunc)
	return t, s, err
}

func (l *Lexer) ScanUnidentified() (TokenType, string, error) {
	return matchToken(TokenUnidentified, l.rs, func(r rune) (bool, bool) {
		v := r != '\n' && !unicode.IsSpace(r)
		return v, v
	})
}

func (l *Lexer) ScanQuotedString() (TokenType, string, error) {
	count := 0
	var endQuote rune
	var escaped bool
	return matchToken(TokenQuotedString, l.rs, func(r rune) (bool, bool) {
		count++
		if escaped {
			escaped = false
			return true, true
		}

		if count == 1 && (r == '"' || r == '\'') {
			endQuote = r
			// don't accept this rune but continue without error
			return false, true
		}

		// if it is an escape sentinel, continue but don't accept it
		if r == '\\' && !escaped {
			escaped = true
			return false, true
		}

		if r == endQuote {
			return false, false
		}
		return true, true
	})
}

func (l *Lexer) ScanNewLine() (TokenType, string, error) {
	return matchToken(TokenNewLine, l.rs, func(r rune) (bool, bool) {
		return r == '\n', false
	})
}

func (l *Lexer) ScanEqual() (TokenType, string, error) {
	return matchToken(TokenEqual, l.rs, func(r rune) (bool, bool) {
		return r == '=', false
	})
}

func (l *Lexer) ScanWhiteSpace() (TokenType, string, error) {
	return matchToken(TokenWhiteSpace, l.rs, func(r rune) (bool, bool) {
		v := r != '\n' && unicode.IsSpace(r)
		return v, v
	})
}

func (l *Lexer) ScanAtom() (TokenType, string, error) {
	count := 0
	return matchToken(TokenAtom, l.rs, func(r rune) (bool, bool) {
		count++
		v := (count == 1 && unicode.IsLetter(r)) || (count > 1 && (unicode.IsLetter(r) || unicode.IsPunct(r) || unicode.IsDigit(r)))
		return v, v
	})
}

func (l *Lexer) scan() (*Token, error) {
	classify := func() (TokenType, string, error) {
		r, err := l.peek()
		if err != nil {
			return TokenError, err.Error(), err
		}
		switch {
		case r == '\n':
			return l.ScanNewLine()
		case r == '\'' || r == '"':
			return l.ScanQuotedString()
		case unicode.IsSpace(r):
			return l.ScanWhiteSpace()
		case unicode.IsLetter(r):
			return l.ScanAtom()
		case r == '=':
			return l.ScanEqual()
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
