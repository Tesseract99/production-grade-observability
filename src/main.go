package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

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
	db, err := sql.Open("mysql", DSN)
	if err != nil {
		return nil, err
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

func insertMovie(db *sql.DB, name string) error {
	query := "INSERT INTO movies (movie) VALUES (?)"
	result, err := db.Exec(query, name)
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

func main() {

	// Load Variables
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, relying on system environment variables")
	}

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
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("welcome"))
	})

	http.HandleFunc("/movie", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(request.Body)
		if err != nil {
			http.Error(writer, "Error reading request body",
				http.StatusInternalServerError)
			return
		}
		defer request.Body.Close()

		var movie map[string]any
		if err := json.Unmarshal(body, &movie); err != nil {
			http.Error(writer, "Invalid JSON", http.StatusBadRequest)
			return
		}

		name, ok := movie["name"].(string)
		if !ok {
			http.Error(writer, "Missing or invalid 'name' field", http.StatusBadRequest)
			return
		}

		if err := insertMovie(db, name); err != nil {
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
