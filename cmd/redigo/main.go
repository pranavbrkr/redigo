package main

import (
	"flag"
	"log"
	"strconv"

	"github.com/pranavbrkr/redigo/internal/aof"
	"github.com/pranavbrkr/redigo/internal/server"
	"github.com/pranavbrkr/redigo/internal/store"
)

func main() {
	port := flag.Int("port", 6379, "TCP port to listen on")
	aofEnabled := flag.Bool("aof-enabled", false, "Enable append-only file persistence")
	aofPath := flag.String("aof-path", "appendonly.aof", "Path to AOF file")

	flag.Parse()

	addr := ":" + strconv.Itoa(*port)

	st := store.New()

	var aw aof.Writer = aof.NewNoop()
	if *aofEnabled {
		// Replay existing AOF into the store
		err := aof.Replay(*aofPath, func(cmd string, args []string) error {
			switch cmd {
			case "SET":
				if len(args) != 2 {
					return nil
				}
				st.Set(args[0], []byte(args[1]))
			case "DEL":
				for _, k := range args {
					st.Del(k)
				}
			case "EXPIRE":
				if len(args) != 2 {
					return nil
				}
				sec, err := strconv.ParseInt(args[1], 10, 64)
				if err != nil {
					return nil
				}
				st.Expire(args[0], sec)
			default:
				// Ignore unknown entries to keep replay resilient
			}
			return nil
		})
		if err != nil {
			log.Fatalf("open replay failed: %v", err)
		}

		// Open AOF for appending
		faof, err := aof.Open(*aofPath)
		if err != nil {
			log.Fatalf("open aof: %v", err)
		}
		aw = faof
	}
	defer aw.Close()

	s, bound, err := server.Start(addr, st, aw)
	if err != nil {
		log.Fatalf("failed to start server on %s: %v", addr, err)
	}
	defer s.Close()

	log.Printf("redigo listening on %s", bound)

	// Adding shutdown later
	select {}

}
