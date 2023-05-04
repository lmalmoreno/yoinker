package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/carlmjohnson/versioninfo"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "modernc.org/sqlite" // Native Go driver for sqlite
)

// db is a spooky global variable to access the database
var db *sql.DB

type Yoink struct {
	ID        int64                  `json:"id"`
	Topic     string                 `json:"topic"`
	Timestamp string                 `json:"timestamp"` // time.Time errors out somewhere so leave out for now Timestamp time.Time `json:"timestamp"`
	Content   map[string]interface{} `json:"content"`
	//TODO: figure out how content generated from query params should be handled
}

//TODO: add error constants
// const ErrX

// HTTPError is a custom HTTP error type
type HTTPError struct {
	Cause  string `json:"error"`  //TODO: rename Cause to Message for clarity
	Detail string `json:"detail"` // the error as a string
	Status int    `json:"status"`
}

// Error returns the custom HTTPError as a string
func (e *HTTPError) Error() string {
	if e.Cause == "" {
		return e.Detail
	}
	return e.Detail + " : " + e.Cause
}

// ResponseBody returns the custom HTTPError as JSON
func (e *HTTPError) ResponseBody() ([]byte, error) {
	body, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("Error while parsing response body: %w", err)
	}
	return body, nil
}

// NewHTTPError produces a native Go error
func NewHTTPError(err string, status int, detail string) error {
	return &HTTPError{
		Cause:  err,
		Detail: detail,
		Status: status,
	}
}

// HTML for the quickstart page as a template string
var quickstartHTML = `<!DOCTYPE html>
<html>
	<head>
		<meta charset="utf-8">
		<link rel=icon href=data:,>
		<title>DataYoinker Quickstart</title>
	</head>
	<body>
		<h1>Publishing data</h1>
		<h2>Publishing your data is as easy as making a GET request!</h2>
		<p>
			The URL is as follows:
			<pre><code>
				https://datayoinker.inherently.xyz/publish/yoink/for/mything?variable=value
			</code></pre>
		</p>
		<p>
			An example with curl:
			<pre><code>
				curl 'https://datayoinker.inherently.xyz/publish/yoink/for/demoESP32?tempreading=25.7&amp;name=home'
			</code></pre>
			And then you get back something like this:
			<pre><code>
				{
				  "id": 1,
				  "topic": "demoESP32",
				  "timestamp": "2022-10-26T11:21:11Z",
				  "content": {
				    "name": "home",
				    "tempreading": 25.7
				  }
				}
			</code></pre>
		</p>
		<h1>Retrieving data</h1>
		<h2>Retrieving your data is also as easy as making a GET request!</h2>
		<p>The URL is as follows:
			<pre><code>
				https://datayoinker.inherently.xyz/get/latest/yoink/from/demoESP32
			</code></pre>
		</p>
		<p>
			An example with curl:
			<pre><code>
				curl 'https://datayoinker.inherently.xyz/get/latest/yoink/from/demoESP32'
			</code></pre>
			And then you get back the same thing we saw before:
			<pre><code>
				{
				  "id": 1,
				  "topic": "demoESP32",
				  "timestamp": "2022-10-26T11:21:11Z",
				  "content": {
				    "name": "home",
				    "tempreading": 25.7
				  }
				}
			</code></pre>
		</p>
	</bodY>
</html>`

// quickstart returns a page explaining how to use the app
func quickstart(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte(quickstartHTML))
}

// getVersionInfo returns the version information for the running instance of the app
func getVersionInfo(w http.ResponseWriter, _ *http.Request) {
	versionInfo := "Version information about datayoinker:" +
		"\n\tVersion: " + versioninfo.Version +
		"\n\tRevision: " + versioninfo.Revision +
		"\n\tLastCommit: " + versioninfo.LastCommit.String() +
		"\n"
	w.Write([]byte(versionInfo))
}

