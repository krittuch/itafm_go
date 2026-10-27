package app

import (
	"container/list"
	"sort"
	"strings"
	"sync"
	"time"

	"aerothai/itafm/model"
)

type surveillanceOutcome string

const (
	surveillanceOutcomeQueued      surveillanceOutcome = "queued"
	surveillanceOutcomeDuplicate   surveillanceOutcome = "duplicate"
	surveillanceOutcomeNotVTBS     surveillanceOutcome = "not_vtbs"
	surveillanceOutcomeIATAFailed  surveillanceOutcome = "iata_failed"
	surveillanceOutcomeDecodeError surveillanceOutcome = "decode_error"
	surveillanceOutcomeRejected    surveillanceOutcome = "rejected"
)

var surveillanceOutcomes = []surveillanceOutcome{
	surveillanceOutcomeQueued,
	surveillanceOutcomeDuplicate,
	surveillanceOutcomeNotVTBS,
	surveillanceOutcomeIATAFailed,
	surveillanceOutcomeDecodeError,
	surveillanceOutcomeRejected,
}

func (o surveillanceOutcome) written() bool {
	return o == surveillanceOutcomeQueued || o == surveillanceOutcomeDuplicate
}

type surveillanceReceiveResult struct {
	Outcome      surveillanceOutcome
	RawCallSign  string
	IATACallSign string
	Data         *model.AODSSurveillance
}

// Payload strings come from an external feed, so each is truncated to keep
// the per-track memory bounded.
const surveillanceTrackMaxFieldLen = 32

type surveillanceTrack struct {
	CallSign        string              `json:"callsign"`
	IATACallSign    string              `json:"iata_callsign"`
	Outcome         surveillanceOutcome `json:"outcome"`
	Departure       string              `json:"departure"`
	Destination     string              `json:"destination"`
	AircraftType    string              `json:"aircraft_type"`
	SSRCode         string              `json:"ssr_code"`
	AircraftAddress string              `json:"aircraft_address"`
	Lat             float64             `json:"lat"`
	Lon             float64             `json:"lon"`
	Altitude        float64             `json:"altitude"`
	GroundSpeed     float64             `json:"ground_speed"`
	Heading         float64             `json:"heading"`
	TrackNumber     int                 `json:"track_number"`
	DateTime        string              `json:"datetime"`
	Messages        uint64              `json:"messages"`
	LastSeen        time.Time           `json:"last_seen"`
}

type surveillanceTrackFilter struct {
	CallSign        string `json:"callsign"`
	Departure       string `json:"departure"`
	Destination     string `json:"destination"`
	AircraftType    string `json:"aircraft_type"`
	SSRCode         string `json:"ssr_code"`
	AircraftAddress string `json:"aircraft_address"`
	Outcome         string `json:"outcome"`
	Sort            string `json:"sort"`
	Limit           int    `json:"limit"`
}

type surveillanceTrackSnapshot struct {
	Tracks        []surveillanceTrack            `json:"tracks"`
	Matched       int                            `json:"matched"`
	Stored        int                            `json:"stored"`
	MaxTracks     int                            `json:"max_tracks"`
	TTLSeconds    int64                          `json:"ttl_seconds"`
	Evicted       uint64                         `json:"evicted"`
	OutcomeCounts map[surveillanceOutcome]uint64 `json:"outcome_counts"`
}

// surveillanceTrackStore keeps the latest message per callsign for the
// monitor page. Memory is bounded by maxTracks (least recently seen evicted
// first) and ttl (tracks not seen within ttl are dropped).
type surveillanceTrackStore struct {
	maxTracks int
	ttl       time.Duration
	now       func() time.Time

	mu            sync.Mutex
	order         *list.List
	byCallSign    map[string]*list.Element
	outcomeCounts map[surveillanceOutcome]uint64
	evicted       uint64
}

