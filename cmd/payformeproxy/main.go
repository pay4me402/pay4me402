package main

import (
	"log"
	"os"

	"github.com/majed/payformeproxy/internal/env"
	"github.com/majed/payformeproxy/internal/proxy"
)

func main() {
	if err := env.Load(".env"); err != nil {
		log.Fatal(err)
	}

	server, err := proxy.New(proxy.Config{
		Addr:      env.Get("PROXY_ADDR", ":8089"),
		CertPath:  env.Get("CA_CERT_FILE", "certs/payformeproxy-ca.crt"),
		CAKeyPath: env.Get("CA_KEY_FILE", "certs/payformeproxy-ca.key"),
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("payformeproxy listening on %s", server.Addr())
	if err := server.ListenAndServe(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
