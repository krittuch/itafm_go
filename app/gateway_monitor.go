package app

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type gatewayRouteMonitor struct {
	Label      string `json:"label"`
	KafkaTopic string `json:"kafka_topic"`

	ReceivedCount      atomic.Uint64
	ProcessedCount     atomic.Uint64
	SkippedCount       atomic.Uint64
	DecodeErrorCount   atomic.Uint64
	ConsumerErrorCount atomic.Uint64
	LastReceivedUnix   atomic.Int64
	LastProcessedUnix  atomic.Int64

	lastErrorMu sync.RWMutex
	lastError   string
}

type gatewayRouteCounterSnapshot struct {
	Received       uint64
	Processed      uint64
	Skipped        uint64
	DecodeErrors   uint64
	ConsumerErrors uint64
}

func (r *gatewayRouteMonitor) RecordReceived() {
	r.ReceivedCount.Add(1)
	r.LastReceivedUnix.Store(time.Now().Unix())
}

func (r *gatewayRouteMonitor) RecordProcessed() {
	r.ProcessedCount.Add(1)
	r.LastProcessedUnix.Store(time.Now().Unix())
}

func (r *gatewayRouteMonitor) RecordSkipped() {
	r.SkippedCount.Add(1)
}

func (r *gatewayRouteMonitor) RecordDecodeError(err error) {
	r.DecodeErrorCount.Add(1)
	r.setLastError(err)
}

func (r *gatewayRouteMonitor) RecordConsumerError(err error) {
	r.ConsumerErrorCount.Add(1)
	r.setLastError(err)
}

func (r *gatewayRouteMonitor) setLastError(err error) {
	if err == nil {
		return
	}
	r.lastErrorMu.Lock()
	r.lastError = err.Error()
	r.lastErrorMu.Unlock()
}

func (r *gatewayRouteMonitor) counterSnapshot() gatewayRouteCounterSnapshot {
	return gatewayRouteCounterSnapshot{
		Received:       r.ReceivedCount.Load(),
		Processed:      r.ProcessedCount.Load(),
		Skipped:        r.SkippedCount.Load(),
		DecodeErrors:   r.DecodeErrorCount.Load(),
		ConsumerErrors: r.ConsumerErrorCount.Load(),
	}
}

func (r *gatewayRouteMonitor) snapshot() map[string]interface{} {
	r.lastErrorMu.RLock()
	lastError := r.lastError
	r.lastErrorMu.RUnlock()

	return map[string]interface{}{
		"label":                r.Label,
		"kafka_topic":          r.KafkaTopic,
		"received_count":       r.ReceivedCount.Load(),
		"processed_count":      r.ProcessedCount.Load(),
		"skipped_count":        r.SkippedCount.Load(),
		"decode_error_count":   r.DecodeErrorCount.Load(),
		"consumer_error_count": r.ConsumerErrorCount.Load(),
		"last_received_at":     formatUnixTimestamp(r.LastReceivedUnix.Load()),
		"last_processed_at":    formatUnixTimestamp(r.LastProcessedUnix.Load()),
		"last_error":           lastError,
	}
}

type gatewayMonitor struct {
	startedAt time.Time
	server    *http.Server
	routes    map[string]*gatewayRouteMonitor
	archiveDB *sql.DB
	basePath  string
}

func newGatewayMonitor(flightTopic, idepTopic, survTopic string, archiveDB *sql.DB) *gatewayMonitor {
	return &gatewayMonitor{
		startedAt: time.Now().UTC(),
		archiveDB: archiveDB,
		basePath:  "/aods-mon",
		routes: map[string]*gatewayRouteMonitor{
			"flight": {
				Label:      "FLIGHT_MOVEMENT",
				KafkaTopic: flightTopic,
			},
			"idep": {
				Label:      "IDEP",
				KafkaTopic: idepTopic,
			},
			"surveillance": {
				Label:      "SURVEILLANCE",
				KafkaTopic: survTopic,
			},
		},
	}
}

