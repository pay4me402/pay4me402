package proxy

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/majed/payformeproxy/internal/algorand"
	"github.com/majed/payformeproxy/internal/db"
	pfsolana "github.com/majed/payformeproxy/internal/solana"
	"github.com/majed/payformeproxy/internal/wallets"
	"github.com/majed/payformeproxy/internal/x402"
)

type Config struct {
	Addr          string
	CertPath      string
	CAKeyPath     string
	Authenticator Authenticator
	Wallets       WalletProvider
	Transactions  TransactionRecorder
	WalletAccess  WalletAccessController
}

type Authenticator interface {
	Authenticate(context.Context, string, string) (bool, error)
}

type UserAuthenticator interface {
	AuthenticateUser(context.Context, string, string) (db.ProxyUser, error)
}

type WalletProvider interface {
	PrivateKeyForChain(context.Context, string) (string, error)
}

type WalletSelector interface {
	WalletForChain(context.Context, string) (db.Wallet, error)
}

type TransactionRecorder interface {
	CreateTransaction(context.Context, db.CreateTransactionParams) (db.Transaction, error)
}

type WalletAccessController interface {
	GetUserWalletByUserAndWallet(context.Context, db.GetUserWalletByUserAndWalletParams) (db.UserWallet, error)
	ListTransactionsByUser(context.Context, string) ([]db.Transaction, error)
}

type contextKey string

const userContextKey contextKey = "proxy_user"

const (
	transactionStatusSuccess               = "success"
	transactionStatusBudgetExceeded        = "budget_exceeded"
	transactionStatusNoMatchingWalletFound = "no_matching_wallet_found"
	transactionStatusFailedUnknown         = "failed_unknown"
)

type paymentAccessError struct {
	status string
	err    error
}

func (e paymentAccessError) Error() string {
	return e.err.Error()
}

func (e paymentAccessError) Unwrap() error {
	return e.err
}

type Server struct {
	addr    string
	handler http.Handler
}

func New(config Config) (*Server, error) {
	certPEM, keyPEM, err := loadCAFiles(config.CertPath, config.CAKeyPath)
	if err != nil {
		return nil, err
	}

	cert, err := parseCA(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = true

	mitm := &goproxy.ConnectAction{Action: goproxy.ConnectMitm, TLSConfig: goproxy.TLSConfigFromCA(cert)}
	var alwaysMITM goproxy.FuncHttpsHandler = func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		user, ok := authenticate(ctx.Req, ctx, config.Authenticator)
		if !ok {
			return rejectConnect(), host
		}
		ctx.UserData = user
		return mitm, host
	}
	proxy.OnRequest().HandleConnect(alwaysMITM)
	proxy.OnRequest().DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		if isMITMRequest(req) {
			if user, ok := ctx.UserData.(db.ProxyUser); ok && user.ID != "" {
				req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
			}
			return req, nil
		}
		user, ok := authenticate(req, ctx, config.Authenticator)
		if !ok {
			return req, proxyAuthRequired(req)
		}
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
		return req, nil
	})

	proxy.OnResponse(goproxy.StatusCodeIs(http.StatusPaymentRequired)).DoFunc(
		func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
			paidResp, err := payAndRetry(resp.Request, resp.Header.Get("Payment-Required"), config.Wallets, config.Transactions, config.WalletAccess)
			if err != nil {
				ctx.Warnf("error handling 402 payment: %v", err)
				return resp
			}
			paidResp.Header.Set("X-402-Proxy", "true")
			return paidResp
		},
	)

	return &Server{addr: config.Addr, handler: proxy}, nil
}

func (s *Server) Addr() string {
	return s.addr
}

func (s *Server) ListenAndServe() error {
	return http.ListenAndServe(s.addr, s.handler)
}

