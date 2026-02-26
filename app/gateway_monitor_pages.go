package app

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
)

type gatewayNavLink struct {
	Label  string
	Href   string
	Active bool
}

type gatewayJSONPageView struct {
	Title       string
	Description string
	Payload     interface{}
	MainURL     string
	RawJSONURL  string
	Nav         []gatewayNavLink
}

type gatewayIndexView struct {
	MainURL          string
	HealthURL        string
	HealthJSONURL    string
	RoutesURL        string
	RoutesJSONURL    string
	ArchiveURL       string
	ArchiveSearchURL string
}

var gatewayIndexTemplate = template.Must(template.New("gateway-index").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Gateway Monitor</title>
  <style>
    :root { --bg:#f2efe8; --panel:#fffdf8; --line:#d9d0c2; --ink:#17140f; --muted:#665e52; --a:#0d6c66; --b:#9f4b22; }
    * { box-sizing:border-box; }
    body { margin:0; font-family:ui-sans-serif,system-ui,sans-serif; background:
      radial-gradient(circle at 10% 10%, #e9dfcf 0, transparent 36%),
      radial-gradient(circle at 90% 0%, #d8ece9 0, transparent 38%), var(--bg); color:var(--ink); }
    .wrap { max-width:1100px; margin:0 auto; padding:20px; }
    .hero { background:var(--panel); border:1px solid var(--line); border-radius:18px; padding:18px; }
    h1 { margin:0 0 6px; font-size:1.35rem; }
    .muted { color:var(--muted); }
    .menu { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:12px; margin-top:14px; }
    @media (max-width: 840px) { .menu { grid-template-columns:1fr; } }
    .card { background:rgba(255,253,248,.95); border:1px solid var(--line); border-radius:14px; padding:14px; }
    .card h2 { margin:0 0 6px; font-size:1rem; }
    .links { display:flex; gap:8px; flex-wrap:wrap; margin-top:10px; }
    .btn { border:1px solid var(--line); background:#fff; color:var(--ink); border-radius:999px; padding:8px 12px; text-decoration:none; }
    .btn.primary { background:var(--a); color:#fff; border-color:transparent; }
    .btn.alt { background:var(--b); color:#fff; border-color:transparent; }
    code { font-family:ui-monospace,SFMono-Regular,Menlo,monospace; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="hero">
      <h1>Gateway Monitor</h1>
      <div class="muted">Navigation page for monitor endpoints and archive search UI.</div>
    </div>
    <div class="menu">
      <div class="card">
        <h2>Health</h2>
        <div class="muted">Gateway monitor service health and timestamps.</div>
        <div class="links">
          <a class="btn primary" href="{{.HealthURL}}">Open Health Page</a>
          <a class="btn" href="{{.HealthJSONURL}}">Raw JSON</a>
        </div>
      </div>
      <div class="card">
        <h2>Routes</h2>
        <div class="muted">Counters and errors for FLMO, IDEP, and Surveillance consumers.</div>
        <div class="links">
          <a class="btn primary" href="{{.RoutesURL}}">Open Routes Page</a>
          <a class="btn" href="{{.RoutesJSONURL}}">Raw JSON</a>
        </div>
      </div>
      <div class="card">
        <h2>Archive Search</h2>
        <div class="muted">Search archived raw broker payloads for <code>FLMO</code> and <code>IDEP</code>.</div>
        <div class="links">
          <a class="btn alt" href="{{.ArchiveURL}}">Open Archive Search</a>
          <a class="btn" href="{{.ArchiveSearchURL}}">Archive JSON API</a>
        </div>
      </div>
    </div>
  </div>
</body>
</html>`))

var gatewayJSONPageTemplate = template.Must(template.New("gateway-json-page").Funcs(template.FuncMap{
	"prettyJSON": func(v interface{}) string {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return "{}"
		}
		return string(b)
	},
}).Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <style>
    :root { --bg:#f2efe8; --panel:#fffdf8; --line:#d9d0c2; --ink:#17140f; --muted:#665e52; --active:#0d6c66; }
    * { box-sizing:border-box; }
    body { margin:0; font-family:ui-sans-serif,system-ui,sans-serif; background:var(--bg); color:var(--ink); }
    .wrap { max-width:1100px; margin:0 auto; padding:20px; }
    .nav { display:flex; gap:8px; flex-wrap:wrap; margin-bottom:12px; }
    .nav a { text-decoration:none; border:1px solid var(--line); background:#fff; color:var(--ink); border-radius:999px; padding:8px 12px; }
    .nav a.active { background:var(--active); color:#fff; border-color:transparent; }
    .panel { background:var(--panel); border:1px solid var(--line); border-radius:16px; padding:16px; }
    h1 { margin:0 0 6px; font-size:1.25rem; }
    .muted { color:var(--muted); }
    .links { display:flex; gap:8px; flex-wrap:wrap; margin-top:10px; }
    .links a { text-decoration:none; border:1px solid var(--line); border-radius:999px; padding:8px 12px; background:#fff; color:var(--ink); }
    pre { margin:14px 0 0; border:1px solid var(--line); border-radius:12px; background:#fbf8f1; padding:12px; overflow:auto; max-height:70vh; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="nav">
      {{range .Nav}}
      <a href="{{.Href}}" class="{{if .Active}}active{{end}}">{{.Label}}</a>
      {{end}}
    </div>
    <div class="panel">
      <h1>{{.Title}}</h1>
      <div class="muted">{{.Description}}</div>
      <div class="links">
        <a href="{{.MainURL}}">Back To Main</a>
        <a href="{{.RawJSONURL}}">Raw JSON</a>
      </div>
      <pre>{{prettyJSON .Payload}}</pre>
    </div>
  </div>
</body>
</html>`))

func wantsHTML(r *http.Request) bool {
	if r == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("format")), "json") {
		return false
	}
	return strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/html")
}

func gatewayNavLinks(basePath string, activeHref string) []gatewayNavLink {
	links := []gatewayNavLink{
		{Label: "Main", Href: joinMonitorPath(basePath, "")},
		{Label: "Health", Href: joinMonitorPath(basePath, "/health")},
		{Label: "Routes", Href: joinMonitorPath(basePath, "/routes")},
		{Label: "Archive", Href: joinMonitorPath(basePath, "/archive")},
	}
	for i := range links {
		links[i].Active = links[i].Href == activeHref
	}
	return links
}

func (m *gatewayMonitor) handleIndex(w http.ResponseWriter, r *http.Request) {
	basePath := m.monitorBasePathForRequest(r, "")
	routesURL := joinMonitorPath(basePath, "/routes")
	healthURL := joinMonitorPath(basePath, "/health")
	archiveURL := joinMonitorPath(basePath, "/archive")
	archiveSearchURL := joinMonitorPath(basePath, "/archive/search")

	payload := map[string]string{
		"service": "gateway-monitor",
		"routes":  routesURL,
		"health":  healthURL,
		"archive": archiveURL,
	}
	if !wantsHTML(r) {
		writeMonitorJSON(w, payload)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = gatewayIndexTemplate.Execute(w, gatewayIndexView{
		MainURL:          joinMonitorPath(basePath, ""),
		HealthURL:        healthURL,
		HealthJSONURL:    healthURL + "?format=json",
		RoutesURL:        routesURL,
		RoutesJSONURL:    routesURL + "?format=json",
		ArchiveURL:       archiveURL,
		ArchiveSearchURL: archiveSearchURL,
	})
}

func writeGatewayJSONPage(w http.ResponseWriter, view gatewayJSONPageView) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = gatewayJSONPageTemplate.Execute(w, view)
}
