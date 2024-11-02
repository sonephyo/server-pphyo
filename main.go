package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/gorilla/mux"
	loggly "github.com/jamespearly/loggly"
	"github.com/joho/godotenv"
)

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

type Status struct {
	Time       time.Time `json:"time"`
	HTTPStatus int       `json:"httpStatus"`
}

type EP_Status struct {
	TableName string `json:"tableName"`
	RecordCount int64 `json:"recordCount"`
}

type Server struct {
	logglyClient *loggly.ClientType
	svc *dynamodb.Client
}


func main() {
	// Load env file
	errEnvFile := godotenv.Load(".env")
	if errEnvFile != nil {
		panic(errEnvFile)
	}

	// Set up Loggly
	tag := "CSC482Server"
	logglyClient := loggly.New(tag)
	if logglyClient == nil {
		log.Printf("logglyClient is nil")
	}

	r := mux.NewRouter()
	r.Use(server.RequestLoggerMiddleware(r))
	r.HandleFunc("/pphyo/status", getStatus).Methods(http.MethodGet)
	r.HandleFunc("/pphyo/all", server.getAll).Methods(http.MethodGet)
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
}

func (s *Server) getAll(rw http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
	defer cancel()

	result,err := s.svc.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String("pphyo_ETH_tradeEntries"),
	})
	if err != nil {
		panic("Error: " + err.Error())
	}
	fmt.Println(*result.Table.ItemCount)

	ep_status := EP_Status{*result.Table.TableName, *result.Table.ItemCount}

	rw.WriteHeader(http.StatusOK)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(ep_status)
}

func getPageNotFound(rw http.ResponseWriter, req *http.Request) {
	sw := NewStatusResponseWriter(rw)
	sw.WriteHeader(http.StatusNotFound)
	sw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]int{
		"httpStatus": sw.statusCode,
	})
}

func notAllowedotherMethods(rw http.ResponseWriter, req *http.Request) {
	sw := NewStatusResponseWriter(rw)
	sw.WriteHeader(http.StatusMethodNotAllowed)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(map[string]int{
		"httpStatus": sw.statusCode,
	})
}

func (s *Server) RequestLoggerMiddleware(r *mux.Router) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()
			sw := NewStatusResponseWriter(w)
			defer func() {
				if s.logglyClient != nil {
					s.logglyClient.EchoSend("info", fmt.Sprintf("[%s] [%v] [%d] %s %s %s",
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