func authenticate(req *http.Request, ctx *goproxy.ProxyCtx, authenticator Authenticator) (db.ProxyUser, bool) {
	if authenticator == nil {
		return db.ProxyUser{}, false
	}
	username, password, ok := req.BasicAuth()
	if !ok {
		username, password, ok = parseBasicAuth(req.Header.Get("Proxy-Authorization"))
	}
	if !ok {
		return db.ProxyUser{}, false
	}
	if userAuthenticator, ok := authenticator.(UserAuthenticator); ok {
		user, err := userAuthenticator.AuthenticateUser(req.Context(), username, password)
		if err != nil {
			ctx.Warnf("proxy authentication error: %v", err)
			return db.ProxyUser{}, false
		}
		return user, user.ID != ""
	}
	valid, err := authenticator.Authenticate(req.Context(), username, password)
	if err != nil {
		ctx.Warnf("proxy authentication error: %v", err)
		return db.ProxyUser{}, false
	}
	if !valid {
		return db.ProxyUser{}, false
	}
	return db.ProxyUser{ID: username, Username: username}, true
}

func isMITMRequest(req *http.Request) bool {
	return req.URL != nil && req.URL.Scheme == "https" && req.Header.Get("Proxy-Authorization") == ""
}

func parseBasicAuth(header string) (string, string, bool) {
	const prefix = "Basic "
	if len(header) < len(prefix) || header[:len(prefix)] != prefix {
		return "", "", false
	}
	req := &http.Request{Header: http.Header{"Authorization": []string{header}}}
	return req.BasicAuth()
}

func proxyAuthRequired(req *http.Request) *http.Response {
	return &http.Response{
		StatusCode: http.StatusProxyAuthRequired,
		Status:     "407 Proxy Authentication Required",
		Header: http.Header{
			"Proxy-Authenticate": []string{`Basic realm="payformeproxy"`},
			"Content-Type":       []string{"text/plain; charset=utf-8"},
		},
		Body:          http.NoBody,
		ContentLength: 0,
		Request:       req,
	}
}

func rejectConnect() *goproxy.ConnectAction {
	return &goproxy.ConnectAction{
		Action: goproxy.ConnectHijack,
		Hijack: func(req *http.Request, client net.Conn, ctx *goproxy.ProxyCtx) {
			_, _ = client.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic realm=\"payformeproxy\"\r\nContent-Length: 0\r\n\r\n"))
			_ = client.Close()
		},
	}
}

func payAndRetry(req *http.Request, paymentRequiredHeader string, walletProvider WalletProvider, transactionRecorder TransactionRecorder, walletAccess WalletAccessController) (*http.Response, error) {
	challenge, err := x402.DecodePaymentRequired(paymentRequiredHeader)
	if err != nil {
		return nil, err
	}

	if walletProvider == nil {
		return nil, errors.New("wallet provider is not configured")
	}

	accepted, chain, wallet, err := selectPayment(req.Context(), challenge, walletProvider, walletAccess)
	if err != nil {
		var accessErr paymentAccessError
		if errors.As(err, &accessErr) && wallet.ID != "" {
			if recordErr := recordTransaction(req.Context(), transactionRecorder, wallet.ID, challenge.Resource.URL, accepted.Amount, accessErr.status); recordErr != nil {
				log.Printf("record denied payment transaction: %v", recordErr)
			}
		}
		return nil, err
	}

	header, txID, err := buildPaymentSignature(req.Context(), challenge, accepted, chain, wallet.PrivateKey)
	if err != nil {
		if recordErr := recordTransaction(req.Context(), transactionRecorder, wallet.ID, challenge.Resource.URL, accepted.Amount, transactionStatusFailedUnknown); recordErr != nil {
			log.Printf("record failed payment transaction: %v", recordErr)
		}
		return nil, err
	}

	retryReq := req.Clone(req.Context())
	retryReq.Header = req.Header.Clone()
	retryReq.Header.Set("PAYMENT-SIGNATURE", header)

	client := &http.Client{Timeout: 30 * time.Second}
	paidResp, err := client.Do(retryReq)
	if err != nil {
		if recordErr := recordTransaction(req.Context(), transactionRecorder, wallet.ID, challenge.Resource.URL, accepted.Amount, transactionStatusFailedUnknown); recordErr != nil {
			log.Printf("record failed payment transaction: %v", recordErr)
		}
		return nil, err
	}

	if paymentResponse := paidResp.Header.Get("PAYMENT-RESPONSE"); paymentResponse != "" {
		logPaymentResponse(chain, paymentResponse)
	} else if txID != "" {
		log.Printf("%s payment transaction prepared: %s", chain, txID)
	} else {
		log.Printf("%s payment retry completed with status: %s", chain, paidResp.Status)
	}

	if paidResp.StatusCode >= 200 && paidResp.StatusCode < 300 {
		if err := recordTransaction(req.Context(), transactionRecorder, wallet.ID, challenge.Resource.URL, accepted.Amount, transactionStatusSuccess); err != nil {
			log.Printf("record payment transaction: %v", err)
		}
	} else if err := recordTransaction(req.Context(), transactionRecorder, wallet.ID, challenge.Resource.URL, accepted.Amount, transactionStatusFailedUnknown); err != nil {
		log.Printf("record failed payment transaction: %v", err)
	}

	return paidResp, nil
}

