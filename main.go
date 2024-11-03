package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/gorilla/mux"
	// "github.com/microcosm-cc/bluemonday"

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

type TradeEntry struct {
    TimeExchange time.Time `json:"time_exchange" dynamodbav:"time_exchange"`
    TimeCoinAPI  time.Time `json:"time_coinapi" dynamodbav:"time_coinapi"`
    UUID         string    `json:"uuid" dynamodbav:"uuid"`
    Price        float64   `json:"price" dynamodbav:"price"`
    Size         float64   `json:"size" dynamodbav:"size"`
    TakerSide    string    `json:"taker_side" dynamodbav:"taker_side"`
}

type ErrorResponse struct {
	HttpStatus int `json:"httpStatus"`
	ErrorDescription string `json:"errorDescription"`
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

	// AWS DyanmoDB Setup
	cfg, err := config.LoadDefaultConfig(context.TODO(), func(o *config.LoadOptions) error {
        o.Region = "us-east-1"
        return nil
    })
    if err != nil {
        panic(err)
    }
	svc := dynamodb.NewFromConfig(cfg)
	if svc == nil {
		panic("The dynamodb configuration were not setup properly")
	}

	server := Server{logglyClient, svc}

	r := mux.NewRouter()
	r.Use(server.RequestLoggerMiddleware(r))
	r.HandleFunc("/pphyo/status", server.getStatus).Methods(http.MethodGet)
	r.HandleFunc("/pphyo/all", server.getAll).Methods(http.MethodGet)
	r.HandleFunc("/pphyo/search", server.getSearch).Methods(http.MethodGet)
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

func (s *Server) getStatus(rw http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
	defer cancel()

	result,err := s.svc.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String("pphyo_ETH_tradeEntries"),
	})
	if err != nil {
		panic("Error: " + err.Error())
	}

	ep_status := EP_Status{*result.Table.TableName, *result.Table.ItemCount}

	rw.WriteHeader(http.StatusOK)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(ep_status)
}

func (s *Server) getAll(rw http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
	defer cancel()

	result,err := s.svc.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String("pphyo_ETH_tradeEntries"),

	})
	if err != nil {
		panic("Error: " + err.Error())
	}

	items := result.Items
	var tradeEntries []TradeEntry
	err = attributevalue.UnmarshalListOfMaps(items, &tradeEntries)
	if err != nil {
		panic ("Error in unmarshalling the list of maps")
	}

	rw.WriteHeader(http.StatusOK)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(tradeEntries)
}

func (s *Server) getSearch(rw http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
	defer cancel()

	queries := req.URL.Query()

	// p := bluemonday.UGCPolicy()
	
	// Clearing data to fit according
	if !queries.Has("filter") {
		rw.WriteHeader(http.StatusBadRequest)
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(ErrorResponse {
			http.StatusBadRequest,
			fmt.Errorf("error: %s", "query parameter `filter` is not included").Error(),
		})
		return
	}

	var result *dynamodb.ScanOutput
	var err error

	switch queries.Get("filter") {
	case "price":
		result, err = s.getSearchByPrice(ctx, queries)
	case "taker-side":
		result, err = s.getSearchByTakerSide(ctx, queries)
	case "time-exchange":
		// Do something
	default:
		err = fmt.Errorf("error: query `%s` is invalid", queries.Get("filter"))
	}
	
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(ErrorResponse{http.StatusBadRequest, err.Error()})
		return
	}

	items := result.Items
	var tradeEntries []TradeEntry
	err = attributevalue.UnmarshalListOfMaps(items, &tradeEntries)
	if err != nil {
		panic ("Error in unmarshalling the list of maps")
	}

	rw.WriteHeader(http.StatusOK)
	rw.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rw).Encode(tradeEntries)
}

func (s *Server) getSearchByPrice(ctx context.Context, queries url.Values) (*dynamodb.ScanOutput, error) {

	for key := range queries {
		if key != "filter" && key != "min-val" && key != "max-val" {
			return nil, fmt.Errorf("error: %s", "`min-val` and `max-val` can only exists for filter `price`")
		}
    }
	if !queries.Has("min-val"){
		queries.Add("min-val", "0")
	}
	if !queries.Has("max-val"){
		queries.Add("max-val", "1000000")
	}
	
	result, err := s.svc.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String("pphyo_ETH_tradeEntries"),
		FilterExpression: aws.String("price > :minValue AND price < :maxValue"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":minValue":        &types.AttributeValueMemberN{Value: queries.Get("min-val")},
			":maxValue":        &types.AttributeValueMemberN{Value: queries.Get("max-val")},

		},
	})
	return result, err
}

func (s *Server) getSearchByTakerSide(ctx context.Context, queries url.Values) (*dynamodb.ScanOutput, error) {

	for key := range queries {
		if key != "filter" && key != "type" {
			return nil, fmt.Errorf("error: %s", "`type` can only exists for filter `taker-side`")
		}
    }

	if !queries.Has("type") {
		return nil, fmt.Errorf("error: %s", "specify `BUY` or `SELL` for query `type`")
	}

	result, err := s.svc.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String("pphyo_ETH_tradeEntries"),
		FilterExpression: aws.String("taker_side = :takerSide"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":takerSide":        &types.AttributeValueMemberS{Value: queries.Get("type")},
		},
	})
	return result, err
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