func (m *gatewayMonitor) start() {
	if !lookupBoolEnvWithDefault("GATEWAY_MONITOR_ENABLED", true) {
		return
	}

	host := lookupEnvWithDefault("GATEWAY_MONITOR_HOST", "0.0.0.0")
	port := lookupIntEnvWithDefault("GATEWAY_MONITOR_PORT", 18081)
	authEnabled := lookupBoolEnvWithDefault("GATEWAY_MONITOR_AUTH_ENABLED", true)
	authUsername := lookupEnvWithDefault("GATEWAY_MONITOR_USERNAME", "aodsMon")
	authPassword := lookupEnvWithDefault("GATEWAY_MONITOR_PASSWORD", "Aero77Secret")
	m.basePath = normalizeMonitorBasePath(lookupEnvWithDefault("GATEWAY_MONITOR_BASE_PATH", "/aods-mon"))

	mux := http.NewServeMux()
	seenPatterns := map[string]struct{}{}
	register := func(pattern string, handler http.HandlerFunc) {
		if pattern == "" {
			return
		}
		if _, exists := seenPatterns[pattern]; exists {
			return
		}
		seenPatterns[pattern] = struct{}{}
		mux.HandleFunc(pattern, handler)
	}

	healthHandler := m.handleHealth
	routesHandler := m.withMonitorBasicAuth(authEnabled, authUsername, authPassword, m.handleRoutes)
	archivePageHandler := m.withMonitorBasicAuth(authEnabled, authUsername, authPassword, m.handleArchivePage)
	archiveSearchHandler := m.withMonitorBasicAuth(authEnabled, authUsername, authPassword, m.handleArchiveSearch)
	indexHandler := m.withMonitorBasicAuth(authEnabled, authUsername, authPassword, m.handleIndex)
	dispatchHandler := func(w http.ResponseWriter, r *http.Request) {
		switch monitorEndpointSuffix(r) {
		case "/health":
			healthHandler(w, r)
		case "/routes":
			routesHandler(w, r)
		case "/archive":
			archivePageHandler(w, r)
		case "/archive/search":
			archiveSearchHandler(w, r)
		default:
			indexHandler(w, r)
		}
	}

	register("/health", healthHandler)
	register(m.monitorPath("/health"), healthHandler)

	register("/routes", routesHandler)
	register(m.monitorPath("/routes"), routesHandler)

	register("/archive", archivePageHandler)
	register(m.monitorPath("/archive"), archivePageHandler)

	register("/archive/search", archiveSearchHandler)
	register(m.monitorPath("/archive/search"), archiveSearchHandler)

	register("/", dispatchHandler)
	register(m.monitorPath(""), indexHandler)
	register(m.monitorPath("/"), indexHandler)

	addr := host + ":" + strconv.Itoa(port)
	m.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		log.Printf(
			"Gateway monitor online at http://%s%s (basic_auth=%t user=%s)",
			addr,
			m.monitorPath(""),
			authEnabled,
			authUsername,
		)
		if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Gateway monitor stopped: %v", err)
		}
	}()

	m.startHourlySummaryLogger()
}

func (m *gatewayMonitor) route(name string) *gatewayRouteMonitor {
	return m.routes[name]
}

func (m *gatewayMonitor) handleHealth(w http.ResponseWriter, r *http.Request) {
	payload := map[string]interface{}{
		"status": "ok",
	}
	writeMonitorJSON(w, payload)
}

func (m *gatewayMonitor) handleRoutes(w http.ResponseWriter, r *http.Request) {
	routes := make([]map[string]interface{}, 0, len(m.routes))
	for _, key := range []string{"flight", "idep", "surveillance"} {
		route := m.routes[key]
		if route == nil {
			continue
		}
		routes = append(routes, route.snapshot())
	}

	payload := map[string]interface{}{
		"service":    "gateway",
		"started_at": m.startedAt.Format(time.RFC3339),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"routes":     routes,
	}
	if wantsHTML(r) {
		writeGatewayJSONPage(w, gatewayJSONPageView{
			Title:       "Gateway Routes",
			Description: "Route-level counters for FLMO, IDEP, and Surveillance consumers.",
			Payload:     payload,
			MainURL:     "./",
			RawJSONURL:  "./routes?format=json",
			Nav:         gatewayNavLinks("./routes"),
		})
		return
	}
	writeMonitorJSON(w, payload)
}

