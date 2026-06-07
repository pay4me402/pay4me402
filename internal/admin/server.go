package admin

import (
	"crypto/subtle"
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/majed/payformeproxy/internal/db"
	"github.com/majed/payformeproxy/internal/users"
	"github.com/majed/payformeproxy/internal/userwallets"
	"github.com/majed/payformeproxy/internal/wallets"
)

type Server struct {
	addr        string
	users       *users.Service
	wallets     *wallets.Service
	userWallets *userwallets.Service
	queries     *db.Queries
	certPath    string
	adminUser   string
	adminPass   string
	template    *template.Template
}

func New(addr string, users *users.Service, wallets *wallets.Service, userWallets *userwallets.Service, queries *db.Queries, certPath string, adminUser string, adminPass string) *Server {
	return &Server{
		addr:        addr,
		users:       users,
		wallets:     wallets,
		userWallets: userWallets,
		queries:     queries,
		certPath:    certPath,
		adminUser:   adminUser,
		adminPass:   adminPass,
		template: template.Must(template.New("users").Funcs(template.FuncMap{
			"monthlyBudget": formatMonthlyBudget,
			"budgetValue":   budgetValue,
			"consumption":   consumption,
		}).Parse(usersPage)),
	}
}

func (s *Server) Addr() string {
	return s.addr
}