// PublishForTopic adds a yoink to a topic
func PublishForTopic(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Store query parameters
		queryParams := r.URL.Query()

		// Build the JSON using simple string concatenation
		jsonContent := `{`
		for k, z := range queryParams {
			// if a parameter is specified twice, it can have 2 values
			// I don't feel like dealing with the edge case so there you go
			if len(z) != 1 {
				e := NewHTTPError("Parameter with more than 1 value found", http.StatusBadRequest, "Bad Request")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(e)
				return
			}
			v := z[0]

			jsonContent += `"` + k + `":`
			// Figure out the type of the value by trying to convert it
			f, errf := strconv.ParseFloat(v, 64)
			if errf == nil {
				jsonContent += strconv.FormatFloat(f, 'f', -1, 64) + `,`
				continue
			}
			i, erri := strconv.Atoi(v)
			if erri == nil {
				jsonContent += strconv.Itoa(i) + `,`
				continue
			}
			if errf != nil && erri != nil {
				jsonContent += `"` + v + `",`
			}
		}
		jsonContent = strings.TrimSuffix(jsonContent, `,`)
		jsonContent += `}`

		// Validate JSON in case something was malformed
		isValidJSON := json.Valid([]byte(jsonContent)) // Alternative for faster validation: https://github.com/valyala/fastjson
		if !isValidJSON {
			e := NewHTTPError("Invalid JSON", http.StatusBadRequest, "Bad Request")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(e)
			return
		}

		// Get topic name from the URL
		topic := chi.URLParam(r, "topic")
		if topic == "" {
			e := NewHTTPError("Error parsing topic", http.StatusBadRequest, "Bad Request")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(e)
			return
		}

		// Store the content and the topic (might change table structures in the future but we'll see)
		rows, err := db.Query(
			// Return all fields to be extra sure that what we send to the client is what was saved
			`INSERT INTO Yoinks (topic, content) VALUES (?, ?) RETURNING id, topic, timestamp, content;`,
			topic,
			jsonContent,
		)
		if err != nil {
			e := NewHTTPError(err.Error(), http.StatusInternalServerError, "Error inserting data to database")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(e)
			return
		}

		// Map the results from the database query to a struct
		y := Yoink{} // Struct to be filled in by database results
		for rows.Next() {
			if rows.Err() != nil {
				e := NewHTTPError(err.Error(), http.StatusBadRequest, "Error encountered while iterating over returned row(s)")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(e)
				return
			}

			tempJSON := "" // JSON is stored as text in sqlite and can't be directly mapped to a map[string]interface{}
			err = rows.Scan(
				&y.ID,
				&y.Topic,
				&y.Timestamp,
				&tempJSON,
			)
			if err != nil {
				e := NewHTTPError(err.Error(), http.StatusBadRequest, "Error mapping query results to struct")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(e)
				return
			}
			err := json.NewDecoder(strings.NewReader(tempJSON)).Decode(&y.Content)
			if err != nil {
				e := NewHTTPError(err.Error(), http.StatusBadRequest, "Error decoding content from JSON")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(e)
				return
			}
		}

		// If everything has gone well, return the JSON-encoded Yoink struct
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(y)
	}
	//TODO: add code to handle POST
}

// GetLatestYoinkFromTopic returns the latest yoink for the provided topic
func GetLatestYoinkFromTopic(w http.ResponseWriter, r *http.Request) {
	// Get topic name from the URL
	topic := chi.URLParam(r, "topic")
	if topic == "" {
		e := NewHTTPError("Error parsing topic", http.StatusBadRequest, "Bad Request")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(e)
		return
	}

	// Retrieve all fields of the last inserted row for the specified topic
	rows, err := db.Query(
		// Return all fields to be extra sure that what we send to the client is what was saved
		`SELECT id, topic, timestamp, content FROM yoinks WHERE topic = ? ORDER BY timestamp DESC LIMIT 1;`,
		topic,
	)
	if err != nil {
		e := NewHTTPError(err.Error(), http.StatusBadRequest, "Error getting data from database")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(e)
		return
	}

	// Map the results from the database query to a struct
	y := Yoink{} // Struct to be filled in by database results
	for rows.Next() {
		if rows.Err() != nil {
			e := NewHTTPError(err.Error(), http.StatusBadRequest, "Error accessing returned row")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(e)
			return
		}

		tempJSON := "" // JSON is stored as text in sqlite and can't be directly mapped to a map[string]interface{}
		err = rows.Scan(
			&y.ID,
			&y.Topic,
			&y.Timestamp,
			&tempJSON,
		)
		if err != nil {
			e := NewHTTPError(err.Error(), http.StatusBadRequest, "Error mapping query results to struct")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(e)
			return
		}
		err := json.NewDecoder(strings.NewReader(tempJSON)).Decode(&y.Content)
		if err != nil {
			e := NewHTTPError(err.Error(), http.StatusBadRequest, "Error decoding content from JSON")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(e)
			return
		}
	}

	// If everything has gone well, return the JSON-encoded Yoink struct
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(y)
}

