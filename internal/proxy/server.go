package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/majed/payformeproxy/internal/algorand"
	"github.com/majed/payformeproxy/internal/x402"
)

type Config struct {
	Addr      string
	CertPath  string
	CAKeyPath string
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
		return mitm, host
	}
	proxy.OnRequest().HandleConnect(alwaysMITM)

	proxy.OnResponse(goproxy.StatusCodeIs(http.StatusPaymentRequired)).DoFunc(
		func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
			paidResp, err := payAndRetry(resp.Request, resp.Header.Get("Payment-Required"))
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

func payAndRetry(req *http.Request, paymentRequiredHeader string) (*http.Response, error) {
	mnemonic := os.Getenv("ALGORAND_MNEMONIC")
	if mnemonic == "" {
		return nil, fmt.Errorf("set ALGORAND_MNEMONIC to the 25-word mnemonic for the paying Algorand account")
	}

	challenge, err := x402.DecodePaymentRequired(paymentRequiredHeader)
	if err != nil {
		return nil, err
	}

	accepted, err := x402.SelectAlgorandPayment(challenge)
	if err != nil {
		return nil, err
	}

	header, txID, err := algorand.BuildPaymentSignature(challenge, accepted, mnemonic)
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
