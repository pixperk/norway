package dsl

import "fmt"

type Parser struct {
	tokens []Token
	pos    int
}

func NewParser(tokens []Token) *Parser {
	return &Parser{tokens: tokens, pos: 0}
}

func (p *Parser) peek() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TOKEN_EOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) advance() Token {
	t := p.peek()
	p.pos++
	return t
}

func (p *Parser) expect(typ TokenType) (Token, error) {
	t := p.advance()
	if t.Type != typ {
		return t, fmt.Errorf("%d:%d: expected token type %d, got %d (%q)", t.Line, t.Col, typ, t.Type, t.Value)
	}
	return t, nil
}

func (p *Parser) skipNewlines() {
	for p.peek().Type == TOKEN_NEWLINE {
		p.advance()
	}
}

// Parse walks tokens and builds a Config from top-level blocks
func (p *Parser) Parse() (*Config, error) {
	cfg := &Config{}
	p.skipNewlines()

	for p.peek().Type != TOKEN_EOF {
		directive := p.peek()
		if directive.Type != TOKEN_IDENT {
			return nil, fmt.Errorf("%d:%d: expected directive, got %q", directive.Line, directive.Col, directive.Value)
		}

		switch directive.Value {
		case "entrypoint":
			ep, err := p.parseEntrypoint()
			if err != nil {
				return nil, err
			}
			cfg.Entrypoints = append(cfg.Entrypoints, ep)
		case "service":
			svc, err := p.parseService()
			if err != nil {
				return nil, err
			}
			cfg.Services = append(cfg.Services, svc)
		case "middleware":
			mw, err := p.parseMiddleware()
			if err != nil {
				return nil, err
			}
			cfg.Middlewares = append(cfg.Middlewares, mw)
		case "route":
			rt, err := p.parseRoute()
			if err != nil {
				return nil, err
			}
			cfg.Routes = append(cfg.Routes, rt)
		default:
			return nil, fmt.Errorf("%d:%d: unknown directive %q", directive.Line, directive.Col, directive.Value)
		}

		p.skipNewlines()
	}

	return cfg, nil
}

// entrypoint <name> { listen <addr> [tls { cert <path> key <path> }] }
func (p *Parser) parseEntrypoint() (Entrypoint, error) {
	p.advance() // consume "entrypoint"
	ep := Entrypoint{}

	name, err := p.expect(TOKEN_IDENT)
	if err != nil {
		return ep, fmt.Errorf("entrypoint: expected name: %w", err)
	}
	ep.Name = name.Value

	if _, err := p.expect(TOKEN_LBRACE); err != nil {
		return ep, err
	}
	p.skipNewlines()

	for p.peek().Type != TOKEN_RBRACE && p.peek().Type != TOKEN_EOF {
		key := p.advance()
		switch key.Value {
		case "listen":
			addr, err := p.expect(TOKEN_IDENT)
			if err != nil {
				return ep, fmt.Errorf("entrypoint %s: expected listen address: %w", ep.Name, err)
			}
			ep.Listen = addr.Value
		case "tls":
			tls, err := p.parseTLS(ep.Name)
			if err != nil {
				return ep, err
			}
			ep.TLS = &tls
		default:
			return ep, fmt.Errorf("%d:%d: unknown entrypoint directive %q", key.Line, key.Col, key.Value)
		}
		p.skipNewlines()
	}

	if _, err := p.expect(TOKEN_RBRACE); err != nil {
		return ep, fmt.Errorf("entrypoint %s: missing closing brace: %w", ep.Name, err)
	}
	return ep, nil
}

func (p *Parser) parseTLS(parent string) (TLSConfig, error) {
	tls := TLSConfig{}
	if _, err := p.expect(TOKEN_LBRACE); err != nil {
		return tls, err
	}
	p.skipNewlines()

	for p.peek().Type != TOKEN_RBRACE && p.peek().Type != TOKEN_EOF {
		key := p.advance()
		switch key.Value {
		case "cert":
			v, err := p.expect(TOKEN_IDENT)
			if err != nil {
				return tls, fmt.Errorf("tls in %s: expected cert path: %w", parent, err)
			}
			tls.CertPath = v.Value
		case "key":
			v, err := p.expect(TOKEN_IDENT)
			if err != nil {
				return tls, fmt.Errorf("tls in %s: expected key path: %w", parent, err)
			}
			tls.KeyPath = v.Value
		default:
			return tls, fmt.Errorf("%d:%d: unknown tls directive %q", key.Line, key.Col, key.Value)
		}
		p.skipNewlines()
	}

	if _, err := p.expect(TOKEN_RBRACE); err != nil {
		return tls, fmt.Errorf("tls in %s: missing closing brace: %w", parent, err)
	}
	return tls, nil
}

