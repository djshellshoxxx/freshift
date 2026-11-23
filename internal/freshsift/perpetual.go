package freshsift

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const maxRememberedCandidates = 20000

// SetStateFile enables opt-in restart persistence for perpetual scans.
// Ordinary NewEngine() instances are in-memory unless explicitly configured.
func (e *Engine) SetStateFile(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stateFile = path
}

// persistConfig is always called with e.mu held; never persists API credentials.
func (e *Engine) persistConfig(c Config) error {
	path := e.stateFile
	if path == "" {
		return nil
	}
	if !c.Perpetual {
		err := os.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".freshsift-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		// Windows may not replace an existing destination with os.Rename.
		if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			return err
		}
		if err = os.Rename(f.Name(), path); err != nil {
			return err
		}
	}
	return nil
}

// ResumeSavedScan resumes only an explicitly enabled perpetual job.
func (e *Engine) ResumeSavedScan() (bool, error) {
	e.mu.RLock()
	path := e.stateFile
	e.mu.RUnlock()
	if path == "" {
		return false, nil
	}
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if stat.Size() > 1<<20 {
		return false, fmt.Errorf("saved scan exceeds 1 MiB")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return false, fmt.Errorf("saved perpetual scan invalid: %w", err)
	}
	if !c.Perpetual {
		return false, fmt.Errorf("saved scan is not marked perpetual")
	}
	if err := e.Start(c); err != nil {
		return false, err
	}
	return true, nil
}

func (e *Engine) rememberCandidate(c Candidate) {
	if c.URL == "" {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.revisit[c.URL]; ok {
		return
	}
	if len(e.revisitOrder) >= maxRememberedCandidates {
		delete(e.revisit, e.revisitOrder[0])
		e.revisitOrder = e.revisitOrder[1:]
		if e.revisitCursor > 0 {
			e.revisitCursor--
		}
	}
	e.revisit[c.URL] = c
	e.revisitOrder = append(e.revisitOrder, c.URL)
}

func (e *Engine) revisitCandidates(limit int) []Candidate {
	e.mu.Lock()
	defer e.mu.Unlock()
	count := len(e.revisitOrder)
	if count == 0 || limit < 1 {
		return nil
	}
	if limit > count {
		limit = count
	}
	out := make([]Candidate, 0, limit)
	start := e.revisitCursor % count
	for i := 0; i < limit; i++ {
		c := e.revisit[e.revisitOrder[(start+i)%count]]
		c.Source = "revisit"
		out = append(out, c)
	}
	e.revisitCursor = (start + limit) % count
	return out
}

// waitForRound honours Stop immediately and Pause before beginning a new round.
func (e *Engine) waitForRound(ctx context.Context, due time.Time) bool {
	for {
		if !e.awaitResume(ctx) {
			return false
		}
		remaining := time.Until(due)
		if remaining <= 0 {
			return ctx.Err() == nil
		}
		if remaining > 200*time.Millisecond {
			remaining = 200 * time.Millisecond
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(remaining):
		}
	}
}