func selectPayment(ctx context.Context, challenge x402.Challenge, walletProvider WalletProvider, walletAccess WalletAccessController) (x402.PaymentOption, string, db.Wallet, error) {
	var supportedChains []string
	for _, option := range challenge.Accepts {
		chain, ok := paymentChain(option)
		if !ok {
			continue
		}
		supportedChains = append(supportedChains, chain)
		wallet, err := walletForChain(ctx, walletProvider, chain)
		if err != nil {
			return x402.PaymentOption{}, "", db.Wallet{}, err
		}
		if wallet.PrivateKey != "" {
			if err := authorizeWalletPayment(ctx, walletAccess, wallet.ID, option.Amount); err != nil {
				var accessErr paymentAccessError
				if errors.As(err, &accessErr) {
					return option, chain, wallet, err
				}
				return option, chain, wallet, paymentAccessError{status: transactionStatusFailedUnknown, err: err}
			}
			return option, chain, wallet, nil
		}
	}
	if len(supportedChains) == 0 {
		return x402.PaymentOption{}, "", db.Wallet{}, errors.New("Payment-Required header did not include a supported exact payment option")
	}
	return x402.PaymentOption{}, "", db.Wallet{}, fmt.Errorf("create a wallet for one of the accepted payment chains: %v", supportedChains)
}

