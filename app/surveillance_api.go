package app

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"aerothai/itafm/model"
)

type surveillancePositionReader interface {
	GetSurveillancePosition(ctx context.Context, callsigns []string) (*model.SurveillancePosition, error)
}

type surveillancePositionResponse struct {
	CallSign     string  `json:"callsign"`
	Departure    string  `json:"departure"`
	Destination  string  `json:"destination"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	Altitude     float64 `json:"altitude"`
	Heading      float64 `json:"heading"`
	PositionTime string  `json:"position_time"`
	LastUpdate   string  `json:"last_update"`
}

var surveillanceAPICallSignPattern = regexp.MustCompile(`^[A-Z0-9]{2,10}$`)

const surveillanceAPIQueryTimeout = 5 * time.Second

func surveillanceAPIKeysFromEnv() []string {
	keys := []string{}
	for _, key := range strings.Split(lookupEnvWithDefault("GATEWAY_SURVEILLANCE_API_KEYS", ""), ",") {
		if key = strings.TrimSpace(key); key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

// withSurveillanceAPIKey is separate from the monitor's basic auth. The key is
// accepted only in a header, so it never ends up in access logs as part of a URL.
func withSurveillanceAPIKey(keys []string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if len(keys) == 0 {
			writeSurveillanceAPIError(w, http.StatusServiceUnavailable, "surveillance API disabled")
			return
		}
		given := strings.TrimSpace(r.Header.Get("X-API-Key"))
		matched := false
		for _, key := range keys {
			if secureStringEqual(given, key) {
				matched = true
			}
		}
		if given == "" || !matched {
			writeSurveillanceAPIError(w, http.StatusUnauthorized, "missing or invalid X-API-Key")
			return
		}
		next(w, r)
	}
}

func handleSurveillancePosition(reader surveillancePositionReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeSurveillanceAPIError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		callsign := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("callsign")))
		if !surveillanceAPICallSignPattern.MatchString(callsign) {
			writeSurveillanceAPIError(w, http.StatusBadRequest, "callsign must be 2-10 letters or digits, e.g. TG661 or THA661")
			return
		}
		if reader == nil {
			writeSurveillanceAPIError(w, http.StatusServiceUnavailable, "surveillance database unavailable")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), surveillanceAPIQueryTimeout)
		defer cancel()
		position, err := reader.GetSurveillancePosition(ctx, surveillanceAPICallSignCandidates(callsign))
		if err != nil {
			log.Printf("surveillance API lookup failed callsign=%s: %v", callsign, err)
			writeSurveillanceAPIError(w, http.StatusInternalServerError, "lookup failed")
			return
		}
		if position == nil {
			writeSurveillanceAPIError(w, http.StatusNotFound, "no position for callsign "+callsign)
			return
		}

		resp := surveillancePositionResponse{
			CallSign:     position.CallSign,
			Departure:    position.Departure,
			Destination:  position.Destination,
			Latitude:     position.Lat,
			Longitude:    position.Lon,
			Altitude:     position.Altitude,
			Heading:      position.Heading,
			PositionTime: position.PositionTime,
		}
		if !position.UpdatedAt.IsZero() {
			resp.LastUpdate = position.UpdatedAt.UTC().Format(time.RFC3339)
		}
		writeMonitorJSON(w, resp)
	}
}

// Rows are stored under the IATA callsign, so an ICAO callsign (THA661) is
// also looked up as its IATA form (TG661).
func surveillanceAPICallSignCandidates(callsign string) []string {
	candidates := []string{callsign}
	if iata, ok := ConvertToIATA(callsign); ok && iata != callsign {
		candidates = append(candidates, iata)
	}
	return candidates
}

func writeSurveillanceAPIError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
