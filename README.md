# Pay4MeProxy

HTTP proxy that retries x402 `402 Payment Required` responses using preconfigured payment options.

## Setup

1. Copy `.env.example` to `.env` and fill in an Algorand testnet mnemonic.
2. Provide a local `certs.json` containing `caCert` and `caKey` PEM strings.
3. Trust the CA certificate in the client/system that will use the proxy.

## Run

```bash
make run
```

The proxy listens on `:8089` by default. Override with `PROXY_ADDR`.

## Build

```bash
make build
```

The binary is written to `bin/payformeproxy`.

## Try it

Configure your client to use the proxy, then request:

```bash
curl -x http://localhost:8089 https://x402.goplausible.xyz/examples/weather
```

For HTTPS requests, the client must trust the CA from `certs.json`.
