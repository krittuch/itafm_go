package app

import "strings"

// vtbsStandList is the set of valid VTBS parking stands. A bay outside this
// list is not written to a flight.
var vtbsStandList = []string{
	"101L", "101R", "102L", "102R", "103L", "103R", "104L", "104R", "105L", "105R",
	"106L", "106R", "107L", "107R", "108L", "108R", "109L", "109R", "110", "110L",
	"110R", "111L", "111R", "112", "112L", "112R", "113", "113L", "113R", "114",
	"114L", "114R", "115", "115L", "116", "116L", "117", "117L", "117R", "118",
	"118R", "119", "119L", "119R", "120", "120R", "121", "121L", "121R", "122",
	"123", "124", "125", "126", "126L", "126R", "127", "127R", "128", "128R",
	"129", "133", "201", "202", "203", "301", "302", "303", "304", "305",
	"306", "307", "308", "401", "402", "403", "501", "502", "503", "504",
	"505", "506", "507", "507R", "508", "509", "509L", "509R", "510", "510L",
	"510R", "511", "512", "512L", "513", "513L", "514", "515", "516", "517",
	"518", "519", "520", "521", "522", "523", "524", "525", "A1", "A2",
	"A3", "A4", "A5", "A6", "B1", "B2", "B3", "B4", "B5", "B6",
	"C1", "C10", "C2", "C3", "C4", "C5", "C6", "C7", "C8", "C9",
	"D1", "D2", "D3", "D4", "D5", "D6", "D7", "D8", "E1", "E10",
	"E2", "E3", "E4", "E5", "E6", "E7", "E8", "E9", "F1", "F2",
	"F3", "F4", "F5", "F6", "G1", "G2", "G3", "G4", "G5", "S101",
	"S102", "S103", "S104", "S105", "S106", "S107", "S108", "S109", "S110", "S111",
	"S112", "S113", "S114", "S115", "S116", "S117", "S118", "S119", "S120", "S121",
	"S122", "S123", "S124", "S125", "S126", "S127",
}

var vtbsStands = func() map[string]struct{} {
	stands := make(map[string]struct{}, len(vtbsStandList))
	for _, stand := range vtbsStandList {
		stands[stand] = struct{}{}
	}
	return stands
}()

func isVTBSStand(bay string) bool {
	_, ok := vtbsStands[strings.ToUpper(strings.TrimSpace(bay))]
	return ok
}
