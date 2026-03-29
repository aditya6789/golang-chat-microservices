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
	dir := "frontend/out"
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Fatal("run from repo root: frontend/out not found — run: cd frontend && npm install && npm run build")
	}
	log.Printf("Orbit Chat UI: http://127.0.0.1:%s  (API via gateway :8080)\n", port)
	log.Fatal(http.ListenAndServe(":"+port, http.FileServer(http.Dir(dir))))
}
