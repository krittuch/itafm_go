package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"aerothai/itafm/controller"
	"aerothai/itafm/model"
)

// Compiled once at package init instead of per message; these run on every
// Kafka message on the flight movement and IDEP topics.
var (
	airlineCodeRegex        = regexp.MustCompile(`^[A-Z]{3}`)
	numberRegex             = regexp.MustCompile(`\d+`)
	sixDigitDateRegex       = regexp.MustCompile(`^\d{6}$`)
	isoDateRegex            = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	chgDestinationTimeRegex = regexp.MustCompile(`-\d+/([A-Z]{4})(\d{4})`)
	chgRegisterRegex        = regexp.MustCompile(`\bREG/([A-Z0-9-]+)`)
	fplDOFRegex             = regexp.MustCompile(`\bDOF/(\d{6})\b`)
	chgAircraftRegex        = regexp.MustCompile(`-\s*9/([A-Z0-9-]+)/[A-Z0-9-]+`)
	firTokenRegex           = regexp.MustCompile(`^[A-Z]{4}(\d{4})$`)
	rawDurationRegex        = regexp.MustCompile(`^(\d{4})$`)
)

func onFPLReceive(
	body []byte,
	db *sql.DB,
	flightController *controller.FlightController) bool {
	fplData := model.FlightPlan{}
	err := json.Unmarshal(body, &fplData)
	if err != nil {
		return false
	}

	if dof, ok := parseFPLDOFFromItem18(fplData.ITEM18); ok {
		fplData.DOF = dof
	}

	if len(fplData.CALLSIGN) < 4 || len(fplData.DOF) < 6 || len(fplData.ETD) < 4 {
		return false
	}

	register, hasRegister := parseCHGRegisterFromItem(fplData.ITEM18)

	postFlight := model.PostFlight{
		AircraftType: fplData.ACTYPE,
		NextStation:  fplData.DESTINATION,
		PrevStation:  fplData.DEPARTURE,
		Register:     register,
	}

	icaoCode := airlineCodeRegex.FindString(fplData.CALLSIGN)

	iata, success := ConvertToIATA(icaoCode)

	if !success {
		return false
	}

	matchString := numberRegex.FindString(fplData.CALLSIGN)
	if len(matchString) <= 0 {
		return false
	}

	flightNumber := strings.TrimLeft(matchString, "0")

	timeStr := fplData.ETD

	dateOfFlight := "20" + fplData.DOF[:2] + "-" + fplData.DOF[2:4] + "-" + fplData.DOF[4:]
	std := strings.Join([]string{dateOfFlight, " ", timeStr[:2], ":", timeStr[2:4], ":00+00"}, "")

	postFlight.FlightNumber = fmt.Sprint(iata, " ", flightNumber)

	beforeFlight, beforeErr := flightController.GetFlightByTypeAndSchedule(postFlight.FlightNumber, "DEP", std)
	usedDOFETDFallback := false
	if beforeErr != nil {
		fallbackFlight, fallbackErr := flightController.FindDepartureFlightByDestinationAndDate(
			postFlight.FlightNumber,
			postFlight.NextStation,
			dateOfFlight,
		)
		if fallbackErr == nil {
			oldSchedule := formatFlightTimeForChangeLog(fallbackFlight.ScheduleFlightTime)
			if oldSchedule != "" && oldSchedule != std {
				flightController.UpdateScheduleFlightTimeByID(fallbackFlight.ID, std)
				usedDOFETDFallback = true
			}
			beforeFlight = fallbackFlight
			beforeErr = nil
		}
	}

	flightController.UpdateUncanceledFlight(postFlight.FlightNumber, std)
	if hasRegister {
		flightController.UpdateRegister(postFlight.FlightNumber, postFlight.Register, std)
	}

	hasEET := false
	if estimateTime, ok := buildCHGArrivalEstimateFromEET(fplData.ITEM18, fplData.DOF, fplData.ETD); ok {
		hasEET = true
		flightController.UpdateDepartureEstimateFlightBySchedule(postFlight.FlightNumber, std, estimateTime)
	}

	if beforeErr == nil {
		if afterFlight, afterErr := flightController.GetFlightByTypeAndSchedule(postFlight.FlightNumber, "DEP", std); afterErr == nil {
			if usedDOFETDFallback {
				insertChangeLogIfChanged(
					db,
					"schedule_flight_time",
					beforeFlight,
					formatFlightTimeForChangeLog(beforeFlight.ScheduleFlightTime),
					formatFlightTimeForChangeLog(afterFlight.ScheduleFlightTime),
				)
			}

			insertChangeLogIfChanged(
				db,
				"canceled",
				beforeFlight,
				strconv.FormatBool(beforeFlight.Canceled),
				strconv.FormatBool(afterFlight.Canceled),
			)

			if hasRegister {
				insertChangeLogIfChanged(
					db,
					"ac_register",
					beforeFlight,
					beforeFlight.ACRegister,
					afterFlight.ACRegister,
				)
			}

			if hasEET {
				insertChangeLogIfChanged(
					db,
					"estimate_flight_time",
					beforeFlight,
					formatFlightTimeForChangeLog(beforeFlight.EstimateFlightTime),
					formatFlightTimeForChangeLog(afterFlight.EstimateFlightTime),
				)
			}
		}
	}

	return true
}

