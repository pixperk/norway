package dsl

type TokenType int

/*
	 entrypoint web {
	    listen :80
	}

	entrypoint websecure {
	    listen :443
	    tls {
	        cert /path/to/cert.pem
	        key  /path/to/key.pem
	    }
	}

	service api {
	    balance round-robin

	    health {
	        path     /health
	        interval 10s
	        timeout  2s
	    }

	    server http://localhost:8001 {
	        weight 3
	    }
	    server http://localhost:8002 {
	        weight 1
	    }
	}

	service app {
	    balance least-conn
	    server http://localhost:3000
	}

	middleware rate-limit {
	    type ratelimit
	    rate  100
	    burst 50
	}

	middleware add-headers {
	    type headers
	    set X-Proxy "norway"
	    set X-Request-ID "{{uuid}}"
	    remove Server
	}

	middleware logger {
	    type logging
	    format json
	}

	route api {
	    entrypoints web websecure
	    host api.example.com
	    path /v1/*

	    service api
	    use rate-limit
	    use add-headers
	    use logger
	}

	route app {
	    entrypoints web
	    host app.example.com

	    service app
	    use logger
	}
*/
const (
	TOKEN_IDENT  TokenType = iota
	TOKEN_STRING           // "..." or '...'
	TOKEN_NUMBER

	TOKEN_LBRACE  // {
	TOKEN_RBRACE  // }
	TOKEN_NEWLINE // statement separator
	TOKEN_EOF
)

type Token struct {
	Type  TokenType
	Value string
	Line  int
	Col   int
}
