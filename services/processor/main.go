package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	fmt.Printf("processor listening on :%s\n", port)
	http.ListenAndServe(":"+port, nil)
}