func onCMDReceive(
	body []byte,
	db *sql.DB,
	flightController *controller.FlightController) bool {
	fmvData := model.AODSFlightMovement{}
	err := json.Unmarshal(body, &fmvData)
	if err != nil {
		return false
	}

	// Change icao to iata

	airlineController := controller.NewAirlineController(db)

	icaoCode := airlineCodeRegex.FindString(fmvData.CALLSIGN)

	airline, errAirline := airlineController.GetAirline(icaoCode)

	if errAirline != nil {
		return false
	}

	flightNumber := fmt.Sprint(airline.IATA, " ", fmvData.CALLSIGN[3:])

	// Create ATD
	dateOfFlight := ""
	timeStr := ""
	std := ""

	if fmvData.CMD == "DEP" {
		timeStr = fmvData.TIME1
		if len(timeStr) < 4 {
			return false
		}
		var ok bool
		dateOfFlight, ok = normalizeAODSDOFDate(fmvData.DOF)
		if !ok {
			return false
		}
		std = strings.Join([]string{dateOfFlight, " ", timeStr[:2], ":", timeStr[2:4], ":00+00"}, "")
	} else if fmvData.CMD == "ARR" {
		timeStr = fmvData.TIME2
		if len(timeStr) < 4 {
			return false
		}
		if parsedDate, ok := normalizeAODSDOFDate(fmvData.DOF); ok {
			dateOfFlight = parsedDate
		} else {
			t := time.Now().UTC()
			timeString := t.Format("2006-01-02 15:04:05")
			dateOfFlight = strings.Split(timeString, " ")[0]
		}
		std = strings.Join([]string{
			dateOfFlight, " ",
			timeStr[:2], ":",
			timeStr[2:4], ":00+00",
		}, "")
	} else {
		return false
	}

	if fmvData.CMD == "DEP" {
		beforeFlight, beforeErr := flightController.GetFlightByTypeAndDate(flightNumber, "DEP", dateOfFlight)
		flightController.UpdateDepartureFlight(flightNumber, dateOfFlight, std)
		if beforeErr == nil {
			if afterFlight, afterErr := flightController.GetFlightByTypeAndDate(flightNumber, "DEP", dateOfFlight); afterErr == nil {
				insertDebugChangeLogIfChanged(
					db,
					"actual_flight_time",
					beforeFlight,
					formatFlightTimeForChangeLog(beforeFlight.ActualFlightTime),
					formatFlightTimeForChangeLog(afterFlight.ActualFlightTime),
				)
			}
		}
	} else if fmvData.CMD == "ARR" {
		beforeFlight, beforeErr := flightController.GetFlightByTypeAndDate(flightNumber, "ARR", dateOfFlight)
		flightController.UpdateArrivalFlight(flightNumber, dateOfFlight, std)
		if beforeErr == nil {
			if afterFlight, afterErr := flightController.GetFlightByTypeAndDate(flightNumber, "ARR", dateOfFlight); afterErr == nil {
				insertDebugChangeLogIfChanged(
					db,
					"actual_flight_time",
					beforeFlight,
					formatFlightTimeForChangeLog(beforeFlight.ActualFlightTime),
					formatFlightTimeForChangeLog(afterFlight.ActualFlightTime),
				)
			}
		}
	}

	return true
}

func normalizeAODSDOFDate(dof string) (string, bool) {
	trimmed := strings.TrimSpace(dof)
	if len(trimmed) == 6 {
		if !sixDigitDateRegex.MatchString(trimmed) {
			return "", false
		}
		full := "20" + trimmed
		return full[:4] + "-" + full[4:6] + "-" + full[6:], true
	}

	if len(trimmed) == 10 {
		if isoDateRegex.MatchString(trimmed) {
			return trimmed, true
		}
	}

	return "", false
}

