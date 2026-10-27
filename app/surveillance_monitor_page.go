package app

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type surveillancePageView struct {
	surveillanceTrackSnapshot
	Filter        surveillanceTrackFilter
	Outcomes      []surveillanceOutcome
	ServerTimeUTC string
	RawJSONURL    string
	Nav           []gatewayNavLink
	now           time.Time
}

func (v surveillancePageView) Age(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := v.now.Sub(t).Round(time.Second)
	if d < 0 {
		d = 0
	}
	return d.String() + " ago"
}

func (v surveillancePageView) OutcomeURL(outcome surveillanceOutcome) string {
	q := url.Values{}
	q.Set("outcome", string(outcome))
	return "./surveillance?" + q.Encode()
}

var surveillancePageTemplate = template.Must(template.New("surveillance").Funcs(template.FuncMap{
	"fmtTime": func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.UTC().Format(time.RFC3339)
	},
	"num": func(v float64) string {
		return fmt.Sprintf("%.0f", v)
	},
	"coord": func(v float64) string {
		return fmt.Sprintf("%.4f", v)
	},
}).Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Surveillance Monitor</title>
  <style>
    :root { --bg:#f2efe8; --panel:#fffdf8; --line:#d9d0c2; --ink:#17140f; --muted:#665e52; --active:#0d6c66; --alt:#9f4b22; --chip:#efe4d5; }
    * { box-sizing:border-box; }
    body { margin:0; font-family:ui-sans-serif,system-ui,sans-serif; background:var(--bg); color:var(--ink); }
    .wrap { max-width:1400px; margin:0 auto; padding:20px; }
    .nav { display:flex; gap:8px; flex-wrap:wrap; margin-bottom:12px; }
    .nav a { text-decoration:none; border:1px solid var(--line); background:#fff; color:var(--ink); border-radius:999px; padding:8px 12px; }
    .nav a.active { background:var(--active); color:#fff; border-color:transparent; }
    .panel { background:var(--panel); border:1px solid var(--line); border-radius:16px; padding:16px; margin-bottom:12px; }
    h1 { margin:0 0 6px; font-size:1.25rem; }
    .muted { color:var(--muted); }
    .stats { display:flex; gap:8px; flex-wrap:wrap; margin-top:10px; }
    .stat { border:1px solid var(--line); background:#fff; border-radius:999px; padding:6px 12px; text-decoration:none; color:var(--ink); }
    a.stat:hover { border-color:var(--active); }
    form { display:grid; grid-template-columns:repeat(6,minmax(0,1fr)); gap:10px; }
    @media (max-width: 900px) { form { grid-template-columns:repeat(2,minmax(0,1fr)); } }
    label { display:block; font-size:.8rem; color:var(--muted); margin-bottom:4px; }
    input, select { width:100%; border:1px solid #ccb79c; background:#fff; border-radius:10px; padding:8px 9px; color:var(--ink); }
    .actions { grid-column:1/-1; display:flex; gap:10px; flex-wrap:wrap; }
    .btn { border:1px solid var(--line); background:#fff; color:var(--ink); border-radius:999px; padding:9px 14px; text-decoration:none; cursor:pointer; font:inherit; }
    .btn.primary { background:var(--active); color:#fff; border-color:transparent; }
    .table-wrap { overflow-x:auto; }
    table { width:100%; border-collapse:collapse; font-size:.9rem; }
    th, td { text-align:left; padding:7px 8px; border-bottom:1px solid var(--line); white-space:nowrap; }
    th { color:var(--muted); font-weight:600; }
    td.n { text-align:right; font-variant-numeric:tabular-nums; }
    .chip { display:inline-block; background:var(--chip); border-radius:999px; padding:2px 8px; font-size:.8rem; }
    .chip.queued, .chip.duplicate { background:#d8ece9; }
    .chip.decode_error, .chip.rejected, .chip.iata_failed { background:#f3d9cb; }
    code { font-family:ui-monospace,SFMono-Regular,Menlo,monospace; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="nav">
      {{range .Nav}}<a href="{{.Href}}" class="{{if .Active}}active{{end}}">{{.Label}}</a>{{end}}
    </div>

    <div class="panel">
      <h1>Surveillance Monitor</h1>
      <div class="muted">Latest message per callsign since the gateway started, kept in memory: at most {{.MaxTracks}} tracks, each dropped after {{.TTLSeconds}}s without a message. Server time {{.ServerTimeUTC}}.</div>
      <div class="stats">
        <span class="stat">Stored: <strong>{{.Stored}}</strong> / {{.MaxTracks}}</span>
        <span class="stat">Evicted (cap): <strong>{{.Evicted}}</strong></span>
        {{range .Outcomes}}<a class="stat" href="{{$.OutcomeURL .}}">{{.}}: <strong>{{index $.OutcomeCounts .}}</strong></a>{{end}}
        <a class="stat" href="{{.RawJSONURL}}">Raw JSON</a>
      </div>
      <div class="muted" style="margin-top:8px">Outcome counters cover every message since start. <code>queued</code> and <code>duplicate</code> count as processed on the Routes page; the rest count as skipped.</div>
    </div>

    <div class="panel">
      <form method="get" action="./surveillance">
        <div><label for="callsign">Callsign contains</label><input id="callsign" name="callsign" value="{{.Filter.CallSign}}" placeholder="THA123 or TG123"></div>
        <div><label for="dep">Departure</label><input id="dep" name="dep" value="{{.Filter.Departure}}" placeholder="VTBS"></div>
        <div><label for="dest">Destination</label><input id="dest" name="dest" value="{{.Filter.Destination}}" placeholder="VTBS"></div>
        <div><label for="actype">Aircraft type contains</label><input id="actype" name="actype" value="{{.Filter.AircraftType}}" placeholder="B77W"></div>
        <div><label for="ssr">SSR code contains</label><input id="ssr" name="ssr" value="{{.Filter.SSRCode}}" placeholder="2101"></div>
        <div><label for="acaddress">Aircraft address contains</label><input id="acaddress" name="acaddress" value="{{.Filter.AircraftAddress}}" placeholder="8840A1"></div>
        <div>
          <label for="outcome">Outcome</label>
          <select id="outcome" name="outcome">
            <option value="" {{if eq .Filter.Outcome ""}}selected{{end}}>All</option>
            {{range .Outcomes}}<option value="{{.}}" {{if eq $.Filter.Outcome (printf "%s" .)}}selected{{end}}>{{.}}</option>{{end}}
          </select>
        </div>
        <div>
          <label for="sort">Sort</label>
          <select id="sort" name="sort">
            <option value="last_seen" {{if eq .Filter.Sort "last_seen"}}selected{{end}}>Last seen</option>
            <option value="callsign" {{if eq .Filter.Sort "callsign"}}selected{{end}}>Callsign</option>
            <option value="altitude" {{if eq .Filter.Sort "altitude"}}selected{{end}}>Altitude (high first)</option>
          </select>
        </div>
        <div><label for="limit">Limit</label><input id="limit" name="limit" value="{{.Filter.Limit}}"></div>
        <div class="actions">
          <button class="btn primary" type="submit">Filter</button>
          <a class="btn" href="./surveillance">Clear Filters</a>
        </div>
      </form>
    </div>

    <div class="panel">
      <div class="muted">Showing {{len .Tracks}} of {{.Matched}} matching tracks.</div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Last seen</th><th>Callsign</th><th>IATA</th><th>Outcome</th><th>Dep</th><th>Dest</th><th>Type</th>
              <th>SSR</th><th>Address</th><th>Alt</th><th>GS</th><th>Hdg</th><th>Lat</th><th>Lon</th><th>Track</th><th>Datetime</th><th>Msgs</th>
            </tr>
          </thead>
          <tbody>
            {{range .Tracks}}
            <tr>
              <td title="{{fmtTime .LastSeen}}">{{$.Age .LastSeen}}</td>
              <td><strong>{{.CallSign}}</strong></td>
              <td>{{.IATACallSign}}</td>
              <td><span class="chip {{.Outcome}}">{{.Outcome}}</span></td>
              <td>{{.Departure}}</td>
              <td>{{.Destination}}</td>
              <td>{{.AircraftType}}</td>
              <td>{{.SSRCode}}</td>
              <td>{{.AircraftAddress}}</td>
              <td class="n">{{num .Altitude}}</td>
              <td class="n">{{num .GroundSpeed}}</td>
              <td class="n">{{num .Heading}}</td>
              <td class="n">{{coord .Lat}}</td>
              <td class="n">{{coord .Lon}}</td>
              <td class="n">{{.TrackNumber}}</td>
              <td>{{.DateTime}}</td>
              <td class="n">{{.Messages}}</td>
            </tr>
            {{else}}
            <tr><td colspan="17" class="muted">No tracks match.</td></tr>
            {{end}}
          </tbody>
        </table>
      </div>
    </div>
  </div>
</body>
</html>`))

func (m *gatewayMonitor) handleSurveillance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filter := parseSurveillanceTrackFilter(r)
	snapshot := m.surveillanceTracks.snapshot(filter)
	now := time.Now().UTC()

	if !wantsHTML(r) {
		writeMonitorJSON(w, map[string]interface{}{
			"filter":          filter,
			"server_time_utc": now.Format(time.RFC3339),
			"result":          snapshot,
		})
		return
	}

	q := cloneURLValues(r.URL.Query())
	q.Set("format", "json")
	view := surveillancePageView{
		surveillanceTrackSnapshot: snapshot,
		Filter:                    filter,
		Outcomes:                  surveillanceOutcomes,
		ServerTimeUTC:             now.Format(time.RFC3339),
		RawJSONURL:                "./surveillance?" + q.Encode(),
		Nav:                       gatewayNavLinks("./surveillance"),
		now:                       now,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := surveillancePageTemplate.Execute(w, view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func parseSurveillanceTrackFilter(r *http.Request) surveillanceTrackFilter {
	q := r.URL.Query()

	sortBy := strings.ToLower(strings.TrimSpace(q.Get("sort")))
	if sortBy != "callsign" && sortBy != "altitude" {
		sortBy = "last_seen"
	}

	outcome := strings.ToLower(strings.TrimSpace(q.Get("outcome")))
	known := false
	for _, o := range surveillanceOutcomes {
		if string(o) == outcome {
			known = true
			break
		}
	}
	if !known {
		outcome = ""
	}

	return surveillanceTrackFilter{
		CallSign:        strings.TrimSpace(q.Get("callsign")),
		Departure:       strings.TrimSpace(q.Get("dep")),
		Destination:     strings.TrimSpace(q.Get("dest")),
		AircraftType:    strings.TrimSpace(q.Get("actype")),
		SSRCode:         strings.TrimSpace(q.Get("ssr")),
		AircraftAddress: strings.TrimSpace(q.Get("acaddress")),
		Outcome:         outcome,
		Sort:            sortBy,
		Limit:           parseArchiveInt(q.Get("limit"), 200, 1, 2000),
	}
}
