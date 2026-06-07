package proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/majed/payformeproxy/internal/algorand"
	"github.com/majed/payformeproxy/internal/wallets"
	"github.com/majed/payformeproxy/internal/x402"
)

type Config struct {
	Addr          string
	CertPath      string
	CAKeyPath     string
	Authenticator Authenticator
	Wallets       WalletProvider
}

type Authenticator interface {
	Authenticate(context.Context, string, string) (bool, error)
}

type WalletProvider interface {
	PrivateKeyForChain(context.Context, string) (string, error)
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
		if !authorized(ctx.Req, ctx, config.Authenticator) {
			return rejectConnect(), host
		}
		return mitm, host
	}
	proxy.OnRequest().HandleConnect(alwaysMITM)
	proxy.OnRequest().DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		if isMITMRequest(req) {
			return req, nil
		}
		if !authorized(req, ctx, config.Authenticator) {
			return req, proxyAuthRequired(req)
		}
		return req, nil
	})

	proxy.OnResponse(goproxy.StatusCodeIs(http.StatusPaymentRequired)).DoFunc(
		func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
			paidResp, err := payAndRetry(resp.Request, resp.Header.Get("Payment-Required"), config.Wallets)
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

func authorized(req *http.Request, ctx *goproxy.ProxyCtx, authenticator Authenticator) bool {
	if authenticator == nil {
		return false
	}
	username, password, ok := req.BasicAuth()
	if !ok {
		username, password, ok = parseBasicAuth(req.Header.Get("Proxy-Authorization"))
	}
	if !ok {
		return false
	}
	valid, err := authenticator.Authenticate(req.Context(), username, password)
	if err != nil {
		ctx.Warnf("proxy authentication error: %v", err)
		return false
	}
	return valid
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

func payAndRetry(req *http.Request, paymentRequiredHeader string, walletProvider WalletProvider) (*http.Response, error) {
	challenge, err := x402.DecodePaymentRequired(paymentRequiredHeader)
	if err != nil {
		return nil, err
	}

	accepted, err := x402.SelectAlgorandPayment(challenge)
	if err != nil {
		return nil, err
	}

	if walletProvider == nil {
		return nil, errors.New("wallet provider is not configured")
	}
	privateKey, err := walletProvider.PrivateKeyForChain(req.Context(), wallets.ChainAlgorand)
	if err != nil {
		return nil, err
	}
	if privateKey == "" {
		return nil, errors.New("create an algorand wallet in the admin UI before handling Algorand payments")
	}

	header, txID, err := algorand.BuildPaymentSignature(challenge, accepted, privateKey)
	if err != nil {
		return nil, err
	}

	retryReq := req.Clone(req.Context())
	retryReq.Header = req.Header.Clone()
	retryReq.Header.Set("PAYMENT-SIGNATURE", header)

	client := &http.Client{Timeout: 30 * time.Second}
	paidResp, err := client.Do(retryReq)
	if err != nil {
		return nil, err
	}

	log.Printf("Algorand payment transaction: %s", txID)
	if paymentResponse := paidResp.Header.Get("PAYMENT-RESPONSE"); paymentResponse != "" {
		log.Printf("PAYMENT-RESPONSE: %s", paymentResponse)
	}

	return paidResp, nil
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