func onCNLReceive(
	body []byte,
	db *sql.DB,
	flightController *controller.FlightController) {
	fmvData := model.AODSFlightMovement{}
	err := json.Unmarshal(body, &fmvData)
	if err != nil {
		return
	}

	// Change icao to iata

	airlineController := controller.NewAirlineController(db)

	icaoCode := airlineCodeRegex.FindString(fmvData.CALLSIGN)

	airline, errAirline := airlineController.GetAirline(icaoCode)
	if errAirline != nil {
		return
	}

	if len(fmvData.TIME1) < 4 || len(fmvData.DOF) < 6 {
		return
	}

	dateOfFlight := "20" + fmvData.DOF
	dateOfFlight = dateOfFlight[:4] + "-" + dateOfFlight[4:6] + "-" + dateOfFlight[6:]
	std := strings.Join([]string{dateOfFlight, " ", fmvData.TIME1[:2], ":", fmvData.TIME1[2:4], ":00+00"}, "")
	flightNumber := fmt.Sprint(airline.IATA, " ", fmvData.CALLSIGN[3:])

	beforeFlight, beforeErr := flightController.GetFlightByTypeAndSchedule(flightNumber, "DEP", std)
	flightController.UpdateCanceledFlight(flightNumber, std)
	if beforeErr == nil {
		if afterFlight, afterErr := flightController.GetFlightByTypeAndSchedule(flightNumber, "DEP", std); afterErr == nil {
			insertChangeLogIfChanged(
				db,
				"canceled",
				beforeFlight,
				strconv.FormatBool(beforeFlight.Canceled),
				strconv.FormatBool(afterFlight.Canceled),
			)
		}
	}
}

func onDLYReceive(
	body []byte,
	db *sql.DB,
	flightController *controller.FlightController) {
	fmvData := model.AODSFlightMovement{}
	err := json.Unmarshal(body, &fmvData)
	if err != nil {
		return
	}

	// Change icao to iata

	airlineController := controller.NewAirlineController(db)

	icaoCode := airlineCodeRegex.FindString(fmvData.CALLSIGN)

	airline, errAirline := airlineController.GetAirline(icaoCode)
	if errAirline != nil {
		return
	}

	if len(fmvData.TIME1) < 4 || len(fmvData.DOF) < 6 {
		return
	}

	dateOfFlight := "20" + fmvData.DOF
	dateOfFlight = dateOfFlight[:4] + "-" + dateOfFlight[4:6] + "-" + dateOfFlight[6:]
	std := strings.Join([]string{dateOfFlight, " ", fmvData.TIME1[:2], ":", fmvData.TIME1[2:4], ":00+00"}, "")
	flightNumber := fmt.Sprint(airline.IATA, " ", fmvData.CALLSIGN[3:])

	beforeFlight, beforeErr := flightController.GetFlightByTypeAndSchedule(flightNumber, "DEP", std)
	flightController.UpdateDelayedFlight(flightNumber, std)
	if beforeErr == nil {
		if afterFlight, afterErr := flightController.GetFlightByTypeAndSchedule(flightNumber, "DEP", std); afterErr == nil {
			insertChangeLogIfChanged(
				db,
				"delayed",
				beforeFlight,
				strconv.FormatBool(beforeFlight.Delayed),
				strconv.FormatBool(afterFlight.Delayed),
			)
		}
	}
}

