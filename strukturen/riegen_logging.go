package strukturen

import "time"

// RiegenLogging entspricht der Klasse "riegenLogging"
type RiegenLogging struct {
	RiegenID                 any       `json:"riegenID"`   // Pointer → Riege
	StationsID               any       `json:"stationsID"` // Pointer → Station
	AnzAbsolvierterStationen int       `json:"anzAbsolvierterStationen,omitempty"`
	LetzteStationUm          time.Time `json:"letzteStationUm,omitempty"`
}
