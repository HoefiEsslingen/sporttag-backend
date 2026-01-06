package strukturen

// Station entspricht der Klasse "Station" in Back4App
type Station struct {
	StationsName   string `json:"stationsName,omitempty"`
	StationsNummer int    `json:"stationsNummer,omitempty"`
	NurZehnKampf   bool   `json:"nurZehnKampf"`
	Beschreibung   any    `json:"beschreibung"` // Parse File → generisch
}
