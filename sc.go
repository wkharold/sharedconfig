package sharedconfig

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/go-fsnotify/fsnotify"
)

// SharedConfig represents a shared (global) configuration backed by a file.
type SharedConfig struct {
	scchans
	h schandler
}

type schandler struct {
	scchans
	scmap map[string]string
	fnm   string
}

type scchans struct {
	done chan struct{}
	kch  chan string
	vch  chan string
}

// New creates a new shared configuration backed by the specified file.
func New(fnm string) (*SharedConfig, error) {
	done := make(chan struct{})
	kch := make(chan string)
	vch := make(chan string)

	h := schandler{scchans{done, kch, vch}, make(map[string]string), fnm}
	if err := h.run(); err != nil {
		return nil, err
	}

	return &SharedConfig{scchans{done, kch, vch}, h}, nil
}

// Close releases all the resources associated with this shared configuration.
func (sc SharedConfig) Close() {
	sc.done <- struct{}{}
}

// Get returns the shared configuration value associated with the given key, if any.
// If there is no associated value, or the shared config is closed, the empty string
// is returned.
func (sc SharedConfig) Get(key string) string {
	select {
	case <-sc.done:
		return ""
	case sc.kch <- key:
		return <-sc.vch
	}
}

func (h schandler) run() error {
	err := h.loadConfig()
	if err != nil {
		return err
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	err = w.Add(h.fnm)
	if err != nil {
		return err
	}

	go h.loop(w)

	return nil
}

func (h schandler) loop(w *fsnotify.Watcher) {
HandlerLoop:
	for {
		select {
		case <-h.done:
			break HandlerLoop
		case e := <-w.Events:
			if e.Op&fsnotify.Write == fsnotify.Write {
				err := h.loadConfig()
				if err != nil {
					log.Println("can't load config: ", err)
				}
			}
		case key := <-h.kch:
			h.vch <- h.scmap[key]
		}
	}
}

func (h *schandler) loadConfig() error {
	f, err := os.Open(h.fnm)
	if err != nil {
		return err
	}

	dc := json.NewDecoder(f)
	if dc == nil {
		return fmt.Errorf("can't create decoder")
	}

	if err = dc.Decode(&h.scmap); err != nil {
		return err
	}

	return nil
}
