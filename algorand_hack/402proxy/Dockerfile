FROM debian:stable-slim

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY 402proxy/402proxy /app/402proxy
COPY certs.json /app/certs.json

EXPOSE 8089

ENTRYPOINT ["/app/402proxy"]