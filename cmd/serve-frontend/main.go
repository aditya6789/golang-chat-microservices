package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("FRONTEND_PORT")
	if port == "" {
		port = "8888"
	}
	dir := "frontend"
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Fatal("run from repo root: frontend/ not found")
	}
	log.Printf("test UI: http://127.0.0.1:%s  (API stays on gateway :8080)\n", port)
	log.Fatal(http.ListenAndServe(":"+port, http.FileServer(http.Dir(dir))))
}
