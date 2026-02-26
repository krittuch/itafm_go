package app

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
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
}

func newGatewayMonitor(flightTopic, idepTopic, survTopic string, archiveDB *sql.DB) *gatewayMonitor {
	return &gatewayMonitor{
		startedAt: time.Now().UTC(),
		archiveDB: archiveDB,
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

	mux := http.NewServeMux()
	mux.HandleFunc("/health", m.handleHealth)
	mux.HandleFunc("/routes", m.handleRoutes)
	mux.HandleFunc("/archive", m.handleArchivePage)
	mux.HandleFunc("/archive/search", m.handleArchiveSearch)
	mux.HandleFunc("/", m.handleIndex)

	addr := host + ":" + strconv.Itoa(port)
	m.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		log.Printf("Gateway monitor online at http://%s", addr)
		if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Gateway monitor stopped: %v", err)
		}
	}()
}

func (m *gatewayMonitor) route(name string) *gatewayRouteMonitor {
	return m.routes[name]
}

func (m *gatewayMonitor) handleHealth(w http.ResponseWriter, r *http.Request) {
	payload := map[string]interface{}{
		"status":      "ok",
		"started_at":  m.startedAt.Format(time.RFC3339),
		"route_count": len(m.routes),
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}
	if wantsHTML(r) {
		writeGatewayJSONPage(w, gatewayJSONPageView{
			Title:       "Gateway Health",
			Description: "Health status for the gateway monitor service.",
			Payload:     payload,
			RawJSONURL:  "/health?format=json",
			Nav:         gatewayNavLinks("/health"),
		})
		return
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
			RawJSONURL:  "/routes?format=json",
			Nav:         gatewayNavLinks("/routes"),
		})
		return
	}
	writeMonitorJSON(w, payload)
}

func writeMonitorJSON(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
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

func formatUnixTimestamp(unix int64) string {
	if unix <= 0 {
		return ""
	}
	return time.Unix(unix, 0).UTC().Format(time.RFC3339)
}
