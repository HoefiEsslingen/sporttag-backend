package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
		// Uhrzeit in UTc liegt zwei Stunden vor unserer Zeit
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

func (h *KindHandler) GetKinder(w http.ResponseWriter, r *http.Request) {
	req, err := http.NewRequest(
		"GET",
		h.ParseServerURL+"/classes/Kind",
		nil,
	)
	if err != nil {
		http.Error(w, "Fehler beim Request", http.StatusInternalServerError)
		return
	}

	req.Header.Set("X-Parse-Application-Id", h.ParseAppID)
	req.Header.Set("X-Parse-Javascript-Key", h.ParseJSKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Fehler bei Parse", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		http.Error(w, "Parse-Fehler", resp.StatusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{`))

	// Antwort 1:1 durchreichen
	w.Write([]byte(`"results":`))
	io.Copy(w, resp.Body)
	w.Write([]byte(`}`))
}

func (h *KindHandler) UpdateKind(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Methode nicht erlaubt", http.StatusMethodNotAllowed)
		return
	}

	objectId := strings.TrimPrefix(r.URL.Path, "/kinder/")
	if objectId == "" {
		http.Error(w, "objectId fehlt", http.StatusBadRequest)
		return
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Ungültige Daten", http.StatusBadRequest)
		return
	}

	body, _ := json.Marshal(payload)

	req, err := http.NewRequest(
		"PUT",
		h.ParseServerURL+"/classes/Kind/"+objectId,
		bytes.NewBuffer(body),
	)
	if err != nil {
		http.Error(w, "Fehler beim Request", http.StatusInternalServerError)
		return
	}

	req.Header.Set("X-Parse-Application-Id", h.ParseAppID)
	req.Header.Set("X-Parse-Javascript-Key", h.ParseJSKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Fehler bei Parse", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		http.Error(w, "Update fehlgeschlagen", resp.StatusCode)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Kind aktualisiert",
	})
}
func (h *KindHandler) GetKindByCriteria(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	vorName := q.Get("vorName")
	nachName := q.Get("nachName")
	jahrgangStr := q.Get("jahrgang")
	geschlecht := q.Get("geschlecht")

	if vorName == "" || nachName == "" || jahrgangStr == "" || geschlecht == "" {
		http.Error(w, "Fehlende Suchparameter", http.StatusBadRequest)
		return
	}

	jahrgang, err := strconv.Atoi(jahrgangStr)
	if err != nil {
		http.Error(w, "jahrgang muss numerisch sein", http.StatusBadRequest)
		return
	}

	where := map[string]interface{}{
		"vorName":    vorName,
		"nachName":   nachName,
		"jahrgang":   jahrgang,
		"geschlecht": geschlecht,
	}

	whereJSON, _ := json.Marshal(where)

	req, err := http.NewRequest(
		"GET",
		h.ParseServerURL+"/classes/Kind?where="+url.QueryEscape(string(whereJSON)),
		nil,
	)
	if err != nil {
		http.Error(w, "Fehler beim Request", http.StatusInternalServerError)
		return
	}

	req.Header.Set("X-Parse-Application-Id", h.ParseAppID)
	req.Header.Set("X-Parse-Javascript-Key", h.ParseJSKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Fehler bei Parse", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var result struct {
		Results []map[string]interface{} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		http.Error(w, "Ungültige Parse-Antwort", http.StatusInternalServerError)
		return
	}

	if len(result.Results) == 0 {
		http.Error(w, "Kind nicht gefunden", http.StatusNotFound)
		return
	}

	if len(result.Results) > 1 {
		http.Error(w, "Mehrdeutige Anfrage", http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result.Results[0])
}

func (h *KindHandler) UpdateKindByCriteria(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Methode nicht erlaubt", http.StatusMethodNotAllowed)
		return
	}

	var reqBody struct {
		Search struct {
			VorName    string `json:"vorName"`
			NachName   string `json:"nachName"`
			Jahrgang   int    `json:"jahrgang"`
			Geschlecht string `json:"geschlecht"`
		} `json:"search"`
		Update map[string]interface{} `json:"update"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Ungültige JSON-Daten", http.StatusBadRequest)
		return
	}

	s := reqBody.Search
	if s.VorName == "" || s.NachName == "" || s.Geschlecht == "" || s.Jahrgang == 0 {
		http.Error(w, "Unvollständige Suchdaten", http.StatusBadRequest)
		return
	}

	if len(reqBody.Update) == 0 {
		http.Error(w, "Keine Update-Daten angegeben", http.StatusBadRequest)
		return
	}

	// 1️⃣ Kind suchen
	where := map[string]interface{}{
		"vorName":    s.VorName,
		"nachName":   s.NachName,
		"jahrgang":   s.Jahrgang,
		"geschlecht": s.Geschlecht,
	}

	whereJSON, _ := json.Marshal(where)

	findReq, _ := http.NewRequest(
		"GET",
		h.ParseServerURL+"/classes/Kind?where="+url.QueryEscape(string(whereJSON)),
		nil,
	)

	findReq.Header.Set("X-Parse-Application-Id", h.ParseAppID)
	findReq.Header.Set("X-Parse-Javascript-Key", h.ParseJSKey)

	findResp, err := http.DefaultClient.Do(findReq)
	if err != nil {
		http.Error(w, "Fehler bei Parse (Search)", http.StatusInternalServerError)
		return
	}
	defer findResp.Body.Close()

	var result struct {
		Results []map[string]interface{} `json:"results"`
	}

	if err := json.NewDecoder(findResp.Body).Decode(&result); err != nil {
		http.Error(w, "Ungültige Parse-Antwort", http.StatusInternalServerError)
		return
	}

	if len(result.Results) == 0 {
		http.Error(w, "Kind nicht gefunden", http.StatusNotFound)
		return
	}

	if len(result.Results) > 1 {
		http.Error(w, "Mehrdeutige Suchkriterien", http.StatusConflict)
		return
	}

	objectId := result.Results[0]["objectId"].(string)

	// 2️⃣ Update durchführen
	updateBody, _ := json.Marshal(reqBody.Update)

	updateReq, _ := http.NewRequest(
		"PUT",
		h.ParseServerURL+"/classes/Kind/"+objectId,
		bytes.NewBuffer(updateBody),
	)

	updateReq.Header.Set("X-Parse-Application-Id", h.ParseAppID)
	updateReq.Header.Set("X-Parse-Javascript-Key", h.ParseJSKey)
	updateReq.Header.Set("Content-Type", "application/json")

	updateResp, err := http.DefaultClient.Do(updateReq)
	if err != nil {
		http.Error(w, "Fehler bei Parse (Update)", http.StatusInternalServerError)
		return
	}
	defer updateResp.Body.Close()

	if updateResp.StatusCode != 200 {
		http.Error(w, "Update fehlgeschlagen", updateResp.StatusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message":  "Kind aktualisiert",
		"objectId": objectId,
	})
}
