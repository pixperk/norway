# norway

A focused, observable reverse proxy with a clean config DSL. An alternative to Traefik for people who don't need Kubernetes, service discovery, or a 100MB binary.

Norway is a single binary that reads a `.conf` file and proxies HTTP traffic to your backends with route matching, middleware chains, load balancing, and health checks. No YAML indentation hell, no 47-page documentation to read before you can route a request.

## Architecture

```mermaid
graph TB
    subgraph Clients
        C1[Client 1]
        C2[Client 2]
        C3[Client N]
    end

    subgraph Norway
        subgraph Entrypoints
            EP1[":80 HTTP"]
            EP2[":443 TLS"]
        end

        subgraph Core
            RM[Route Matcher<br/>radix tree per host]
            MC[Middleware Chain]
            LB[Load Balancer]
        end

        subgraph Config Engine
            DSL[DSL Parser<br/>lexer - parser - AST]
            VAL[Validator]
            HR[Hot Reloader]
        end

        subgraph Health
            HC[Health Checker]
        end
    end

    subgraph Backends
        B1[backend:8001]
        B2[backend:8002]
        B3[backend:9001]
    end

    C1 & C2 & C3 --> EP1 & EP2
    EP1 & EP2 --> RM
    RM --> MC
    MC --> LB
    LB --> B1 & B2 & B3
    HC -.->|poll| B1 & B2 & B3
    DSL --> VAL --> HR
    HR -.->|atomic swap| RM
```

## The Config DSL

Norway uses its own config language. No YAML, no TOML, no JSON. Just a clean block-based DSL that's purpose-built for proxy configuration.

The DSL goes through a full compilation pipeline: `text -> tokens -> AST -> config structs -> validation`. Errors report exact line and column numbers.

```nginx
# entrypoints define where norway listens
entrypoint web {
    listen :80
}

entrypoint websecure {
    listen :443
    tls {
        cert /etc/norway/cert.pem
        key  /etc/norway/key.pem
    }
}

# services define backend pools
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
    server http://localhost:8002
}

# middlewares are reusable across routes
middleware rate-limit {
    type ratelimit
    rate 100
    burst 50
}

middleware logger {
    type logging
    format json
}

# routes are the glue: match requests and send them to services
route api {
    entrypoints web websecure
    host api.example.com
    path /v1/*
    service api
    use rate-limit
    use logger
}
```

Four block types, three layers of abstraction:
- **Entrypoints** define where to listen
- **Services** define where to forward (backends + load balancing + health checks)
- **Routes** match requests (host + path) and connect entrypoints to services
- **Middlewares** transform requests/responses, reusable across routes

### DSL Pipeline

```mermaid
graph LR
    A["norway.conf<br/>(raw text)"] --> B["Lexer<br/>text -> tokens"]
    B --> C["Parser<br/>tokens -> AST"]
    C --> D["Validator<br/>semantic checks"]
    D --> E["Config structs<br/>ready to serve"]
```

The lexer tokenizes the raw text into typed tokens (idents, strings, numbers, braces, newlines). The parser consumes tokens and builds an AST of entrypoint/service/middleware/route nodes. The validator checks semantic correctness: do referenced services exist? Are there duplicate names? Is the balance strategy valid?

## Request Lifecycle

```mermaid
sequenceDiagram
    participant C as Client
    participant EP as Entrypoint
    participant R as Router
    participant MW as Middleware Chain
    participant LB as Load Balancer
    participant B as Backend

    C->>EP: GET api.example.com/v1/users
    EP->>R: Match host + path (radix tree)
    R->>MW: Matched route
    Note over MW: logging -> rate-limit -> headers
    MW->>LB: Pick backend
    Note over LB: round-robin / weighted / least-conn
    LB->>B: Forward via httputil.ReverseProxy
    B-->>LB: 200 OK
    LB-->>MW: Response
    Note over MW: Response walks back up the chain
    MW-->>EP: Response
    EP-->>C: 200 OK
```

## Progress

### Implemented
- [x] Custom DSL with lexer, parser, and AST
- [x] Config validation with semantic checks
- [x] Skeleton reverse proxy with `httputil.ReverseProxy`

### Coming Up
- [ ] Radix tree routing
- [ ] Middleware chain
- [ ] Load balancing + health checks
- [ ] Rate limiting + stats endpoint
- [ ] Dynamic config reload
- [ ] TLS termination
- [ ] TUI dashboard
