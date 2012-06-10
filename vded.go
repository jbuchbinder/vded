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
	"sort"
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
	Host         string            `json:"host"`
	Name         string            `json:"name"`
	Spoof        bool              `json:"spoof"`
	SubmitMetric bool              `json:"submit_metric"`
	Units        string            `json:"units"`
	Group        string            `json:"group"`
	LatestValue  uint64            `json:"latest_value"`
	LastDiff     uint64            `json:"last_diff"`
	PerMinute    float64           `json:"per_minute"`
	Per5Minutes  float64           `json:"per_5min"`
	Per10Minutes float64           `json:"per_10min"`
	Per30Minutes float64           `json:"per_30min"`
	PerHour      float64           `json:"per_hour"`
	Mutex        *sync.RWMutex     `json:"-"`
	Values       map[string]uint64 `json:"values"`
}

type Switch struct {
	Host        string          `json:"host"`
	Name        string          `json:"name"`
	LatestValue bool            `json:"latest_value"`
	Mutex       *sync.RWMutex   `json:"-"`
	Values      map[string]bool `json:"values"`
}

type SaveState struct {
	Vectors  map[string]*Vector `json:"vectors"`
	Switches map[string]*Switch `json:"switches"`
}

// Store of vectors
var vectors map[string]*Vector
var switches map[string]*Switch

func httpTestHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "[\"This is a test\",{\"a\":1,\"b\":2}]")
}

