package apiserver

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"

	"time"

	"github.com/abhigod/k8s-lite/internal/api"
	"github.com/abhigod/k8s-lite/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	Store  storage.Store
	Router *chi.Mux
}

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_request_duration_seconds",
			Help: "HTTP request duration in seconds",
		},
		[]string{"method", "path"},
	)
)

func NewServer(store storage.Store) *Server {
	s := &Server{
		Store:  store,
		Router: chi.NewRouter(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.Router.Use(middleware.Logger)
	s.Router.Use(middleware.Recoverer)
	s.Router.Use(render.SetContentType(render.ContentTypeJSON))
	s.Router.Use(render.SetContentType(render.ContentTypeJSON))
	s.Router.Use(s.prometheusMiddleware)
	s.Router.Use(s.authMiddleware)

	s.Router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("K8s-Lite API Server"))
	})

	s.Router.Handle("/metrics", promhttp.Handler())

	// /api/v1/pods
	s.registerResourceRoutes("/api/v1", "pods", &api.Pod{}, &api.PodList{})

	// /api/v1/nodes
	s.registerResourceRoutes("/api/v1", "nodes", &api.Node{}, &api.NodeList{})

	// /apis/apps/v1/replicasets (simplifying to /api/v1 for MVP simplicity if desired, but sticking to structure)
	s.registerResourceRoutes("/apis/apps/v1", "replicasets", &api.ReplicaSet{}, &api.ReplicaSetList{})

	// /apis/apps/v1/deployments
	s.registerResourceRoutes("/apis/apps/v1", "deployments", &api.Deployment{}, &api.DeploymentList{})

	// /api/v1/services
	s.registerResourceRoutes("/api/v1", "services", &api.Service{}, &api.ServiceList{})

	// /api/v1/endpoints
	s.registerResourceRoutes("/api/v1", "endpoints", &api.Endpoints{}, &api.EndpointsList{})

	// /api/v1/leases
	s.registerResourceRoutes("/api/v1", "leases", &api.Lease{}, &api.LeaseList{})
}

func (s *Server) registerResourceRoutes(prefix, resource string, objKind interface{}, listKind interface{}) {
	// e.g. /api/v1/pods
	s.Router.Route(path.Join(prefix, resource), func(r chi.Router) {
		r.Get("/", s.handleList(resource, listKind))
		r.Post("/", s.handleCreate(resource, objKind))

		r.Route("/{name}", func(r chi.Router) {
			r.Get("/", s.handleGet(resource, objKind))
			r.Delete("/", s.handleDelete(resource))
			r.Put("/", s.handleUpdate(resource, objKind))
		})
	})
}

func (s *Server) handleUpdate(resource string, _ interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		key := fmt.Sprintf("/registry/%s/%s", resource, name)

		// Decode new obj
		var obj interface{}
		switch resource {
		case "pods":
			obj = &api.Pod{}
		case "nodes":
			obj = &api.Node{}
		case "replicasets":
			obj = &api.ReplicaSet{}
		case "deployments":
			obj = &api.Deployment{}
		case "services":
			obj = &api.Service{}
		case "endpoints":
			obj = &api.Endpoints{}
		case "leases":
			obj = &api.Lease{}
		}

		if err := json.NewDecoder(r.Body).Decode(obj); err != nil {
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}

		if err := s.Store.Update(r.Context(), key, obj); err != nil {
			if err == storage.ErrNotFound {
				render.Render(w, r, ErrNotFound)
			} else {
				render.Render(w, r, ErrInternal(err))
			}
			return
		}

		render.JSON(w, r, obj)
	}
}

