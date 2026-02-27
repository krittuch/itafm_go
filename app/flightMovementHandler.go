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

func onFPLReceive(
	body []byte,
	db *sql.DB,
	flightController *controller.FlightController) bool {
	fplData := model.FlightPlan{}
	err := json.Unmarshal(body, &fplData)
	if err != nil {
		return false
	}

	r, err2 := regexp.Compile(`(DOF\/)\w+`)

	if err2 == nil {
		dof := r.FindString(fplData.ITEM18)
		fplData.DOF = strings.Replace(dof, `DOF/`, "", 1)
	}

	regex, err3 := regexp.Compile(`(REG\/)\w+`)

	register := ""

	if err3 == nil {
		register = regex.FindString(fplData.ITEM18)
		register = strings.Replace(register, `REG/`, "", 1)
	}

	postFlight := model.PostFlight{
		AircraftType: fplData.ACTYPE,
		NextStation:  fplData.DESTINATION,
		PrevStation:  fplData.DEPARTURE,
		Register:     register,
	}

	airlineCodeRegex := regexp.MustCompile(`^[A-Z]{3}`)
	icaoCode := airlineCodeRegex.FindString(fplData.CALLSIGN)

	iata, success := ConvertToIATA(icaoCode)

	if !success {
		return false
	}

	numberRegex := regexp.MustCompile(`\d+`)
	matchString := numberRegex.FindString(fplData.CALLSIGN)
	if len(matchString) <= 0 {
		return false
	}

	flightNumber := strings.TrimLeft(matchString, "0")

	timeStr := fplData.ETD

	dateOfFlight := "20" + fplData.DOF[:2] + "-" + fplData.DOF[2:4] + "-" + fplData.DOF[4:]
	std := strings.Join([]string{dateOfFlight, " ", timeStr[:2], ":", timeStr[2:4], ":00+00"}, "")

	postFlight.FlightNumber = fmt.Sprint(iata, " ", flightNumber)

	flightController.UpdateRegister(postFlight.FlightNumber, postFlight.Register, std)

	flight, err := flightController.GetFlight(postFlight.FlightNumber, std)

	if err == nil {

		flightChangeLogController := controller.NewFlightChangeLogController(db)

		err = flightChangeLogController.Insert(model.PostFlightChangeLog{
			FlightID: uint(flight.ID),
			Field:    "ac_register",
			OldValue: flight.ACRegister,
			NewValue: postFlight.Register,
		})

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

	airlineCodeRegex := regexp.MustCompile(`^[A-Z]{3}`)
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
		dateOfFlight = strings.Join([]string{"20", fmvData.DOF}, "")
		if len(dateOfFlight) < 4 {
			return false
		}
		dateOfFlight = dateOfFlight[:4] + "-" + dateOfFlight[4:6] + "-" + dateOfFlight[6:]
		std = strings.Join([]string{dateOfFlight, " ", timeStr[:2], ":", timeStr[2:4], ":00+00"}, "")
	} else if fmvData.CMD == "ARR" {
		timeStr = fmvData.TIME2
		t := time.Now().UTC()
		timeString := t.Format("2006-01-02 15:04:05")
		dString := strings.Split(timeString, " ")[0]

		fmvData.DOF = dString
		std = strings.Join([]string{
			dString, " ",
			timeStr[:2], ":",
			timeStr[2:4], ":00+00",
		}, "")
	} else {
		return false
	}

	if fmvData.CMD == "DEP" {
		beforeFlight, beforeErr := flightController.GetFlightByTypeAndSchedule(flightNumber, "DEP", std)
		flightController.UpdateDepartureFlight(flightNumber, fmvData.DOF, std)
		if beforeErr == nil {
			if afterFlight, afterErr := flightController.GetFlightByTypeAndSchedule(flightNumber, "DEP", std); afterErr == nil {
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
		beforeFlight, beforeErr := flightController.GetFlightByTypeAndSchedule(flightNumber, "ARR", std)
		flightController.UpdateArrivalFlight(flightNumber, fmvData.DOF, std)
		if beforeErr == nil {
			if afterFlight, afterErr := flightController.GetFlightByTypeAndSchedule(flightNumber, "ARR", std); afterErr == nil {
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

	airlineCodeRegex := regexp.MustCompile(`^[A-Z]{3}`)
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
			insertDebugChangeLogIfChanged(
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

	airlineCodeRegex := regexp.MustCompile(`^[A-Z]{3}`)
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
			insertDebugChangeLogIfChanged(
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

	airlineCodeRegex := regexp.MustCompile(`^[A-Z]{3}`)
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
	re := regexp.MustCompile(`-\d+/([A-Z]{4})(\d{4})`)
	match := re.FindStringSubmatch(normalized)
	if len(match) != 3 {
		return "", "", false
	}
	return match[1], match[2], true
}

func parseCHGRegisterFromItem(item string) (string, bool) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(item, "\r", " "), "\n", " "))
	re := regexp.MustCompile(`\bREG/([A-Z0-9-]+)`)
	match := re.FindStringSubmatch(normalized)
	if len(match) != 2 {
		return "", false
	}
	return match[1], true
}

func parseCHGAircraftFromItem(item string) (string, bool) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(item, "\r", " "), "\n", " "))
	re := regexp.MustCompile(`-\s*9/([A-Z0-9-]+)/[A-Z0-9-]+`)
	match := re.FindStringSubmatch(normalized)
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

	firTokenRegex := regexp.MustCompile(`^[A-Z]{4}(\d{4})$`)
	rawDurationRegex := regexp.MustCompile(`^(\d{4})$`)
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
