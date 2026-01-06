package strukturen

// Riege entspricht der Klasse "Riege" in Back4App
type Riege struct {
	RiegenNummer     int  `json:"riegenNummer,omitempty"`
	FuenfKampf       bool `json:"fuenfKampf"`
	WettkampfBeendet bool `json:"wetttkampfBeendet"`
}
