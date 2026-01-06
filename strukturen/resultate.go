package strukturen

import "time"

// Resultate entspricht der Klasse "resultate"
type Resultate struct {
	KindID     any       `json:"kindID,omitempty"`     // Pointer → Kind
	StationsID any       `json:"stationsID,omitempty"` // Pointer → Station
	Punkte     int       `json:"punkte,omitempty"`
	ErreichtUm time.Time `json:"erreichtUm"`
}
