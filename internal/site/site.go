// Package site implements `trafaret serve` — a local, live-rendered manual for
// the library trafaret manages.
//
// The page is rendered from manifest.yaml + templates (built in, plus the
// <config>/templates overlay) + projects/ on EVERY request: an agent's edit to any
// of the on-disk files is, by construction, an edit to this site. Nothing is
// published anywhere — the server binds localhost only.
package site

import (
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/iamnikolie/trafaret/internal/engine"
)

//go:embed assets/mermaid.min.js
var mermaidJS []byte

// --- server -----------------------------------------------------------------

// Serve runs the operator site for eng on addr (a host:port, or a bare port on
// localhost; empty means 127.0.0.1:4477) until the listener fails. The startup
// line goes to out.
func Serve(eng *engine.Engine, addr string, out io.Writer) error {
	if addr == "" {
		addr = "127.0.0.1:4477"
	}
	if !strings.Contains(addr, ":") {
		addr = "127.0.0.1:" + addr
	}
	h := Handler(eng)
	fmt.Fprintf(out, "trafaret site → http://%s  (live: re-rendered from %s on every refresh; Ctrl-C to stop)\n", addr, eng.ConfigDir)
	return http.ListenAndServe(addr, h)
}

// Handler is the operator site: /<lang>/<page> for en and ru, the vendored
// mermaid script, and a language redirect at the root. Every page is rendered
// from eng's config directory on each request.
func Handler(eng *engine.Engine) http.Handler {
	page := template.Must(template.New("site").Parse(siteSkeleton))
	langs := map[string]map[string]template.HTML{"en": siteEN, "ru": siteRU}

	servePage := func(w http.ResponseWriter, lang, slug string, t map[string]template.HTML) {
		d, err := collectSiteData(eng)
		if err != nil {
			http.Error(w, "collect: "+err.Error(), 500)
			return
		}
		pd := sitePageData{siteData: d, Lang: lang, Page: slug, T: t}
		for i, sp := range sitePages {
			link := sitePageLink{Slug: sp.Slug, Title: t[sp.TitleKey], No: fmt.Sprintf("%02d", i+1)}
			pd.PageList = append(pd.PageList, link)
			if sp.Slug == slug {
				if i > 0 {
					prev := sitePageLink{Slug: sitePages[i-1].Slug, Title: t[sitePages[i-1].TitleKey]}
					pd.Prev = &prev
				}
				if i < len(sitePages)-1 {
					next := sitePageLink{Slug: sitePages[i+1].Slug, Title: t[sitePages[i+1].TitleKey]}
					pd.Next = &next
				}
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := page.Execute(w, pd); err != nil {
			fmt.Fprintln(os.Stderr, "trafaret: render site:", err)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/mermaid.min.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		_, _ = w.Write(mermaidJS)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			// forwards to the language saved in localStorage (default EN)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(langIndex))
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		t, ok := langs[parts[0]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if len(parts) == 1 {
			http.Redirect(w, r, "/"+parts[0]+"/overview", http.StatusFound)
			return
		}
		slug := parts[1]
		known := false
		for _, sp := range sitePages {
			if sp.Slug == slug {
				known = true
				break
			}
		}
		if !known || len(parts) > 2 {
			http.NotFound(w, r)
			return
		}
		servePage(w, parts[0], slug, t)
	})

	return mux
}
