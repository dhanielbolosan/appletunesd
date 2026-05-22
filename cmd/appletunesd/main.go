package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dhanielbolosan/appletunesd/internal/api"
	"github.com/dhanielbolosan/appletunesd/internal/browser"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var shutdownChan = make(chan struct{})

func main() {
	b := browser.NewHeadless()

	if err := b.NavigateAndWait(); err != nil {
		log.Fatalf("Background browser failed to start: %v\n", err)
	}

	defer b.Close()

	if ok, _ := b.IsAuthorized(); ok {
		log.Println("Session restored from cookies")
	} else {
		log.Println("No active session. Run 'appletunes login' to log in.")
	}

	log.Println("Background browser ready.")

	h := api.New(b)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Post("/login", h.Login)
	r.Post("/logout", h.Logout)
	r.Get("/account", h.Account)
	r.Post("/quit", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Shutting down appletunesd\n"))
		go func() {
			time.Sleep(100 * time.Millisecond)
			shutdownChan <- struct{}{}
		}()
	})

	server := &http.Server{Addr: ":8080", Handler: r}

	quitOS := make(chan os.Signal, 1)
	signal.Notify(quitOS, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		log.Println("appletunesd listening on http://localhost:8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Listen error: %v\n", err)
		}
	}()

	select {
	case <-quitOS:
		log.Println("Received OS interrupt...")
	case <-shutdownChan:
		log.Println("Received quit command from CLI...")
	}

	log.Println("Cleaning up and exiting...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(shutdownCtx)
}
