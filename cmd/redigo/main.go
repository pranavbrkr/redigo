package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/pranavbrkr/redigo/internal/aof"
	"github.com/pranavbrkr/redigo/internal/server"
	"github.com/pranavbrkr/redigo/internal/store"
)

func main() {
	port := flag.Int("port", 6379, "TCP port to listen on")
	aofEnabled := flag.Bool("aof-enabled", false, "Enable append-only file persistence")
	aofPath := flag.String("aof-path", "appendonly.aof", "Path to AOF file")
	aofFsync := flag.String("aof-fsync", "everysec", "AOF fsync policy: always|everysec|never")

	flag.Parse()
	policy := aof.ParseFsyncPolicy(*aofFsync)

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
				// Backwards compat with older AOFs (relative expiry)
				if len(args) != 2 {
					return nil
				}
				sec, err := strconv.ParseInt(args[1], 10, 64)
				if err != nil {
					return nil
				}
				st.Expire(args[0], sec)

			case "EXPIREAT":
				if len(args) != 2 {
					return nil
				}
				ts, err := strconv.ParseInt(args[1], 10, 64)
				if err != nil {
					return nil
				}
				st.ExpireAt(args[0], ts)

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

	s, bound, err := server.Start(addr, st, aw, policy)
	if err != nil {
		log.Fatalf("failed to start server on %s: %v", addr, err)
	}

	log.Printf("redigo listening on %s", bound)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	sig := <-sigCh
	signal.Stop(sigCh)
	log.Printf("shutdown signal received: %v", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	shutdownDone := make(chan struct{})
	go func() {
		// Close server first
		_ = s.Close()
		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
		log.Printf("shutdown complete")
	case <-ctx.Done():
		log.Printf("shutdown timed out")
	}
}
