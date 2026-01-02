package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"sporttag/strukturen"
)

/*
type KindHandler struct {
	Deadline       time.Time
	ParseAppID     string
	ParseJSKey     string
	ParseServerURL string
}
*/
// z.B. in kind_handler.go oder handler_struct.go

type KindHandler struct {
	Deadline       time.Time
	ParseServerURL string
	ParseAppID     string
	ParseJSKey     string

	locks sync.Map // map[string]*sync.Mutex
}

func (h *KindHandler) RegisterKind(w http.ResponseWriter, r *http.Request) {
	// CORS (optional, aber sauber)
	w.Header().Set("Access-Control-Allow-Origin", "https://sporttag.b4a.app")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 1️⃣ Method Guard
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 2️⃣ JSON einlesen
	var k strukturen.Kind
	if err := json.NewDecoder(r.Body).Decode(&k); err != nil {
		http.Error(w, "Ungültige JSON-Daten", http.StatusBadRequest)
		return
	}

	// 3️⃣ Pflichtfelder prüfen (Consistency)
	if k.VorName == "" || k.NachName == "" || k.Geschlecht == "" || k.Jahrgang == 0 {
		http.Error(w, "Pflichtfelder fehlen", http.StatusBadRequest)
		return
	}

	// 4️⃣ Deadline prüfen (UTC!)
	now := time.Now().UTC()
	if now.After(h.Deadline) {
		// Uhrzeit in UTC liegt zwei Stunden vor unserer Zeit
		http.Error(w, "Anmeldung geschlossen.", http.StatusForbidden)
		return
	}

	// 5️⃣ Eindeutigkeitsprüfung (Business-Key)
	query := `{"vorName":"` + k.VorName +
		`","nachName":"` + k.NachName +
		`","jahrgang":` + strconv.Itoa(k.Jahrgang) +
		`,"geschlecht":"` + k.Geschlecht + `"}`

	queryURL := h.ParseServerURL + "/classes/Kind?where=" + url.QueryEscape(query)

	req, err := http.NewRequest(http.MethodGet, queryURL, nil)
	if err != nil {
		http.Error(w, "Interner Fehler", http.StatusInternalServerError)
		return
	}

	req.Header.Set("X-Parse-Application-Id", h.ParseAppID)
	req.Header.Set("X-Parse-Javascript-Key", h.ParseJSKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Fehler bei der Duplikatprüfung", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var checkResult struct {
		Results []map[string]interface{} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&checkResult); err != nil {
		http.Error(w, "Antwort von Parse ungültig", http.StatusInternalServerError)
		return
	}

	// 6️⃣ Harte Eindeutigkeitsregeln
	switch len(checkResult.Results) {
	case 0:
		// OK → weiter
	case 1:
		http.Error(w, "Kind existiert bereits", http.StatusConflict)
		return
	default:
		http.Error(w, "Dateninkonsistenz: mehrere gleiche Kinder", http.StatusConflict)
		return
	}

	// 7️⃣ Insert (Commit)
	payload := map[string]interface{}{
		"vorName":    k.VorName,
		"nachName":   k.NachName,
		"jahrgang":   k.Jahrgang,
		"geschlecht": k.Geschlecht,
		"bezahlt":    false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "Interner Fehler", http.StatusInternalServerError)
		return
	}

	req, err = http.NewRequest(http.MethodPost, h.ParseServerURL+"/classes/Kind", bytes.NewBuffer(body))
	if err != nil {
		http.Error(w, "Interner Fehler", http.StatusInternalServerError)
		return
	}

	req.Header.Set("X-Parse-Application-Id", h.ParseAppID)
	req.Header.Set("X-Parse-Javascript-Key", h.ParseJSKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Fehler beim Speichern", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var pe map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&pe)

		msg := "Unbekannter Fehler bei Parse"
		if e, ok := pe["error"].(string); ok {
			msg = e
		}

		http.Error(w, "Parse: "+msg, resp.StatusCode)
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

func (h *KindHandler) mutexForKey(key string) *sync.Mutex {
	actual, _ := h.locks.LoadOrStore(key, &sync.Mutex{})
	return actual.(*sync.Mutex)
}

func kindBusinessKey(s KindSearch) string {
	return s.VorName + "|" +
		s.NachName + "|" +
		strconv.Itoa(s.Jahrgang) + "|" +
		s.Geschlecht
}
