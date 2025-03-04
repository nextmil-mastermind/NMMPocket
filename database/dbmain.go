package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"

	_ "github.com/lib/pq" // Register the Postgres driver
)

// Global variable for the DB connection.
var Pg *sql.DB

// InitDB initializes the database connection using the pgurl environment variable.
func InitDB() {
	pgURL := os.Getenv("pgurl")
	if pgURL == "" {
		log.Fatal("pgurl not set")
	}
	u, err := url.Parse(pgURL)
	if err != nil {
		log.Fatalf("Invalid pgurl: %v", err)
	}

	// Extract connection parameters.
	user := u.User.Username()
	password, _ := u.User.Password()
	host := u.Hostname()
	port := u.Port()
	dbName := u.Path
	if len(dbName) > 0 && dbName[0] == '/' {
		dbName = dbName[1:]
	}

	// Build a DSN for the pq driver.
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=require", user, password, host, port, dbName)
	Pg, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	if err = Pg.Ping(); err != nil {
		log.Fatalf("Failed to ping DB: %v", err)
	}
	log.Println("Connected to Postgres successfully")
}

// rowsToMap converts sql.Rows into a slice of maps for easy JSON marshaling.
func rowsToMap(rows *sql.Rows) ([]map[string]interface{}, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	results := []map[string]interface{}{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}
		rowMap := make(map[string]interface{})
		for i, colName := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				strVal := string(b)
				// Check if the value is a JSON array (starts with '[')
				if len(strVal) > 0 && strVal[0] == '[' {
					var arr []interface{}
					if err := json.Unmarshal(b, &arr); err == nil {
						rowMap[colName] = arr
						continue
					}
				}
				// Check if the value is a boolean
				if boolVal, err := strconv.ParseBool(strVal); err == nil {
					rowMap[colName] = boolVal
					continue
				}
				// Try to parse as integer
				if intVal, err := strconv.Atoi(strVal); err == nil {
					rowMap[colName] = intVal
					continue
				}
				// Try to parse as float
				if floatVal, err := strconv.ParseFloat(strVal, 64); err == nil {
					rowMap[colName] = floatVal
					continue
				}
				rowMap[colName] = strVal
			} else {
				rowMap[colName] = val
			}
		}
		results = append(results, rowMap)
	}
	return results, nil
}
