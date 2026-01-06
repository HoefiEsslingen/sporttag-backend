package strukturen

// KinderDerRiege entspricht der Klasse "kinderDerRiege"
type KinderDerRiege struct {
	KindID   any `json:"kindID"`   // Pointer → Kind
	RiegenID any `json:"riegenID"` // Pointer → Riege
	Position int `json:"position"`
}

// Später kannst du daraus z. B. machen:
/*
type ParsePointer struct {
	Type      string `json:"__type"`
	ClassName string `json:"className"`
	ObjectID  string `json:"objectId"`
}
*/
