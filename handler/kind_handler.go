package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"sporttag/strukturen"
)

// ===== Handler-Struktur =====
type KindHandler struct {
	Deadline       time.Time
	ParseServerURL string
	ParseAppID     string
	ParseJSKey     string
	// Sperrmechanismus für Business-Keys
	// Business-Key = VorName|NachName|Jahrgang|Geschlecht (Primary Key als Kombination)
	locks sync.Map // map[string]chan struct{}
}

// ======  Hilfsfunktionen  ======

// Sperrt einen Business-Key für die Dauer einer Operation
func (h *KindHandler) lockForKey(key string) chan struct{} {
	actual, _ := h.locks.LoadOrStore(key, make(chan struct{}, 1))
	return actual.(chan struct{})
}

// Erzeugt den Business-Key aus Suchkriterien
func kindBusinessKey(s strukturen.Kind) string {
	return s.VorName + "|" +
		s.NachName + "|" +
		strconv.Itoa(s.Jahrgang) + "|" +
		s.Geschlecht
}

/*
*
// Erzeugt den Business-Key aus einem Kind-Objekt

	func kindBusinessKeyFromKind(k strukturen.Kind) string {
		return k.VorName + "|" + k.NachName + "|" +
			strconv.Itoa(k.Jahrgang) + "|" + k.Geschlecht
	}

*
*/
func (h *KindHandler) KindRouter(w http.ResponseWriter, r *http.Request) {
	// ---- CORS ----
	// Erlaube Anfragen von der Frontend-Domain auf sporttag.b4a.app
	w.Header().Set("Access-Control-Allow-Origin", "https://sporttag.b4a.app")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Routen zu den jeweiligen Methoden
	switch r.Method {
	case http.MethodGet:
		h.GetKinder(w, r)
	case http.MethodPost:
		h.RegisterKind(w, r)
	case http.MethodPut, http.MethodPatch:
		h.UpdateKindByCriteria(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ======  Register Kind -- neues Kind in der Datenbank erfassen ======

func (h *KindHandler) RegisterKind(w http.ResponseWriter, r *http.Request) {
	// --- PANIC Abfangen ---
	defer func() {
		if r := recover(); r != nil {
			log.Println("PANIC:", r)
			http.Error(w, "Interner Serverfehler", http.StatusInternalServerError)
		}
	}()

	// Erlaube POST-Anfragen von der Frontend-Domain auf sporttag.b4a.app
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// ---- JSON-Daten einlesen ----
	var k strukturen.Kind
	if err := json.NewDecoder(r.Body).Decode(&k); err != nil {
		http.Error(w, "Ungültige JSON-Daten", http.StatusBadRequest)
		return
	}

	// ---- Business-Key Sperre ----
	key := kindBusinessKey(k)
	lock := h.lockForKey(key)
	select {
	case lock <- struct{}{}:
		defer func() { <-lock }()
	default:
		http.Error(w, "Kind wird bereits erfasst", http.StatusConflict)
		return
	}

	// ---- Validierung ----
	if k.VorName == "" || k.NachName == "" || k.Geschlecht == "" || k.Jahrgang == 0 {
		http.Error(w, "Pflichtfelder fehlen", http.StatusBadRequest)
		return
	}

	// ---- Deadline prüfen ----
	if time.Now().UTC().After(h.Deadline) {
		http.Error(w, "Anmeldung geschlossen", http.StatusForbidden)
		return
	}

	// ---- Duplikatprüfung ----
	query := `{"vorName":"` + k.VorName +
		`","nachName":"` + k.NachName +
		`","jahrgang":` + strconv.Itoa(k.Jahrgang) +
		`,"geschlecht":"` + k.Geschlecht + `"}`
	req, _ := http.NewRequest(
		http.MethodGet,
		h.ParseServerURL+"/classes/Kind?where="+url.QueryEscape(query),
		nil,
	)
	req.Header.Set("X-Parse-Application-Id", h.ParseAppID)
	req.Header.Set("X-Parse-Javascript-Key", h.ParseJSKey)
	//---- Anfrage an Parse-Server ----
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Duplikatprüfung fehlgeschlagen", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	//---- Prüfen ob bereits ein Eintrag existiert ----
	var existing struct {
		Results []any `json:"results"`
	}
	//---- Antwort parsen ----
	json.NewDecoder(resp.Body).Decode(&existing)
	//---- Wenn Eintrag existiert, Fehler zurückgeben ----
	if len(existing.Results) > 0 {
		http.Error(w, "Kind existiert bereits", http.StatusConflict)
		return
	}

	// ---- Kind anlegen ----
	payload := map[string]any{
		"vorName":    k.VorName,
		"nachName":   k.NachName,
		"jahrgang":   k.Jahrgang,
		"geschlecht": k.Geschlecht,
		"bezahlt":    false,
		"version":    1,
	}
	//---- Anfrage an Parse-Server ----
	body, _ := json.Marshal(payload)
	//---- Neues Kind anlegen ----
	req, _ = http.NewRequest(
		http.MethodPost,
		h.ParseServerURL+"/classes/Kind",
		bytes.NewBuffer(body),
	)
	req.Header.Set("X-Parse-Application-Id", h.ParseAppID)
	req.Header.Set("X-Parse-Javascript-Key", h.ParseJSKey)
	req.Header.Set("Content-Type", "application/json")
	//---- Anfrage an Parse-Server ----
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Speichern fehlgeschlagen", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	//---- Antwort parsen ----
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	//---- Erfolg zurückgeben ----
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"message":  "Kind erfolgreich gespeichert",
		"objectId": out["objectId"],
	})
}

// ======  Get Kinder -- Liste aller Kinder abrufen ======
func (h *KindHandler) GetKinder(w http.ResponseWriter, r *http.Request) {
	// Erlaube GET-Anfragen von der Frontend-Domain auf sporttag.b4a.app
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Anfrage an Parse-Server an die Tabelle 'Kind' weiterleiten
	req, _ := http.NewRequest(
		http.MethodGet,
		h.ParseServerURL+"/classes/Kind",
		nil,
	)

	req.Header.Set("X-Parse-Application-Id", h.ParseAppID)
	req.Header.Set("X-Parse-Javascript-Key", h.ParseJSKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Parse-Fehler", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}