func (s *Server) handleList(resource string, _ interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("watch") == "true" {
			s.handleWatch(resource, w, r)
			return
		}

		ctx := r.Context()
		keyPrefix := fmt.Sprintf("/registry/%s", resource)

		var list interface{}
		switch resource {
		case "pods":
			var pods []api.Pod
			if err := s.Store.List(ctx, keyPrefix, &pods); err != nil {
				render.Render(w, r, ErrInternal(err))
				return
			}
			list = &api.PodList{Items: pods}
		case "nodes":
			var nodes []api.Node
			if err := s.Store.List(ctx, keyPrefix, &nodes); err != nil {
				render.Render(w, r, ErrInternal(err))
				return
			}
			list = &api.NodeList{Items: nodes}
		case "replicasets":
			var rss []api.ReplicaSet
			if err := s.Store.List(ctx, keyPrefix, &rss); err != nil {
				render.Render(w, r, ErrInternal(err))
				return
			}
			list = &api.ReplicaSetList{Items: rss}
		case "deployments":
			var deps []api.Deployment
			if err := s.Store.List(ctx, keyPrefix, &deps); err != nil {
				render.Render(w, r, ErrInternal(err))
				return
			}
			list = &api.DeploymentList{Items: deps}
		case "services":
			var svcs []api.Service
			if err := s.Store.List(ctx, keyPrefix, &svcs); err != nil {
				render.Render(w, r, ErrInternal(err))
				return
			}
			list = &api.ServiceList{Items: svcs}
		case "endpoints":
			var eps []api.Endpoints
			if err := s.Store.List(ctx, keyPrefix, &eps); err != nil {
				render.Render(w, r, ErrInternal(err))
				return
			}
			list = &api.EndpointsList{Items: eps}
		case "leases":
			var leases []api.Lease
			if err := s.Store.List(ctx, keyPrefix, &leases); err != nil {
				render.Render(w, r, ErrInternal(err))
				return
			}
			list = &api.LeaseList{Items: leases}
		default:
			render.Render(w, r, ErrNotFound)
			return
		}

		render.JSON(w, r, list)
	}
}

func (s *Server) handleWatch(resource string, w http.ResponseWriter, r *http.Request) {
	keyPrefix := fmt.Sprintf("/registry/%s", resource)
	watcher, err := s.Store.Watch(r.Context(), keyPrefix)
	if err != nil {
		render.Render(w, r, ErrInternal(err))
		return
	}
	defer watcher.Stop()

	// Set headers for streaming
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		// Fallback or error? For now just log/error
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}
	flusher.Flush()

	encoder := json.NewEncoder(w)

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return
			}
			if err := encoder.Encode(event); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) handleCreate(resource string, _ interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Decode
		var obj interface{}
		switch resource {
		case "pods":
			obj = &api.Pod{}
		case "nodes":
			obj = &api.Node{}
		case "replicasets":
			obj = &api.ReplicaSet{}
		case "deployments":
			obj = &api.Deployment{}
		case "services":
			obj = &api.Service{}
		case "endpoints":
			obj = &api.Endpoints{}
		case "leases":
			obj = &api.Lease{}
		}

		if err := json.NewDecoder(r.Body).Decode(obj); err != nil {
			render.Render(w, r, ErrInvalidRequest(err))
			return
		}

		// Extract Name (simple reflection or type assertion)
		meta, ok := getObjectMeta(obj)
		if !ok || meta.Name == "" {
			render.Render(w, r, ErrInvalidRequest(fmt.Errorf("metadata.name is required")))
			return
		}

		key := fmt.Sprintf("/registry/%s/%s", resource, meta.Name)
		if err := s.Store.Create(r.Context(), key, obj); err != nil {
			if err == storage.ErrConflict {
				render.Render(w, r, ErrConflict(err))
			} else {
				render.Render(w, r, ErrInternal(err))
			}
			return
		}

		render.Status(r, http.StatusCreated)
		render.JSON(w, r, obj)
	}
}

