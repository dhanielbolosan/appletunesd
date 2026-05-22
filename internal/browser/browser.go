package browser

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/chromedp"
)

const (
	Ready = `(function() {
		try { return typeof MusicKit !== 'undefined' && MusicKit.getInstance() !== undefined; }
		catch(e) { return false; }
	})()`

	Authorized = `(function() {
		try { return MusicKit.getInstance().isAuthorized === true; }
		catch(e) { return false; }
	})()`

	Unauthorize = `(function() {
		try { MusicKit.getInstance().unauthorize(); return true; }
		catch(e) { return false; }
	})()`

	musicURL    = "https://music.apple.com"
	loadTimeout = 90 * time.Second
)

type Browser struct {
	allocCtx    context.Context
	cancelAlloc context.CancelFunc
	ctx         context.Context
	cancelCtx   context.CancelFunc
	headless    bool
}

func dataDir() string {
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

func NewHeadless() *Browser {
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("no-sandbox", true),
		chromedp.UserDataDir(dataDir()),
		chromedp.Flag("headless", "new"),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancelCtx := chromedp.NewContext(allocCtx)

	return &Browser{
		allocCtx: allocCtx, cancelAlloc: cancelAlloc,
		ctx: ctx, cancelCtx: cancelCtx,
		headless: true,
	}
}

func NewVisible() *Browser {
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("no-sandbox", true),
		chromedp.UserDataDir(dataDir()),
		chromedp.Flag("headless", false),
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancelCtx := chromedp.NewContext(allocCtx)

	return &Browser{
		allocCtx: allocCtx, cancelAlloc: cancelAlloc,
		ctx: ctx, cancelCtx: cancelCtx,
		headless: false,
	}
}

func (b *Browser) Context() context.Context { return b.ctx }

func (b *Browser) NavigateAndWait() error {
	if err := chromedp.Run(b.ctx,
		chromedp.Navigate(musicURL),
		chromedp.WaitReady("body"),
	); err != nil {
		return fmt.Errorf("navigation failed: %w", err)
	}

	log.Println("Page loaded, waiting for MusicKit JS to initialize...")

	loadCtx, cancel := context.WithTimeout(b.ctx, loadTimeout)
	defer cancel()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-loadCtx.Done():
			return fmt.Errorf("MusicKit did not become ready: %w", loadCtx.Err())
		case <-ticker.C:
			var ready bool

			if err := chromedp.Run(loadCtx, chromedp.Evaluate(Ready, &ready)); err != nil {
				log.Printf("  MusicKit check error (retrying): %v", err)
				continue
			}

			if ready {
				log.Println("MusicKit is ready.")
				return nil
			}

			var title string

			chromedp.Run(loadCtx, chromedp.Evaluate(`document.title`, &title))
			log.Printf("  MusicKit not ready yet (page: %q)", title)
		}
	}
}

func (b *Browser) IsAuthorized() (bool, error) {
	var result bool

	if err := chromedp.Run(b.ctx, chromedp.Evaluate(Authorized, &result)); err != nil {
		return false, fmt.Errorf("auth check failed: %w", err)
	}

	return result, nil
}

func (b *Browser) Unauthorize() error {
	return chromedp.Run(b.ctx, chromedp.Evaluate(Unauthorize, nil))
}

func (b *Browser) Close() {
	if b.ctx != nil && b.ctx.Err() == nil {
		flushCtx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
		chromedp.Run(flushCtx, chromedp.Navigate("about:blank"))
		cancel()
		time.Sleep(2 * time.Second)
	}

	if b.cancelCtx != nil {
		b.cancelCtx()
	}

	if b.cancelAlloc != nil {
		b.cancelAlloc()
	}

	time.Sleep(1 * time.Second)
}
