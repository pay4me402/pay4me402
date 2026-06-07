# Pay4MeProxy

Pay4MeProxy is an HTTP proxy for x402-enabled services. It intercepts `402 Payment Required` responses, builds a supported payment signature from a configured wallet, retries the request, and records proxy activity in SQLite.

The project includes:

- **Forward proxy:** listens on `:8089` by default.
- **Admin UI:** listens on `:8090` by default.
- **Proxy users:** Basic-auth credentials for clients using the proxy.
- **Wallet management:** wallets used to pay x402 challenges.
- **Wallet access controls:** assign wallets and budgets to proxy users.
- **Transaction history:** records successful proxied payments.
- **MITM CA:** used so clients can proxy HTTPS traffic through Pay4MeProxy.

## ERD

![Pay4MeProxy ERD](docs/erd.png)

## Environment

Copy the example file:

```bash
cp .env.example .env
```

`.env.example` intentionally only contains admin credentials and SSL paths:

```env
ADMIN_USER=admin
ADMIN_PASSWORD=change-me
CA_CERT_FILE=certs/payformeproxy-ca.crt
CA_KEY_FILE=certs/payformeproxy-ca.key
```

Other settings are optional and have defaults in the application:

- **`PROXY_ADDR`:** proxy bind address, default `:8089`.
- **`ADMIN_ADDR`:** admin UI bind address, default `:8090`.
- **`DATABASE_URL`:** SQLite database path, default `payformeproxy.db`.

## Dependencies

Runtime/build dependencies:

- **Go 1.25**
- **SQLite**, via `modernc.org/sqlite`
- **sqlc**, invoked by `make generate`
- **OpenSSL**, used by the Docker image to generate the local proxy CA
- **Nginx**, recommended for public HTTPS routing to the admin UI
- **Certbot**, recommended for Let's Encrypt certificates on public web routes

Important Go libraries:

- **`github.com/elazarl/goproxy`:** HTTP proxy and HTTPS MITM handling.
- **`github.com/algorand/go-algorand-sdk/v2`:** Algorand payment signing.
- **`github.com/gagliardetto/solana-go`:** Solana support.
- **`golang.org/x/crypto`:** password hashing.
- **`modernc.org/sqlite`:** embedded SQLite driver.

## Build and run with Make

Generate sqlc code and run locally:

```bash
make run
```

Build the binary:

```bash
make build
```

Run tests:

```bash
make test
```

Format code:

```bash
make fmt
```

The built binary is written to:

```text
bin/payformeproxy
```

## Build and run with Docker

Build the image:

```bash
docker build -t payformeproxy .
```

Run with admin credentials only:

```bash
docker run --rm \
  --name payformeproxy \
  -p 8089:8089 \
  -p 8090:8090 \
  -e ADMIN_USER=admin \
  -e ADMIN_PASSWORD='change-me' \
  -v payformeproxy-data:/app/data \
  payformeproxy
```

If you want the SQLite database inside the mounted volume, pass `DATABASE_URL`:

```bash
docker run --rm \
  --name payformeproxy \
  -p 8089:8089 \
  -p 8090:8090 \
  -e ADMIN_USER=admin \
  -e ADMIN_PASSWORD='change-me' \
  -e DATABASE_URL=/app/data/payformeproxy.db \
  -v payformeproxy-data:/app/data \
  payformeproxy
```

Open the admin UI:

```text
http://localhost:8090
```

Then configure:

1. **Proxy users** for client proxy authentication.
2. **Wallets** for x402 payment signing.
3. **Access/budgets** if you want to restrict which users can spend from which wallets.

## Using the proxy

Configure a client to use:

```text
http://localhost:8089
```

Example with curl:

```bash
curl -x http://USERNAME:PASSWORD@localhost:8089 https://example.com
```

For HTTPS traffic, the client must trust the generated CA certificate:

```text
certs/payformeproxy-ca.crt
```

The Docker image generates a self-signed CA at build time. This CA is for proxy MITM only. It is not a public TLS certificate and is not replaced by Certbot.

## Architecture

