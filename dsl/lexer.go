package dsl

type Lexer struct {
	input string
	pos   int
	line  int
	col   int
}

func NewLexer(input string) *Lexer {
	return &Lexer{
		input: input,
		pos:   0,
		line:  1,
		col:   1,
	}
}

func (l *Lexer) peek() byte {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *Lexer) advance() byte {
	cur := l.peek()
	if cur == 0 {
		return 0
	}
	l.pos++
	if cur == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return cur
}

func (l *Lexer) NextToken() Token {
	// skip spaces, tabs, and comments
	for {
		for l.peek() == ' ' || l.peek() == '\t' {
			l.advance()
		}
		if l.peek() == '#' {
			for l.peek() != '\n' && l.peek() != 0 {
				l.advance()
			}
			if l.peek() == '\n' {
				l.advance()
			}
			continue
		}
		break
	}

	// snapshot position before consuming the token
	startLine, startCol := l.line, l.col

	ch := l.peek()

	switch {
	case ch == 0:
		return Token{TOKEN_EOF, "", startLine, startCol}

	case ch == '\n':
		l.advance()
		// collapse multiple newlines into one
		for l.peek() == '\n' || l.peek() == ' ' || l.peek() == '\t' {
			l.advance()
		}
		return Token{TOKEN_NEWLINE, "\n", startLine, startCol}

	case ch == '{':
		l.advance()
		return Token{TOKEN_LBRACE, "{", startLine, startCol}

	case ch == '}':
		l.advance()
		return Token{TOKEN_RBRACE, "}", startLine, startCol}

	case ch == '"' || ch == '\'':
		quote := l.advance() // skip opening quote, remember which one
		start := l.pos
		for l.peek() != quote && l.peek() != 0 {
			l.advance()
		}
		value := l.input[start:l.pos]
		l.advance() // skip closing quote
		return Token{TOKEN_STRING, value, startLine, startCol}

	case ch >= '0' && ch <= '9':
		start := l.pos
		for c := l.peek(); (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z'); c = l.peek() {
			l.advance()
		}
		return Token{TOKEN_NUMBER, l.input[start:l.pos], startLine, startCol}

	default:
		// ident: greedily collect letters, digits, and _-.:/*
		start := l.pos
		for c := l.peek(); isIdentChar(c); c = l.peek() {
			l.advance()
		}
		return Token{TOKEN_IDENT, l.input[start:l.pos], startLine, startCol}
	}
}

func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '_' || c == '-' || c == '.' ||
		c == '/' || c == ':' || c == '*'
}

func (l *Lexer) Tokenize() []Token {
	var tokens []Token
	for {
		t := l.NextToken()
		tokens = append(tokens, t)
		if t.Type == TOKEN_EOF {
			break
		}
	}
	return tokens
}
