// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate sh -c "(echo '// echo GENERATED; DO NOT EDIT'; echo; echo 'package parse; const specialHelpMessage=`'; sed -n '/^.) [a-z]/,/^$DOLLAR/s/^	//p' ../doc.go; echo '`') | gofmt >help.go"

package parse // import "robpike.io/ivy/parse"

import (
	"bufio"
	"fmt"
	"os"

	"robpike.io/ivy/config"
	"robpike.io/ivy/scan"
	"robpike.io/ivy/value"
)

const defaultFile = "save.ivy"

func (p *Parser) need(want ...scan.Type) scan.Token {
	tok := p.next()
	for _, w := range want {
		if tok.Type == w {
			return tok
		}
	}
	// Make the output look nice; usually there is only one item.
	if len(want) == 1 {
		p.errorf("expected %s, got %s", want[0], tok)
	}
	p.errorf("expected %s, got %s", want, tok)
	panic("not reached")
}

// nextDecimalNumber returns the next number, which
// must be a non-negative int32 (even though the result is of type int).
func (p *Parser) nextDecimalNumber() int {
	n64 := p.nextDecimalNumber64()
	if n64 != int64(int32(n64)) {
		p.errorf("value too large: %v", n64)
	}
	return int(n64)
}

// nextDecimalNumber64 returns the next number, which
// must fit in a non-negative int64.
func (p *Parser) nextDecimalNumber64() int64 {
	ibase, obase := p.config.Base()
	defer p.config.SetBase(ibase, obase)
	p.config.SetBase(10, obase)
	v, err := value.Parse(p.need(scan.Number).Text)
	if err != nil {
		p.errorf("%s", err)
	}
	var n int64 = -1
	switch num := v.(type) {
	case value.Int:
		n = int64(num)
	case value.BigInt:
		// The Int64 method produces undefined results if
		// the value is too large, so we must check.
		if num.Int.Sign() < 0 || num.Int.Cmp(value.MaxBigInt63) > 0 {
			p.errorf("value out of range: %v", v)
		}
		n = num.Int64()
	}
	if n < 0 {
		p.errorf("value must be a positive integer: %v", v)
	}
	return n
}

func truth(x bool) int {
	if x {
		return 1
	}
	return 0
}