func newSurveillanceTrackStore(maxTracks int, ttl time.Duration) *surveillanceTrackStore {
	if maxTracks <= 0 {
		maxTracks = 2000
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return &surveillanceTrackStore{
		maxTracks:     maxTracks,
		ttl:           ttl,
		now:           time.Now,
		order:         list.New(),
		byCallSign:    map[string]*list.Element{},
		outcomeCounts: map[surveillanceOutcome]uint64{},
	}
}

func (s *surveillanceTrackStore) record(result surveillanceReceiveResult) {
	if s == nil {
		return
	}

	now := s.now().UTC()
	key := truncateSurveillanceField(strings.TrimSpace(result.RawCallSign))

	s.mu.Lock()
	defer s.mu.Unlock()

	s.outcomeCounts[result.Outcome]++
	s.pruneExpiredLocked(now)
	if key == "" {
		return
	}

	var track *surveillanceTrack
	if elem, ok := s.byCallSign[key]; ok {
		track = elem.Value.(*surveillanceTrack)
		s.order.MoveToFront(elem)
	} else {
		track = &surveillanceTrack{CallSign: key}
		s.byCallSign[key] = s.order.PushFront(track)
		for s.order.Len() > s.maxTracks {
			s.removeLocked(s.order.Back())
			s.evicted++
		}
	}

	track.Outcome = result.Outcome
	track.IATACallSign = truncateSurveillanceField(result.IATACallSign)
	track.Messages++
	track.LastSeen = now
	if d := result.Data; d != nil {
		track.Departure = truncateSurveillanceField(d.Departure)
		track.Destination = truncateSurveillanceField(d.Destination)
		track.AircraftType = truncateSurveillanceField(d.AircraftType)
		track.SSRCode = truncateSurveillanceField(d.SSRCode)
		track.AircraftAddress = truncateSurveillanceField(d.AircraftAddress)
		track.Lat = d.Lat
		track.Lon = d.Lon
		track.Altitude = d.Altitude
		track.GroundSpeed = d.GroundSpeed
		track.Heading = d.Heading
		track.TrackNumber = d.TrackNumber
		track.DateTime = truncateSurveillanceField(d.DateTime)
	}
}

func (s *surveillanceTrackStore) pruneExpiredLocked(now time.Time) {
	cutoff := now.Add(-s.ttl)
	for elem := s.order.Back(); elem != nil; elem = s.order.Back() {
		if !elem.Value.(*surveillanceTrack).LastSeen.Before(cutoff) {
			return
		}
		s.removeLocked(elem)
	}
}

func (s *surveillanceTrackStore) removeLocked(elem *list.Element) {
	if elem == nil {
		return
	}
	track := s.order.Remove(elem).(*surveillanceTrack)
	delete(s.byCallSign, track.CallSign)
}

func (s *surveillanceTrackStore) snapshot(filter surveillanceTrackFilter) surveillanceTrackSnapshot {
	result := surveillanceTrackSnapshot{
		Tracks:        []surveillanceTrack{},
		OutcomeCounts: map[surveillanceOutcome]uint64{},
	}
	if s == nil {
		return result
	}

	s.mu.Lock()
	s.pruneExpiredLocked(s.now().UTC())
	for elem := s.order.Front(); elem != nil; elem = elem.Next() {
		track := elem.Value.(*surveillanceTrack)
		if filter.matches(track) {
			result.Tracks = append(result.Tracks, *track)
		}
	}
	result.Stored = s.order.Len()
	result.MaxTracks = s.maxTracks
	result.TTLSeconds = int64(s.ttl / time.Second)
	result.Evicted = s.evicted
	for _, outcome := range surveillanceOutcomes {
		result.OutcomeCounts[outcome] = s.outcomeCounts[outcome]
	}
	s.mu.Unlock()

	result.Matched = len(result.Tracks)
	sortSurveillanceTracks(result.Tracks, filter.Sort)
	if filter.Limit > 0 && len(result.Tracks) > filter.Limit {
		result.Tracks = result.Tracks[:filter.Limit]
	}
	return result
}

func (f surveillanceTrackFilter) matches(t *surveillanceTrack) bool {
	if f.CallSign != "" && !containsFold(t.CallSign, f.CallSign) && !containsFold(t.IATACallSign, f.CallSign) {
		return false
	}
	if f.Departure != "" && !strings.EqualFold(t.Departure, f.Departure) {
		return false
	}
	if f.Destination != "" && !strings.EqualFold(t.Destination, f.Destination) {
		return false
	}
	if f.AircraftType != "" && !containsFold(t.AircraftType, f.AircraftType) {
		return false
	}
	if f.SSRCode != "" && !containsFold(t.SSRCode, f.SSRCode) {
		return false
	}
	if f.AircraftAddress != "" && !containsFold(t.AircraftAddress, f.AircraftAddress) {
		return false
	}
	if f.Outcome != "" && string(t.Outcome) != f.Outcome {
		return false
	}
	return true
}

func sortSurveillanceTracks(tracks []surveillanceTrack, by string) {
	switch by {
	case "callsign":
		sort.SliceStable(tracks, func(i, j int) bool { return tracks[i].CallSign < tracks[j].CallSign })
	case "altitude":
		sort.SliceStable(tracks, func(i, j int) bool { return tracks[i].Altitude > tracks[j].Altitude })
	}
}

func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToUpper(s), strings.ToUpper(substr))
}

func truncateSurveillanceField(s string) string {
	if len(s) > surveillanceTrackMaxFieldLen {
		// Clone so the slice does not pin the oversized original string.
		return strings.Clone(s[:surveillanceTrackMaxFieldLen])
	}
	return s
}
