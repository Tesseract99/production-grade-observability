package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/XSAM/otelsql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// OTel
var tracer = otel.Tracer("movie-handlers")

func mysqlConnection() (*sql.DB, error) {

	DB_USERNAME := os.Getenv("DB_USERNAME")
	DB_PASSWORD := os.Getenv("DB_PASSWORD")
	DB_HOST := os.Getenv("DB_HOST")
	DB_PORT := os.Getenv("DB_PORT")
	DB_NAME := os.Getenv("DB_NAME")

	if DB_USERNAME == "" || DB_PASSWORD == "" || DB_HOST == "" || DB_PORT == "" || DB_NAME == "" {
		return nil, fmt.Errorf("missing required DB environment variables")
	}

	

	DSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", DB_USERNAME, DB_PASSWORD, DB_HOST, DB_PORT, DB_NAME)
	// db, err := sql.Open("mysql", DSN)
	// if err != nil {
	// 	return nil, err
	// }

	db, err := otelsql.Open("mysql", DSN,
    otelsql.WithAttributes(semconv.DBSystemMySQL, semconv.DBNamespace(DB_NAME)), // Tag it as MySQL
	)
	if err != nil {
		return nil, err
	}

	_, err = otelsql.RegisterDBStatsMetrics(db, otelsql.WithAttributes(
    semconv.DBSystemMySQL,))
	if err != nil {
		log.Printf("Could not register DB stats: %v", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	fmt.Println("Successfully connected to MySQL!")

	return db, nil
}

func insertMovieFromDB(ctx context.Context, db *sql.DB, name string) error {
	query := "INSERT INTO movies (movie) VALUES (?)"
	result, err := db.ExecContext(ctx, query, name)
	if err != nil {
		return fmt.Errorf("failed to insert movie: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	fmt.Printf("Inserted movie with ID: %d\n", id)
	return nil
}

func getMoviesFromDB(ctx context.Context,db *sql.DB) ([]map[string]any, error) {
	query := "SELECT * FROM movies"
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get movies: %w", err)
	}
	defer rows.Close()

	var movies []map[string]any
	for rows.Next() {
		var id int
		var movie string
		var created_at string
		err := rows.Scan(&id, &movie, &created_at)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		movies = append(movies, map[string]any{
			"id":         id,
			"movie":      movie,
			"created_at": created_at,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return movies, nil
}

func main() {

	// Load ENV Variables
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, relying on system environment variables")
	}

	fmt.Printf("Connecting to OTEL_EXPORTER_OTLP_ENDPOINT: %s \n", os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))

	// otel
	shutdown := initTracer()
	defer shutdown(context.Background())

	// Database Connection
	db, err := mysqlConnection()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// HTTP Server
	myServer(db)

	PORT := "8003"
	s := &http.Server{
		Addr:           fmt.Sprintf(":%s", PORT),
		Handler:        nil,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Printf("Listening on PORT: %s", PORT)
	log.Fatal(s.ListenAndServe())

}

func myServer(db *sql.DB) {
	// Default
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("welcome"))
	})

	// Post /movie
	postMovie(db)

	// Get /movies
	getMovies(db)

}

func postMovie(db *sql.DB) {
	http.HandleFunc("/movie", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Otel
		ctx, span := tracer.Start(request.Context(), "insert-movie-handler")
		defer span.End()
		span.SetAttributes(
			attribute.String("http.method", request.Method),
			attribute.String("http.path", "/movie"),
		)

		body, err := io.ReadAll(request.Body)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			http.Error(writer, "Error reading request body",
				http.StatusInternalServerError)
			return
		}
		defer request.Body.Close()

		var movie map[string]any
		if err := json.Unmarshal(body, &movie); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			http.Error(writer, "Invalid JSON", http.StatusBadRequest)
			return
		}

		name, ok := movie["name"].(string)
		if !ok {
			http.Error(writer, "Missing or invalid 'name' field", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(ctx, 3*time.Second) 
		defer cancel()
		if err := insertMovieFromDB(ctx, db, name); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}

		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		json.NewEncoder(writer).Encode(map[string]string{
			"message": "Movie inserted successfully",
		})
	})
}

func getMovies(db *sql.DB) {
	http.HandleFunc("/movies", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// otel
		ctx, span := tracer.Start(request.Context(), "get-movies-handler")
		defer span.End()
		span.SetAttributes(
			attribute.String("http.method", request.Method),
			attribute.String("http.path", "/movies"),
		)

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second) 
		defer cancel()

		movies, err := getMoviesFromDB(ctx, db)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}

		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		json.NewEncoder(writer).Encode(movies)

	})
}
