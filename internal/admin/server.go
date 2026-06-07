package admin

import (
	"html/template"
	"log"
	"net/http"

	"github.com/majed/payformeproxy/internal/users"
	"github.com/majed/payformeproxy/internal/wallets"
)

type Server struct {
	addr     string
	users    *users.Service
	wallets  *wallets.Service
	template *template.Template
}

func New(addr string, users *users.Service, wallets *wallets.Service) *Server {
	return &Server{
		addr:     addr,
		users:    users,
		wallets:  wallets,
		template: template.Must(template.New("users").Parse(usersPage)),
	}
}

func (s *Server) Addr() string {
	return s.addr
}

func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.listUsers)
	mux.HandleFunc("POST /users", s.createUser)
	mux.HandleFunc("POST /users/delete", s.deleteUser)
	mux.HandleFunc("POST /wallets", s.createWallet)
	mux.HandleFunc("POST /wallets/delete", s.deleteWallet)
	return http.ListenAndServe(s.addr, mux)
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	items, err := s.users.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	walletItems, err := s.wallets.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.template.Execute(w, map[string]any{"Users": items, "Wallets": walletItems}); err != nil {
		log.Printf("render admin users page: %v", err)
	}
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := s.users.Create(r.Context(), r.FormValue("username"), r.FormValue("password")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.users.Delete(r.Context(), r.FormValue("id")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) createWallet(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := s.wallets.Create(r.Context(), r.FormValue("name"), r.FormValue("chain"), r.FormValue("private_key")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) deleteWallet(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.wallets.Delete(r.Context(), r.FormValue("id")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

const usersPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Proxy Users</title>
  <style>
    body { font-family: system-ui, sans-serif; max-width: 760px; margin: 40px auto; padding: 0 16px; color: #111827; }
    form { display: flex; gap: 8px; align-items: center; margin: 16px 0; }
    input { padding: 8px; border: 1px solid #d1d5db; border-radius: 6px; }
    button { padding: 8px 12px; border: 0; border-radius: 6px; background: #111827; color: white; cursor: pointer; }
    table { width: 100%; border-collapse: collapse; margin-top: 24px; }
    th, td { text-align: left; border-bottom: 1px solid #e5e7eb; padding: 10px 8px; }
    .danger { background: #dc2626; }
  </style>
</head>
<body>
  <h1>PayForMe Proxy Admin</h1>
  <h2>Proxy Users</h2>
  <form method="post" action="/users">
    <input name="username" placeholder="username" required>
    <input name="password" type="password" placeholder="password" required>
    <button type="submit">Add user</button>
  </form>
  <table>
    <thead><tr><th>Username</th><th>Created</th><th>Actions</th></tr></thead>
    <tbody>
      {{range .Users}}
      <tr>
        <td>{{.Username}}</td>
        <td>{{.CreatedAt}}</td>
        <td>
          <form method="post" action="/users/delete">
            <input type="hidden" name="id" value="{{.ID}}">
            <button class="danger" type="submit">Delete</button>
          </form>
        </td>
      </tr>
      {{else}}
      <tr><td colspan="3">No users yet.</td></tr>
      {{end}}
    </tbody>
  </table>
  <h2>Wallets</h2>
  <form method="post" action="/wallets">
    <input name="name" placeholder="wallet name" required>
    <select name="chain" required>
      <option value="algorand">algorand</option>
      <option value="solana">solana</option>
    </select>
    <input name="private_key" type="password" placeholder="private key / mnemonic" required>
    <button type="submit">Add wallet</button>
  </form>
  <table>
    <thead><tr><th>Name</th><th>Chain</th><th>Created</th><th>Actions</th></tr></thead>
    <tbody>
      {{range .Wallets}}
      <tr>
        <td>{{.Name}}</td>
        <td>{{.Chain}}</td>
        <td>{{.CreatedAt}}</td>
        <td>
          <form method="post" action="/wallets/delete">
            <input type="hidden" name="id" value="{{.ID}}">
            <button class="danger" type="submit">Delete</button>
          </form>
        </td>
      </tr>
      {{else}}
      <tr><td colspan="4">No wallets yet.</td></tr>
      {{end}}
    </tbody>
  </table>
</body>
</html>`
