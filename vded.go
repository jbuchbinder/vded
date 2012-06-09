package main

import (
	"encoding/json"
	"flag"
	"fmt"
	//"github.com/jbuchbinder/go-gmetric/gmetric"
	"log"
	//"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

var (
	port       = flag.Int("port", 48333, "port to listen for requests")
	state      = flag.String("state", "/var/lib/vded/state.json", "path for save state file")
	ghost      = flag.String("ghost", "localhost", "ganglia host")
	gport      = flag.Int("gport", 8649, "ganglia port")
	spoof      = flag.String("gspoof", "", "ganglia default spoof")
	maxEntries = flag.Int("max", 300, "maximum number of entries to retain")
)

type Vector struct {
	host         string
	name         string
	spoof        bool
	submitMetric bool
	units        string
	group        string
	latestValue  uint64
	lastDiff     uint64
	perMinute    float64
	per5Minutes  float64
	per10Minutes float64
	per30Minutes float64
	perHour      float64
	mutex        *sync.RWMutex
	values       map[time.Time]uint64
}

// Store of vectors
var vectors map[string]Vector

func httpTestHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "[\"This is a test\",{\"a\":1,\"b\":2}]")
}

func httpVectorHandler(w http.ResponseWriter, r *http.Request) {
	// Grab all proper variables
	pHost := r.FormValue("host")
	pVector := r.FormValue("vector")
	pValue := r.FormValue("value")
	pTs := r.FormValue("ts")
	pSubmitMetric := parseBoolean(r.FormValue("submit_metric"), true)
	pUnits := r.FormValue("units")
	pSpoof := parseBoolean(r.FormValue("spoof"), true)
	pGroup := r.FormValue("group")

	if pHost == "" || pVector == "" || pValue == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Parse non string values
	rawTs, _ := strconv.ParseInt(pTs, 10, 64)
	ts := time.Unix(rawTs, 0)
	value, _ := strconv.ParseUint(pValue, 10, 64)
	vectorKey := getKeyName(pHost, pVector)

	log.Println("Received vector request for " + vectorKey)

	if _, ok := vectors[vectorKey]; ok {
		// Key exists
		vectors[vectorKey].mutex.Lock()
		vectors[vectorKey].values[ts] = value
		vectors[vectorKey].mutex.Unlock()
	} else {
		// Create new vector
		vectors[vectorKey] = Vector{
			host:         pHost,
			name:         pVector,
			submitMetric: pSubmitMetric,
			spoof:        pSpoof,
			units:        pUnits,
			group:        pGroup,
			latestValue:  value,
			lastDiff:     0,
			perMinute:    0,
			per5Minutes:  0,
			per10Minutes: 0,
			per30Minutes: 0,
			perHour:      0,
			mutex:        new(sync.RWMutex),
			values:       make(map[time.Time]uint64),
		}
		vectors[vectorKey].mutex.Lock()
		vectors[vectorKey].values[ts] = value
		vectors[vectorKey].mutex.Unlock()
	}

	// Handle building values, async
	go buildVectorKey(vectorKey)

	// Easy response, no data, since we're handling building the values
	// and aggregation asynchronously
	fmt.Fprintf(w, "OK")
}

// Helper functions

func buildVectorKey(key string) {
}

func getKeyName(hostName, vectorName string) string {
	return hostName + "/" + vectorName
}

func parseBoolean(v string, defaultValue bool) bool {
	if v == "" {
		return defaultValue
	}
	if v == "1" || v == "true" || v == "TRUE" || v == "True" {
		return true
	}
	return false
}

func serializeToFile() {
	log.Println("serializeToFile()")
	v, err := json.Marshal(vectors)
	if err != nil {

	}

	log.Println(v)
	// Output:
	// "{\"vectors\":" + v + ",\"switches\":" + s + "}"

	// TODO: implement
}

// Main body

func main() {
	vectors = make(map[string]Vector)

	// Thread to purge old values
	purgeThread := func() {
		log.Println("[PURGE] Thread started")
		for {
			time.Sleep(300 * time.Second)
			for k, _ := range vectors {
				vectors[k].mutex.Lock()
				if len(vectors[k].values) > *maxEntries {
					targetPurge := len(vectors[k].values) - *maxEntries
					purgeCount := 0
					for mk, _ := range vectors[k].values {
						if uint64(purgeCount) < uint64(targetPurge) {
							log.Println("[PURGE] %s : %d", k, mk)
							delete(vectors[k].values, mk)
							purgeCount++
						}
					}
				}
				vectors[k].mutex.Unlock()
			}
		}
	}
	go purgeThread()

	// Thread to flush state to disk
	flushThread := func() {
		serializeToFile()
	}
	go flushThread()

	// Spin up HTTP server for requests
	httpServer := &http.Server{
		Addr:           fmt.Sprintf(":%d", *port),
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	http.HandleFunc("/test", httpTestHandler)
	http.HandleFunc("/vector", httpVectorHandler)
	log.Fatal(httpServer.ListenAndServe())
}