// getLastNumberOfYoinksFromTopic returns the latest/last specified number of yoinks for the provided topic
func getLastNumberOfYoinksFromTopic(w http.ResponseWriter, r *http.Request) {
	// Validate topic name
	topic := chi.URLParam(r, "topic")
	if topic == "" {
		e := NewHTTPError("topic is empty", http.StatusBadRequest, "Error validating topic name")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(e)
		return
	}

	// Parse and validate number of yoinks
	number := chi.URLParam(r, "number")
	num, err := strconv.Atoi(number)
	if err != nil {
		e := NewHTTPError(err.Error(), http.StatusBadRequest, "Error parsing number of yoinks")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(e)
		return
	}
	if num < 1 {
		e := NewHTTPError("number is less than 1", http.StatusBadRequest, "Error validating number of yoinks")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(e)
		return
	}

	// Retrieve all fields of the last inserted row for the specified topic
	rows, err := db.Query(
		// Return all fields to be extra sure that what we send to the client is what was saved
		`SELECT id, topic, timestamp, content FROM yoinks WHERE topic = ? ORDER BY timestamp DESC LIMIT ?;`,
		topic,
		number,
	)
	if err != nil {
		e := NewHTTPError(err.Error(), http.StatusBadRequest, "Error getting data from database")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(e)
		return
	}

	yoinks := []*Yoink{}
	// Map the results from the database query to a struct
	for rows.Next() {
		if rows.Err() != nil {
			e := NewHTTPError(err.Error(), http.StatusBadRequest, "Error accessing returned row")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(e)
			return
		}

		y := Yoink{}   // Struct to be filled in by database results
		tempJSON := "" // JSON is stored as text in sqlite and can't be directly mapped to a map[string]interface{}
		err = rows.Scan(
			&y.ID,
			&y.Topic,
			&y.Timestamp,
			&tempJSON,
		)
		if err != nil {
			e := NewHTTPError(err.Error(), http.StatusBadRequest, "Error mapping query results to struct")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(e)
			return
		}
		err := json.NewDecoder(strings.NewReader(tempJSON)).Decode(&y.Content)
		if err != nil {
			e := NewHTTPError(err.Error(), http.StatusBadRequest, "Error decoding content from JSON")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(e)
			return
		}
		yoinks = append(yoinks, &y)
	}

	// If everything has gone well, return the JSON-encoded list of Yoink structs
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(yoinks)
}

// GetAllYoinksFromTopic returns all yoinks for the provided topic
func GetAllYoinksFromTopic(w http.ResponseWriter, r *http.Request) {
	// Validate and parse topic name and number
	topic := chi.URLParam(r, "topic")
	if topic == "" {
		e := NewHTTPError("topic is empty", http.StatusBadRequest, "Error validating topic name")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(e)
		return
	}

	// Retrieve all fields of the last inserted row for the specified topic
	rows, err := db.Query(
		// Return all fields to be extra sure that what we send to the client is what was saved
		`SELECT id, topic, timestamp, content FROM yoinks WHERE topic = ? ORDER BY timestamp DESC;`,
		topic,
	)
	if err != nil {
		e := NewHTTPError(err.Error(), http.StatusBadRequest, "Error getting data from database")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(e)
		return
	}

	yoinks := []*Yoink{}
	// Map the results from the database query to a struct
	for rows.Next() {
		if rows.Err() != nil {
			e := NewHTTPError(err.Error(), http.StatusBadRequest, "Error accessing returned row")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(e)
			return
		}

		y := Yoink{}   // Struct to be filled in by database results
		tempJSON := "" // JSON is stored as text in sqlite and can't be directly mapped to a map[string]interface{}
		err = rows.Scan(
			&y.ID,
			&y.Topic,
			&y.Timestamp,
			&tempJSON,
		)
		if err != nil {
			e := NewHTTPError(err.Error(), http.StatusBadRequest, "Error mapping query results to struct")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(e)
			return
		}
		err := json.NewDecoder(strings.NewReader(tempJSON)).Decode(&y.Content)
		if err != nil {
			e := NewHTTPError(err.Error(), http.StatusBadRequest, "Error decoding content from JSON")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(e)
			return
		}
		yoinks = append(yoinks, &y)
	}

	// If everything has gone well, return the JSON-encoded list of Yoink structs
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(yoinks)
}

