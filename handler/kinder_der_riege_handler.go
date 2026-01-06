package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
)

// Create / Update / Delete
type KinderDerRiegeRequest struct {
	KindObjectID  string `json:"kindObjectId"`
	RiegeObjectID string `json:"riegeObjectId"`
	Position      int    `json:"position,omitempty"`
}

// Hilfsfunktion für Lock-Key
func kinderDerRiegeKey(kindID, riegeID string) string {
	return kindID + "|" + riegeID
}

// ===== CREATE =====
func (h *KindHandler) AssignKindToRiege(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req KinderDerRiegeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Ungültiges JSON", http.StatusBadRequest)
		return
	}

	if req.KindObjectID == "" || req.RiegeObjectID == "" || req.Position <= 0 {
		http.Error(w, "Pflichtfelder fehlen", http.StatusBadRequest)
		return
	}

	lockKey := kinderDerRiegeKey(req.KindObjectID, req.RiegeObjectID)
	lock := h.lockForKey(lockKey)

	select {
	case lock <- struct{}{}:
		defer func() { <-lock }()
	default:
		http.Error(w, "Zuordnung wird bereits verarbeitet", http.StatusConflict)
		return
	}

	// ---- Duplikatprüfung ----
	where := map[string]any{
		"kindID": map[string]any{
			"__type":    "Pointer",
			"className": "Kind",
			"objectId":  req.KindObjectID,
		},
	}

	whereJSON, _ := json.Marshal(where)
	url := h.ParseServerURL + "/classes/kinderDerRiege?where=" +
		url.QueryEscape(string(whereJSON))

	checkReq, _ := http.NewRequest(http.MethodGet, url, nil)
	checkReq.Header.Set("X-Parse-Application-Id", h.ParseAppID)
	checkReq.Header.Set("X-Parse-Javascript-Key", h.ParseJSKey)

	resp, err := http.DefaultClient.Do(checkReq)
	if err != nil {
		http.Error(w, "Parse-Fehler", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	var existing struct {
		Results []any `json:"results"`
	}
	json.NewDecoder(resp.Body).Decode(&existing)

	if len(existing.Results) > 0 {
		http.Error(w, "Kind ist bereits einer Riege zugeordnet", http.StatusConflict)
		return
	}

	// ---- Insert ----
	payload := map[string]any{
		"kindID": map[string]any{
			"__type":    "Pointer",
			"className": "Kind",
			"objectId":  req.KindObjectID,
		},
		"riegenID": map[string]any{
			"__type":    "Pointer",
			"className": "Riege",
			"objectId":  req.RiegeObjectID,
		},
		"position": req.Position,
	}

	body, _ := json.Marshal(payload)

	createReq, _ := http.NewRequest(
		http.MethodPost,
		h.ParseServerURL+"/classes/kinderDerRiege",
		bytes.NewBuffer(body),
	)

	createReq.Header.Set("X-Parse-Application-Id", h.ParseAppID)
	createReq.Header.Set("X-Parse-Javascript-Key", h.ParseJSKey)
	createReq.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(createReq)
	if err != nil {
		http.Error(w, "Speichern fehlgeschlagen", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"message": "Kind erfolgreich Riege zugeordnet",
	})
}

// ===== UPDATE =====
func (h *KindHandler) UpdateKindRiegePosition(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req KinderDerRiegeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Ungültiges JSON", http.StatusBadRequest)
		return
	}

	if req.KindObjectID == "" || req.RiegeObjectID == "" || req.Position <= 0 {
		http.Error(w, "Pflichtfelder fehlen", http.StatusBadRequest)
		return
	}

	lock := h.lockForKey(kinderDerRiegeKey(req.KindObjectID, req.RiegeObjectID))
	select {
	case lock <- struct{}{}:
		defer func() { <-lock }()
	default:
		http.Error(w, "Zuordnung wird bearbeitet", http.StatusConflict)
		return
	}

	// ---- Suche Zuordnung ----
	where := map[string]any{
		"kindID": map[string]any{
			"__type":    "Pointer",
			"className": "Kind",
			"objectId":  req.KindObjectID,
		},
		"riegenID": map[string]any{
			"__type":    "Pointer",
			"className": "Riege",
			"objectId":  req.RiegeObjectID,
		},
	}

	whereJSON, _ := json.Marshal(where)
	searchURL := h.ParseServerURL + "/classes/kinderDerRiege?where=" +
		url.QueryEscape(string(whereJSON))

	searchReq, _ := http.NewRequest(http.MethodGet, searchURL, nil)
	searchReq.Header.Set("X-Parse-Application-Id", h.ParseAppID)
	searchReq.Header.Set("X-Parse-Javascript-Key", h.ParseJSKey)

	resp, err := http.DefaultClient.Do(searchReq)
	if err != nil {
		http.Error(w, "Parse-Fehler", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	var result struct {
		Results []struct {
			ObjectID string `json:"objectId"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		http.Error(w, "Antwort ungültig", http.StatusInternalServerError)
		return
	}

	if len(result.Results) == 0 {
		http.Error(w, "Zuordnung nicht gefunden", http.StatusNotFound)
		return
	}

	objectID := result.Results[0].ObjectID

	// ---- Update ----
	payload := map[string]any{
		"position": req.Position,
	}

	body, _ := json.Marshal(payload)

	updateReq, _ := http.NewRequest(
		http.MethodPut,
		h.ParseServerURL+"/classes/kinderDerRiege/"+objectID,
		bytes.NewBuffer(body),
	)

	updateReq.Header.Set("X-Parse-Application-Id", h.ParseAppID)
	updateReq.Header.Set("X-Parse-Javascript-Key", h.ParseJSKey)
	updateReq.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(updateReq)
	if err != nil {
		http.Error(w, "Update fehlgeschlagen", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	json.NewEncoder(w).Encode(map[string]any{
		"message": "Position erfolgreich aktualisiert",
	})
}

// ===== DELETE =====
func (h *KindHandler) RemoveKindFromRiege(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req KinderDerRiegeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Ungültiges JSON", http.StatusBadRequest)
		return
	}

	if req.KindObjectID == "" || req.RiegeObjectID == "" {
		http.Error(w, "Pflichtfelder fehlen", http.StatusBadRequest)
		return
	}

	lock := h.lockForKey(kinderDerRiegeKey(req.KindObjectID, req.RiegeObjectID))
	select {
	case lock <- struct{}{}:
		defer func() { <-lock }()
	default:
		http.Error(w, "Zuordnung wird bearbeitet", http.StatusConflict)
		return
	}

	// ---- Suche Zuordnung ----
	where := map[string]any{
		"kindID": map[string]any{
			"__type":    "Pointer",
			"className": "Kind",
			"objectId":  req.KindObjectID,
		},
		"riegenID": map[string]any{
			"__type":    "Pointer",
			"className": "Riege",
			"objectId":  req.RiegeObjectID,
		},
	}

	whereJSON, _ := json.Marshal(where)
	searchURL := h.ParseServerURL + "/classes/kinderDerRiege?where=" +
		url.QueryEscape(string(whereJSON))

	searchReq, _ := http.NewRequest(http.MethodGet, searchURL, nil)
	searchReq.Header.Set("X-Parse-Application-Id", h.ParseAppID)
	searchReq.Header.Set("X-Parse-Javascript-Key", h.ParseJSKey)

	resp, err := http.DefaultClient.Do(searchReq)
	if err != nil {
		http.Error(w, "Parse-Fehler", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	var result struct {
		Results []struct {
			ObjectID string `json:"objectId"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		http.Error(w, "Antwort ungültig", http.StatusInternalServerError)
		return
	}

	if len(result.Results) == 0 {
		http.Error(w, "Zuordnung nicht gefunden", http.StatusNotFound)
		return
	}

	objectID := result.Results[0].ObjectID

	// ---- Delete ----
	deleteReq, _ := http.NewRequest(
		http.MethodDelete,
		h.ParseServerURL+"/classes/kinderDerRiege/"+objectID,
		nil,
	)

	deleteReq.Header.Set("X-Parse-Application-Id", h.ParseAppID)
	deleteReq.Header.Set("X-Parse-Javascript-Key", h.ParseJSKey)

	resp, err = http.DefaultClient.Do(deleteReq)
	if err != nil {
		http.Error(w, "Löschen fehlgeschlagen", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	json.NewEncoder(w).Encode(map[string]any{
		"message": "Kind erfolgreich aus Riege entfernt",
	})
}
