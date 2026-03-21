package dsl

import (
	"testing"
)

func TestBasicTokens(t *testing.T) {
	tokens := NewLexer(`entrypoint web {
    listen :80
}`).Tokenize()

	expected := []struct {
		typ TokenType
		val string
	}{
		{TOKEN_IDENT, "entrypoint"},
		{TOKEN_IDENT, "web"},
		{TOKEN_LBRACE, "{"},
		{TOKEN_NEWLINE, "\n"},
		{TOKEN_IDENT, "listen"},
		{TOKEN_IDENT, ":80"},
		{TOKEN_NEWLINE, "\n"},
		{TOKEN_RBRACE, "}"},
		{TOKEN_EOF, ""},
	}

	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %+v", len(expected), len(tokens), tokens)
	}

	for i, exp := range expected {
		if tokens[i].Type != exp.typ || tokens[i].Value != exp.val {
			t.Errorf("token[%d]: expected {%d %q}, got {%d %q}", i, exp.typ, exp.val, tokens[i].Type, tokens[i].Value)
		}
	}
}

func TestStrings(t *testing.T) {
	tokens := NewLexer(`set X-Proxy "norway"
set X-ID '{{uuid}}'`).Tokenize()

	expected := []struct {
		typ TokenType
		val string
	}{
		{TOKEN_IDENT, "set"},
		{TOKEN_IDENT, "X-Proxy"},
		{TOKEN_STRING, "norway"},
		{TOKEN_NEWLINE, "\n"},
		{TOKEN_IDENT, "set"},
		{TOKEN_IDENT, "X-ID"},
		{TOKEN_STRING, "{{uuid}}"},
		{TOKEN_EOF, ""},
	}

	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %+v", len(expected), len(tokens), tokens)
	}

	for i, exp := range expected {
		if tokens[i].Type != exp.typ || tokens[i].Value != exp.val {
			t.Errorf("token[%d]: expected {%d %q}, got {%d %q}", i, exp.typ, exp.val, tokens[i].Type, tokens[i].Value)
		}
	}
}

func TestNumbers(t *testing.T) {
	tokens := NewLexer(`rate 100
interval 10s`).Tokenize()

	expected := []struct {
		typ TokenType
		val string
	}{
		{TOKEN_IDENT, "rate"},
		{TOKEN_NUMBER, "100"},
		{TOKEN_NEWLINE, "\n"},
		{TOKEN_IDENT, "interval"},
		{TOKEN_NUMBER, "10s"},
		{TOKEN_EOF, ""},
	}

	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %+v", len(expected), len(tokens), tokens)
	}

	for i, exp := range expected {
		if tokens[i].Type != exp.typ || tokens[i].Value != exp.val {
			t.Errorf("token[%d]: expected {%d %q}, got {%d %q}", i, exp.typ, exp.val, tokens[i].Type, tokens[i].Value)
		}
	}
}

func TestComments(t *testing.T) {
	tokens := NewLexer(`# this is a comment
entrypoint web {
    # another comment
    listen :80
}`).Tokenize()

	// comments should be skipped entirely
	expected := []struct {
		typ TokenType
		val string
	}{
		{TOKEN_IDENT, "entrypoint"},
		{TOKEN_IDENT, "web"},
		{TOKEN_LBRACE, "{"},
		{TOKEN_NEWLINE, "\n"},
		{TOKEN_IDENT, "listen"},
		{TOKEN_IDENT, ":80"},
		{TOKEN_NEWLINE, "\n"},
		{TOKEN_RBRACE, "}"},
		{TOKEN_EOF, ""},
	}

	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %+v", len(expected), len(tokens), tokens)
	}

	for i, exp := range expected {
		if tokens[i].Type != exp.typ || tokens[i].Value != exp.val {
			t.Errorf("token[%d]: expected {%d %q}, got {%d %q}", i, exp.typ, exp.val, tokens[i].Type, tokens[i].Value)
		}
	}
}

func TestFullConfig(t *testing.T) {
	input := `service api {
    balance round-robin
    server http://localhost:8001 {
        weight 3
    }
    server http://localhost:8002
}

route api {
    host api.example.com
    path /v1/*
    service api
    use logger
}`

	tokens := NewLexer(input).Tokenize()

	// just check key tokens exist and types are right
	idents := []string{}
	for _, tok := range tokens {
		if tok.Type == TOKEN_IDENT {
			idents = append(idents, tok.Value)
		}
	}

	want := []string{
		"service", "api", "balance", "round-robin",
		"server", "http://localhost:8001", "weight", "server", "http://localhost:8002",
		"route", "api", "host", "api.example.com", "path", "/v1/*",
		"service", "api", "use", "logger",
	}

	if len(idents) != len(want) {
		t.Fatalf("expected %d idents, got %d: %v", len(want), len(idents), idents)
	}

	for i, w := range want {
		if idents[i] != w {
			t.Errorf("ident[%d]: expected %q, got %q", i, w, idents[i])
		}
	}
}

func TestLineCol(t *testing.T) {
	tokens := NewLexer("entrypoint web {\n    listen :80\n}").Tokenize()

	// "entrypoint" should be line 1, col 1
	if tokens[0].Line != 1 || tokens[0].Col != 1 {
		t.Errorf("expected 1:1, got %d:%d", tokens[0].Line, tokens[0].Col)
	}

	// "listen" should be line 2
	for _, tok := range tokens {
		if tok.Value == "listen" {
			if tok.Line != 2 {
				t.Errorf("'listen' expected line 2, got line %d", tok.Line)
			}
			break
		}
	}

	// "}" should be line 3
	for _, tok := range tokens {
		if tok.Value == "}" {
			if tok.Line != 3 {
				t.Errorf("'}' expected line 3, got line %d", tok.Line)
			}
			break
		}
	}
}

func TestCollapseNewlines(t *testing.T) {
	tokens := NewLexer("a\n\n\n\nb").Tokenize()

	expected := []struct {
		typ TokenType
		val string
	}{
		{TOKEN_IDENT, "a"},
		{TOKEN_NEWLINE, "\n"},
		{TOKEN_IDENT, "b"},
		{TOKEN_EOF, ""},
	}

	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %+v", len(expected), len(tokens), tokens)
	}

	for i, exp := range expected {
		if tokens[i].Type != exp.typ {
			t.Errorf("token[%d]: expected type %d, got %d", i, exp.typ, tokens[i].Type)
		}
	}
}

func TestEmptyInput(t *testing.T) {
	tokens := NewLexer("").Tokenize()
	if len(tokens) != 1 || tokens[0].Type != TOKEN_EOF {
		t.Errorf("expected single EOF, got: %+v", tokens)
	}
}