// service <name> { balance <strategy> [health { ... }] server <url> [{ weight <n> }] ... }
func (p *Parser) parseService() (Service, error) {
	p.advance() // consume "service"
	svc := Service{}

	name, err := p.expect(TOKEN_IDENT)
	if err != nil {
		return svc, fmt.Errorf("service: expected name: %w", err)
	}
	svc.Name = name.Value

	if _, err := p.expect(TOKEN_LBRACE); err != nil {
		return svc, err
	}
	p.skipNewlines()

	for p.peek().Type != TOKEN_RBRACE && p.peek().Type != TOKEN_EOF {
		key := p.advance()
		switch key.Value {
		case "balance":
			v, err := p.expect(TOKEN_IDENT)
			if err != nil {
				return svc, fmt.Errorf("service %s: expected balance strategy: %w", svc.Name, err)
			}
			svc.Balance = v.Value
		case "health":
			hc, err := p.parseHealth(svc.Name)
			if err != nil {
				return svc, err
			}
			svc.Health = &hc
		case "server":
			srv, err := p.parseServer(svc.Name)
			if err != nil {
				return svc, err
			}
			svc.Servers = append(svc.Servers, srv)
		default:
			return svc, fmt.Errorf("%d:%d: unknown service directive %q", key.Line, key.Col, key.Value)
		}
		p.skipNewlines()
	}

	if _, err := p.expect(TOKEN_RBRACE); err != nil {
		return svc, fmt.Errorf("service %s: missing closing brace: %w", svc.Name, err)
	}
	return svc, nil
}

func (p *Parser) parseHealth(parent string) (HealthCheckConfig, error) {
	hc := HealthCheckConfig{}
	if _, err := p.expect(TOKEN_LBRACE); err != nil {
		return hc, err
	}
	p.skipNewlines()

	for p.peek().Type != TOKEN_RBRACE && p.peek().Type != TOKEN_EOF {
		key := p.advance()
		switch key.Value {
		case "path":
			v, err := p.expect(TOKEN_IDENT)
			if err != nil {
				return hc, fmt.Errorf("health in %s: expected path: %w", parent, err)
			}
			hc.Path = v.Value
		case "interval":
			v, err := p.expect(TOKEN_NUMBER)
			if err != nil {
				return hc, fmt.Errorf("health in %s: expected interval: %w", parent, err)
			}
			hc.Interval = v.Value
		case "timeout":
			v, err := p.expect(TOKEN_NUMBER)
			if err != nil {
				return hc, fmt.Errorf("health in %s: expected timeout: %w", parent, err)
			}
			hc.Timeout = v.Value
		default:
			return hc, fmt.Errorf("%d:%d: unknown health directive %q", key.Line, key.Col, key.Value)
		}
		p.skipNewlines()
	}

	if _, err := p.expect(TOKEN_RBRACE); err != nil {
		return hc, fmt.Errorf("health in %s: missing closing brace: %w", parent, err)
	}
	return hc, nil
}

// server <url> or server <url> { weight <n> }
func (p *Parser) parseServer(parent string) (Server, error) {
	srv := Server{Weight: 1}

	url, err := p.expect(TOKEN_IDENT)
	if err != nil {
		return srv, fmt.Errorf("server in %s: expected url: %w", parent, err)
	}
	srv.URL = url.Value

	// optional block with weight
	if p.peek().Type == TOKEN_LBRACE {
		p.advance()
		p.skipNewlines()

		for p.peek().Type != TOKEN_RBRACE && p.peek().Type != TOKEN_EOF {
			key := p.advance()
			switch key.Value {
			case "weight":
				w, err := p.expect(TOKEN_NUMBER)
				if err != nil {
					return srv, fmt.Errorf("server %s: expected weight value: %w", srv.URL, err)
				}
				n := 0
				for _, c := range w.Value {
					n = n*10 + int(c-'0')
				}
				srv.Weight = n
			default:
				return srv, fmt.Errorf("%d:%d: unknown server directive %q", key.Line, key.Col, key.Value)
			}
			p.skipNewlines()
		}

		if _, err := p.expect(TOKEN_RBRACE); err != nil {
			return srv, fmt.Errorf("server %s: missing closing brace: %w", srv.URL, err)
		}
	}

	return srv, nil
}

