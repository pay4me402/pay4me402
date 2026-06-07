package admin

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/majed/payformeproxy/internal/db"
	"github.com/majed/payformeproxy/internal/users"
	"github.com/majed/payformeproxy/internal/userwallets"
	"github.com/majed/payformeproxy/internal/wallets"
)

const sessionCookieName = "payformeproxy_admin_session"

type Server struct {
	addr         string
	users        *users.Service
	wallets      *wallets.Service
	userWallets  *userwallets.Service
	queries      *db.Queries
	certPath     string
	adminUser    string
	adminPass    string
	sessionToken string
	templates    *template.Template
}

func New(addr string, users *users.Service, wallets *wallets.Service, userWallets *userwallets.Service, queries *db.Queries, certPath string, adminUser string, adminPass string) *Server {
	return &Server{
		addr:         addr,
		users:        users,
		wallets:      wallets,
		userWallets:  userWallets,
		queries:      queries,
		certPath:     certPath,
		adminUser:    adminUser,
		adminPass:    adminPass,
		sessionToken: newSessionToken(),
		templates:    template.Must(parseTemplates()),
	}
}

func (s *Server) Addr() string {
	return s.addr
}

func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	protected := s.requireAuth
	mux.Handle("GET /static/", http.StripPrefix("/static/", staticFileServer()))
	mux.HandleFunc("GET /ca.crt", s.serveCACert)
	mux.HandleFunc("GET /login", s.loginPage)
	mux.HandleFunc("POST /login", s.login)
	mux.HandleFunc("POST /logout", s.logout)
	mux.HandleFunc("GET /", protected(s.dashboardPage))
	mux.HandleFunc("GET /users", protected(s.usersPage))
	mux.HandleFunc("POST /users", protected(s.createUser))
	mux.HandleFunc("POST /users/delete", protected(s.deleteUser))
	mux.HandleFunc("GET /wallets", protected(s.walletsPage))
	mux.HandleFunc("POST /wallets", protected(s.createWallet))
	mux.HandleFunc("POST /wallets/delete", protected(s.deleteWallet))
	mux.HandleFunc("GET /access", protected(s.accessPage))
	mux.HandleFunc("POST /user-wallets", protected(s.createUserWallet))
	mux.HandleFunc("POST /user-wallets/budget", protected(s.updateUserWalletBudget))
	mux.HandleFunc("POST /user-wallets/delete", protected(s.deleteUserWallet))
	mux.HandleFunc("GET /transactions", protected(s.transactionsPage))
	return http.ListenAndServe(s.addr, mux)
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.authenticated(r) {
			next(w, r)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

func (s *Server) authenticated(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(s.sessionToken)) == 1
}

func (s *Server) validAdminCredentials(username string, password string) bool {
	if s.adminUser == "" || s.adminPass == "" {
		return false
	}
	userMatch := subtle.ConstantTimeCompare([]byte(username), []byte(s.adminUser)) == 1
	passMatch := subtle.ConstantTimeCompare([]byte(password), []byte(s.adminPass)) == 1
	return userMatch && passMatch
}

func (s *Server) serveCACert(w http.ResponseWriter, r *http.Request) {
	cert, err := os.ReadFile(s.certPath)
	if err != nil {
		http.Error(w, "CA certificate not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/x-x509-ca-cert")
	w.Header().Set("Content-Disposition", `attachment; filename="payformeproxy-ca.crt"`)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(cert); err != nil {
		log.Printf("serve CA cert: %v", err)
	}
}

func newSessionToken() string {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		panic(err)
	}
	return hex.EncodeToString(raw)
}
