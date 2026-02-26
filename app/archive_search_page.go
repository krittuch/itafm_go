package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type archiveSearchParams struct {
	Stream          string `json:"stream"`
	Command         string `json:"command"`
	Topic           string `json:"topic"`
	Q               string `json:"q"`
	PayloadContains string `json:"payload_contains"`
	JSONKey         string `json:"json_key"`
	JSONValue       string `json:"json_value"`
	Callsign        string `json:"callsign"`
	AircraftID      string `json:"aircraft_id"`
	From            string `json:"from"`
	To              string `json:"to"`
	Sort            string `json:"sort"`
	Limit           int    `json:"limit"`
	Offset          int    `json:"offset"`
}

type archiveMessageRow struct {
	ID         int64     `json:"id"`
	Stream     string    `json:"stream"`
	KafkaTopic string    `json:"kafka_topic"`
	Command    string    `json:"command"`
	Payload    string    `json:"payload"`
	ReceivedAt time.Time `json:"received_at"`
}

type archiveSearchResult struct {
	Enabled       bool                `json:"enabled"`
	Params        archiveSearchParams `json:"params"`
	Rows          []archiveMessageRow `json:"rows"`
	Count         int                 `json:"count"`
	HasMore       bool                `json:"has_more"`
	NextOffset    int                 `json:"next_offset"`
	PrevOffset    int                 `json:"prev_offset"`
	DurationMs    int64               `json:"duration_ms"`
	Error         string              `json:"error,omitempty"`
	ArchiveTable  string              `json:"archive_table"`
	ServerTimeUTC string              `json:"server_time_utc"`
}

type archivePageView struct {
	archiveSearchResult
	QueryStringNoOffset string
	PrevURL             string
	NextURL             string
}

