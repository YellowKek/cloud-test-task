package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	var port string
	if len(os.Args) > 1 {
		port = os.Args[1]
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
		log.Print("pong")
	})

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	fmt.Printf("Starting server at port %s\n", port)

	if err := server.ListenAndServe(); err != nil {
		fmt.Println("Error starting the server:", err)
	}
}
