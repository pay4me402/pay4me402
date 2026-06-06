package admin

import (
	"html/template"
	"log"
	"net/http"

	"github.com/majed/payformeproxy/internal/users"
)

type Server struct {
	addr     string
	users    *users.Service
	template *template.Template
}

func New(addr string, users *users.Service) *Server {
	return &Server{
		addr:     addr,
		users:    users,
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
	return http.ListenAndServe(s.addr, mux)
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	items, err := s.users.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.template.Execute(w, map[string]any{"Users": items}); err != nil {
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
  <h1>Proxy Users</h1>
  <form method="post" action="/users">
    <input name="username" placeholder="username" required>
    <input name="password" type="password" placeholder="password" required>
    <button type="submit">Add user</button>
  </form>
  <table>
    <thead><tr><th>Username</th><th>Created</th><th></th></tr></thead>
    <tbody>
      {{range .Users}}
      <tr>
        <td>{{.Username}}</td>
        <td>{{if .CreatedAt.Valid}}{{.CreatedAt.Time}}{{end}}</td>
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
</body>
</html>`
