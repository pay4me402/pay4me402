package admin

import (
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := s.users.Create(r.Context(), r.FormValue("username"), r.FormValue("password")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/users", http.StatusSeeOther)
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
	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

func (s *Server) createWallet(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	rpcEndpoint := strings.TrimSpace(r.FormValue("rpc_endpoint"))
	if customEndpoint := strings.TrimSpace(r.FormValue("custom_rpc_endpoint")); customEndpoint != "" {
		rpcEndpoint = customEndpoint
	}
	if rpcEndpoint == "custom" {
		rpcEndpoint = ""
	}
	if _, err := s.wallets.Create(r.Context(), r.FormValue("name"), r.FormValue("chain"), r.FormValue("private_key"), rpcEndpoint, r.FormValue("rpc_token")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/wallets", http.StatusSeeOther)
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
	http.Redirect(w, r, "/wallets", http.StatusSeeOther)
}

func (s *Server) createUserWallet(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var monthlyBudget *float64
	if raw := strings.TrimSpace(r.FormValue("monthly_budget")); raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			http.Error(w, "monthly budget must be a number", http.StatusBadRequest)
			return
		}
		monthlyBudget = &value
	}
	if _, err := s.userWallets.Assign(r.Context(), r.FormValue("user_id"), r.FormValue("wallet_id"), monthlyBudget); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/access", http.StatusSeeOther)
}

func (s *Server) updateUserWalletBudget(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var monthlyBudget *float64
	if raw := strings.TrimSpace(r.FormValue("monthly_budget")); raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			http.Error(w, "monthly budget must be a number", http.StatusBadRequest)
			return
		}
		monthlyBudget = &value
	}
	if _, err := s.userWallets.UpdateMonthlyBudget(r.Context(), r.FormValue("id"), monthlyBudget); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/access", http.StatusSeeOther)
}

func (s *Server) deleteUserWallet(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.userWallets.Delete(r.Context(), r.FormValue("id")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/access", http.StatusSeeOther)
}
