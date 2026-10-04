package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
)

type recorder struct {
	mu     sync.Mutex
	events []string
}

func (r *recorder) add(e string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

func (r *recorder) has(e string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.events {
		if x == e {
			return true
		}
	}
	return false
}

func (r *recorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

type fakeWorker struct {
	name string
	rec  *recorder
}

func (w *fakeWorker) Start() { w.rec.add("start " + w.name) }
func (w *fakeWorker) Stop()  { w.rec.add("stop " + w.name) }

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return ln
}

func TestServe_ShutdownStopsWorkersThenDrainsInFlightRequests(t *testing.T) {
	rec := &recorder{}
	release := make(chan struct{})
	entered := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		rec.add("request finished")
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{Handler: mux}
	ln := listen(t)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, srv, ln, ccc.NopLogger, 5*time.Second, &fakeWorker{"a", rec}, &fakeWorker{"b", rec})
	}()

	respCode := make(chan int, 1)
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String() + "/slow")
		if err != nil {
			respCode <- -1
			return
		}
		resp.Body.Close()
		respCode <- resp.StatusCode
	}()
	<-entered

	cancel()
	waitFor(t, func() bool { return rec.has("stop a") && rec.has("stop b") })
	select {
	case err := <-done:
		t.Fatalf("serve returned while a request was in flight: %v", err)
	case <-time.After(30 * time.Millisecond):
	}

	close(release)
	if code := <-respCode; code != http.StatusOK {
		t.Fatalf("in-flight request got %d, want 200", code)
	}
	if err := <-done; err != nil {
		t.Fatalf("serve = %v", err)
	}

	want := []string{"start a", "start b", "stop a", "stop b", "request finished"}
	if got := rec.snapshot(); len(got) != len(want) {
		t.Fatalf("events = %v, want %v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("events = %v, want %v", got, want)
			}
		}
	}
}

func TestServe_ServerFailureStopsWorkersAndReturnsError(t *testing.T) {
	rec := &recorder{}
	ln := listen(t)
	ln.Close() // Serve fails immediately

	err := serve(context.Background(), &http.Server{}, ln, ccc.NopLogger, time.Second, &fakeWorker{"a", rec})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !rec.has("stop a") {
		t.Fatalf("workers not stopped: %v", rec.snapshot())
	}
}

func TestServe_ShutdownTimeoutForcesCloseAndReturnsError(t *testing.T) {
	entered := make(chan struct{})
	block := make(chan struct{})
	defer close(block)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-block
	})}
	ln := listen(t)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, srv, ln, ccc.NopLogger, 20*time.Millisecond) }()

	go http.Get("http://" + ln.Addr().String() + "/")
	<-entered
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v, want deadline exceeded", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve did not give up after the shutdown timeout")
	}
}
