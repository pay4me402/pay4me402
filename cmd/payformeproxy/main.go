package main

import (
	"context"
	"database/sql"
	"log"
	"os"

	"github.com/majed/payformeproxy/internal/admin"
	"github.com/majed/payformeproxy/internal/db"
	"github.com/majed/payformeproxy/internal/env"
	"github.com/majed/payformeproxy/internal/proxy"
	"github.com/majed/payformeproxy/internal/users"
	"github.com/majed/payformeproxy/internal/userwallets"
	"github.com/majed/payformeproxy/internal/wallets"
	_ "modernc.org/sqlite"
)

func main() {
	if err := env.Load(".env"); err != nil {
		log.Fatal(err)
	}

	database, err := sql.Open("sqlite", env.Get("DATABASE_URL", "payformeproxy.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	if err := db.Migrate(context.Background(), database); err != nil {
		log.Fatal(err)
	}

	queries := db.New(database)
	userService := users.NewService(queries)
	walletService := wallets.NewService(queries)
	userWalletService := userwallets.NewService(queries)
	adminServer := admin.New(env.Get("ADMIN_ADDR", ":8090"), userService, walletService, userWalletService, queries)
	go func() {
		log.Printf("payformeproxy admin listening on %s", adminServer.Addr())
		if err := adminServer.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()

	server, err := proxy.New(proxy.Config{
		Addr:          env.Get("PROXY_ADDR", ":8089"),
		CertPath:      env.Get("CA_CERT_FILE", "certs/payformeproxy-ca.crt"),
		CAKeyPath:     env.Get("CA_KEY_FILE", "certs/payformeproxy-ca.key"),
		Authenticator: userService,
		Wallets:       walletService,
		Transactions:  queries,
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