func (s *Server) handleGet(resource string, _ interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		key := fmt.Sprintf("/registry/%s/%s", resource, name)

		var obj interface{}
		switch resource {
		case "pods":
			obj = &api.Pod{}
		case "nodes":
			obj = &api.Node{}
		case "replicasets":
			obj = &api.ReplicaSet{}
		case "deployments":
			obj = &api.Deployment{}
		case "services":
			obj = &api.Service{}
		case "endpoints":
			obj = &api.Endpoints{}
		case "leases":
			obj = &api.Lease{}
		}

		if err := s.Store.Get(r.Context(), key, obj); err != nil {
			if err == storage.ErrNotFound {
				render.Render(w, r, ErrNotFound)
			} else {
				render.Render(w, r, ErrInternal(err))
			}
			return
		}

		render.JSON(w, r, obj)
	}
}

func (s *Server) handleDelete(resource string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		key := fmt.Sprintf("/registry/%s/%s", resource, name)

		if err := s.Store.Delete(r.Context(), key); err != nil {
			if err == storage.ErrNotFound {
				render.Render(w, r, ErrNotFound)
			} else {
				render.Render(w, r, ErrInternal(err))
			}
			return
		}

		render.Status(r, http.StatusOK) // or NoContent
		render.JSON(w, r, map[string]string{"status": "deleted"})
	}
}

// Helpers

func getObjectMeta(obj interface{}) (*api.ObjectMeta, bool) {
	// Quick hack without full reflection library
	switch v := obj.(type) {
	case *api.Pod:
		return &v.ObjectMeta, true
	case *api.Node:
		return &v.ObjectMeta, true
	case *api.ReplicaSet:
		return &v.ObjectMeta, true
	case *api.Deployment:
		return &v.ObjectMeta, true
	case *api.Service:
		return &v.ObjectMeta, true
	case *api.Endpoints:
		return &v.ObjectMeta, true
	case *api.Lease:
		return &v.ObjectMeta, true
	}
	return nil, false
}

// Errors

type ErrResponse struct {
	Err            error `json:"-"` // low-level runtime error
	HTTPStatusCode int   `json:"-"` // http response status code

	StatusText string `json:"status"`          // user-level status message
	AppCode    int64  `json:"code,omitempty"`  // application-specific error code
	ErrorText  string `json:"error,omitempty"` // application-level error message, for debugging
}

func (e *ErrResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

func ErrInvalidRequest(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: 400,
		StatusText:     "Invalid request",
		ErrorText:      err.Error(),
	}
}

func ErrConflict(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: 409,
		StatusText:     "Resource already exists",
		ErrorText:      err.Error(),
	}
}

var ErrNotFound = &ErrResponse{HTTPStatusCode: 404, StatusText: "Resource not found"}

func ErrInternal(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: 500,
		StatusText:     "Internal server error",
		ErrorText:      err.Error(),
	}
}

func (s *Server) prometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		duration := time.Since(start).Seconds()
		status := fmt.Sprintf("%d", ww.Status())
		path := r.URL.Path // Simplified path cardinality for now

		httpRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
	})
}

// ServeTLS starts the server with mTLS enabled
func (s *Server) ServeTLS(addr, certFile, keyFile, caFile string) error {
	// Load CA
	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		return fmt.Errorf("read ca cert: %v", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	// TLS Config
	tlsConfig := &tls.Config{
		ClientCAs:  caCertPool,
		ClientAuth: tls.RequireAndVerifyClientCert,
	}

	server := &http.Server{
		Addr:      addr,
		Handler:   s.Router,
		TLSConfig: tlsConfig,
	}

	return server.ListenAndServeTLS(certFile, keyFile)
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Check TLS Client Cert
		if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
			// cn := r.TLS.PeerCertificates[0].Subject.CommonName
			next.ServeHTTP(w, r)
			return
		}
		// If insecure listner or no cert, allow (for now, or reject if enforcement required)
		// Since we use RequireAndVerifyClientCert in TLSConfig, non-cert mutual TLS won't happen.
		// So this handles HTTP fallback if any.
		next.ServeHTTP(w, r)
	})
}





