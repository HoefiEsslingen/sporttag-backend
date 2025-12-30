package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"sporttag/strukturen"
)

type KindHandler struct {
	Deadline       time.Time
	ParseAppID     string
	ParseJSKey     string
	ParseServerURL string
}

func (h *KindHandler) RegisterKind(w http.ResponseWriter, r *http.Request) {
	// Wenn kein Duplikat: Kind wie bisher hinzufügen…
	if time.Now().After(h.Deadline) {
		http.Error(w, "Anmeldung geschlossen.", http.StatusForbidden)
		return
	}

	// 1. Kind aus Request auslesen
	var k strukturen.Kind

	if err := json.NewDecoder(r.Body).Decode(&k); err != nil {
		http.Error(w, "Ungültige Daten.", http.StatusBadRequest)
		return
	}

	// 2. Prüfen, ob Kind (VorN, NachN, Jahrgang) schon existiert
	queryURL := h.ParseServerURL + "/classes/Kind?where=" +
		url.QueryEscape(`{"vorName":"`+k.VorName+`","nachName":"`+k.NachName+`","jahrgang":`+strconv.Itoa(k.Jahrgang)+`}`)
	req, _ := http.NewRequest("GET", queryURL, nil)
	req.Header.Set("X-Parse-Application-Id", h.ParseAppID)
	req.Header.Set("X-Parse-Javascript-Key", h.ParseJSKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Fehler bei der Duplikat-Prüfung", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	var checkResult struct {
		Results []interface{} `json:"results"`
	}
	json.NewDecoder(resp.Body).Decode(&checkResult)
	if len(checkResult.Results) > 0 {
		http.Error(w, "Kind existiert bereits", http.StatusConflict)
		return
	}
	payload := map[string]interface{}{
		"vorName":    k.VorName,
		"nachName":   k.NachName,
		"jahrgang":   k.Jahrgang,
		"geschlecht": k.Geschlecht,
		"bezahlt":    false,
	}

	// …und an Parse senden
	w.Header().Set("Access-Control-Allow-Origin", "https://sporttag.b4a.app") // besser: domain deiner Webapp
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	body, _ := json.Marshal(payload)
	req, err = http.NewRequest("POST", h.ParseServerURL+"/classes/Kind", bytes.NewBuffer(body))
	if err != nil {
		http.Error(w, "Fehler beim Erstellen", http.StatusInternalServerError)
		return
	}
	req.Header.Set("X-Parse-Application-Id", h.ParseAppID)
	req.Header.Set("X-Parse-Javascript-Key", h.ParseJSKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Fehler bei Parse", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		var pe map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&pe)
		http.Error(w, "Parse: "+pe["error"].(string), resp.StatusCode)
		return
	}
	var out map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&out)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"objectId": out["objectId"],
		"message":  "Kind erfolgreich gespeichert",
	})
}
