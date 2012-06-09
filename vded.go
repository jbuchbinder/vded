// VDED - Vector Delta Engine Daemon
// https://github.com/jbuchbinder/vded
//
// vim: tabstop=4:softtabstop=4:shiftwidth=4:noexpandtab

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/jbuchbinder/go-gmetric/gmetric"
	"log"
	"net"
	"net/http"
	"os"
	//"sort"
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
	gIP, _     = net.ResolveIPAddr("ip4", *ghost)
	gm         = gmetric.Gmetric{gIP.IP, *gport, *spoof, *spoof}
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

type SaveState struct {
	vectors map[string]*Vector
}

// Store of vectors
var vectors map[string]*Vector

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
		vectors[vectorKey] = &Vector{
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
		vectors[vectorKey].latestValue = value
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
	// Adjust values
	vectors[key].mutex.Lock()

	if len(vectors[key].values) == 1 {
		vectors[key].lastDiff = vectors[key].latestValue
		vectors[key].perMinute = 0
		vectors[key].per5Minutes = 0
		vectors[key].per10Minutes = 0
		vectors[key].per30Minutes = 0
		vectors[key].perHour = 0
	} else {
		var keys []time.Time
		var i int
		for k, _ := range vectors[key].values {
			keys[i] = k
			i++
		}
		//sort.Sort(&keys) // can't sort on time.Time
		max1 := keys[i-1]
		max2 := keys[i-2]
		tsDiff := max1.Unix() - max2.Unix()
		if tsDiff < 0 {
			tsDiff = -tsDiff
		}
		if vectors[key].values[max1] < vectors[key].values[max2] {
			// Deal with vector value resets, not perfect, but good enough
			vectors[key].lastDiff = vectors[key].values[max1]
		} else {
			vectors[key].lastDiff = vectors[key].values[max1] - vectors[key].values[max2]
		}

		// TODO: FIXME: Track back in history over time periods

		if tsDiff < 30 {
			vectors[key].perMinute = 0
		} else {
			vectors[key].perMinute = float64(vectors[key].lastDiff / (uint64(tsDiff) / 60))
		}
	}

	vectors[key].mutex.Unlock()

	// TODO: IMPLEMENT: XXX

	// Submit metric
	go gm.SendMetric(vectors[key].name, fmt.Sprint(vectors[key].latestValue), gmetric.VALUE_UNSIGNED_INT, vectors[key].units, gmetric.SLOPE_BOTH, 300, 600, vectors[key].group)
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

func readState() {
	file, err := os.Open(*state)
	if err != nil {
		log.Print(err)
	}
	// TODO: FIXME: Should not be hardcoding 10M limit here, should be
	// figuring this out from the size of the file from which we are
	// reading data.
	data := make([]byte, 1024*1024*16)
	count, err := file.Read(data)
	log.Printf("Read %d bytes from statefile %s", count, *state)
	if count == 0 {
		// No data read, let's just skip anything else, no fatal errors
		return
	}
	if err != nil {
		file.Close()
		log.Fatal(err)
	}
	file.Close()

	// Attempt to unmarshal the json data
	var savestate SaveState
	umerr := json.Unmarshal(data, &savestate)
	if umerr != nil {
		log.Print("Could not read data from savestate")
	} else {
		vectors = savestate.vectors
	}

}

func serializeToFile() {
	log.Println("serializeToFile()")

	savestate := &SaveState{
		vectors: vectors,
	}

	s, err := json.Marshal(savestate)
	if err != nil {

	}

	os.Stdout.Write(s)
	// Output:
	// "{\"vectors\":" + v + ",\"switches\":" + s + "}"

	// TODO: implement
}

// Main body

func main() {
	log.Printf("[VDED] Initializing VDED server")
	vectors = make(map[string]*Vector)

	// Read state
	readState()

	// Set up gmetric connection

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
		log.Println("[FLUSH] Thread started")
		for {
			serializeToFile()
			time.Sleep(1800 * time.Second)
		}
	}
	go flushThread()

	// Spin up HTTP server for requests
	log.Printf("[VDED] Starting HTTP service on :%d", *port)
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
