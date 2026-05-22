package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func userDataDir() string {
	home, err := os.UserHomeDir()

	if err != nil {
		log.Fatalf("Cannot determine home directory: %v", err)
	}

	dir := filepath.Join(home, ".config", "appletunesd")

	if err := os.MkdirAll(dir, 0700); err != nil {
		log.Fatalf("Cannot create data directory %s: %v", dir, err)
	}

	return dir
}

type MusicDaemon struct {
	mu          sync.Mutex
	allocCtx    context.Context
	cancelAlloc context.CancelFunc
	ctx         context.Context
	cancelCtx   context.CancelFunc
}

func (m *MusicDaemon) startHeadless() error {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.UserDataDir(userDataDir()),
	)

	m.allocCtx, m.cancelAlloc = chromedp.NewExecAllocator(context.Background(), opts...)
	m.ctx, m.cancelCtx = chromedp.NewContext(m.allocCtx)

	log.Println("Initializing Apple Music in the background...")

	err := chromedp.Run(m.ctx,
		chromedp.Navigate("https://music.apple.com"),
		chromedp.Poll(`
			window.MusicKit !== undefined &&
			window.MusicKit.getInstance() !== undefined
		`, nil, chromedp.WithPollingInterval(1*time.Second)),
	)

	if err != nil {
		return fmt.Errorf("failed to initialize background browser: %w", err)
	}

	log.Println("Background browser ready.")
	return nil
}

func (m *MusicDaemon) stopHeadless() {
	if m.cancelCtx != nil {
		m.cancelCtx()
		m.cancelCtx = nil
	}

	if m.cancelAlloc != nil {
		m.cancelAlloc()
		m.cancelAlloc = nil
	}

	time.Sleep(500 * time.Millisecond)
}

func (m *MusicDaemon) handleLogin(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopHeadless()

	log.Println("Launching Chromium for login...")

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.UserDataDir(userDataDir()),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()

	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()

	ctx, cancelTimeout := context.WithTimeout(ctx, 10*time.Minute)
	defer cancelTimeout()

	err := chromedp.Run(ctx,
		chromedp.Navigate("https://music.apple.com"),
	)

	if err != nil {
		http.Error(w, "Login failed", http.StatusInternalServerError)
		m.startHeadless()
		return
	}

	for {
		var isAuthorized bool

		evalErr := chromedp.Run(ctx, chromedp.Evaluate(`
			window.MusicKit !== undefined &&
			window.MusicKit.getInstance() !== undefined &&
			window.MusicKit.getInstance().isAuthorized === true
		`, &isAuthorized))

		if evalErr == nil && isAuthorized {
			break
		}

		if ctx.Err() != nil {
			log.Printf("Login failed or timed out\n")
			http.Error(w, "Login failed", http.StatusInternalServerError)
			m.startHeadless()
			return
		}

		time.Sleep(2 * time.Second)
	}

	chromedp.Run(ctx, chromedp.Navigate("about:blank"))
	time.Sleep(3 * time.Second)

	w.Write([]byte("Login successful. Session saved\n"))

	m.startHeadless()
}

func (m *MusicDaemon) handleLogout(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	err := chromedp.Run(m.ctx,
		chromedp.Evaluate(`
			window.MusicKit.getInstance().unauthorize();
		`, nil),
	)

	if err != nil {
		http.Error(w, "Logout failed", http.StatusInternalServerError)
		return
	}

	w.Write([]byte("Logout successful\n"))
}

func (m *MusicDaemon) handleAccount(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var isAuthorized bool

	err := chromedp.Run(m.ctx,
		chromedp.Evaluate(`
			window.MusicKit !== undefined &&
			window.MusicKit.getInstance() !== undefined &&
			window.MusicKit.getInstance().isAuthorized === true
		`, &isAuthorized),
	)

	if err != nil {
		http.Error(w, "Failed to check account status", http.StatusInternalServerError)
		return
	}

	if isAuthorized {
		w.Write([]byte("Already logged in\n"))
	} else {
		w.Write([]byte("Not logged in\n"))
	}
}

func main() {
	daemon := &MusicDaemon{}

	if err := daemon.startHeadless(); err != nil {
		log.Fatalf("Background browser failed to start: %v\n", err)
	}

	defer daemon.stopHeadless()

	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Post("/login", daemon.handleLogin)
	r.Post("/logout", daemon.handleLogout)
	r.Get("/account", daemon.handleAccount)

	server := &http.Server{Addr: ":8080", Handler: r}

	log.Println("appletunesd listening on http://localhost:8080")

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
