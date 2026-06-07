package admin

import "net/http"

func (s *Server) dashboardPage(w http.ResponseWriter, r *http.Request) {
	data, err := s.commonData(r, "Dashboard", "dashboard")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, r, "dashboard.html", data)
}

func (s *Server) usersPage(w http.ResponseWriter, r *http.Request) {
	data, err := s.commonData(r, "Users", "users")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, r, "users.html", data)
}

func (s *Server) walletsPage(w http.ResponseWriter, r *http.Request) {
	data, err := s.commonData(r, "Wallets", "wallets")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, r, "wallets.html", data)
}

func (s *Server) accessPage(w http.ResponseWriter, r *http.Request) {
	data, err := s.commonData(r, "Access Control", "access")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, r, "access.html", data)
}

func (s *Server) transactionsPage(w http.ResponseWriter, r *http.Request) {
	data, err := s.commonData(r, "Transactions", "transactions")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, r, "transactions.html", data)
}
