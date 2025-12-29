package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
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

func loadConfig() (Config, error) {
	var config Config
	b, err := ioutil.ReadFile("config.json")
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(b, &config)
	return config, err
}

func main() {
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Config-Fehler: %v", err)
	}
	kindHandler := &handler.KindHandler{
		Deadline:       config.Deadline,
		ParseAppID:     config.ParseAppID,
		ParseJSKey:     config.ParseJSKey,
		ParseServerURL: config.ParseServerURL,
	}
	http.HandleFunc("/registerKind", kindHandler.RegisterKind)
	log.Println("Server läuft auf :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
