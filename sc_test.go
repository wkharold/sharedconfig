package sharedconfig

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"
)

func TestReadConfig(t *testing.T) {
	cfg := map[string]string{
		"height": "180",
		"weight": "77",
	}

	fnm, err := writeTestConfig(cfg)
	if err != nil {
		t.Fatal("can't write test config: ", err)
	}

	sc, err := New(fnm)
	if err != nil {
		t.Fatal("can't read test config: ", err)
	}
	defer sc.Close()

	for k, v := range cfg {
		if scv := sc.Get(k); scv != v {
			t.Fatalf("expected %s got %s", v, scv)
		}
	}
}

func TestMultiReadConfig(t *testing.T) {
	cfg := map[string]string{
		"height": "180",
		"weight": "77",
	}

	fnm, err := writeTestConfig(cfg)
	if err != nil {
		t.Fatal("can't write test config: ", err)
	}

	rdr := func(id int, sc *SharedConfig, cfg map[string]string, wg *sync.WaitGroup, doit chan struct{}) {
		select {
		case <-doit:
			for i := 0; i < 100000; i++ {
				for k, v := range cfg {
					if scv := sc.Get(k); scv != v {
						t.Fatal("expected %s got %s", v, scv)
					}
				}
			}
		}
		wg.Done()
	}

	sc, err := New(fnm)
	if err != nil {
		t.Fatal("can't read test config: ", err)
	}
	defer sc.Close()

	var wg sync.WaitGroup
	doit := make(chan struct{})

	wg.Add(6)

	go rdr(1, sc, cfg, &wg, doit)
	go rdr(2, sc, cfg, &wg, doit)
	go rdr(3, sc, cfg, &wg, doit)
	go rdr(4, sc, cfg, &wg, doit)
	go rdr(5, sc, cfg, &wg, doit)
	go rdr(6, sc, cfg, &wg, doit)

	close(doit)

	wg.Wait()
}

func TestConfigChange(t *testing.T) {
	cfg := map[string]string{
		"height": "180",
		"weight": "77",
	}

	fnm, err := writeTestConfig(cfg)
	if err != nil {
		t.Fatal("can't write test config: ", err)
	}

	sc, err := New(fnm)
	if err != nil {
		t.Fatal("can't read test config: ", err)
	}

	done := make(chan chan bool)

	go func(sc *SharedConfig, cfg map[string]string, done chan chan bool) {
		var changed bool
	GetLoop:
		for {
			select {
			case rch := <-done:
				rch <- changed
				break GetLoop
			default:
				for k, v := range cfg {
					if scv := sc.Get(k); scv != v {
						changed = true
					}
				}
			}
		}
	}(sc, cfg, done)

	time.Sleep(2 * time.Second)

	cfg = map[string]string{
		"height": "180",
		"weight": "85",
	}

	f, err := os.Create(fnm)
	if err != nil {
		t.Fatal("can't open config file: ", err)
	}
	defer f.Close()

	ec := json.NewEncoder(f)
	if ec == nil {
		t.Fatal("can't create encoder")
	}

	if err = ec.Encode(&cfg); err != nil {
		t.Fatal("can't encode new config: ", err)
	}

	time.Sleep(2 * time.Second)

	rch := make(chan bool)
	done <- rch

	if changed := <-rch; !changed {
		t.Fatal("didn't detect change")
	}
}

func writeTestConfig(config map[string]string) (string, error) {
	f, err := ioutil.TempFile("/tmp", "sctest")
	if err != nil {
		return "", err
	}
	defer f.Close()

	ec := json.NewEncoder(f)
	if ec == nil {
		return "", fmt.Errorf("can't create encoder")
	}

	if err := ec.Encode(&config); err != nil {
		return "", err
	}

	return f.Name(), nil
}
