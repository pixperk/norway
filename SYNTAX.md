# Norway DSL Syntax

The Norway config DSL is a block-based language with four top-level constructs: `entrypoint`, `service`, `middleware`, and `route`. The entire DSL goes through a real compilation pipeline (lexer, parser, validator) and reports errors with exact line and column numbers.

## Table of contents

- [Lexical elements](#lexical-elements)
- [Top-level structure](#top-level-structure)
- [Entrypoints](#entrypoints)
- [Services](#services)
- [Middlewares](#middlewares)
- [Routes](#routes)
- [Validation rules](#validation-rules)
- [Complete example](#complete-example)
- [Common errors](#common-errors)

## Lexical elements

### Identifiers

Bare words. Used for names, paths, URLs, durations, and most values. Allowed characters: letters, digits, `_`, `-`, `.`, `:`, `/`, `*`. No quoting needed.

```
web
api.example.com
http://localhost:8001
/v1/*
10s
```

### Strings

Quoted with either `"..."` or `'...'`. Use when a value contains characters not allowed in identifiers (spaces, special chars).

```
"X-Powered-By: norway"
'norway/0.1'
```

### Numbers

Bare integer digits. Used for weights, rate limits, etc. Numbers may include a trailing letter suffix for durations (`10s`, `2s`, `500ms`); these are tokenized as a single number token.

```
100
50
3
10s
```

### Comments

Start with `#` and run to the end of the line.

```
# this is a comment
service api {  # trailing comment
    balance round-robin
}
```

### Whitespace

Newlines separate statements. Spaces and tabs within a line are ignored.

## Top-level structure

A config file contains zero or more of these block types in any order:

```
entrypoint <name> { ... }
service    <name> { ... }
middleware <name> { ... }
route      <name> { ... }
```

Names must be unique within their type (two entrypoints cannot share a name, but an entrypoint and a service can).

## Entrypoints

An entrypoint defines where Norway listens for incoming connections. Each entrypoint binds to one address.

### Plain HTTP

```
entrypoint web {
    listen :8080
}
```

### HTTPS (TLS termination)

```
entrypoint websecure {
    listen :8443
    tls {
        cert certs/cert.pem
        key  certs/key.pem
    }
}
```

### Directives

| Directive | Required | Type | Description |
|-----------|----------|------|-------------|
| `listen`  | yes | address | Address to bind, e.g. `:8080` or `127.0.0.1:8080` |
| `tls`     | no  | block | If present, the entrypoint terminates TLS |

Inside the `tls` block:

| Directive | Required | Description |
|-----------|----------|-------------|
| `cert`    | yes | Path to PEM-encoded certificate |
| `key`     | yes | Path to PEM-encoded private key |

## Services

A service is a pool of backend servers and a load balancing strategy. Optionally includes a health check config.

### Basic service

```
service api {
    server http://localhost:8001
    server http://localhost:8002
}
```

### Full service

```
service api {
    balance weighted

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
```

### Directives

| Directive | Required | Type | Description |
|-----------|----------|------|-------------|
| `balance` | no | identifier | Strategy: `round-robin`, `weighted`, or `least-conn`. Default is `round-robin`, auto-upgraded to `weighted` if any backend has weight greater than 1 |
| `health`  | no | block | Active health check config |
| `server`  | yes (1+) | URL [{block}] | Backend server. May have an optional `{ weight N }` block |

Inside the `health` block:

| Directive | Required | Type | Default | Description |
|-----------|----------|------|---------|-------------|
| `path`     | yes | path | -- | URL path to ping for health checks |
| `interval` | no  | duration | `10s` | How often to check |
| `timeout`  | no  | duration | `2s` | Per-check timeout |

Inside a `server` block (optional):

| Directive | Required | Type | Default | Description |
|-----------|----------|------|---------|-------------|
| `weight` | no | integer | `1` | Relative weight for the `weighted` strategy |

### Load balancing strategies

| Strategy      | Behavior |
|---------------|----------|
| `round-robin` | Atomic counter, each request goes to the next backend in rotation |
| `weighted`    | Backends with higher weight get proportionally more traffic. Weight 3 gets 3x weight 1 |
| `least-conn`  | Picks the backend with the fewest active connections |

All strategies skip unhealthy backends automatically.

## Middlewares

A middleware is a reusable request/response transformer. Defined once at the top level, attached per-route via `use`.

### Logging

```
middleware logger {
    type logging
    format json
}
```

| Key      | Required | Description |
|----------|----------|-------------|
| `type`   | yes | Must be `logging` |
| `format` | no  | Output format. Currently only `json` is supported |

Emits one structured JSON line per request to stdout with method, host, path, status, duration, bytes, client IP, user agent, and protocol.

### Rate limiting (token bucket)

```
middleware rate-limit {
    type ratelimit
    rate  100
    burst 50
}
```

| Key     | Required | Description |
|---------|----------|-------------|
| `type`  | yes | Must be `ratelimit` |
| `rate`  | no  | Tokens per second per client IP. Default `100` |
| `burst` | no  | Maximum bucket size before throttling. Default `50` |

Returns 429 with a `Retry-After` header when the bucket is empty. Buckets are per-IP and cleaned up after 10 minutes idle.

### Headers (inject and remove)

```
middleware add-headers {
    type headers
    set X-Proxy "norway"
    set X-Version "0.1"
    remove Server
}
```

| Key       | Required | Description |
|-----------|----------|-------------|
| `type`    | yes | Must be `headers` |
| `set`     | no, repeatable | `set <header-name> <value>`, adds a response header |
| `remove`  | no, repeatable | `remove <header-name>`, removes a response header |

### HTTPS redirect

```
middleware force-https {
    type https-redirect
    host example.com:8443
}
```

| Key    | Required | Description |
|--------|----------|-------------|
| `type` | yes | Must be `https-redirect` |
| `host` | no  | Target host for the redirect. If omitted, reuses the original request host |

Issues a `301 Moved Permanently` to the HTTPS equivalent of the request URL. Skips redirect if the request is already TLS.

## Routes

A route matches incoming requests against a host and path, then sends them to a service through a chain of middlewares.

```
route api {
    entrypoints web websecure
    host api.example.com
    path /v1/*
    service api
    use rate-limit
    use add-headers
    use logger
}
```

### Directives

| Directive     | Required | Type | Description |
|---------------|----------|------|-------------|
| `entrypoints` | yes | identifier(s) | One or more entrypoint names. Route only matches on listed entrypoints |
| `host`        | yes | hostname | Match the `Host` header (port is stripped automatically) |
| `path`        | no  | path | Path pattern to match. Supports static, `:param`, and `*wildcard` segments |
| `service`     | yes | identifier | Name of the service to forward to |
| `use`         | no, repeatable | identifier | Middleware to apply, in declaration order |

### Path matching

Paths are matched by a radix tree with three segment types:

| Type | Example | Match |
|------|---------|-------|
| Static | `/api/v1/users` | Exact match |
| Param  | `/users/:id` | Captures one segment as the named param |
| Wildcard | `/static/*filepath` | Captures everything remaining |

### Middleware order

Middlewares run in declaration order on the way in, reverse on the way out:

```
use logger      # outermost: sees request first, response last
use rate-limit
use headers     # innermost: sees request last, response first
```

The first `use` is the outermost wrap (closest to the client). The last `use` is the innermost (closest to the proxy).

## Validation rules

The validator runs after parsing and rejects configs that are syntactically valid but logically broken:

- Config must have at least one entrypoint, one service, and one route
- Names must be unique within their block type
- Every entrypoint must have a `listen` address
- A `tls` block must have both `cert` and `key`
- Every service must have at least one server
- `balance` must be one of `round-robin`, `weighted`, or `least-conn`
- Server weights must be non-negative
- Every route must reference an existing service
- Every route must list at least one entrypoint, all of which must exist
- Every middleware referenced via `use` must exist

Errors include line and column numbers from the original source.

## Complete example

```
# norway.conf

entrypoint web {
    listen :8080
}

entrypoint websecure {
    listen :8443
    tls {
        cert certs/cert.pem
        key  certs/key.pem
    }
}

service api {
    balance weighted

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
    set X-Version "0.1"
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
```

## Common errors

| Error                                              | Cause |
|----------------------------------------------------|-------|
| `unknown directive "servr"`                       | Typo in a directive name |
| `route "api": references unknown service "apii"`  | Name in `service` does not match any service block |
| `service "api": unknown balance strategy "rr"`    | Use `round-robin`, not `rr` |
| `entrypoint "web": tls missing cert path`         | `tls` block needs both `cert` and `key` |
| `duplicate middleware name "logger"`              | Two blocks of the same type cannot share a name |
| `config must have at least 1 route`               | Need at least one route to do anything useful |