func onCHGReceive(
	body []byte,
	db *sql.DB,
	flightController *controller.FlightController) bool {
	fmvData := model.AODSFlightMovement{}
	err := json.Unmarshal(body, &fmvData)
	if err != nil {
		return false
	}

	airlineController := controller.NewAirlineController(db)
	if len(fmvData.CALLSIGN) < 4 {
		return false
	}

	itemRaw := strings.TrimSpace(fmvData.ITEM)
	if itemRaw == "" {
		itemRaw = strings.TrimSpace(fmvData.ITEM18)
	}
	if itemRaw == "" {
		return false
	}
	updated := false

	destinationCode, hhmm, ok := parseCHGDestinationTimeFromItem(itemRaw)
	if ok {
		if len(fmvData.DOF) < 6 {
			return false
		}

		messageDestination := strings.ToUpper(strings.TrimSpace(fmvData.DESTINATION))
		if messageDestination != "" && destinationCode != messageDestination {
			// Start with destination ETA changes only when ITEM target matches the FLMO destination.
			return false
		}
	}

	icaoCode := airlineCodeRegex.FindString(fmvData.CALLSIGN)
	if len(icaoCode) != 3 {
		return false
	}
	airline, errAirline := airlineController.GetAirline(icaoCode)
	if errAirline != nil {
		return false
	}
	if len(fmvData.DOF) < 6 {
		return false
	}
	dateOfFlight := "20" + fmvData.DOF
	dateOfFlight = dateOfFlight[:4] + "-" + dateOfFlight[4:6] + "-" + dateOfFlight[6:]
	flightNumber := fmt.Sprint(airline.IATA, " ", fmvData.CALLSIGN[3:])

	if ok {
		beforeFlight, beforeErr := flightController.FindDepartureFlightByDestinationAndDate(flightNumber, destinationCode, dateOfFlight)
		estimateTime := strings.Join([]string{dateOfFlight, " ", hhmm[:2], ":", hhmm[2:4], ":00+00"}, "")
		flightController.UpdateEstimateFlightByDestination(flightNumber, destinationCode, dateOfFlight, estimateTime)
		if beforeErr == nil {
			if afterFlight, afterErr := flightController.FindDepartureFlightByDestinationAndDate(flightNumber, destinationCode, dateOfFlight); afterErr == nil {
				insertDebugChangeLogIfChanged(
					db,
					"estimate_flight_time",
					beforeFlight,
					formatFlightTimeForChangeLog(beforeFlight.EstimateFlightTime),
					formatFlightTimeForChangeLog(afterFlight.EstimateFlightTime),
				)
			}
		}
		updated = true
	}

	hasSTD := len(fmvData.TIME1) >= 4
	std := ""
	if hasSTD {
		std = strings.Join([]string{dateOfFlight, " ", fmvData.TIME1[:2], ":", fmvData.TIME1[2:4], ":00+00"}, "")
	}

	if reg, ok := parseCHGRegisterFromItem(itemRaw); ok && hasSTD {
		beforeFlight, beforeErr := flightController.GetFlightByTypeAndSchedule(flightNumber, "DEP", std)
		flightController.UpdateRegister(flightNumber, reg, std)
		if beforeErr == nil {
			if afterFlight, afterErr := flightController.GetFlightByTypeAndSchedule(flightNumber, "DEP", std); afterErr == nil {
				insertDebugChangeLogIfChanged(
					db,
					"ac_register",
					beforeFlight,
					beforeFlight.ACRegister,
					afterFlight.ACRegister,
				)
			}
		}
		updated = true
	}

	if aircraft, ok := parseCHGAircraftFromItem(itemRaw); ok && hasSTD {
		beforeFlight, beforeErr := flightController.GetFlightByTypeAndSchedule(flightNumber, "DEP", std)
		flightController.UpdateAircraft(flightNumber, aircraft, std)
		if beforeErr == nil {
			if afterFlight, afterErr := flightController.GetFlightByTypeAndSchedule(flightNumber, "DEP", std); afterErr == nil {
				insertChangeLogIfChanged(
					db,
					"aircraft",
					beforeFlight,
					beforeFlight.AircraftType,
					afterFlight.AircraftType,
				)
			}
		}
		updated = true
	}

	if arrivalEstimate, ok := buildCHGArrivalEstimateFromEET(itemRaw, fmvData.DOF, fmvData.TIME1); ok {
		departureCode := strings.ToUpper(strings.TrimSpace(fmvData.DEPARTURE))
		destinationCode := strings.ToUpper(strings.TrimSpace(fmvData.DESTINATION))
		beforeFlight, beforeErr := flightController.FindArrivalFlightByRouteAndDate(
			flightNumber,
			departureCode,
			destinationCode,
			dateOfFlight,
		)
		flightController.UpdateArrivalEstimateFlightByRoute(
			flightNumber,
			departureCode,
			destinationCode,
			dateOfFlight,
			arrivalEstimate,
		)
		if beforeErr == nil {
			if afterFlight, afterErr := flightController.FindArrivalFlightByRouteAndDate(
				flightNumber,
				departureCode,
				destinationCode,
				dateOfFlight,
			); afterErr == nil {
				insertDebugChangeLogIfChanged(
					db,
					"estimate_flight_time",
					beforeFlight,
					formatFlightTimeForChangeLog(beforeFlight.EstimateFlightTime),
					formatFlightTimeForChangeLog(afterFlight.EstimateFlightTime),
				)
			}
		}
		updated = true
	}

	return updated
}

func parseCHGDestinationTimeFromItem(item string) (string, string, bool) {
	normalized := strings.ToUpper(strings.TrimSpace(item))
	// Example: "-13/VTBD0045" (may have trailing CRLF or more tokens)
	match := chgDestinationTimeRegex.FindStringSubmatch(normalized)
	if len(match) != 3 {
		return "", "", false
	}
	return match[1], match[2], true
}

