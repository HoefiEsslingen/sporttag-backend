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

// Suchkriterien = Business Key (unveränderlich)
type KindSearch struct {
	VorName    string `json:"vorName"`
	NachName   string `json:"nachName"`
	Jahrgang   int    `json:"jahrgang"`
	Geschlecht string `json:"geschlecht"`
}

// Update-Request mit klarer Trennung
type KindUpdateRequest struct {
	Search KindSearch             `json:"search"`
	Update map[string]interface{} `json:"update"`
}

//
// ===== Haupt-Handler =====
//

func (h *KindHandler) UpdateKindByCriteria(w http.ResponseWriter, r *http.Request) {
	// CORS
	w.Header().Set("Access-Control-Allow-Origin", "https://sporttag.b4a.app")
	w.Header().Set("Access-Control-Allow-Methods", "PATCH, PUT, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 1️⃣ Nur PATCH oder PUT
	if r.Method != http.MethodPatch && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 2️⃣ Request dekodieren
	var req KindUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Ungültige JSON-Daten", http.StatusBadRequest)
		return
	}

	// 3️⃣ Pflichtfelder für Suche prüfen (Consistency)
	s := req.Search
	if s.VorName == "" || s.NachName == "" || s.Geschlecht == "" || s.Jahrgang == 0 {
		http.Error(w, "Pflichtfeld fehlt", http.StatusBadRequest)
		return
	}

	// 4️⃣ Kind eindeutig suchen (Atomicity-Teil 1)
	kind, err := h.findKindBySearch(s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	switch len(kind) {
	case 0:
		http.Error(w, "Kind nicht gefunden", http.StatusNotFound)
		return
	case 1:
		// OK
	default:
		http.Error(w, "Dateninkonsistenz: mehrere gleiche Kinder", http.StatusConflict)
		return
	}

	obj := kind[0]
	objectId, ok := obj["objectId"].(string)
	if !ok {
		http.Error(w, "Ungültige Objekt-ID", http.StatusInternalServerError)
		return
	}

	// 5️⃣ PATCH: bezahlt = true (idempotent)
	if r.Method == http.MethodPatch {
		if bezahlt, ok := obj["bezahlt"].(bool); ok && bezahlt {
			http.Error(w, "Kind bereits bezahlt", http.StatusConflict)
			return
		}

		h.doUpdate(w, objectId, map[string]interface{}{
			"bezahlt": true,
		})
		return
	}

	// 6️⃣ PUT: kompletter Datensatz
	if len(req.Update) == 0 {
		http.Error(w, "Update-Daten fehlen", http.StatusBadRequest)
		return
	}

	h.doUpdate(w, objectId, req.Update)
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
