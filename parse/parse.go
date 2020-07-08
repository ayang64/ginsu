package parse

import (
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/ayang64/ginsu/lex"
)

type Parser struct {
	r   io.Reader
	log *log.Logger
}

func WithReader(r io.Reader) func(*Parser) error {
	return func(p *Parser) error {
		p.r = r
		return nil
	}
}

func WithLogger(lggr *log.Logger) func(*Parser) error {
	return func(p *Parser) error {
		p.log = lggr
		return nil
	}
}

func NewParser(opts ...func(*Parser) error) (*Parser, error) {
	parser := Parser{
		log: log.New(ioutil.Discard, "", 0),
		r:   os.Stdin,
	}

	for _, opt := range opts {
		if err := opt(&parser); err != nil {
			return nil, err
		}
	}
	return &parser, nil
}

func (p *Parser) Parse() <-chan map[string]interface{} {
	ch := make(chan map[string]interface{})
	go func() {
		err := p.parse(ch)
		if err != nil {
			p.log.Printf("err %v", err)
		}
		close(ch)
	}()
	return ch
}

func (p *Parser) parse(ch chan map[string]interface{}) error {
	lexer, err := lex.NewLexer(lex.WithReader(p.r), lex.WithLogger(p.log))
	if err != nil {
		return err
	}

	tokens := []lex.Token{}

	kvp := map[string]interface{}{}
	for tok := range lexer.Lex() {
		if tok.Type == lex.TokenWhiteSpace {
			continue // skip white space and unknown tokens.
		}

		tokens = append(tokens, tok)
		p.log.Printf("tokens: %v", tokens)

		// look at the last three tokens
		if len(tokens) >= 3 {
			cur := tokens[len(tokens)-3:]
			p.log.Printf(">>> TOP THREE TOKENS: %v", cur)
			// basically this is:
			//
			// kvp := ATOM '=' value
			// 				;
			//
			// value := QSTRING | ATOM
			//					;
			//
			// but way uglier.
			//
			if cur[0].Type == lex.TokenAtom && cur[1].Type == lex.TokenEqual && cur[2].Type == lex.TokenAtom {
				kvp[cur[0].Value.(string)] = cur[2].Value
				// shift token slice
				p.log.Printf("reducing tokens after parsing a key/value pair")
				p.log.Printf("kvp is now %#v", kvp)
				tokens = tokens[:0]
				continue
			}
		}
		if len(tokens) > 0 {
			cur := tokens[len(tokens)-1:]
			if curType := cur[0].Type; curType == lex.TokenNewLine || curType == lex.TokenError {
				// we've reached the end of the line
				p.log.Printf("SENDING KVP TO CALLER: %#v", kvp)
				ch <- kvp
				kvp = map[string]interface{}{}
				p.log.Printf("reducing tokens after parsing a newline")
				tokens = tokens[:len(tokens)-1]

				if curType == lex.TokenError {
					break
				}
				continue
			}

		}

		// if we're here, we should probably shift the tokens by 3
		if len(tokens) > 2 {
			tokens = tokens[len(tokens)-2:]
		}

	}

	return nil
}
