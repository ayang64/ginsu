package main

import (
	"flag"
	"log"
	"os"
	"text/template"

	"github.com/ayang64/ginsu/parse"
)

func main() {
	expr := flag.String("t", "{{.}}", "template to parse for each log line")
	file := flag.String("f", "/dev/stdin", "path of file to parse")
	output := flag.String("o", "/dev/stdout", "path to send output")
	flag.Parse()

	inf, err := os.Open(*file)
	if err != nil {
		log.Fatal(err)
	}
	defer inf.Close()

	outf, err := os.OpenFile(*output, os.O_CREATE, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer outf.Close()

	p, err := parse.NewParser(parse.WithReader(inf))
	if err != nil {
		log.Fatal(err)
	}

	tmpl, err := template.New("x").Parse(*expr)
	if err != nil {
		log.Fatalf("could not parse template %q: %v", *expr, err)
	}

	for m := range p.Parse() {
		if len(m) == 0 {
			continue
		}
		tmpl.Execute(outf, m)
	}
}
