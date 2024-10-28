package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	loggly "github.com/jamespearly/loggly"
)

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

type Status struct {
	Time       time.Time `json:"time"`
	HTTPStatus int       `json:"httpStatus"`
}

var logglyClient *loggly.ClientType = nil

func main() {

	// Set up Loggly
	tag := "CSC482Server"
	logglyClient = loggly.New(tag)
	if logglyClient == nil {
		log.Printf("logglyClient is nil")
	}

	r := mux.NewRouter()
	r.Use(RequestLoggerMiddleware(r))
	r.HandleFunc("/pphyo/status", getStatus).Methods(http.MethodGet)
	r.PathPrefix("/").HandlerFunc(getPageNotFound).Methods(http.MethodGet)
	r.PathPrefix("/").HandlerFunc(notAllowedotherMethods)
	log.Fatal(http.ListenAndServe(":8080", r))
	
}

// NewStatusResponseWriter returns pointer to a new statusResponseWriter object
func NewStatusResponseWriter(w http.ResponseWriter) *statusResponseWriter {
	return &statusResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

// WriteHeader assigns status code and header to ResponseWriter of statusResponseWriter object
func (sw *statusResponseWriter) WriteHeader(statusCode int) {
	sw.statusCode = statusCode
	sw.ResponseWriter.WriteHeader(statusCode)
}

func getStatus(rw http.ResponseWriter, req *http.Request) {

	sw := NewStatusResponseWriter(rw)
	status := Status{time.Now(), sw.statusCode}

	rw.WriteHeader(http.StatusOK)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(status)
	return
}

func getPageNotFound(rw http.ResponseWriter, req *http.Request) {
	sw := NewStatusResponseWriter(rw)
	sw.WriteHeader(http.StatusNotFound)
	sw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]int{
		"httpStatus": sw.statusCode,
	})
	return
}

func notAllowedotherMethods(rw http.ResponseWriter, req *http.Request) {
	sw := NewStatusResponseWriter(rw)
	sw.WriteHeader(http.StatusMethodNotAllowed)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]int{
		"httpStatus": sw.statusCode,
	})
	return
}

func RequestLoggerMiddleware(r *mux.Router) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()
			sw := NewStatusResponseWriter(w)
			defer func() {
				if logglyClient != nil {
					logglyClient.EchoSend("info", fmt.Sprintf("[%s] [%v] [%d] %s %s %s",
						req.Method,
						time.Since(start),
						sw.statusCode,
						req.RemoteAddr,
						req.URL.Path,
						req.URL.RawQuery,
					))
				}
			}()
			next.ServeHTTP(sw, req)
		})
	}
}