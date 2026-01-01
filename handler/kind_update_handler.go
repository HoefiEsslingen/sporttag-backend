package handler

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strconv"
)

//
// ===== Request-Strukturen =====
//

// Business-Key (unveränderlich)
type KindSearch struct {
	VorName    string `json:"vorName"`
	NachName   string `json:"nachName"`
	Jahrgang   int    `json:"jahrgang"`
	Geschlecht string `json:"geschlecht"`
}

// PUT – vollständiges Update
type KindUpdateFull struct {
	VorName    string `json:"vorName"`
	NachName   string `json:"nachName"`
	Jahrgang   int    `json:"jahrgang"`
	Geschlecht string `json:"geschlecht"`
	Bezahlt    bool   `json:"bezahlt"`
}

// PATCH – nur bezahlt=true
type KindUpdatePaid struct {
	Bezahlt bool `json:"bezahlt"`
}

// Root-Request
type KindUpdateRequest struct {
	Search          KindSearch      `json:"search"`
	Update          json.RawMessage `json:"update"`
	ExpectedVersion int             `json:"expectedVersion"`
}

//
// ===== Handler =====
//

func (h *KindHandler) UpdateKindByCriteria(w http.ResponseWriter, r *http.Request) {

	defer func() {
		if r := recover(); r != nil {
			log.Println("PANIC:", r)
			http.Error(w, "Interner Serverfehler", http.StatusInternalServerError)
		}
	}()

	allowedUpdateKeys := map[string]bool{
		"vorName":    true,
		"nachName":   true,
		"jahrgang":   true,
		"geschlecht": true,
		"bezahlt":    true,
	}

	// ---- CORS ----
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

	// ---- Decode Root ----
	var req KindUpdateRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		http.Error(w, "Ungültiges JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// ---- Validate Search ----
	s := req.Search
	if s.VorName == "" || s.NachName == "" || s.Geschlecht == "" || s.Jahrgang == 0 {
		http.Error(w, "Pflichtfeld in search fehlt", http.StatusBadRequest)
		return
	}

	if req.ExpectedVersion <= 0 {
		http.Error(w, "expectedVersion fehlt oder ungültig", http.StatusBadRequest)
		return
	}

	// ---- Find Kind ----
	kinder, err := h.findKindBySearch(s)
	if err != nil {
		http.Error(w, "Fehler bei der Suche", http.StatusInternalServerError)
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

	objectId, ok := obj["objectId"].(string)
	if !ok {
		http.Error(w, "objectId fehlt", http.StatusInternalServerError)
		return
	}

	currentVersion := int(obj["version"].(float64))
	if currentVersion != req.ExpectedVersion {
		http.Error(w, "Konflikt: Version veraltet", http.StatusConflict)
		return
	}

	// ---- Validate Update Keys ----
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(req.Update, &raw); err != nil {
		http.Error(w, "Ungültiges Update-JSON", http.StatusBadRequest)
		return
	}

	for k := range raw {
		if !allowedUpdateKeys[k] {
			http.Error(w, "Ungültiges Update-Feld: "+k, http.StatusBadRequest)
			return
		}
	}

	// ---- PATCH: bezahlt=true ----
	if r.Method == http.MethodPatch {

		var upd KindUpdatePaid
		dec := json.NewDecoder(bytes.NewReader(req.Update))
		dec.DisallowUnknownFields()

		if err := dec.Decode(&upd); err != nil {
			http.Error(w, "Ungültiges PATCH-Update", http.StatusBadRequest)
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

		h.doConditionalUpdateWithVersion(
			w,
			objectId,
			req.ExpectedVersion,
			map[string]interface{}{
				"bezahlt": true,
			},
		)
		return
	}

	// ---- PUT: kompletter Datensatz ----
	var upd KindUpdateFull
	dec = json.NewDecoder(bytes.NewReader(req.Update))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&upd); err != nil {
		http.Error(w, "Ungültiges PUT-Update", http.StatusBadRequest)
		return
	}

	if upd.VorName == "" || upd.NachName == "" || upd.Geschlecht == "" || upd.Jahrgang == 0 {
		http.Error(w, "Pflichtfeld im Update fehlt", http.StatusBadRequest)
		return
	}

	// ---- echte Änderung? ----
	if obj["vorName"] == upd.VorName &&
		obj["nachName"] == upd.NachName &&
		int(obj["jahrgang"].(float64)) == upd.Jahrgang &&
		obj["geschlecht"] == upd.Geschlecht &&
		obj["bezahlt"] == upd.Bezahlt {

		http.Error(w, "Update hätte keine Änderung bewirkt", http.StatusConflict)
		return
	}

	h.doConditionalUpdateWithVersion(
		w,
		objectId,
		req.ExpectedVersion,
		map[string]interface{}{
			"vorName":    upd.VorName,
			"nachName":   upd.NachName,
			"jahrgang":   upd.Jahrgang,
			"geschlecht": upd.Geschlecht,
			"bezahlt":    upd.Bezahlt,
		},
	)
}

//
// ===== Hilfsfunktionen =====
//

func (h *KindHandler) findKindBySearch(s KindSearch) ([]map[string]interface{}, error) {
	query := `{"vorName":"` + s.VorName +
		`","nachName":"` + s.NachName +
		`","jahrgang":` + strconv.Itoa(s.Jahrgang) +
		`,"geschlecht":"` + s.Geschlecht + `"}`

	url := h.ParseServerURL + "/classes/Kind?where=" + url.QueryEscape(query)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Parse-Application-Id", h.ParseAppID)
	req.Header.Set("X-Parse-Javascript-Key", h.ParseJSKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out struct {
		Results []map[string]interface{} `json:"results"`
	}
	err = json.NewDecoder(resp.Body).Decode(&out)
	return out.Results, err
}

//
// ===== Conditional PUT mit Version-Locking =====
//

func (h *KindHandler) doConditionalUpdateWithVersion(
	w http.ResponseWriter,
	objectId string,
	expectedVersion int,
	update map[string]interface{},
) {
	where := map[string]interface{}{
		"objectId": objectId,
		"version":  expectedVersion,
	}

	whereJSON, _ := json.Marshal(where)
	query := "?where=" + url.QueryEscape(string(whereJSON))

	//update["version"] = expectedVersion + 1
	update["version"] = map[string]interface{}{
		"__op":   "Increment",
		"amount": 1,
	}

	body, err := json.Marshal(update)
	if err != nil {
		http.Error(w, "Interner Fehler", http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequest(
		http.MethodPut,
		h.ParseServerURL+"/classes/Kind"+query,
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
		http.Error(w, "Update fehlgeschlagen", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		http.Error(w, "Ungültige Parse-Antwort", http.StatusInternalServerError)
		return
	}

	/*
		if _, ok := out["updatedAt"]; !ok {
			http.Error(
				w,
				"Konflikt: Datensatz wurde zwischenzeitlich geändert",
				http.StatusConflict,
			)
			return
		}
	*/

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Kind erfolgreich aktualisiert",
	})
}