var archiveSearchPageTemplate = template.Must(template.New("archive-search").Funcs(template.FuncMap{
	"fmtTime": func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.UTC().Format(time.RFC3339)
	},
	"prettyJSON": func(s string) string {
		if strings.TrimSpace(s) == "" {
			return ""
		}
		var v interface{}
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			return s
		}
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return s
		}
		return string(b)
	},
	"payloadPreview": func(s string) string {
		const max = 220
		if len(s) <= max {
			return s
		}
		return s[:max] + "..."
	},
	"hasFilters": func(p archiveSearchParams) bool {
		return p.Stream != "" || p.Command != "" || p.Topic != "" || p.Q != "" ||
			p.PayloadContains != "" || p.JSONKey != "" || p.JSONValue != "" ||
			p.Callsign != "" || p.AircraftID != "" || p.From != "" || p.To != ""
	},
}).Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>AODS Archive Search</title>
  <style>
    :root {
      --bg: #f5f0e8;
      --panel: #fffaf2;
      --ink: #1f1b16;
      --muted: #6d6358;
      --line: #d7c7b2;
      --accent: #0f6d66;
      --accent-2: #a7441a;
      --chip: #efe4d5;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: ui-sans-serif, system-ui, sans-serif;
      color: var(--ink);
      background:
        radial-gradient(circle at 15% 10%, #efe1cc 0, transparent 35%),
        radial-gradient(circle at 85% 0%, #d9ebe6 0, transparent 35%),
        var(--bg);
    }
    .wrap { max-width: 1400px; margin: 0 auto; padding: 20px; }
    .hero {
      background: linear-gradient(135deg, rgba(15,109,102,.12), rgba(167,68,26,.08));
      border: 1px solid var(--line);
      border-radius: 18px;
      padding: 18px;
      margin-bottom: 16px;
    }
    h1 { margin: 0 0 6px; font-size: 1.4rem; }
    .muted { color: var(--muted); }
    .row {
      display: grid;
      grid-template-columns: repeat(12, minmax(0, 1fr));
      gap: 10px;
      margin-bottom: 10px;
    }
    .field {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 14px;
      padding: 10px;
    }
    .field label { display: block; font-size: .8rem; color: var(--muted); margin-bottom: 5px; }
    .field input, .field select {
      width: 100%;
      border: 1px solid #ccb79c;
      background: #fff;
      border-radius: 10px;
      padding: 8px 9px;
      color: var(--ink);
    }
    .c12 { grid-column: span 12; }
    .c6 { grid-column: span 6; }
    .c4 { grid-column: span 4; }
    .c3 { grid-column: span 3; }
    .c2 { grid-column: span 2; }
    @media (max-width: 900px) {
      .c6, .c4, .c3, .c2 { grid-column: span 12; }
    }
    .actions {
      display: flex;
      gap: 10px;
      align-items: center;
      flex-wrap: wrap;
      margin-top: 8px;
    }
    .btn {
      border: 1px solid var(--line);
      background: var(--panel);
      color: var(--ink);
      border-radius: 999px;
      padding: 9px 14px;
      text-decoration: none;
      cursor: pointer;
    }
    .btn.primary {
      background: var(--accent);
      color: #fff;
      border-color: transparent;
    }
    .btn.alt {
      background: var(--accent-2);
      color: #fff;
      border-color: transparent;
    }
    .grid {
      display: grid;
      grid-template-columns: 1fr;
      gap: 12px;
    }
    .card {
      background: rgba(255,250,242,.95);
      border: 1px solid var(--line);
      border-radius: 16px;
      overflow: hidden;
    }
    .card-head {
      display: flex;
      justify-content: space-between;
      gap: 12px;
      padding: 10px 12px;
      border-bottom: 1px solid var(--line);
      background: rgba(239,228,213,.65);
      align-items: center;
      flex-wrap: wrap;
    }
    .chips { display: flex; gap: 8px; flex-wrap: wrap; }
    .chip {
      font-size: .75rem;
      background: var(--chip);
      border: 1px solid var(--line);
      border-radius: 999px;
      padding: 3px 8px;
    }
    .meta { font-size: .8rem; color: var(--muted); }
    .payload-preview {
      font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
      font-size: .78rem;
      color: var(--muted);
      padding: 12px;
      border-bottom: 1px dashed var(--line);
      white-space: pre-wrap;
      word-break: break-word;
    }
    details { padding: 10px 12px 12px; }
    details summary { cursor: pointer; font-weight: 600; }
    pre {
      margin: 8px 0 0;
      background: #fbf7ef;
      border: 1px solid var(--line);
      border-radius: 12px;
      padding: 10px;
      overflow: auto;
      max-height: 420px;
      font-size: .78rem;
      line-height: 1.35;
    }
    .stats {
      display: flex;
      gap: 10px;
      flex-wrap: wrap;
      margin: 10px 0 14px;
    }
    .stat {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 12px;
      padding: 8px 10px;
      font-size: .85rem;
    }
    .notice, .error {
      border-radius: 12px;
      padding: 10px 12px;
      margin: 10px 0;
      border: 1px solid var(--line);
      background: var(--panel);
    }
    .error {
      border-color: #d69580;
      background: #fff1eb;
      color: #6e2310;
    }
    code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
	    .links { display: flex; gap: 10px; flex-wrap: wrap; margin-top: 8px; }
	    .presets {
	      margin-top: 12px;
	      display: flex;
	      gap: 8px;
	      flex-wrap: wrap;
	    }
	    .preset-label {
	      font-size: .78rem;
	      color: var(--muted);
	      align-self: center;
	      margin-right: 2px;
	    }
	  </style>
</head>
<body>
  <div class="wrap">
    <div class="hero">
      <h1>AODS Archive Search</h1>
      <div class="muted">Search raw archived broker payloads for <code>FLMO</code> and <code>IDEP</code>. Surveillance is intentionally excluded.</div>
	      <div class="links">
	        <a class="btn" href="/">Main</a>
	        <a class="btn" href="/health">Health</a>
	        <a class="btn" href="/routes">Routes</a>
	        <a class="btn" href="/archive">Reset</a>
	        <a class="btn" href="/archive/search?{{.QueryStringNoOffset}}">JSON API (current filters)</a>
	      </div>
	      <div class="presets">
	        <span class="preset-label">Presets:</span>
	        <a class="btn" href="/archive?stream=FLMO&sort=desc&limit=100">Latest FLMO</a>
	        <a class="btn" href="/archive?stream=IDEP&sort=desc&limit=100">Latest IDEP</a>
	        <a class="btn alt" href="/archive?stream=FLMO&command=DLA&sort=desc&limit=100">Recent DLA</a>
	        <a class="btn alt" href="/archive?stream=FLMO&command=DLY&sort=desc&limit=100">Recent DLY</a>
	        <a class="btn alt" href="/archive?stream=FLMO&command=CNL&sort=desc&limit=100">Recent CNL</a>
	        <a class="btn" href="/archive?stream=FLMO&command=DEP&sort=desc&limit=100">Recent DEP</a>
	        <a class="btn" href="/archive?stream=FLMO&command=ARR&sort=desc&limit=100">Recent ARR</a>
	        <a class="btn" href="/archive?stream=FLMO&command=FPL&sort=desc&limit=100">Recent FPL</a>
	      </div>
	    </div>

    {{if not .Enabled}}
      <div class="error">Archive database is not connected. Set <code>AODS_ARCHIVE_DB_*</code> on the gateway and restart.</div>
    {{end}}

    <form method="get" action="/archive">
      <div class="row">
        <div class="field c6">
          <label for="q">General search (stream/topic/command/payload)</label>
          <input id="q" name="q" value="{{.Params.Q}}" placeholder="TG123, DLA, VTBS, parking stand, any text">
        </div>
        <div class="field c3">
          <label for="stream">Stream</label>
          <select id="stream" name="stream">
            <option value="" {{if eq .Params.Stream ""}}selected{{end}}>All</option>
            <option value="FLMO" {{if eq .Params.Stream "FLMO"}}selected{{end}}>FLMO</option>
            <option value="IDEP" {{if eq .Params.Stream "IDEP"}}selected{{end}}>IDEP</option>
          </select>
        </div>
        <div class="field c3">
          <label for="command">Command (FLMO)</label>
          <input id="command" name="command" value="{{.Params.Command}}" placeholder="FPL / DEP / ARR / CNL / DLA">
        </div>
      </div>

      <div class="row">
        <div class="field c4">
          <label for="callsign">CALLSIGN contains</label>
          <input id="callsign" name="callsign" value="{{.Params.Callsign}}" placeholder="TG123 or THA123">
        </div>
        <div class="field c4">
          <label for="aircraft_id">AircraftID contains (IDEP)</label>
          <input id="aircraft_id" name="aircraft_id" value="{{.Params.AircraftID}}" placeholder="THA0616">
        </div>
        <div class="field c4">
          <label for="topic">Kafka topic contains</label>
          <input id="topic" name="topic" value="{{.Params.Topic}}" placeholder="itafm.flight_movement">
        </div>
      </div>

      <div class="row">
        <div class="field c4">
          <label for="payload_contains">Payload contains</label>
          <input id="payload_contains" name="payload_contains" value="{{.Params.PayloadContains}}" placeholder="raw substring search">
        </div>
        <div class="field c4">
          <label for="json_key">JSON key</label>
          <input id="json_key" name="json_key" value="{{.Params.JSONKey}}" placeholder="CMD / CALLSIGN / AircraftID / DOF">
        </div>
        <div class="field c4">
          <label for="json_value">JSON value contains</label>
          <input id="json_value" name="json_value" value="{{.Params.JSONValue}}" placeholder="value for the JSON key filter">
        </div>
      </div>

      <div class="row">
        <div class="field c3">
          <label for="from">From (UTC)</label>
          <input id="from" name="from" value="{{.Params.From}}" placeholder="2026-02-26 or 2026-02-26T10:30">
        </div>
        <div class="field c3">
          <label for="to">To (UTC, exclusive)</label>
          <input id="to" name="to" value="{{.Params.To}}" placeholder="2026-02-27 or 2026-02-26T12:00">
        </div>
        <div class="field c2">
          <label for="sort">Sort</label>
          <select id="sort" name="sort">
            <option value="desc" {{if eq .Params.Sort "desc"}}selected{{end}}>Newest first</option>
            <option value="asc" {{if eq .Params.Sort "asc"}}selected{{end}}>Oldest first</option>
          </select>
        </div>
        <div class="field c2">
          <label for="limit">Limit</label>
          <input id="limit" name="limit" value="{{.Params.Limit}}">
        </div>
        <div class="field c2">
          <label for="offset">Offset</label>
          <input id="offset" name="offset" value="{{.Params.Offset}}">
        </div>
      </div>

      <div class="actions">
        <button class="btn primary" type="submit">Search Archive</button>
        <a class="btn" href="/archive">Clear Filters</a>
        {{if .PrevURL}}<a class="btn" href="{{.PrevURL}}">Previous Page</a>{{end}}
        {{if .NextURL}}<a class="btn alt" href="{{.NextURL}}">Next Page</a>{{end}}
      </div>
    </form>

    <div class="stats">
      <div class="stat">Rows: <strong>{{.Count}}</strong>{{if .HasMore}} + more{{end}}</div>
      <div class="stat">Query time: <strong>{{.DurationMs}} ms</strong></div>
      <div class="stat">Offset: <strong>{{.Params.Offset}}</strong></div>
      <div class="stat">Limit: <strong>{{.Params.Limit}}</strong></div>
      <div class="stat">Server time: <strong>{{.ServerTimeUTC}}</strong></div>
    </div>

    {{if .Error}}
      <div class="error">{{.Error}}</div>
    {{else if and .Enabled (eq .Count 0)}}
      <div class="notice">
        No archive rows matched{{if hasFilters .Params}} your filters{{end}}.
        Try broadening <code>q</code> or removing <code>json_key/json_value</code>.
      </div>
    {{end}}

    <div class="grid">
      {{range .Rows}}
      <div class="card">
        <div class="card-head">
          <div class="chips">
            <span class="chip">#{{.ID}}</span>
            <span class="chip">{{.Stream}}</span>
            {{if .Command}}<span class="chip">CMD {{.Command}}</span>{{end}}
            <span class="chip">{{.KafkaTopic}}</span>
          </div>
          <div class="meta">{{fmtTime .ReceivedAt}}</div>
        </div>
        <div class="payload-preview">{{payloadPreview .Payload}}</div>
        <details>
          <summary>Show payload</summary>
          <pre>{{prettyJSON .Payload}}</pre>
        </details>
      </div>
      {{end}}
    </div>
  </div>
</body>
</html>`))

func (m *gatewayMonitor) handleArchivePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	result := m.runArchiveSearch(r)
	view := archivePageView{
		archiveSearchResult: result,
		QueryStringNoOffset: archiveQueryStringNoOffset(r.URL.Query()),
	}
	if result.PrevOffset >= 0 {
		view.PrevURL = archiveURLWithOffset(r.URL.Query(), result.PrevOffset)
	}
	if result.HasMore {
		view.NextURL = archiveURLWithOffset(r.URL.Query(), result.NextOffset)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := archiveSearchPageTemplate.Execute(w, view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (m *gatewayMonitor) handleArchiveSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeMonitorJSON(w, m.runArchiveSearch(r))
}

func (m *gatewayMonitor) runArchiveSearch(r *http.Request) archiveSearchResult {
	start := time.Now()
	params := parseArchiveSearchParams(r)
	result := archiveSearchResult{
		Enabled:       m != nil && m.archiveDB != nil,
		Params:        params,
		ArchiveTable:  "aods_broker_messages",
		ServerTimeUTC: time.Now().UTC().Format(time.RFC3339),
		PrevOffset:    -1,
	}

	if m == nil || m.archiveDB == nil {
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	rows, hasMore, err := queryArchiveMessages(r.Context(), m.archiveDB, params)
	if err != nil {
		result.Error = err.Error()
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	result.Rows = rows
	result.Count = len(rows)
	result.HasMore = hasMore
	result.NextOffset = params.Offset + params.Limit
	if params.Offset-params.Limit >= 0 {
		result.PrevOffset = params.Offset - params.Limit
	}
	result.DurationMs = time.Since(start).Milliseconds()
	return result
}

func queryArchiveMessages(ctx context.Context, db *sql.DB, p archiveSearchParams) ([]archiveMessageRow, bool, error) {
	where := []string{"1=1"}
	args := make([]interface{}, 0, 20)
	addArg := func(v interface{}) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if p.Stream != "" {
		where = append(where, "stream = "+addArg(strings.ToUpper(p.Stream)))
	}
	if p.Command != "" {
		where = append(where, "UPPER(COALESCE(command, '')) = UPPER("+addArg(p.Command)+")")
	}
	if p.Topic != "" {
		where = append(where, "kafka_topic ILIKE "+addArg("%"+p.Topic+"%"))
	}

	fromTime, fromOK, fromErr := parseArchiveTimeFilter(p.From, false)
	if fromErr != "" {
		return nil, false, fmt.Errorf("invalid from time: %s", fromErr)
	}
	if fromOK {
		where = append(where, "received_at >= "+addArg(fromTime))
	}

	toTime, toOK, toErr := parseArchiveTimeFilter(p.To, true)
	if toErr != "" {
		return nil, false, fmt.Errorf("invalid to time: %s", toErr)
	}
	if toOK {
		where = append(where, "received_at < "+addArg(toTime))
	}

	if p.Q != "" {
		pattern := "%" + p.Q + "%"
		a := addArg(pattern)
		b := addArg(pattern)
		c := addArg(pattern)
		d := addArg(pattern)
		where = append(where, "(stream ILIKE "+a+" OR kafka_topic ILIKE "+b+" OR COALESCE(command,'') ILIKE "+c+" OR payload ILIKE "+d+")")
	}

	if p.PayloadContains != "" {
		where = append(where, "payload ILIKE "+addArg("%"+p.PayloadContains+"%"))
	}

	if p.JSONKey != "" && p.JSONValue != "" {
		keyArg := addArg(p.JSONKey)
		valArg := addArg("%" + p.JSONValue + "%")
		where = append(where, "COALESCE(payload::jsonb ->> "+keyArg+", '') ILIKE "+valArg)
	}

	if p.Callsign != "" {
		pattern := "%" + p.Callsign + "%"
		a := addArg(pattern)
		b := addArg(pattern)
		c := addArg(pattern)
		where = append(where, "(COALESCE(payload::jsonb ->> 'CALLSIGN','') ILIKE "+a+" OR COALESCE(payload::jsonb ->> 'CallSign','') ILIKE "+b+" OR payload ILIKE "+c+")")
	}

	if p.AircraftID != "" {
		pattern := "%" + p.AircraftID + "%"
		a := addArg(pattern)
		b := addArg(pattern)
		where = append(where, "(COALESCE(payload::jsonb ->> 'AircraftID','') ILIKE "+a+" OR payload ILIKE "+b+")")
	}

	orderDir := "DESC"
	if strings.EqualFold(p.Sort, "asc") {
		orderDir = "ASC"
	}

	limitArg := addArg(p.Limit + 1)
	offsetArg := addArg(p.Offset)

	query := fmt.Sprintf(`
SELECT id, stream, kafka_topic, COALESCE(command, ''), payload, received_at
FROM aods_broker_messages
WHERE %s
ORDER BY received_at %s, id %s
LIMIT %s OFFSET %s`,
		strings.Join(where, " AND "),
		orderDir,
		orderDir,
		limitArg,
		offsetArg,
	)

	sqlRows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer sqlRows.Close()

	results := make([]archiveMessageRow, 0, p.Limit+1)
	for sqlRows.Next() {
		var row archiveMessageRow
		if err := sqlRows.Scan(&row.ID, &row.Stream, &row.KafkaTopic, &row.Command, &row.Payload, &row.ReceivedAt); err != nil {
			return nil, false, err
		}
		results = append(results, row)
	}
	if err := sqlRows.Err(); err != nil {
		return nil, false, err
	}

	hasMore := len(results) > p.Limit
	if hasMore {
		results = results[:p.Limit]
	}

	return results, hasMore, nil
}

func parseArchiveSearchParams(r *http.Request) archiveSearchParams {
	q := r.URL.Query()

	limit := parseArchiveInt(q.Get("limit"), 100, 1, 500)
	offset := parseArchiveInt(q.Get("offset"), 0, 0, 1000000)
	sort := strings.ToLower(strings.TrimSpace(q.Get("sort")))
	if sort != "asc" {
		sort = "desc"
	}

	return archiveSearchParams{
		Stream:          strings.ToUpper(strings.TrimSpace(q.Get("stream"))),
		Command:         strings.ToUpper(strings.TrimSpace(q.Get("command"))),
		Topic:           strings.TrimSpace(q.Get("topic")),
		Q:               strings.TrimSpace(q.Get("q")),
		PayloadContains: strings.TrimSpace(q.Get("payload_contains")),
		JSONKey:         strings.TrimSpace(q.Get("json_key")),
		JSONValue:       strings.TrimSpace(q.Get("json_value")),
		Callsign:        strings.TrimSpace(q.Get("callsign")),
		AircraftID:      strings.TrimSpace(q.Get("aircraft_id")),
		From:            strings.TrimSpace(q.Get("from")),
		To:              strings.TrimSpace(q.Get("to")),
		Sort:            sort,
		Limit:           limit,
		Offset:          offset,
	}
}

func parseArchiveInt(raw string, fallback, min, max int) int {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func parseArchiveTimeFilter(raw string, endExclusiveDate bool) (time.Time, bool, string) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, false, ""
	}

	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), true, ""
	}

	layouts := []string{
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t.UTC(), true, ""
		}
	}

	if t, err := time.ParseInLocation("2006-01-02", s, time.UTC); err == nil {
		if endExclusiveDate {
			return t.Add(24 * time.Hour).UTC(), true, ""
		}
		return t.UTC(), true, ""
	}

	return time.Time{}, false, "expected RFC3339 or YYYY-MM-DD[THH:MM[:SS]]"
}

func archiveURLWithOffset(values url.Values, offset int) string {
	cloned := cloneURLValues(values)
	cloned.Set("offset", strconv.Itoa(offset))
	return "/archive?" + cloned.Encode()
}

func archiveQueryStringNoOffset(values url.Values) string {
	cloned := cloneURLValues(values)
	cloned.Del("offset")
	if _, ok := cloned["limit"]; !ok {
		cloned.Set("limit", "100")
	}
	if _, ok := cloned["sort"]; !ok {
		cloned.Set("sort", "desc")
	}
	return cloned.Encode()
}

func cloneURLValues(v url.Values) url.Values {
	out := make(url.Values, len(v))
	for key, vals := range v {
		copied := make([]string, len(vals))
		copy(copied, vals)
		out[key] = copied
	}
	return out
}
