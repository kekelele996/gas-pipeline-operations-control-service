package httpapi

// server.go assembles the HTTP server: middleware (request logging, panic
// recovery, CORS) and the root handler mux. It also wires error mapping from
// platform errors to HTTP status codes.

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"gas-pipeline-operations-control-service/internal/platform"
)

// Deps bundles every domain service the handlers need. It is constructed once
// in cmd/server and passed to the router.
type Deps struct {
	Network    NetworkService
	SCADA      ScadaService
	Metering   MeteringService
	Contract   ContractService
	Nomination NominationService
	Permit     PermitService
	Dispatch   DispatchService
	Incident   IncidentService
	Leak       LeakService
	Audit      AuditService
	Notify     NotifyService
}

// Server wraps an *http.Server with graceful shutdown.
type Server struct {
	srv *http.Server
}

// New builds a Server with middleware applied.
func New(addr string, deps Deps) *Server {
	mux := NewRouter(deps)
	handler := recoverMiddleware(requestLogMiddleware(corsMiddleware(mux)))
	return &Server{
		srv: &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      20 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}
}

// ListenAndServe starts the server.
func (s *Server) ListenAndServe() error {
	log.Printf("http: listening on %s", s.srv.Addr)
	return s.srv.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

// Addr returns the configured listen address.
func (s *Server) Addr() string { return s.srv.Addr }

// ---- middleware ----

// statusRecorder captures the response status for logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// requestLogMiddleware logs each request with method, path, status, duration.
func requestLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 0}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %s %d %dB %s",
			r.Method, r.URL.Path, r.Proto, rec.status, rec.bytes, time.Since(start))
	})
}

// recoverMiddleware turns a panic into a 500 and logs the stack-free message.
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v %s %s", rec, r.Method, r.URL.Path)
				platform.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware injects permissive CORS headers and answers OPTIONS.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-Id")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// mapError translates a platform error into an HTTP status and writes it.
func mapError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	var pe *platform.Error
	if errors.As(err, &pe) {
		switch {
		case errors.Is(err, platform.ErrNotFound):
			platform.WriteError(w, http.StatusNotFound, "not_found", pe.Message)
		case errors.Is(err, platform.ErrConflict):
			platform.WriteError(w, http.StatusConflict, "conflict", pe.Message)
		case errors.Is(err, platform.ErrInvalid):
			platform.WriteError(w, http.StatusBadRequest, "invalid", pe.Message)
		case errors.Is(err, platform.ErrState):
			platform.WriteError(w, http.StatusConflict, "state", pe.Message)
		case errors.Is(err, platform.ErrExhausted):
			platform.WriteError(w, http.StatusConflict, "exhausted", pe.Message)
		case errors.Is(err, platform.ErrUnauthorized):
			platform.WriteError(w, http.StatusUnauthorized, "unauthorized", pe.Message)
		default:
			platform.WriteError(w, http.StatusInternalServerError, "internal", pe.Message)
		}
		return
	}
	platform.WriteError(w, http.StatusInternalServerError, "internal", err.Error())
}
