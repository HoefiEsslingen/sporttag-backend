package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

//
// ===== Request-Strukturen =====
//

// ===== Suchkriterien (Business-Key) =====
type KindSearch struct {
	VorName    string `json:"vorName"`
	NachName   string `json:"nachName"`
	Jahrgang   int    `json:"jahrgang"`
	Geschlecht string `json:"geschlecht"`
}

// ===== Vollständiges Update (PUT) =====
type KindUpdateFull struct {
	VorName    string `json:"vorName"`
	NachName   string `json:"nachName"`
	Jahrgang   int    `json:"jahrgang"`
	Geschlecht string `json:"geschlecht"`
	Bezahlt    bool   `json:"bezahlt"`
}

// ===== PATCH-Request (bezahlt=true) =====
type KindUpdatePaid struct {
	Bezahlt bool `json:"bezahlt"`
}

// ===== Gesamt-Request =====
type KindUpdateRequest struct {
	Search            KindSearch      `json:"search"`
	Update            json.RawMessage `json:"update"`
	ExpectedUpdatedAt string          `json:"expectedUpdatedAt"`
}

//
// ===== Haupt-Handler =====
//

func (h *KindHandler) UpdateKindByCriteria(w http.ResponseWriter, r *http.Request) {
	var allowedUpdateKeys = map[string]bool{
		"vorName":    true,
		"nachName":   true,
		"jahrgang":   true,
		"geschlecht": true,
		"bezahlt":    true,
	}

	// CORS
	w.Header().Set("Access-Control-Allow-Origin", "https://sporttag.b4a.app")
	w.Header().Set("Access-Control-Allow-Methods", "PUT, PATCH, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// ===== Decode Root =====
	var req KindUpdateRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		http.Error(w, "Ungültiges JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// ===== Validate Search =====
	s := req.Search
	if s.VorName == "" || s.NachName == "" || s.Geschlecht == "" || s.Jahrgang == 0 {
		http.Error(w, "Pflichtfeld in search fehlt", http.StatusBadRequest)
		return
	}

	// ===== Find Kind (Atomicity Teil 1) =====
	kinder, err := h.findKindBySearch(s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(kinder) == 0 {
		http.Error(w, "Kind nicht gefunden", http.StatusNotFound)
		return
	}
	if len(kinder) > 1 {
		http.Error(w, "Dateninkonsistenz: mehrere gleiche Kinder", http.StatusConflict)
		return
	}

	obj := kinder[0]
	objectId := obj["objectId"].(string)
	// Optimistic Locking
	// ===== Atomicity Teil 2: Prüfen, ob Datensatz unverändert ist =====
	// letztes Update-Zeitstempel aus DB mit erwartetem Wert vergleichen: curl https://sporttag-backend.onrender.com/kinder
	// bzw.

	dbUpdatedAt, ok := obj["updatedAt"].(string)
	if !ok {
		http.Error(w, "updatedAt fehlt im Datensatz", http.StatusInternalServerError)
		return
	}
	if req.ExpectedUpdatedAt == "" {
		http.Error(w, "expectedUpdatedAt fehlt", http.StatusBadRequest)
		return
	}

	if req.ExpectedUpdatedAt != dbUpdatedAt {
		http.Error(
			w,
			"Datensatz wurde zwischenzeitlich geändert",
			http.StatusConflict,
		)
		return
	}

	// ===== Validate Update =====
	var rawUpdate map[string]json.RawMessage
	if err := json.Unmarshal(req.Update, &rawUpdate); err != nil {
		http.Error(w, "Ungültiges Update-JSON", http.StatusBadRequest)
		return
	}

	for key := range rawUpdate {
		if !allowedUpdateKeys[key] {
			http.Error(
				w,
				"Ungültiges Feld im Update: "+key,
				http.StatusBadRequest,
			)
			return
		}
	}

	// ===== PATCH: bezahlt=true =====
	if r.Method == http.MethodPatch {
		var upd KindUpdatePaid
		dec := json.NewDecoder(bytes.NewReader(req.Update))
		dec.DisallowUnknownFields()

		if err := dec.Decode(&upd); err != nil {
			http.Error(w, "Ungültiges PATCH-Update: "+err.Error(), http.StatusBadRequest)
			return
		}

		if upd.Bezahlt != true {
			http.Error(w, "PATCH erlaubt nur bezahlt=true", http.StatusBadRequest)
			return
		}

		if obj["bezahlt"] == true {
			http.Error(w, "Kind bereits bezahlt", http.StatusConflict)
			return
		}

		h.doUpdate(w, objectId, map[string]interface{}{
			"bezahlt": true,
		})
		return
	}

	// ===== PUT: kompletter Datensatz =====
	var upd KindUpdateFull
	dec = json.NewDecoder(bytes.NewReader(req.Update))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&upd); err != nil {
		http.Error(w, "Ungültiges PUT-Update: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Pflichtfelder prüfen
	if upd.VorName == "" || upd.NachName == "" || upd.Geschlecht == "" || upd.Jahrgang == 0 {
		http.Error(w, "Pflichtfeld im Update fehlt", http.StatusBadRequest)
		return
	}

	// ===== Maßnahme C: echte Änderung? =====
	if obj["vorName"] == upd.VorName &&
		obj["nachName"] == upd.NachName &&
		int(obj["jahrgang"].(float64)) == upd.Jahrgang &&
		obj["geschlecht"] == upd.Geschlecht &&
		obj["bezahlt"] == upd.Bezahlt {

		http.Error(w, "Update hätte keine Änderung bewirkt", http.StatusConflict)
		return
	}

	h.doUpdate(w, objectId, map[string]interface{}{
		"vorName":    upd.VorName,
		"nachName":   upd.NachName,
		"jahrgang":   upd.Jahrgang,
		"geschlecht": upd.Geschlecht,
		"bezahlt":    upd.Bezahlt,
	})
}

//
// ===== Hilfsfunktionen =====
//

// Kind eindeutig über Business-Key suchen
func (h *KindHandler) findKindBySearch(s KindSearch) ([]map[string]interface{}, error) {
	query := `{"vorName":"` + s.VorName +
		`","nachName":"` + s.NachName +
		`","jahrgang":` + strconv.Itoa(s.Jahrgang) +
		`,"geschlecht":"` + s.Geschlecht + `"}`

	queryURL := h.ParseServerURL + "/classes/Kind?where=" + url.QueryEscape(query)

	req, err := http.NewRequest(http.MethodGet, queryURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Parse-Application-Id", h.ParseAppID)
	req.Header.Set("X-Parse-Javascript-Key", h.ParseJSKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Results []map[string]interface{} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Results, nil
}

// Tatsächliches Update (Commit)
func (h *KindHandler) doUpdate(w http.ResponseWriter, objectId string, update map[string]interface{}) {
	body, err := json.Marshal(update)
	if err != nil {
		http.Error(w, "Interner Fehler", http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequest(
		http.MethodPut,
		h.ParseServerURL+"/classes/Kind/"+objectId,
		bytes.NewBuffer(body),
	)
	if err != nil {
		http.Error(w, "Interner Fehler", http.StatusInternalServerError)
		return
	}

	req.Header.Set("X-Parse-Application-Id", h.ParseAppID)
	req.Header.Set("X-Parse-Javascript-Key", h.ParseJSKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Fehler beim Update", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Update fehlgeschlagen", resp.StatusCode)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Kind erfolgreich aktualisiert",
	})
}
