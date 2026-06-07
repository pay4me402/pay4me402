package admin

import (
	"database/sql"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/majed/payformeproxy/internal/db"
)

type pageData struct {
	Title            string
	Active           string
	Authenticated    bool
	Users            []db.ProxyUser
	Wallets          []db.Wallet
	UserWallets      []db.UserWallet
	Consumption      map[string]float64
	Transactions     []db.Transaction
	SelectedUserID   string
	SelectedWalletID string
	SelectedResource string
	ResourceBaseURLs []string
	UsernamesByID    map[string]string
	WalletNamesByID  map[string]string
	Error            string
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data pageData) {
	data.Authenticated = s.authenticated(r)
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		logTemplateError(name, err)
	}
}

func logTemplateError(name string, err error) {
	println("render admin template " + name + ": " + err.Error())
}

func (s *Server) commonData(r *http.Request, title string, active string) (pageData, error) {
	items, err := s.users.List(r.Context())
	if err != nil {
		return pageData{}, err
	}
	walletItems, err := s.wallets.List(r.Context())
	if err != nil {
		return pageData{}, err
	}
	allTransactions, transactionItems, err := s.filteredTransactions(r)
	if err != nil {
		return pageData{}, err
	}
	userWalletItems, err := s.userWallets.List(r.Context())
	if err != nil {
		return pageData{}, err
	}
	return pageData{
		Title:            title,
		Active:           active,
		Users:            items,
		Wallets:          walletItems,
		UserWallets:      userWalletItems,
		Consumption:      consumptionByUserWallet(allTransactions),
		Transactions:     transactionItems,
		SelectedUserID:   r.URL.Query().Get("user_id"),
		SelectedWalletID: r.URL.Query().Get("wallet_id"),
		SelectedResource: r.URL.Query().Get("resource_base_url"),
		ResourceBaseURLs: resourceBaseURLs(allTransactions),
		UsernamesByID:    usernamesByID(items),
		WalletNamesByID:  walletNamesByID(walletItems),
	}, nil
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

func statusClass(status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	normalized = strings.ReplaceAll(normalized, " ", "-")
	normalized = strings.ReplaceAll(normalized, "_", "-")
	if normalized == "" {
		return "unknown"
	}
	return normalized
}