// SetupDB initializes the database and returns a client to it
func SetupDB() (*sql.DB, error) {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "yoink.db"
	}

	// Check if file exists and if not, create it
	fileInfo, err := os.Stat(dbPath)
	if err != nil {
		_, err := os.Create(dbPath)
		if err != nil {
			return nil, err
		}
	}
	// Avoid nil pointer dereference and check if file is a regular file
	if fileInfo != nil && !fileInfo.Mode().IsRegular() {
		return nil, errors.New(dbPath + " is not a regular file")
	}
	// Since the file exists, use it for sqlite
	sqlite, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Ping the database to make sure we can access it and use it
	err = sqlite.Ping()
	if err != nil {
		return nil, err
	}

	// Create schema
	_, err = sqlite.Exec(`CREATE TABLE if not exists yoinks (
		id INTEGER NOT NULL,
		topic TEXT NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		content TEXT NOT NULL,
		PRIMARY KEY (id AUTOINCREMENT)
	);`)
	if err != nil {
		return nil, err
	}
	return sqlite, nil
}

// setupRouter configures the handler that the server will use
func setupRouter() http.Handler {
	// Create new chi router
	r := chi.NewRouter()

	// Apply middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Set up routes

	// Quickstart webpage endpoint
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<!DOCTYPE html>
<html>
	<head>
		<meta charset="utf-8">
		<link rel=icon href=data:,>
		<title>DataYoinker</title>
	</head>
	<body>
		<h1>Welcome</h1>
		<p>Welcome to DataYoinker, take a look at
			<a href="/quickstart">the quickstart guide</a>
		for usage information</p>
	</body>
</html>`))
	})

	// Info endpoint so users know what version is running
	r.Get("/info", getVersionInfo)

	// Quickstart endpoint to help users get started
	r.Get("/quickstart", quickstart)

	// HAPI endpoints
	// more info at https://github.com/jheising/HAPI
	r.Get("/publish/yoink/for/{topic}", PublishForTopic)
	r.Get("/get/all/yoinks/from/{topic}", GetAllYoinksFromTopic)
	r.Get("/get/latest/yoink/from/{topic}", GetLatestYoinkFromTopic)
	r.Get("/get/last/{number}/yoinks/from/{topic}", getLastNumberOfYoinksFromTopic)
	r.Get("/get/{number}/last/yoinks/from/{topic}", getLastNumberOfYoinksFromTopic)
	r.Get("/get/latest/{number}/yoinks/from/{topic}", getLastNumberOfYoinksFromTopic)
	r.Get("/get/{number}/latest/yoinks/from/{topic}", getLastNumberOfYoinksFromTopic)

	// REST API endpoints
	r.Post("/yoink/{topic}", PublishForTopic) //TODO: expand PublishForTopic to handle POST correctly
	r.Get("/yoink/{topic}", GetLatestYoinkFromTopic)
	r.Get("/yoinks/{topic}/{number}", getLastNumberOfYoinksFromTopic)
	r.Get("/yoinks/{topic}", GetAllYoinksFromTopic)

	return r
}

// SetupPort configures the port that the app listens on
func SetupPort() string {
	port := os.Getenv("DATAYOINKER_PORT")
	if port == "" {
		port = "3333"
	}
	return port
}

func main() {
	// Set up http router
	r := setupRouter()

	// Set up http port
	port := SetupPort()

	// Set up database
	sqlite, err := SetupDB()
	if err != nil {
		log.Fatalln("failed setting up database:", err)
	}
	// assign database to global variable
	db = sqlite

	// Create server with timeouts set
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	// Provide feedback about the server starting
	log.Println("server starting at port " + port)

	// Run the http server
	log.Fatal(srv.ListenAndServe())
}