func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	protected := s.requireAuth
	mux.HandleFunc("GET /", protected(s.listUsers))
	mux.HandleFunc("GET /ca.crt", s.serveCACert)
	mux.HandleFunc("POST /users", protected(s.createUser))
	mux.HandleFunc("POST /users/delete", protected(s.deleteUser))
	mux.HandleFunc("POST /wallets", protected(s.createWallet))
	mux.HandleFunc("POST /wallets/delete", protected(s.deleteWallet))
	mux.HandleFunc("POST /user-wallets", protected(s.createUserWallet))
	mux.HandleFunc("POST /user-wallets/budget", protected(s.updateUserWalletBudget))
	mux.HandleFunc("POST /user-wallets/delete", protected(s.deleteUserWallet))
	return http.ListenAndServe(s.addr, mux)
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.validAdminCredentials(r) {
			next(w, r)
			return
		}
		w.Header().Set("WWW-Authenticate", `Basic realm="payformeproxy admin"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}
}

func (s *Server) validAdminCredentials(r *http.Request) bool {
	username, password, ok := r.BasicAuth()
	if !ok || s.adminUser == "" || s.adminPass == "" {
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
	allTransactions, transactionItems, err := s.filteredTransactions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	userWalletItems, err := s.userWallets.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	currentMonthConsumption := consumptionByUserWallet(allTransactions)
	if err := s.template.Execute(w, map[string]any{
		"Users":            items,
		"Wallets":          walletItems,
		"UserWallets":      userWalletItems,
		"Consumption":      currentMonthConsumption,
		"Transactions":     transactionItems,
		"SelectedUserID":   r.URL.Query().Get("user_id"),
		"SelectedWalletID": r.URL.Query().Get("wallet_id"),
		"SelectedResource": r.URL.Query().Get("resource_base_url"),
		"ResourceBaseURLs": resourceBaseURLs(allTransactions),
		"UsernamesByID":    usernamesByID(items),
		"WalletNamesByID":  walletNamesByID(walletItems),
	}); err != nil {
		log.Printf("render admin users page: %v", err)
	}
}

func (s *Server) filteredTransactions(r *http.Request) ([]db.Transaction, []db.Transaction, error) {
	if s.queries == nil {
		return nil, nil, nil
	}
	transactions, err := s.queries.ListTransactions(r.Context())
	if err != nil {
		return nil, nil, err
	}
	userID := r.URL.Query().Get("user_id")
	walletID := r.URL.Query().Get("wallet_id")
	resourceBaseURL := r.URL.Query().Get("resource_base_url")
	filtered := transactions[:0]
	for _, transaction := range transactions {
		if userID != "" && transaction.UserID != userID {
			continue
		}
		if walletID != "" && transaction.WalletID != walletID {
			continue
		}
		if resourceBaseURL != "" && baseURL(transaction.Resource) != resourceBaseURL {
			continue
		}
		filtered = append(filtered, transaction)
	}
	return transactions, filtered, nil
}

func usernamesByID(users []db.ProxyUser) map[string]string {
	values := make(map[string]string, len(users))
	for _, user := range users {
		values[user.ID] = user.Username
	}
	return values
}

func walletNamesByID(wallets []db.Wallet) map[string]string {
	values := make(map[string]string, len(wallets))
	for _, wallet := range wallets {
		values[wallet.ID] = wallet.Name
	}
	return values
}

func resourceBaseURLs(transactions []db.Transaction) []string {
	seen := map[string]bool{}
	var values []string
	for _, transaction := range transactions {
		value := baseURL(transaction.Resource)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		values = append(values, value)
	}
	return values
}

func baseURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimSpace(raw)
	}
	return parsed.Scheme + "://" + parsed.Host
}

func formatMonthlyBudget(value sql.NullFloat64) string {
	if !value.Valid {
		return "Unlimited"
	}
	return strconv.FormatFloat(value.Float64, 'f', -1, 64) + " USDC"
}

func budgetValue(value sql.NullFloat64) string {
	if !value.Valid {
		return ""
	}
	return strconv.FormatFloat(value.Float64, 'f', -1, 64)
}

func consumption(values map[string]float64, userID string, walletID string) string {
	return strconv.FormatFloat(values[userWalletKey(userID, walletID)], 'f', -1, 64) + " USDC"
}

func consumptionByUserWallet(transactions []db.Transaction) map[string]float64 {
	values := map[string]float64{}
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	for _, transaction := range transactions {
		if transaction.Status != "success" || transaction.CreatedAt.Before(startOfMonth) {
			continue
		}
		values[userWalletKey(transaction.UserID, transaction.WalletID)] += transaction.Amount
	}
	return values
}

func userWalletKey(userID string, walletID string) string {
	return userID + ":" + walletID
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
	http.Redirect(w, r, "/", http.StatusSeeOther)
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
	http.Redirect(w, r, "/", http.StatusSeeOther)
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
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

const usersPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Proxy Users</title>
  <style>
    body { font-family: system-ui, sans-serif; max-width: 1100px; margin: 40px auto; padding: 0 16px; color: #111827; }
    form { display: flex; gap: 8px; align-items: center; margin: 16px 0; }
    input, select { padding: 8px; border: 1px solid #d1d5db; border-radius: 6px; }
    button { padding: 8px 12px; border: 0; border-radius: 6px; background: #111827; color: white; cursor: pointer; }
    table { width: 100%; border-collapse: collapse; margin-top: 24px; }
    th, td { text-align: left; border-bottom: 1px solid #e5e7eb; padding: 10px 8px; }
    td.resource { max-width: 360px; overflow-wrap: anywhere; }
    .filters { flex-wrap: wrap; }
    .secondary { color: #374151; text-decoration: none; }
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
    <select id="wallet-chain" name="chain" required>
      <option value="algorand">algorand</option>
      <option value="solana">solana</option>
    </select>
    <select id="wallet-rpc-endpoint" name="rpc_endpoint">
      <option value="">Default for selected chain</option>
    </select>
    <input id="custom-rpc-endpoint" name="custom_rpc_endpoint" placeholder="custom RPC endpoint URL">
    <input name="rpc_token" type="password" placeholder="RPC token (optional)">
    <input name="private_key" type="password" placeholder="private key / mnemonic" required>
    <button type="submit">Add wallet</button>
  </form>
  <table>
    <thead><tr><th>Name</th><th>Chain</th><th>RPC Endpoint</th><th>Created</th><th>Actions</th></tr></thead>
    <tbody>
      {{range .Wallets}}
      <tr>
        <td>{{.Name}}</td>
        <td>{{.Chain}}</td>
        <td class="resource">{{.RpcEndpoint}}</td>
        <td>{{.CreatedAt}}</td>
        <td>
          <form method="post" action="/wallets/delete">
            <input type="hidden" name="id" value="{{.ID}}">
            <button class="danger" type="submit">Delete</button>
          </form>
        </td>
      </tr>
      {{else}}
      <tr><td colspan="5">No wallets yet.</td></tr>
      {{end}}
    </tbody>
  </table>
  <h2>User Wallet Access</h2>
  <form method="post" action="/user-wallets">
    <select name="user_id" required>
      <option value="">Select user</option>
      {{range .Users}}
      <option value="{{.ID}}">{{.Username}}</option>
      {{end}}
    </select>
    <select name="wallet_id" required>
      <option value="">Select wallet</option>
      {{range .Wallets}}
      <option value="{{.ID}}">{{.Name}} ({{.Chain}})</option>
      {{end}}
    </select>
    <input name="monthly_budget" type="number" step="0.000001" min="0" placeholder="monthly budget USDC (optional)">
    <button type="submit">Assign wallet</button>
  </form>
  <table>
    <thead><tr><th>User</th><th>Wallet</th><th>Monthly Budget</th><th>Current Month Consumption</th><th>Created</th><th>Actions</th></tr></thead>
    <tbody>
      {{range .UserWallets}}
      <tr>
        <td>{{index $.UsernamesByID .UserID}}</td>
        <td>{{index $.WalletNamesByID .WalletID}}</td>
        <td>
          <form method="post" action="/user-wallets/budget">
            <input type="hidden" name="id" value="{{.ID}}">
            <input name="monthly_budget" type="number" step="0.000001" min="0" placeholder="unlimited" value="{{budgetValue .MonthlyBudget}}">
            <button type="submit">Save</button>
          </form>
          <div class="secondary">Current: {{monthlyBudget .MonthlyBudget}}</div>
        </td>
        <td>{{consumption $.Consumption .UserID .WalletID}}</td>
        <td>{{.CreatedAt}}</td>
        <td>
          <form method="post" action="/user-wallets/delete">
            <input type="hidden" name="id" value="{{.ID}}">
            <button class="danger" type="submit">Delete</button>
          </form>
        </td>
      </tr>
      {{else}}
      <tr><td colspan="6">No user wallet assignments yet.</td></tr>
      {{end}}
    </tbody>
  </table>
  <h2>Transactions</h2>
  <form class="filters" method="get" action="/">
    <select name="user_id">
      <option value="">All users</option>
      {{range .Users}}
      <option value="{{.ID}}" {{if eq $.SelectedUserID .ID}}selected{{end}}>{{.Username}}</option>
      {{end}}
    </select>
    <select name="wallet_id">
      <option value="">All wallets</option>
      {{range .Wallets}}
      <option value="{{.ID}}" {{if eq $.SelectedWalletID .ID}}selected{{end}}>{{.Name}} ({{.Chain}})</option>
      {{end}}
    </select>
    <select name="resource_base_url">
      <option value="">All resource base URLs</option>
      {{range .ResourceBaseURLs}}
      <option value="{{.}}" {{if eq $.SelectedResource .}}selected{{end}}>{{.}}</option>
      {{end}}
    </select>
    <button type="submit">Filter</button>
    <a class="secondary" href="/">Clear</a>
  </form>
  <table>
    <thead><tr><th>User</th><th>Wallet</th><th>Amount</th><th>Status</th><th>Resource</th><th>Created</th></tr></thead>
    <tbody>
      {{range .Transactions}}
      <tr>
        <td>{{index $.UsernamesByID .UserID}}</td>
        <td>{{index $.WalletNamesByID .WalletID}}</td>
        <td>{{.Amount}}</td>
        <td>{{.Status}}</td>
        <td class="resource">{{.Resource}}</td>
        <td>{{.CreatedAt}}</td>
      </tr>
      {{else}}
      <tr><td colspan="6">No transactions found.</td></tr>
      {{end}}
    </tbody>
  </table>
  <script>
    const rpcEndpointsByChain = {
      algorand: [
        ["", "Default: TestNet AlgoNode (https://testnet-api.algonode.cloud)"],
        ["https://testnet-api.algonode.cloud", "TestNet AlgoNode"],
        ["https://mainnet-api.algonode.cloud", "MainNet AlgoNode"],
        ["https://betanet-api.algonode.cloud", "BetaNet AlgoNode"],
        ["custom", "Custom endpoint"]
      ],
      solana: [
        ["", "Default: MainNet Beta (https://api.mainnet-beta.solana.com)"],
        ["https://api.mainnet-beta.solana.com", "MainNet Beta"],
        ["https://api.devnet.solana.com", "DevNet"],
        ["https://api.testnet.solana.com", "TestNet"],
        ["custom", "Custom endpoint"]
      ]
    };
    const walletChain = document.getElementById("wallet-chain");
    const walletRpcEndpoint = document.getElementById("wallet-rpc-endpoint");
    const customRpcEndpoint = document.getElementById("custom-rpc-endpoint");
    function updateRpcEndpointOptions() {
      const options = rpcEndpointsByChain[walletChain.value] || [];
      walletRpcEndpoint.replaceChildren(...options.map(([value, label]) => {
        const option = document.createElement("option");
        option.value = value;
        option.textContent = label;
        return option;
      }));
      updateCustomRpcEndpoint();
    }
    function updateCustomRpcEndpoint() {
      const custom = walletRpcEndpoint.value === "custom";
      customRpcEndpoint.disabled = !custom;
      customRpcEndpoint.required = custom;
      if (!custom) {
        customRpcEndpoint.value = "";
      }
    }
    walletChain.addEventListener("change", updateRpcEndpointOptions);
    walletRpcEndpoint.addEventListener("change", updateCustomRpcEndpoint);
    updateRpcEndpointOptions();
  </script>
</body>
</html>`
