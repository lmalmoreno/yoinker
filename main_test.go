package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// resetPortEnv should be called at the end of a test
// the parameter should be the initial value of the env var
func resetPortEnv(port string) {
	os.Setenv("DATAYOINKER_PORT", port)
}

// cleanupPortEnv should be called at the beginning of a test
// it empties the env var to avoid interference
func cleanupPortEnv() {
	os.Setenv("DATAYOINKER_PORT", "")
}

// resetPathEnv should be called at the end of a test
// the parameter should be the initial value of the env var
func resetPathEnv(path string) {
	os.Setenv("DB_PATH", path)
}

// cleanupPathEnv should be called at the beginning of a test
// it empties the env var to avoid interference
func cleanupPathEnv() {
	os.Setenv("DB_PATH", "")
}

func buildUp() (string, string, error) {
	initialPath := os.Getenv("DB_PATH")
	testPath := "/tmp/yoinker.db"
	os.Setenv("DB_PATH", testPath)
	testdb, err := SetupDB()
	if err != nil {
		return "", "", fmt.Errorf("setting up the database failed: %w", err)
	}
	db = testdb
	return initialPath, testPath, nil
}

func tearDown(initialPath, testPath string) error {
	err := os.Remove(testPath)
	if err != nil {
		return fmt.Errorf("cleaning up temporary test file failed: %w", err)
	}

	resetPathEnv(initialPath)
	return nil
}

// TestSetupPort tests the setupPort function
// It checks that the port has the correct default value and can be correctly changed
func TestSetupPort(t *testing.T) {
	initialPortValue := os.Getenv("DATAYOINKER_PORT")

	cleanupPortEnv()

	if SetupPort() != "3333" {
		t.Fatal("Default app port value was not the expected one")
	}

	testPort := "53333"
	os.Setenv("DATAYOINKER_PORT", testPort)
	if SetupPort() != testPort {
		t.Fatal("Altered app port value was not the specified one")
	}

	resetPortEnv(initialPortValue)
}

// TestSetupDB tests that the setupDB function does not error out
// It also checks that an alternate database path can be used without issue
func TestSetupDB(t *testing.T) {
	initialPathValue := os.Getenv("DB_PATH")

	cleanupPathEnv()

	testPath := "/tmp/yoinker.db"
	os.Setenv("DB_PATH", testPath)

	db, err := SetupDB()
	if err != nil {
		if db != nil {
			t.Fatal("Error was encountered but database isn't nil")
		}
		t.Fatalf("Setting up the database failed: %v", err)
	}

	err = os.Remove(testPath)
	if err != nil {
		t.Errorf("Cleaning up temporary test file failed: %v", err)
	}

	resetPathEnv(initialPathValue)
}

func TestPublishForTopic(t *testing.T) {
	initialPathValue := os.Getenv("DB_PATH")
	testPath := "/tmp/yoinker.db"
	os.Setenv("DB_PATH", testPath)
	testdb, err := SetupDB()
	if err != nil {
		t.Errorf("Setting up the database failed: %v", err)
	}
	db = testdb

	req := httptest.NewRequest(http.MethodGet, "/publish/yoink/for/testtopic?num=666.666&threads=7&result=discard", nil)

	w := httptest.NewRecorder()

	router := setupRouter()
	router.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Errorf("Reading response body failed: %v", err)
	}
	got := string(data)
	t.Log("Yoink response: ", got)

	y := &Yoink{}

	err = json.NewDecoder(strings.NewReader(got)).Decode(y)
	if err != nil {
		t.Errorf("Decoding response failed: %v", err)
	}

	if y.ID != 1 || y.Topic != "testtopic" {
		t.Fatalf("1 wrong yoink returned: %#v", y)
	}
	if y.Content["num"] != 666.666 {
		t.Fatalf("2 wrong yoink returned: %#v", y)
	}
	if y.Content["threads"] == 7 {
		t.Log("threads:", y.Content["threads"])
		t.Fatalf("3 wrong yoink returned: %#v", y)
	}
	if y.Content["result"] != "discard" {
		t.Fatalf("4 wrong yoink returned: %#v", y)
	}

	err = os.Remove(testPath)
	if err != nil {
		t.Errorf("Cleaning up temporary test file failed: %v", err)
	}

	resetPathEnv(initialPathValue)
}

