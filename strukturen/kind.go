package strukturen

type Kind struct {
	VorName    string `json:"vorName"`
	NachName   string `json:"nachName"`
	Jahrgang   int    `json:"jahrgang"`
	Geschlecht string `json:"geschlecht"`
}