func writeMonitorJSON(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (m *gatewayMonitor) withMonitorBasicAuth(enabled bool, username string, password string, next http.HandlerFunc) http.HandlerFunc {
	if !enabled {
		return next
	}

	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || !secureStringEqual(user, username) || !secureStringEqual(pass, password) {
			w.Header().Set("WWW-Authenticate", `Basic realm="Gateway Monitor"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func secureStringEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (m *gatewayMonitor) startHourlySummaryLogger() {
	if m == nil || !lookupBoolEnvWithDefault("GATEWAY_SUMMARY_LOG_ENABLED", true) {
		return
	}

	interval := lookupDurationEnvWithDefault("GATEWAY_SUMMARY_LOG_INTERVAL", time.Hour)
	if interval <= 0 {
		interval = time.Hour
	}

	prev := map[string]gatewayRouteCounterSnapshot{}
	for key, route := range m.routes {
		if route == nil {
			continue
		}
		prev[key] = route.counterSnapshot()
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for tickAt := range ticker.C {
			windowEnd := tickAt.UTC()
			windowStart := windowEnd.Add(-interval)

			for _, key := range []string{"flight", "idep", "surveillance"} {
				route := m.routes[key]
				if route == nil {
					continue
				}

				now := route.counterSnapshot()
				before := prev[key]
				prev[key] = now

				log.Printf(
					"gateway hourly summary window=%s..%s route=%s topic=%s delta(received=%d processed=%d skipped=%d decode_errors=%d consumer_errors=%d) total(received=%d processed=%d skipped=%d decode_errors=%d consumer_errors=%d)",
					windowStart.Format(time.RFC3339),
					windowEnd.Format(time.RFC3339),
					route.Label,
					route.KafkaTopic,
					diffUint64(now.Received, before.Received),
					diffUint64(now.Processed, before.Processed),
					diffUint64(now.Skipped, before.Skipped),
					diffUint64(now.DecodeErrors, before.DecodeErrors),
					diffUint64(now.ConsumerErrors, before.ConsumerErrors),
					now.Received,
					now.Processed,
					now.Skipped,
					now.DecodeErrors,
					now.ConsumerErrors,
				)
			}
		}
	}()
}

func diffUint64(current, previous uint64) uint64 {
	if current < previous {
		return current
	}
	return current - previous
}

func lookupBoolEnvWithDefault(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func lookupIntEnvWithDefault(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		parsed, err := strconv.Atoi(value)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func lookupDurationEnvWithDefault(key string, fallback time.Duration) time.Duration {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return fallback
}

func formatUnixTimestamp(unix int64) string {
	if unix <= 0 {
		return ""
	}
	return time.Unix(unix, 0).UTC().Format(time.RFC3339)
}

func normalizeMonitorBasePath(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" || s == "/" {
		return ""
	}
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	s = strings.TrimRight(s, "/")
	if s == "" || s == "/" {
		return ""
	}
	return s
}

func joinMonitorPath(basePath string, suffix string) string {
	base := normalizeMonitorBasePath(basePath)
	switch strings.TrimSpace(suffix) {
	case "":
		if base == "" {
			return "/"
		}
		return base
	case "/":
		if base == "" {
			return "/"
		}
		return base + "/"
	}

	s := suffix
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	if base == "" {
		return s
	}
	return base + s
}

func (m *gatewayMonitor) monitorPath(suffix string) string {
	if m == nil {
		return joinMonitorPath("/aods-mon", suffix)
	}
	return joinMonitorPath(m.basePath, suffix)
}

func monitorEndpointSuffix(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}
	path := strings.TrimRight(strings.TrimSpace(r.URL.Path), "/")
	if path == "" {
		return ""
	}
	switch {
	case strings.HasSuffix(path, "/archive/search"):
		return "/archive/search"
	case strings.HasSuffix(path, "/archive"):
		return "/archive"
	case strings.HasSuffix(path, "/routes"):
		return "/routes"
	case strings.HasSuffix(path, "/health"):
		return "/health"
	default:
		return ""
	}
}

func (m *gatewayMonitor) monitorBasePathForRequest(r *http.Request, endpointSuffix string) string {
	if r == nil {
		return normalizeMonitorBasePath(m.basePath)
	}

	if v := normalizeMonitorBasePath(r.Header.Get("X-Forwarded-Prefix")); v != "" {
		return v
	}

	for _, originalPath := range []string{
		strings.TrimSpace(r.Header.Get("X-Original-URI")),
		strings.TrimSpace(r.Header.Get("X-Rewrite-URL")),
	} {
		if base, ok := deriveMonitorBasePathFromPath(originalPath, endpointSuffix); ok {
			return base
		}
	}

	if base, ok := deriveMonitorBasePathFromPath(r.URL.Path, endpointSuffix); ok {
		return base
	}

	return normalizeMonitorBasePath(m.basePath)
}

func deriveMonitorBasePathFromPath(path string, endpointSuffix string) (string, bool) {
	s := strings.TrimSpace(path)
	if s == "" {
		return "", false
	}

	if endpointSuffix == "" {
		if s == "/" {
			return "", true
		}
		return normalizeMonitorBasePath(s), true
	}

	cleanPath := strings.TrimRight(s, "/")
	cleanEndpoint := strings.TrimRight(strings.TrimSpace(endpointSuffix), "/")
	if cleanPath == "" {
		cleanPath = "/"
	}
	if cleanEndpoint == "" {
		return normalizeMonitorBasePath(cleanPath), true
	}
	if cleanPath == cleanEndpoint {
		return "", true
	}
	if strings.HasSuffix(cleanPath, cleanEndpoint) {
		return normalizeMonitorBasePath(strings.TrimSuffix(cleanPath, cleanEndpoint)), true
	}
	return "", false
}
