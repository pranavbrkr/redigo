package main

import (
	"flag"
	"log"
	"strconv"

	"github.com/pranavbrkr/redigo/internal/server"
	"github.com/pranavbrkr/redigo/internal/store"
)

func main() {
	port := flag.Int("port", 6379, "TCP port to listen on")
	flag.Parse()

	addr := ":" + strconv.Itoa(*port)

	st := store.New()
	s, bound, err := server.Start(addr, st)
	if err != nil {
		log.Fatalf("failed to start server on %s: %v", addr, err)
	}
	defer s.Close()

	log.Printf("redigo listening on %s", bound)

	// Adding shutdown later
	select {}

}
