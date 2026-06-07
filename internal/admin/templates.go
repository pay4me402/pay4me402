package admin

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
)

//go:embed templates/*.html static/*
var adminAssets embed.FS

func parseTemplates() (*template.Template, error) {
	return template.New("admin").Funcs(template.FuncMap{
		"monthlyBudget": formatMonthlyBudget,
		"budgetValue":   budgetValue,
		"consumption":   consumption,
		"statusClass":   statusClass,
	}).ParseFS(adminAssets, "templates/*.html")
}

func staticFileServer() http.Handler {
	staticFS, err := fs.Sub(adminAssets, "static")
	if err != nil {
		return http.NotFoundHandler()
	}
	return http.FileServer(http.FS(staticFS))
}
