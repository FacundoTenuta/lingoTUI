package chatgptauth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
)

const callbackPath = "/callback"

type LocalCallbackWaiter struct {
	listener net.Listener
	server   *http.Server
	result   chan Callback

	mu            sync.RWMutex
	expectedState string
	waiting       bool

	serveOnce sync.Once
	closeOnce sync.Once
	waitOnce  sync.Once
	closeErr  error
}

func NewLocalCallbackWaiter() (*LocalCallbackWaiter, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen for ChatGPT OAuth callback: %w", err)
	}
	return NewLocalCallbackWaiterWithListener(listener)
}

func NewLocalCallbackWaiterWithListener(listener net.Listener) (*LocalCallbackWaiter, error) {
	if listener == nil {
		return nil, fmt.Errorf("ChatGPT OAuth callback listener is required")
	}
	server := &http.Server{}
	w := &LocalCallbackWaiter{
		listener: listener,
		server:   server,
		result:   make(chan Callback, 1),
	}
	server.Handler = http.HandlerFunc(w.handleCallback)
	return w, nil
}

func (w *LocalCallbackWaiter) RedirectURL() string {
	return "http://" + w.listener.Addr().String() + callbackPath
}

func (w *LocalCallbackWaiter) Wait(ctx context.Context, expectedState string) (Callback, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if expectedState == "" {
		return Callback{}, fmt.Errorf("ChatGPT OAuth callback state is required")
	}
	if err := ctx.Err(); err != nil {
		_ = w.Close()
		return Callback{}, err
	}

	var firstWait bool
	w.waitOnce.Do(func() {
		firstWait = true
		w.mu.Lock()
		w.expectedState = expectedState
		w.waiting = true
		w.mu.Unlock()
	})
	if !firstWait {
		return Callback{}, fmt.Errorf("ChatGPT OAuth callback waiter already used")
	}

	w.serveOnce.Do(func() {
		go func() {
			if err := w.server.Serve(w.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				_ = w.Close()
			}
		}()
	})

	select {
	case callback := <-w.result:
		_ = w.Close()
		return callback, nil
	case <-ctx.Done():
		_ = w.Close()
		return Callback{}, ctx.Err()
	}
}

func (w *LocalCallbackWaiter) Close() error {
	w.closeOnce.Do(func() {
		var errs []error
		if w.server != nil {
			errs = appendCloseError(errs, w.server.Close())
		}
		errs = appendCloseError(errs, w.listener.Close())
		w.closeErr = errors.Join(errs...)
	})
	return w.closeErr
}

func appendCloseError(errs []error, err error) []error {
	if err == nil || errors.Is(err, net.ErrClosed) || errors.Is(err, http.ErrServerClosed) {
		return errs
	}
	return append(errs, err)
}

func (w *LocalCallbackWaiter) handleCallback(rw http.ResponseWriter, req *http.Request) {
	if req.URL.Path != callbackPath {
		http.NotFound(rw, req)
		return
	}
	if req.Method != http.MethodGet {
		rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
		rw.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = rw.Write([]byte("Unsupported callback request."))
		return
	}

	w.mu.RLock()
	expectedState := w.expectedState
	waiting := w.waiting
	w.mu.RUnlock()
	if !waiting || expectedState == "" || req.URL.Query().Get("state") != expectedState {
		rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
		rw.WriteHeader(http.StatusBadRequest)
		_, _ = rw.Write([]byte("Unable to complete ChatGPT OAuth login."))
		return
	}

	query := req.URL.Query()
	callback := Callback{State: expectedState}
	status := http.StatusOK
	message := "ChatGPT OAuth callback received. You can return to the terminal."
	if oauthError := query.Get("error"); oauthError != "" {
		callback.Error = oauthError
	} else if code := query.Get("code"); code != "" {
		callback.Code = code
	} else {
		callback.Error = "missing_code"
		status = http.StatusBadRequest
		message = "Unable to complete ChatGPT OAuth login. You can return to the terminal."
	}

	rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
	rw.WriteHeader(status)
	_, _ = rw.Write([]byte(message))
	w.complete(callback)
}

func (w *LocalCallbackWaiter) complete(callback Callback) {
	select {
	case w.result <- callback:
	default:
	}
}