func parseCHGRegisterFromItem(item string) (string, bool) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(item, "\r", " "), "\n", " "))
	match := chgRegisterRegex.FindStringSubmatch(normalized)
	if len(match) != 2 {
		return "", false
	}
	return match[1], true
}

func parseFPLDOFFromItem18(item18 string) (string, bool) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(item18, "\r", " "), "\n", " "))
	match := fplDOFRegex.FindStringSubmatch(normalized)
	if len(match) != 2 {
		return "", false
	}
	return match[1], true
}

func parseCHGAircraftFromItem(item string) (string, bool) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(item, "\r", " "), "\n", " "))
	match := chgAircraftRegex.FindStringSubmatch(normalized)
	if len(match) != 2 {
		return "", false
	}
	return match[1], true
}

func buildCHGArrivalEstimateFromEET(item string, dof string, departureHHMM string) (string, bool) {
	if len(dof) < 6 || len(departureHHMM) < 4 {
		return "", false
	}

	lastDurationHHMM, ok := parseCHGLastEETDurationHHMM(item)
	if !ok {
		return "", false
	}

	baseTime, ok := parseCHGUTCDateTime(dof, departureHHMM)
	if !ok {
		return "", false
	}

	duration, ok := parseCHGHHMMDuration(lastDurationHHMM)
	if !ok {
		return "", false
	}

	return baseTime.Add(duration).UTC().Format("2006-01-02 15:04:05+00"), true
}

func parseCHGLastEETDurationHHMM(item string) (string, bool) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(item, "\r", " "), "\n", " "))
	idx := strings.Index(normalized, "EET/")
	if idx < 0 {
		return "", false
	}

	section := normalized[idx+4:]
	tokens := strings.Fields(section)
	if len(tokens) == 0 {
		return "", false
	}

	last := ""

	for _, token := range tokens {
		if strings.Contains(token, "/") {
			break
		}
		if match := firTokenRegex.FindStringSubmatch(token); len(match) == 2 {
			last = match[1]
			continue
		}
		if match := rawDurationRegex.FindStringSubmatch(token); len(match) == 2 {
			last = match[1]
			continue
		}
	}

	if last == "" {
		return "", false
	}
	return last, true
}

func parseCHGUTCDateTime(dof string, hhmm string) (time.Time, bool) {
	if len(dof) < 6 || len(hhmm) < 4 {
		return time.Time{}, false
	}
	date := "20" + dof
	date = date[:4] + "-" + date[4:6] + "-" + date[6:]
	timestamp := date + "T" + hhmm[:2] + ":" + hhmm[2:4] + ":00Z"
	parsed, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func parseCHGHHMMDuration(hhmm string) (time.Duration, bool) {
	if len(hhmm) != 4 {
		return 0, false
	}
	hours, err := strconv.Atoi(hhmm[:2])
	if err != nil {
		return 0, false
	}
	minutes, err := strconv.Atoi(hhmm[2:4])
	if err != nil {
		return 0, false
	}
	if minutes < 0 || minutes > 59 {
		return 0, false
	}
	return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute, true
}

func insertDebugChangeLogIfChanged(db *sql.DB, field string, flight model.Flight, oldValue string, newValue string) {
	if !isDebugChangeLogEnabled() {
		return
	}
	insertChangeLogIfChanged(db, field, flight, oldValue, newValue)
}

func insertChangeLogIfChanged(db *sql.DB, field string, flight model.Flight, oldValue string, newValue string) {
	if flight.ID <= 0 {
		return
	}
	if oldValue == newValue {
		return
	}

	flightChangeLogController := controller.NewFlightChangeLogController(db)
	err := flightChangeLogController.Insert(model.PostFlightChangeLog{
		FlightID: uint(flight.ID),
		Field:    field,
		OldValue: oldValue,
		NewValue: newValue,
	})
	if err != nil {
		return
	}
}

func isDebugChangeLogEnabled() bool {
	for _, key := range []string{"DEBUG_CHANGELOG_ENABLED", "CHG_DEBUG_CHANGELOG_ENABLED"} {
		value := strings.TrimSpace(os.Getenv(key))
		if strings.EqualFold(value, "1") || strings.EqualFold(value, "true") || strings.EqualFold(value, "yes") {
			return true
		}
	}
	return false
}

func formatFlightTimeForChangeLog(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02 15:04:05+00")
}
