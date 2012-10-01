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
	"log/syslog"
	"net"
	"net/http"
	"os"
	//	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	//	"syscall"
	"time"
)

var (
	port          = flag.Int("port", 48333, "port to listen for requests")
	state         = flag.String("state", "/var/lib/vded/state.json", "path for save state file")
	ghost         = flag.String("ghost", "localhost", "ganglia host(s), comma separated")
	gport         = flag.Int("gport", 8649, "ganglia port")
	spoof         = flag.String("gspoof", "", "ganglia default spoof")
	maxEntries    = flag.Int("max", 300, "maximum number of entries to retain")
	daemonize     = flag.Bool("daemon", false, "fork off daemon process")
	gm            gmetric.Gmetric
	log, _        = syslog.New(syslog.LOG_DEBUG, "vded")
	serializeLock *sync.RWMutex
	vectorQueue   = make(chan *VectorWork)
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

type VectorWork struct {
	VectorName   string
	Host         string
	Vector       string
	Value        uint64
	Ts           string
	SubmitMetric bool
	Units        string
	Spoof        bool
	Group        string
}

// Store of vectors
var vectors map[string]*Vector
var switches map[string]*Switch

func httpTestHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "[\"This is a test\",{\"a\":1,\"b\":2}]")
}

func httpControlHandler(w http.ResponseWriter, r *http.Request) {
	r.Close = true
	pAction := r.FormValue("action")

	if pAction == "" {
		log.Warning("action was not defined")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch {

	case pAction == "serialize":
		{
			go serializeToFile()
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%s", "OK: started serialize job")
		}

	case pAction == "shutdown":
		{
			fmt.Fprintf(w, "%s", "Started shutdown process.")
			log.Warning("[VDED] Shutting down server from control action")
			serializeToFile()
			log.Warning("[VDED] Shutting down NOW")
			os.Exit(0)
		}

	default:
		{
			fmt.Fprintf(w, "%s", "BAD: invalid action")
			w.WriteHeader(http.StatusBadRequest)
		}

	}
}

func httpVectorDumpHandler(w http.ResponseWriter, r *http.Request) {
	r.Close = true
	//log.Printf("httpVectorDumpHandler()")

	pHost := r.FormValue("host")
	pVector := r.FormValue("vector")

	if pHost == "" || pVector == "" {
		log.Warning("Host and/or vector were not defined")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	vectorKey := getKeyName(pHost, pVector)

	log.Debug(fmt.Sprintf("Received dump vector request for %s", vectorKey))

	s, err := json.Marshal(vectors[vectorKey])
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(s)
}

func httpSwitchHandler(w http.ResponseWriter, r *http.Request) {
	r.Close = true
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
				if switches[switchKey].Mutex == nil {
					switches[switchKey].Mutex = new(sync.RWMutex)
				}
				switches[switchKey].Mutex.Lock()
				switches[switchKey].Values[pTs] = value
				switches[switchKey].LatestValue = value
				switches[switchKey].Mutex.Unlock()
			} else {
				// Create new switch
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
}

func httpVectorHandler(w http.ResponseWriter, r *http.Request) {
	r.Close = true
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

	log.Debug(fmt.Sprintf("Received vector request for %s", vectorKey))

	bTimeStart := time.Now()

	// Handle building values, async
	log.Debug(fmt.Sprintf("handleVector: before queue insert %s executed w/duration = %s", vectorKey, time.Now().Sub(bTimeStart).String()))
	go func() {
		vectorQueue <- &VectorWork{
			VectorName:   vectorKey,
			Host:         pHost,
			Vector:       pVector,
			Value:        value,
			Ts:           pTs,
			SubmitMetric: pSubmitMetric,
			Units:        pUnits,
			Spoof:        pSpoof,
			Group:        pGroup,
		}
	}()
	log.Debug(fmt.Sprintf("handleVector: after queue insert %s executed w/duration = %s", vectorKey, time.Now().Sub(bTimeStart).String()))

	// Easy response, no data, since we're handling building the values
	// and aggregation asynchronously
	fmt.Fprintf(w, "OK")

	bTimeEnd := time.Now()
	bDuration := bTimeEnd.Sub(bTimeStart)
	log.Info(fmt.Sprintf("handleVector: %s executed w/duration = %s", vectorKey, bDuration.String()))
}

// Helper functions

func vectorWorker(id int, queue chan *VectorWork) {
	var i *VectorWork
	for {
		log.Info(fmt.Sprintf("vectorWorker %d waiting for work", id))
		i = <-queue
		if i == nil {
			break
		}
		log.Info(fmt.Sprintf("vectorWorker thread %d handling %s", id, i.VectorName))
		bTimeStart := time.Now()
		vectorKey := i.VectorName
		if _, ok := vectors[vectorKey]; ok {
			// Key exists
			if vectors[vectorKey].Mutex == nil {
				vectors[vectorKey].Mutex = new(sync.RWMutex)
			}
			vectors[vectorKey].Mutex.Lock()
			vectors[vectorKey].Values[i.Ts] = i.Value
			vectors[vectorKey].LatestValue = i.Value
			vectors[vectorKey].Mutex.Unlock()
		} else {
			// Create new vector
			vectors[vectorKey] = &Vector{
				Host:         i.Host,
				Name:         i.Vector,
				SubmitMetric: i.SubmitMetric,
				Spoof:        i.Spoof,
				Units:        i.Units,
				Group:        i.Group,
				LatestValue:  i.Value,
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
			vectors[vectorKey].Values[i.Ts] = i.Value
			vectors[vectorKey].Mutex.Unlock()
		}

		buildVectorKey(i.VectorName)

		bTimeEnd := time.Now()
		bDuration := bTimeEnd.Sub(bTimeStart)
		log.Info(fmt.Sprintf("vectorWorker: %s executed w/duration = %s", vectorKey, bDuration.String()))
	}
}

func buildVectorKey(key string) {
	// Adjust values

	bTimeStart := time.Now()

	log.Info(fmt.Sprintf("buildVectorKey len = %d", len(vectors[key].Values)))
	if len(vectors[key].Values) <= 2 {
		vectors[key].Mutex.Lock()
		vectors[key].LastDiff = 0
		vectors[key].PerMinute = 0
		vectors[key].Per5Minutes = 0
		vectors[key].Per10Minutes = 0
		vectors[key].Per30Minutes = 0
		vectors[key].PerHour = 0
		vectors[key].Mutex.Unlock()
	} else {
		keys := make([]string, len(vectors[key].Values))
		i := 0
		for k, _ := range vectors[key].Values {
			keys[i] = k
			i++
		}
		if !sort.StringsAreSorted(keys) {
			sort.Strings(keys)
		}
		max1 := keys[i-1]
		max1int, _ := strconv.ParseUint(max1, 10, 64)
		max2 := keys[i-2]
		max2int, _ := strconv.ParseUint(max2, 10, 64)
		tsDiff := max1int - max2int
		if tsDiff < 0 {
			tsDiff = -tsDiff
		}
		log.Debug(fmt.Sprintf("val1 = %d, val2 = %d", vectors[key].Values[max1], vectors[key].Values[max2]))
		if vectors[key].Values[max1] < vectors[key].Values[max2] {
			// Deal with vector value resets, not perfect, but good enough
			vectors[key].Mutex.Lock()
			vectors[key].LastDiff = vectors[key].Values[max1]
			vectors[key].Mutex.Unlock()
		} else {
			vectors[key].Mutex.Lock()
			vectors[key].LastDiff = vectors[key].Values[max1] - vectors[key].Values[max2]
			vectors[key].Mutex.Unlock()
		}

		// TODO: FIXME: Track back in history over time periods

		vectors[key].Mutex.Lock()
		if tsDiff < 30 {
			vectors[key].PerMinute = 0
		} else {
			vectors[key].PerMinute = float64(float64(vectors[key].LastDiff) / float64(float64(tsDiff)/60))
		}
		vectors[key].Mutex.Unlock()
	}

	bTimeEnd := time.Now()

	// TODO: IMPLEMENT: XXX:

	// Figure out duration
	bDuration := bTimeEnd.Sub(bTimeStart)
	log.Info(fmt.Sprintf("buildVectorKey: %s executed w/duration = %s", key, bDuration.String()))

	// Submit metric
	log.Info(fmt.Sprintf("gm.SendMetric %s = %s", vectors[key].Name, fmt.Sprint(vectors[key].LastDiff)))
	gm.SendMetric(vectors[key].Name, fmt.Sprint(vectors[key].LastDiff), gmetric.VALUE_UNSIGNED_INT, vectors[key].Units, gmetric.SLOPE_BOTH, 300, 600, vectors[key].Group)
	// go gm.SendMetric(fmt.Sprintf("%s_per_1min", vectors[key].Name), fmt.Sprint(vectors[key].PerMinute), gmetric.VALUE_DOUBLE, vectors[key].Units, gmetric.SLOPE_BOTH, 300, 600, vectors[key].Group)
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
	fi, fierr := os.Stat(*state)
	if fierr != nil {
		log.Err(fierr.Error())
		return
	}

	file, err := os.Open(*state)
	if err != nil {
		log.Err(err.Error())
	}

	data := make([]byte, fi.Size())
	count, err := file.Read(data)
	log.Debug(fmt.Sprintf("Read %d bytes from statefile %s", count, *state))
	if count == 0 {
		// No data read, let's just skip anything else, no fatal errors
		return
	}
	if err != nil {
		file.Close()
		log.Err(err.Error())
	}
	file.Close()

	// Attempt to unmarshal the json data
	var savestate SaveState
	umerr := json.Unmarshal(data, &savestate)
	if umerr != nil {
		log.Err("Could not read data from savestate " + umerr.Error())
	} else {
		vectors = savestate.Vectors
	}

}

func serializeToFile() {
	log.Info("serializeToFile()")

	serializeLock.Lock()
	defer serializeLock.Unlock()

	savestate := &SaveState{
		Vectors:  vectors,
		Switches: switches,
	}

	mTimeStart := time.Now()
	s, err := json.Marshal(savestate)
	if err != nil {
		log.Err(err.Error())
	}
	mTimeEnd := time.Now()

	ioTimeStart := time.Now()
	file, ferr := os.Create(*state)
	if ferr != nil {
		log.Err(ferr.Error())
	} else {
		file.Write(s)
		file.Close()
	}
	ioTimeEnd := time.Now()

	// Get some stats
	mDuration := mTimeEnd.Sub(mTimeStart)
	log.Info(fmt.Sprintf("[SERIALIZE] %s marshalling", mDuration.String()))
	ioDuration := ioTimeEnd.Sub(ioTimeStart)
	log.Info(fmt.Sprintf("[SERIALIZE] %s IO to disk", ioDuration.String()))
}

func handleUdpClient(conn *net.UDPConn) {
	var buf []byte
	buf = make([]byte, 512)
	_, _, err := conn.ReadFromUDP(buf[0:])
	if err != nil {
		return
	}

	log.Info(string(buf))

	data := make(map[string]string)
	jsonerr := json.Unmarshal(buf, &data)
	if jsonerr != nil {
		log.Err(fmt.Sprintf("[UDP] error:", err))
		return
	}

	// TODO: FIXME: XXX: Handle UDP data
}

func udpServer() {
	udpaddr, _ := net.ResolveUDPAddr("udp4", fmt.Sprintf(":%d", *port))
	udpconn, udperr := net.ListenUDP("udp", udpaddr)

	if udperr != nil {
		log.Err(udperr.Error())
		return
	} else {
		for {
			handleUdpClient(udpconn)
		}
	}
}

// Main body

func main() {
	flag.Parse()

	gm = gmetric.Gmetric{
		Host:  *spoof,
		Spoof: *spoof,
	}
	gm.SetLogger(log)
	gm.SetVerbose(false)

	if strings.Contains(*ghost, ",") {
		segs := strings.Split(*ghost, ",")
		for i := 0; i < len(segs); i++ {
			gIP, err := net.ResolveIPAddr("ip4", segs[i])
			if err != nil {
				panic(err.Error())
			}
			gm.AddServer(gmetric.GmetricServer{gIP.IP, *gport})
		}
	} else {
		gIP, err := net.ResolveIPAddr("ip4", *ghost)
		if err != nil {
			panic(err.Error())
		}
		gm.AddServer(gmetric.GmetricServer{gIP.IP, *gport})
	}

	log.Info("Initializing VDED server")
	vectors = make(map[string]*Vector)
	switches = make(map[string]*Switch)
	serializeLock = new(sync.RWMutex)

	//signalChannel := make(chan os.Signal)
	//signal.Notify(signalChannel, syscall.SIGHUP, syscall.SIGKILL, syscall.SIGQUIT)

	// Read state
	readState()
	defer serializeToFile()

	// Create workers for vectors
	for vi := 1; vi <= 10; vi++ {
		go vectorWorker(vi, vectorQueue)
	}

	// Set up gmetric connection

	// Thread to purge old values
	purgeThread := func() {
		log.Info("[PURGE] Thread started")
		for {
			time.Sleep(300 * time.Second)
			for k, _ := range vectors {
				if vectors[k].Mutex == nil {
					vectors[k].Mutex = new(sync.RWMutex)
				}
				vectors[k].Mutex.Lock()
				if len(vectors[k].Values) > *maxEntries {
					targetPurge := len(vectors[k].Values) - *maxEntries
					purgeCount := 0
					for mk, _ := range vectors[k].Values {
						if uint64(purgeCount) < uint64(targetPurge) {
							log.Debug(fmt.Sprintf("[PURGE] %s : %d", k, mk))
							delete(vectors[k].Values, mk)
							purgeCount++
						}
					}
				}
				vectors[k].Mutex.Unlock()
			}
			for sk, _ := range switches {
				if switches[sk].Mutex == nil {
					switches[sk].Mutex = new(sync.RWMutex)
				}
				switches[sk].Mutex.Lock()
				if len(switches[sk].Values) > *maxEntries {
					targetPurge := len(switches[sk].Values) - *maxEntries
					purgeCount := 0
					for mk, _ := range switches[sk].Values {
						if uint64(purgeCount) < uint64(targetPurge) {
							log.Debug(fmt.Sprintf("[PURGE] %s : %d", sk, mk))
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
		log.Info("[FLUSH] Thread started")
		for {
			time.Sleep(1800 * time.Second)
			serializeToFile()
		}
	}
	go flushThread()

	if *daemonize {
		log.Info("[VDED] Attempting to fork off daemon process")
		defer runtime.Goexit()
		daemon(false, false)
	}

	// Spin up UDP server for requests
	log.Info(fmt.Sprintf("[VDED] Starting UDP service on :%d", *port))
	go udpServer()

	// Spin up HTTP server for requests
	log.Info(fmt.Sprintf("[VDED] Starting HTTP service on :%d", *port))
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
	http.HandleFunc("/control", httpControlHandler)
	httpServer.ListenAndServe()

	//sig := <-signalChannel
	//log.Warning("[VDED] Shutting down server with " + sig.String())
	//serializeToFile()
	//os.Exit(0)
}
