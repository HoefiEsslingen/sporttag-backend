package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"sporttag/handler"
)

type Config struct {
	Deadline       time.Time `json:"deadline"`
	SuperUserPass  string    `json:"superuser_password"`
	ParseAppID     string    `json:"parse_app_id"`
	ParseJSKey     string    `json:"parse_js_key"`
	ParseServerURL string    `json:"parse_server_url"`
}

// Lädt Konfigurationsdaten insbesondere das Ende-Datum der Registrierung
// ab dem Ende-Datum haben beliebige Anwender keinen Zugriff mehr.
//
// lediglich der Superuser (Passwort in Umgebungsvariable SPORTTAG_SUPERUSER_PASS)
// kann weiterhin Änderungen vornehmen.
func loadConfig() (Config, error) {
	var config Config
	b, err := os.ReadFile("config.json")
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(b, &config)
	return config, err
}

func main() {
	// ---- Konfiguration laden ----
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Config-Fehler: %v", err)
	}

	// ---- Handler initialisieren ----
	kindHandler := &handler.KindHandler{
		Deadline:       config.Deadline,
		ParseAppID:     config.ParseAppID,
		ParseJSKey:     config.ParseJSKey,
		ParseServerURL: config.ParseServerURL,
	}

	// 🔁 EINHEITLICHE RESSOURCE
	http.HandleFunc("/kind", kindHandler.KindRouter)

	// ---- Server starten ----
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// ---- Server starten ----
	log.Println("Server läuft auf :" + port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
