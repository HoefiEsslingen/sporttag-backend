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

type KindHandler struct {
	Deadline       time.Time
	ParseServerURL string
	ParseAppID     string
	ParseJSKey     string
	locks          sync.Map // map[string]chan struct{}
}

// ======  Hilfsfunktionen  ======

func (h *KindHandler) lockForKey(key string) chan struct{} {
	actual, _ := h.locks.LoadOrStore(key, make(chan struct{}, 1))
	return actual.(chan struct{})
}

func kindBusinessKey(s KindSearch) string {
	return s.VorName + "|" +
		s.NachName + "|" +
		strconv.Itoa(s.Jahrgang) + "|" +
		s.Geschlecht
}

func kindBusinessKeyFromKind(k strukturen.Kind) string {
	return k.VorName + "|" + k.NachName + "|" +
		strconv.Itoa(k.Jahrgang) + "|" + k.Geschlecht
}

func (h *KindHandler) KindRouter(w http.ResponseWriter, r *http.Request) {
	// ---- CORS ----
	w.Header().Set("Access-Control-Allow-Origin", "https://sporttag.b4a.app")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

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
	defer func() {
		if r := recover(); r != nil {
			log.Println("PANIC:", r)
			http.Error(w, "Interner Serverfehler", http.StatusInternalServerError)
		}
	}()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var k strukturen.Kind
	if err := json.NewDecoder(r.Body).Decode(&k); err != nil {
		http.Error(w, "Ungültige JSON-Daten", http.StatusBadRequest)
		return
	}

	key := kindBusinessKeyFromKind(k)
	lock := h.lockForKey(key)

	select {
	case lock <- struct{}{}:
		defer func() { <-lock }()
	default:
		http.Error(w, "Kind wird bereits erfasst", http.StatusConflict)
		return
	}

	if k.VorName == "" || k.NachName == "" || k.Geschlecht == "" || k.Jahrgang == 0 {
		http.Error(w, "Pflichtfelder fehlen", http.StatusBadRequest)
		return
	}

	if time.Now().UTC().After(h.Deadline) {
		http.Error(w, "Anmeldung geschlossen", http.StatusForbidden)
		return
	}

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

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Duplikatprüfung fehlgeschlagen", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var existing struct {
		Results []any `json:"results"`
	}
	json.NewDecoder(resp.Body).Decode(&existing)

	if len(existing.Results) > 0 {
		http.Error(w, "Kind existiert bereits", http.StatusConflict)
		return
	}

	payload := map[string]any{
		"vorName":    k.VorName,
		"nachName":   k.NachName,
		"jahrgang":   k.Jahrgang,
		"geschlecht": k.Geschlecht,
		"bezahlt":    false,
		"version":    1,
	}

	body, _ := json.Marshal(payload)

	req, _ = http.NewRequest(
		http.MethodPost,
		h.ParseServerURL+"/classes/Kind",
		bytes.NewBuffer(body),
	)

	req.Header.Set("X-Parse-Application-Id", h.ParseAppID)
	req.Header.Set("X-Parse-Javascript-Key", h.ParseJSKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Speichern fehlgeschlagen", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"message":  "Kind erfolgreich gespeichert",
		"objectId": out["objectId"],
	})
}

// ======  Get Kinder -- Liste aller Kinder abrufen ======
func (h *KindHandler) GetKinder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