// middleware <name> { type <t> <key> <value> ... }
func (p *Parser) parseMiddleware() (Middleware, error) {
	p.advance() // consume "middleware"
	mw := Middleware{Config: map[string]string{}}

	name, err := p.expect(TOKEN_IDENT)
	if err != nil {
		return mw, fmt.Errorf("middleware: expected name: %w", err)
	}
	mw.Name = name.Value

	if _, err := p.expect(TOKEN_LBRACE); err != nil {
		return mw, err
	}
	p.skipNewlines()

	for p.peek().Type != TOKEN_RBRACE && p.peek().Type != TOKEN_EOF {
		key := p.advance()
		if key.Value == "type" {
			v, err := p.expect(TOKEN_IDENT)
			if err != nil {
				return mw, fmt.Errorf("middleware %s: expected type value: %w", mw.Name, err)
			}
			mw.Type = v.Value
		} else {
			// generic key-value: accept ident, string, or number as value
			v := p.advance()
			if v.Type != TOKEN_IDENT && v.Type != TOKEN_STRING && v.Type != TOKEN_NUMBER {
				return mw, fmt.Errorf("%d:%d: expected value for %q, got %q", v.Line, v.Col, key.Value, v.Value)
			}
			mw.Config[key.Value] = v.Value
		}
		p.skipNewlines()
	}

	if _, err := p.expect(TOKEN_RBRACE); err != nil {
		return mw, fmt.Errorf("middleware %s: missing closing brace: %w", mw.Name, err)
	}
	return mw, nil
}

// route <name> { entrypoints <names...> host <h> path <p> service <s> use <mw> ... }
func (p *Parser) parseRoute() (Route, error) {
	p.advance() // consume "route"
	rt := Route{}

	name, err := p.expect(TOKEN_IDENT)
	if err != nil {
		return rt, fmt.Errorf("route: expected name: %w", err)
	}
	rt.Name = name.Value

	if _, err := p.expect(TOKEN_LBRACE); err != nil {
		return rt, err
	}
	p.skipNewlines()

	for p.peek().Type != TOKEN_RBRACE && p.peek().Type != TOKEN_EOF {
		key := p.advance()
		switch key.Value {
		case "entrypoints":
			// consume all idents on this line
			for p.peek().Type == TOKEN_IDENT {
				rt.Entrypoints = append(rt.Entrypoints, p.advance().Value)
			}
		case "host":
			v, err := p.expect(TOKEN_IDENT)
			if err != nil {
				return rt, fmt.Errorf("route %s: expected host: %w", rt.Name, err)
			}
			rt.Host = v.Value
		case "path":
			v, err := p.expect(TOKEN_IDENT)
			if err != nil {
				return rt, fmt.Errorf("route %s: expected path: %w", rt.Name, err)
			}
			rt.Path = v.Value
		case "service":
			v, err := p.expect(TOKEN_IDENT)
			if err != nil {
				return rt, fmt.Errorf("route %s: expected service name: %w", rt.Name, err)
			}
			rt.Service = v.Value
		case "use":
			v, err := p.expect(TOKEN_IDENT)
			if err != nil {
				return rt, fmt.Errorf("route %s: expected middleware name: %w", rt.Name, err)
			}
			rt.Middlewares = append(rt.Middlewares, v.Value)
		default:
			return rt, fmt.Errorf("%d:%d: unknown route directive %q", key.Line, key.Col, key.Value)
		}
		p.skipNewlines()
	}

	if _, err := p.expect(TOKEN_RBRACE); err != nil {
		return rt, fmt.Errorf("route %s: missing closing brace: %w", rt.Name, err)
	}
	return rt, nil
}
