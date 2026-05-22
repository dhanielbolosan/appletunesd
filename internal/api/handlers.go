package api

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/dhanielbolosan/appletunesd/internal/browser"
)

type Handlers struct {
	mu      sync.Mutex
	browser *browser.Browser
}

func New(b *browser.Browser) *Handlers {
	return &Handlers{browser: b}
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if ok, _ := h.browser.IsAuthorized(); ok {
		w.Write([]byte("Already logged in\n"))
		return
	}

	h.browser.Close()
	log.Println("Launching Chromium for login...")

	visible := browser.NewVisible()

	if err := visible.NavigateAndWait(); err != nil {
		visible.Close()
		http.Error(w, "Navigation failed", http.StatusInternalServerError)
		h.restartHeadless()
		return
	}

	loginSuccess := h.waitForAuth(visible)

	if loginSuccess {
		log.Println("Login detected — flushing session to disk...")
		time.Sleep(5 * time.Second)
	}

	visible.Close()
	time.Sleep(2 * time.Second)
	h.restartHeadless()

	if !loginSuccess {
		http.Error(w, "Login timed out or failed", http.StatusRequestTimeout)
		return
	}

	w.Write([]byte("Login successful. Session persisted to disk.\n"))
}

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if ok, _ := h.browser.IsAuthorized(); !ok {
		w.Write([]byte("Not logged in\n"))
		return
	}

	if err := h.browser.Unauthorize(); err != nil {
		http.Error(w, "Logout failed", http.StatusInternalServerError)
		return
	}

	w.Write([]byte("Logout successful\n"))
}

// TODO: MORE INFO ABOUT ACCOUNT?
func (h *Handlers) Account(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	ok, err := h.browser.IsAuthorized()
	if err != nil {
		http.Error(w, "Failed to check auth state", http.StatusInternalServerError)
		return
	}

	if ok {
		w.Write([]byte("Logged in\n"))
	} else {
		w.Write([]byte("Not logged in\n"))
	}
}

func (h *Handlers) waitForAuth(b *browser.Browser) bool {
	ctx := b.Context()
	for {
		if ok, _ := b.IsAuthorized(); ok {
			return true
		}
		if ctx.Err() != nil {
			return false
		}
		time.Sleep(2 * time.Second)
	}
}

func (h *Handlers) restartHeadless() {
	h.browser = browser.NewHeadless()
	if err := h.browser.NavigateAndWait(); err != nil {
		log.Printf("restartHeadless failed: %v", err)
	}
}
