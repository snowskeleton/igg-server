package admin

import (
	"embed"
	"html/template"
	"io"
	"net/http"
)

//go:embed templates/*.html
var templateFS embed.FS

var (
	loginTmpl     *template.Template
	verifyTmpl    *template.Template
	dashboardTmpl *template.Template
	usersTmpl     *template.Template
	carsTmpl      *template.Template
	configTmpl    *template.Template
)

func init() {
	loginTmpl = template.Must(template.ParseFS(templateFS, "templates/login.html"))
	verifyTmpl = template.Must(template.ParseFS(templateFS, "templates/verify.html"))
	dashboardTmpl = template.Must(template.ParseFS(templateFS, "templates/layout.html", "templates/dashboard.html"))
	usersTmpl = template.Must(template.ParseFS(templateFS, "templates/layout.html", "templates/users.html"))
	carsTmpl = template.Must(template.ParseFS(templateFS, "templates/layout.html", "templates/cars.html"))
	configTmpl = template.Must(template.ParseFS(templateFS, "templates/layout.html", "templates/config.html"))
}

// RenderVerifyPage renders the magic link verification page for both iOS and admin flows.
func RenderVerifyPage(w http.ResponseWriter, data VerifyPageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	verifyTmpl.Execute(w, data)
}

// VerifyPageData holds data for the verify template.
type VerifyPageData struct {
	Title    string
	Message  string
	DeepLink string
	Error    string
}

func renderTemplate(w io.Writer, tmpl *template.Template, data any) error {
	return tmpl.ExecuteTemplate(w, "layout", data)
}
