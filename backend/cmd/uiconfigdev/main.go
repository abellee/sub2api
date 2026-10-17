// uiconfigdev is a loopback development companion for Vite using a remote API.
package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/uiconfigdev"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:18130", "Loopback HTTP address")
	path := flag.String("sqlite-path", "data/group_categories.db", "Shared category/menu SQLite path")
	backend := flag.String("auth-backend", "", "Main API used to verify Bearer tokens")
	flag.Parse()
	host, _, err := net.SplitHostPort(*listen)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		log.Fatal("listen must use a loopback IP address")
	}
	parsed, err := url.Parse(*backend)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		log.Fatal("auth-backend must be an HTTP(S) API origin")
	}
	store := service.NewGroupCategoryStore(*path)
	if _, err := store.GetUserMenu(context.Background()); err != nil {
		log.Fatalf("open UI configuration: %v", err)
	}
	server := &http.Server{
		Addr:              *listen,
		Handler:           uiconfigdev.NewHandler(store, *backend),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("UI configuration: %s (SQLite: %s)", *listen, *path)
	log.Fatal(server.ListenAndServe())
}