func (p *Parser) special() {
	p.need(scan.RightParen)
Switch:
	switch text := p.need(scan.Identifier, scan.Op).Text; text {
	case "help":
		p.Println(specialHelpMessage)
		p.Println("More at: https://godoc.org/robpike.io/ivy")
	case "base", "ibase", "obase":
		ibase, obase := p.config.Base()
		if p.peek().Type == scan.Newline {
			p.Printf("ibase\t%d\n", ibase)
			p.Printf("obase\t%d\n", obase)
			break Switch
		}
		base := p.nextDecimalNumber()
		if base != 0 && (base < 2 || 36 < base) {
			p.errorf("illegal base %d", base)
		}
		switch text {
		case "base":
			ibase, obase = base, base
		case "ibase":
			ibase = base
		case "obase":
			obase = base
		}
		p.config.SetBase(ibase, obase)
		// We set the configuration in the scanner here, before it retrieves
		// the following newline. That means that any number it scans
		// at the beginning of the next line will happen after the config
		// has been updated.
	case "debug":
		if p.peek().Type == scan.Newline {
			for _, f := range config.DebugFlags {
				p.Printf("%s\t%d\n", f, truth(p.config.Debug(f)))
			}
			break Switch
		}
		name := p.need(scan.Identifier).Text
		if p.peek().Type == scan.Newline {
			// Toggle the value
			if !p.config.SetDebug(name, !p.config.Debug(name)) {
				p.Println("no such debug flag:", name)
			}
			if p.config.Debug(name) {
				p.Println("1")
			} else {
				p.Println("0")
			}
			break
		}
		number := p.nextDecimalNumber()
		if !p.config.SetDebug(name, number != 0) {
			p.Println("no such debug flag:", name)
		}
	case "format":
		if p.peek().Type == scan.Newline {
			p.Printf("%q\n", p.config.Format())
			break Switch
		}
		p.config.SetFormat(p.getString())
	case "get":
		if p.peek().Type == scan.Newline {
			p.runFromFile(p.context, defaultFile)
		} else {
			p.runFromFile(p.context, p.getString())
		}
	case "maxbits":
		if p.peek().Type == scan.Newline {
			p.Printf("%d\n", p.config.MaxBits())
			break Switch
		}
		max := p.nextDecimalNumber()
		p.config.SetMaxBits(uint(max))
	case "maxdigits":
		if p.peek().Type == scan.Newline {
			p.Printf("%d\n", p.config.MaxDigits())
			break Switch
		}
		max := p.nextDecimalNumber()
		p.config.SetMaxDigits(uint(max))
	case "op":
		name := p.need(scan.Operator, scan.Identifier).Text
		fn := p.context.UnaryFn[name]
		found := false
		if fn != nil {
			p.Println(fn)
			found = true
		}
		fn = p.context.BinaryFn[name]
		if fn != nil {
			p.Println(fn)
			found = true
		}
		if !found {
			p.errorf("%q not defined", name)
		}
	case "origin":
		if p.peek().Type == scan.Newline {
			p.Println(p.config.Origin())
			break Switch

		}
		origin := p.nextDecimalNumber()
		if origin != 0 && origin != 1 {
			p.errorf("illegal origin %d", origin)
		}
		p.config.SetOrigin(origin)
	case "prec":
		if p.peek().Type == scan.Newline {
			p.Printf("%d\n", p.config.FloatPrec())
			break Switch
		}
		prec := p.nextDecimalNumber()
		if prec == 0 || prec > 1e6 {
			p.errorf("illegal prec %d", prec) // TODO: make 0 be disable?
		}
		p.config.SetFloatPrec(uint(prec))
	case "prompt":
		if p.peek().Type == scan.Newline {
			p.Printf("%q\n", p.config.Format())
			break Switch
		}
		p.config.SetPrompt(p.getString())
	case "save":
		if p.peek().Type == scan.Newline {
			save(p.context, defaultFile, p.config)
		} else {
			save(p.context, p.getString(), p.config)
		}
	case "seed":
		if p.peek().Type == scan.Newline {
			p.Println(p.config.Origin())
			break Switch
		}
		p.config.SetRandomSeed(p.nextDecimalNumber64())
	default:
		p.errorf(")%s: not recognized", text)
	}
	p.need(scan.Newline, scan.EOF) // EOF lets this be in a newline-less string we evaluate.
}

// getString returns the value of the string that must be next in the input.
func (p *Parser) getString() string {
	return value.ParseString(p.need(scan.String).Text)
}

var runDepth = 0

// runFromFile executes the contents of the named file.
func (p *Parser) runFromFile(context value.Context, name string) {
	runDepth++
	if runDepth > 10 {
		p.errorf("get %q nested too deep", name)
	}
	defer func() {
		runDepth--
		err := recover()
		if err == nil {
			return
		}
		if err, ok := err.(value.Error); ok {
			fmt.Fprintf(p.config.ErrOutput(), "%s%s\n", p.Loc(), err)
			return
		}
		panic(err)
	}()
	fd, err := os.Open(name)
	if err != nil {
		p.errorf("%s", err)
	}
	scanner := scan.New(p.config, context, name, bufio.NewReader(fd))
	parser := NewParser(p.config, name, scanner, p.context)
	out := p.config.Output()
	for {
		value, ok := parser.Line()
		for _, val := range value {
			val = val.Eval(p.context)
			if val == nil {
				continue
			}
			fmt.Fprintf(out, "%v\n", val)
		}
		if !ok {
			return
		}
	}
}