func authorizeWalletPayment(ctx context.Context, walletAccess WalletAccessController, walletID string, amountValue string) error {
	if walletAccess == nil {
		return errors.New("wallet access controller is not configured")
	}
	user, ok := ctx.Value(userContextKey).(db.ProxyUser)
	if !ok || user.ID == "" {
		return errors.New("authenticated proxy user is missing from request context")
	}
	assignment, err := walletAccess.GetUserWalletByUserAndWallet(ctx, db.GetUserWalletByUserAndWalletParams{
		UserID:   user.ID,
		WalletID: walletID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return paymentAccessError{status: transactionStatusNoMatchingWalletFound, err: errors.New("user is not assigned to this wallet")}
	}
	if err != nil {
		return err
	}
	if !assignment.MonthlyBudget.Valid {
		return nil
	}
	amount, err := parsePaymentAmount(amountValue)
	if err != nil {
		return err
	}
	consumption, err := currentMonthConsumption(ctx, walletAccess, user.ID, walletID)
	if err != nil {
		return err
	}
	if consumption+amount > assignment.MonthlyBudget.Float64 {
		return paymentAccessError{status: transactionStatusBudgetExceeded, err: fmt.Errorf("monthly wallet budget exceeded: %.6f USDC used plus %.6f USDC payment exceeds %.6f USDC budget", consumption, amount, assignment.MonthlyBudget.Float64)}
	}
	return nil
}

func currentMonthConsumption(ctx context.Context, walletAccess WalletAccessController, userID string, walletID string) (float64, error) {
	transactions, err := walletAccess.ListTransactionsByUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	var total float64
	for _, transaction := range transactions {
		if transaction.WalletID != walletID || transaction.Status != transactionStatusSuccess || transaction.CreatedAt.Before(startOfMonth) {
			continue
		}
		total += transaction.Amount
	}
	return total, nil
}

func walletForChain(ctx context.Context, walletProvider WalletProvider, chain string) (db.Wallet, error) {
	if selector, ok := walletProvider.(WalletSelector); ok {
		return selector.WalletForChain(ctx, chain)
	}
	privateKey, err := walletProvider.PrivateKeyForChain(ctx, chain)
	if err != nil {
		return db.Wallet{}, err
	}
	if privateKey == "" {
		return db.Wallet{}, nil
	}
	return db.Wallet{ID: chain, Chain: chain, PrivateKey: privateKey}, nil
}

func paymentChain(option x402.PaymentOption) (string, bool) {
	if option.Scheme != "exact" {
		return "", false
	}
	switch {
	case x402.IsSolanaPayment(option):
		return wallets.ChainSolana, true
	case x402.IsAlgorandPayment(option):
		return wallets.ChainAlgorand, true
	default:
		return "", false
	}
}

func buildPaymentSignature(ctx context.Context, challenge x402.Challenge, accepted x402.PaymentOption, chain string, privateKey string) (string, string, error) {
	switch chain {
	case wallets.ChainSolana:
		return pfsolana.BuildPaymentSignature(ctx, challenge, accepted, privateKey)
	case wallets.ChainAlgorand:
		return algorand.BuildPaymentSignature(challenge, accepted, privateKey)
	default:
		return "", "", fmt.Errorf("unsupported payment chain %q", chain)
	}
}

func recordTransaction(ctx context.Context, recorder TransactionRecorder, walletID string, resource string, amountValue string, status string) error {
	if recorder == nil {
		return nil
	}
	user, ok := ctx.Value(userContextKey).(db.ProxyUser)
	if !ok || user.ID == "" {
		return errors.New("authenticated proxy user is missing from request context")
	}
	amount, err := parsePaymentAmount(amountValue)
	if err != nil {
		return err
	}
	_, err = recorder.CreateTransaction(ctx, db.CreateTransactionParams{
		ID:       newID(),
		UserID:   user.ID,
		WalletID: walletID,
		Resource: resource,
		Amount:   amount,
		Status:   status,
	})
	return err
}

func parsePaymentAmount(value string) (float64, error) {
	amount, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("parse payment amount for transaction record: %w", err)
	}
	return amount / 1_000_000, nil
}

func newID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		panic(err)
	}
	return hex.EncodeToString(raw)
}

type paymentResponseLog struct {
	Success     bool    `json:"success"`
	Transaction string  `json:"transaction"`
	Network     string  `json:"network"`
	Payer       string  `json:"payer"`
	ErrorReason *string `json:"errorReason"`
}

func logPaymentResponse(chain string, encoded string) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		log.Printf("%s payment response received", chain)
		return
	}
	var response paymentResponseLog
	if err := json.Unmarshal(raw, &response); err != nil {
		log.Printf("%s payment response received", chain)
		return
	}
	if response.Success {
		log.Printf("%s payment settled: transaction=%s network=%s payer=%s", chain, response.Transaction, response.Network, response.Payer)
		return
	}
	if response.ErrorReason != nil && *response.ErrorReason != "" {
		log.Printf("%s payment settlement failed: %s", chain, *response.ErrorReason)
		return
	}
	log.Printf("%s payment settlement failed", chain)
}

func loadCAFiles(certPath string, keyPath string) ([]byte, []byte, error) {
	cert, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read CA cert file: %w", err)
	}

	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read CA key file: %w", err)
	}

	return cert, key, nil
}

func parseCA(caCert, caKey []byte) (*tls.Certificate, error) {
	parsedCert, err := tls.X509KeyPair(caCert, caKey)
	if err != nil {
		return nil, err
	}
	if parsedCert.Leaf, err = x509.ParseCertificate(parsedCert.Certificate[0]); err != nil {
		return nil, err
	}
	return &parsedCert, nil
}