```text
Client
  |
  | HTTP proxy / CONNECT
  v
Pay4MeProxy :8089
  |
  | Authenticates proxy user
  | Intercepts 402 Payment Required
  | Loads wallet and access rules from SQLite
  | Builds payment signature
  | Retries upstream request
  v
Upstream x402 service

Admin browser
  |
  v
Admin UI :8090
  |
  | manages users, wallets, access, transactions
  v
SQLite database
```

Main packages:

- **`cmd/payformeproxy`:** application entrypoint and service wiring.
- **`internal/proxy`:** proxy, auth enforcement, MITM handling, x402 retry flow.
- **`internal/admin`:** admin UI, login, forms, dashboards.
- **`internal/users`:** proxy user management and password verification.
- **`internal/wallets`:** wallet storage and chain lookup.
- **`internal/userwallets`:** wallet access and budget management.
- **`internal/db`:** SQLite models, migrations, and sqlc-generated queries.
- **`internal/algorand`:** Algorand payment signature construction.
- **`internal/solana`:** Solana payment support.
- **`internal/x402`:** x402 challenge parsing and payment option selection.

## GoDaddy DNS records

For this layout:

```text
pay402me.com        -> static site directory through Nginx
demo.pay402me.com   -> localhost:8090 through Nginx HTTPS
proxy.pay402me.com  -> localhost:8089, plain HTTP proxy
```

Add these DNS records in GoDaddy:

| Type | Name | Value |
| --- | --- | --- |
| A | `@` | your server public IPv4 |
| A | `demo` | your server public IPv4 |
| A | `proxy` | your server public IPv4 |

Optional, if your server has IPv6:

| Type | Name | Value |
| --- | --- | --- |
| AAAA | `@` | your server public IPv6 |
| AAAA | `demo` | your server public IPv6 |
| AAAA | `proxy` | your server public IPv6 |

You do not need a TLS certificate for `proxy.pay402me.com` if clients connect to it as a plain HTTP proxy.

## Nginx and Certbot walkthrough

The recommended public setup is:

- **`pay402me.com`:** static website served by Nginx.
- **`demo.pay402me.com`:** HTTPS reverse proxy to the admin UI on `localhost:8090`.
- **`proxy.pay402me.com`:** plain HTTP proxy pass to `localhost:8089`, or direct port `8089` access.

Install Nginx and Certbot on Ubuntu/Debian:

```bash
sudo apt update
sudo apt install -y nginx certbot python3-certbot-nginx
```

Create a static web root:

```bash
sudo mkdir -p /var/www/pay402me.com
echo 'pay402me.com' | sudo tee /var/www/pay402me.com/index.html
sudo chown -R www-data:www-data /var/www/pay402me.com
```

Create an Nginx site:

```bash
sudo nano /etc/nginx/sites-available/pay402me.com
```

Use this config:

```nginx
server {
    listen 80;
    server_name pay402me.com www.pay402me.com;

    root /var/www/pay402me.com;
    index index.html;

    location / {
        try_files $uri $uri/ =404;
    }
}

server {
    listen 80;
    server_name demo.pay402me.com;

    location / {
        proxy_pass http://127.0.0.1:8090;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

server {
    listen 80;
    server_name proxy.pay402me.com;

    location / {
        proxy_pass http://127.0.0.1:8089;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

Enable the site:

```bash
sudo ln -s /etc/nginx/sites-available/pay402me.com /etc/nginx/sites-enabled/pay402me.com
sudo nginx -t
sudo systemctl reload nginx
```

Issue Let's Encrypt certificates for the public web routes:

```bash
sudo certbot --nginx -d pay402me.com -d www.pay402me.com -d demo.pay402me.com
```

Do not include `proxy.pay402me.com` if you want it to remain plain HTTP.

Verify auto-renewal:

```bash
sudo certbot renew --dry-run
```

## Important SSL note

There are two separate certificate concepts:

- **Nginx/Certbot TLS certificates:** public HTTPS certificates for browser routes like `https://demo.pay402me.com`.
- **Pay4MeProxy CA certificate:** private CA used by the proxy to intercept HTTPS client traffic.

Certbot cannot replace the proxy CA. Clients using HTTPS through the proxy must still trust:

```text
certs/payformeproxy-ca.crt
```