func httpVectorDumpHandler(w http.ResponseWriter, r *http.Request) {
	//log.Printf("httpVectorDumpHandler()")

	pHost := r.FormValue("host")
	pVector := r.FormValue("vector")

	if pHost == "" || pVector == "" {
		log.Printf("Host and/or vector were not defined")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	vectorKey := getKeyName(pHost, pVector)

	log.Printf("Received dump vector request for %s", vectorKey)

	s, err := json.Marshal(vectors[vectorKey])
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(s)
}

func httpSwitchHandler(w http.ResponseWriter, r *http.Request) {
	// Grab all proper variables
	pAction := r.FormValue("action")
	pHost := r.FormValue("host")
	pSwitch := r.FormValue("switch")
	pValue := r.FormValue("value")
	pTs := r.FormValue("ts")

	if pAction == "" || pHost == "" || pSwitch == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switchKey := getKeyName(pHost, pSwitch)

	switch {
	case pAction == "get":
		{
			if _, ok := switches[switchKey]; ok {
				if switches[switchKey].LatestValue {
					fmt.Fprintf(w, "%s", "true")
				} else {
					fmt.Fprintf(w, "%s", "false")
				}
				return
			} else {
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}
	case pAction == "set":
		{
			value := parseBoolean(pValue, false)
			if _, ok := switches[switchKey]; ok {
				if pTs == "" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				switches[switchKey].Mutex.Lock()
				switches[switchKey].Values[pTs] = value
				switches[switchKey].LatestValue = value
				switches[switchKey].Mutex.Unlock()
			} else {
				// Create new vector
				switches[switchKey] = &Switch{
					Host:        pHost,
					Name:        pSwitch,
					LatestValue: value,
					Mutex:       new(sync.RWMutex),
					Values:      make(map[string]bool),
				}
				switches[switchKey].Mutex.Lock()
				switches[switchKey].Values[pTs] = value
				switches[switchKey].Mutex.Unlock()
			}
		}
	default:
		{
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	// TODO: FIXME: XXX: Need to implement switch handling
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
	value, _ := strconv.ParseUint(pValue, 10, 64)
	vectorKey := getKeyName(pHost, pVector)

	log.Printf("Received vector request for %s", vectorKey)

	if _, ok := vectors[vectorKey]; ok {
		// Key exists
		vectors[vectorKey].Mutex.Lock()
		vectors[vectorKey].Values[pTs] = value
		vectors[vectorKey].LatestValue = value
		vectors[vectorKey].Mutex.Unlock()
	} else {
		// Create new vector
		vectors[vectorKey] = &Vector{
			Host:         pHost,
			Name:         pVector,
			SubmitMetric: pSubmitMetric,
			Spoof:        pSpoof,
			Units:        pUnits,
			Group:        pGroup,
			LatestValue:  value,
			LastDiff:     0,
			PerMinute:    0,
			Per5Minutes:  0,
			Per10Minutes: 0,
			Per30Minutes: 0,
			PerHour:      0,
			Mutex:        new(sync.RWMutex),
			Values:       make(map[string]uint64),
		}
		vectors[vectorKey].Mutex.Lock()
		vectors[vectorKey].Values[pTs] = value
		vectors[vectorKey].Mutex.Unlock()
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
	vectors[key].Mutex.Lock()

	//log.Printf("buildVectorKey len = %d", len(vectors[key].Values))
	if len(vectors[key].Values) <= 1 {
		vectors[key].LastDiff = vectors[key].LatestValue
		vectors[key].PerMinute = 0
		vectors[key].Per5Minutes = 0
		vectors[key].Per10Minutes = 0
		vectors[key].Per30Minutes = 0
		vectors[key].PerHour = 0
	} else {
		keys := make([]string, len(vectors[key].Values))
		i := 0
		for k, _ := range vectors[key].Values {
			keys[i] = k
			i++
		}
		if !sort.StringsAreSorted(keys) {
			//sort.Sort(&keys)
		}
		max1 := keys[i-1]
		max1int, _ := strconv.ParseUint(max1, 10, 64)
		max2 := keys[i-2]
		max2int, _ := strconv.ParseUint(max2, 10, 64)
		tsDiff := max1int - max2int
		if tsDiff < 0 {
			tsDiff = -tsDiff
		}
		if vectors[key].Values[max1] < vectors[key].Values[max2] {
			// Deal with vector value resets, not perfect, but good enough
			vectors[key].LastDiff = vectors[key].Values[max1]
		} else {
			vectors[key].LastDiff = vectors[key].Values[max1] - vectors[key].Values[max2]
		}

		// TODO: FIXME: Track back in history over time periods

		if tsDiff < 30 {
			vectors[key].PerMinute = 0
		} else {
			vectors[key].PerMinute = float64(vectors[key].LastDiff / (uint64(tsDiff) / 60))
		}
	}

	// TODO: IMPLEMENT: XXX:

	vectors[key].Mutex.Unlock()

	// Submit metric
	go gm.SendMetric(vectors[key].Name, fmt.Sprint(vectors[key].LatestValue), gmetric.VALUE_UNSIGNED_INT, vectors[key].Units, gmetric.SLOPE_BOTH, 300, 600, vectors[key].Group)
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
		vectors = savestate.Vectors
	}

}

func serializeToFile() {
	log.Println("serializeToFile()")

	savestate := &SaveState{
		Vectors:  vectors,
		Switches: switches,
	}

	s, err := json.Marshal(savestate)
	if err != nil {
		log.Println(err.Error())
	}

	os.Stdout.Write(s)

	// TODO: implement writing to state file
}

func handleUdpClient(conn *net.UDPConn) {
	var buf []byte
	buf = make([]byte, 512)
	_, _, err := conn.ReadFromUDP(buf[0:])
	if err != nil {
		return
	}

	log.Printf(string(buf))

	data := make(map[string]string)
	jsonerr := json.Unmarshal(buf, &data)
	if jsonerr != nil {
		log.Println("[UDP] error:", err)
		return
	}

	// TODO: FIXME: XXX: Handle UDP data
}

func udpServer() {
	udpaddr, _ := net.ResolveUDPAddr("udp4", fmt.Sprintf(":%d", *port))
	udpconn, udperr := net.ListenUDP("udp", udpaddr)

	if udperr != nil {
		log.Printf(udperr.Error())
		return
	} else {
		for {
			handleUdpClient(udpconn)
		}
	}
}

// Main body

func main() {
	log.Printf("[VDED] Initializing VDED server")
	vectors = make(map[string]*Vector)
	switches = make(map[string]*Switch)

	// Read state
	readState()

	// Set up gmetric connection

	// Thread to purge old values
	purgeThread := func() {
		log.Println("[PURGE] Thread started")
		for {
			time.Sleep(300 * time.Second)
			for k, _ := range vectors {
				vectors[k].Mutex.Lock()
				if len(vectors[k].Values) > *maxEntries {
					targetPurge := len(vectors[k].Values) - *maxEntries
					purgeCount := 0
					for mk, _ := range vectors[k].Values {
						if uint64(purgeCount) < uint64(targetPurge) {
							log.Println("[PURGE] %s : %d", k, mk)
							delete(vectors[k].Values, mk)
							purgeCount++
						}
					}
				}
				vectors[k].Mutex.Unlock()
			}
			for sk, _ := range switches {
				switches[sk].Mutex.Lock()
				if len(switches[sk].Values) > *maxEntries {
					targetPurge := len(switches[sk].Values) - *maxEntries
					purgeCount := 0
					for mk, _ := range switches[sk].Values {
						if uint64(purgeCount) < uint64(targetPurge) {
							log.Println("[PURGE] %s : %d", sk, mk)
							delete(switches[sk].Values, mk)
							purgeCount++
						}
					}
				}
				switches[sk].Mutex.Unlock()
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

	// Spin up UDP server for requests
	log.Printf("[VDED] Starting UDP service on :%d", *port)
	go udpServer()

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
	http.HandleFunc("/switch", httpSwitchHandler)
	http.HandleFunc("/dumpvector", httpVectorDumpHandler)
	log.Fatal(httpServer.ListenAndServe())
}