func TestGetAllYoinksFromTopic(t *testing.T) {
	initialPath, testPath, err := buildUp()
	if err != nil {
		t.Fatalf("buildup failed: %v", err)
	}

	y1, err := publishYoink("testtopic", "num=666.666&threads=7&result=discard")
	if err != nil {
		t.Fatalf("publish yoink failed: %v", err)
	}
	y2, err := publishYoink("testtopic", "yoinker=zoinker&zooted=false")
	if err != nil {
		t.Fatalf("publish yoink failed: %v", err)
	}
	testYs := []*Yoink{y1, y2}

	// get all yoinks which should be the just the one we published above
	req := httptest.NewRequest(http.MethodGet, "/get/all/yoinks/from/testtopic", nil)
	w := httptest.NewRecorder()

	setupRouter().ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Errorf("reading response body failed: %v", err)
	}
	got := string(body)
	t.Log("yoink response: ", got)

	ys := []*Yoink{}

	err = json.NewDecoder(strings.NewReader(got)).Decode(&ys)
	if err != nil {
		t.Errorf("decoding response failed: %v", err)
	}

	for i, testY := range testYs {
		for key, value := range ys[i].Content {
			for k, val := range testY.Content {
				if key == k {
					if value != val {
						t.Fatalf("string yoink content mismatch, expected: %v got: %v", val, value)
					}
					t.Log("val: ", val, "value: ", value)
				}
			}
		}
	}

	err = tearDown(initialPath, testPath)
	if err != nil {
		t.Fatalf("teardown failed: %v", err)
	}
}

func TestGetLatestYoinkFromTopic(t *testing.T) {
	initialPathValue := os.Getenv("DB_PATH")
	testPath := "/tmp/yoinker.db"
	os.Setenv("DB_PATH", testPath)
	testdb, err := SetupDB()
	if err != nil {
		t.Errorf("Setting up the database failed: %v", err)
	}
	db = testdb

	// publish a yoink
	req := httptest.NewRequest(http.MethodGet, "/publish/yoink/for/testtopic?num=666.666&threads=7&result=discard", nil)
	w := httptest.NewRecorder()
	setupRouter().ServeHTTP(w, req)
	res1 := w.Result()
	defer res1.Body.Close()
	data1, err := io.ReadAll(res1.Body)
	if err != nil {
		t.Errorf("Reading response body failed: %v", err)
	}
	got1 := string(data1)

	// get the latest one which should be the same we published above
	r := httptest.NewRequest(http.MethodGet, "/get/latest/yoink/from/testtopic", nil)
	rr := httptest.NewRecorder()

	setupRouter().ServeHTTP(rr, r)

	res2 := rr.Result()
	defer res2.Body.Close()

	data2, err := io.ReadAll(res2.Body)
	if err != nil {
		t.Errorf("Reading response body failed: %v", err)
	}
	got2 := string(data2)
	t.Log("Yoink response: ", got2)

	y := &Yoink{}

	err = json.NewDecoder(strings.NewReader(got2)).Decode(y)
	if err != nil {
		t.Errorf("Decoding response failed: %v", err)
	}

	if got1 != got2 {
		t.Fatalf("yoink mismatch, expected: %s got: %s", got1, got2)
	}

	err = os.Remove(testPath)
	if err != nil {
		t.Errorf("Cleaning up temporary test file failed: %v", err)
	}

	resetPathEnv(initialPathValue)
}

func publishYoink(topic, data string) (*Yoink, error) {
	// publish a yoink
	req := httptest.NewRequest(http.MethodGet, "/publish/yoink/for/"+topic+"?"+data, nil)
	w := httptest.NewRecorder()
	setupRouter().ServeHTTP(w, req)
	res := w.Result()
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body failed: %w", err)
	}
	got := string(body)

	y := &Yoink{}

	err = json.NewDecoder(strings.NewReader(got)).Decode(y)
	if err != nil {
		return nil, fmt.Errorf("decoding response failed: %w", err)
	}
	return y, nil
}
