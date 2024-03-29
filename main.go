package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"immufluent/delaybuffer"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type logMsg struct {
	Date       float64 `json:"date"`
	Time       string  `json:"time"`
	Stream     string  `json:"stream"`
	P          string  `json:"_p"`
	Log        string  `json:"log"`
	AssignedId string  `json:"assigned_id"`
	Kubernetes struct {
		PodName        string            `json:"pod_name"`
		Namespace      string            `json:"namespace_name"`
		PodId          string            `json:"pod_id"`
		Labels         map[string]string `json:"labels"`
		Host           string            `json:"host"`
		ContainerName  string            `json"container_name"`
		DockerId       string            `json:"docker_id"`
		ContainerHash  string            `json:"container_hash"`
		ContainerImage string            `json:"container_image"`
	} `json:kubernetes`
}

type Response struct {
	IsBase64Encoded   bool             `json:"isBase64Encoded"`
	StatusCode        int              `json:"statusCode"`
	StatusDescription string           `json:"statusDescription"`
	Headers           *ResponseHeaders `json:"headers"`
	Body              string           `json:"body"`
}

type ResponseHeaders struct {
	ContentType string `json:"Content-Type"`
}
type Status struct {
	State string `json:"state"`
	Error string `json:"error,omitempty"`
}

func buildResponse(status string, err error) (*Response, error) {
	responseHeaders := new(ResponseHeaders)
	responseHeaders.ContentType = "application/json"
	response := new(Response)
	response.IsBase64Encoded = false
	response.Headers = responseHeaders
	var body []byte
	if err == nil {
		response.StatusCode = 200
		response.StatusDescription = "200 OK"
		body, err = json.Marshal(Status{State: "Ok"})
	} else {
		response.StatusCode = 504
		response.StatusDescription = "504 Something strange happened"
		body, err = json.Marshal(Status{State: "Fail", Error: err.Error()})
	}
	if err != nil {
		log.Printf("Error marshaling response: %s", err.Error())
	}
	response.Body = string(body)
	return response, err
}

func logHandler(idb immuConnection, pushFunc func(logMsg)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Invalid method\n", http.StatusBadRequest)
		}
		var msg []logMsg
		defer r.Body.Close()
		err := json.NewDecoder(r.Body).Decode(&msg)
		if err != nil {
			log.Printf("Error decoding msg: %s", err.Error())
			http.Error(w, "Error decoding json\n", http.StatusBadRequest)
			return
		}
		for _, m := range msg {
			pushFunc(m)
		}
		log.Printf("%d Message(s) buffered", len(msg))
		fmt.Fprintf(w, "OK")
		return
	}
}

func rotator(idb immuConnection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Invalid method\n", http.StatusBadRequest)
			return
		}
		log.Printf("Rotate request")
		if idb.rotate() {
			log.Printf("Rotated!")
			fmt.Fprintf(w, "Rotated\n")
			return
		}
		fmt.Fprintf(w, "OK")
	}
}

func autorotate(idb immuConnection) {
	t := viper.GetInt("autorotate")
	if t <= 0 {
		return
	}
	ticker := time.NewTicker(time.Duration(t) * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				log.Printf("Autorotating")
				idb.rotate()
			}
		}
	}()
}

func ping(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "PONG\n")
}

func init() {
	pflag.String("address", "0.0.0.0", "Binding address")
	pflag.Int("port", 8090, "Listening port")
	pflag.String("immudb-hostname", "127.0.0.1", "immudb server address")
	pflag.Int("immudb-port", 3322, "immudb server port")
	pflag.String("immudb-username", "immudb", "immudb admin username")
	pflag.String("immudb-password", "immudb", "immudb admin password")
	pflag.String("immudb-pattern", "log_%Y_%m", "database pattern name (with strftime variables)")
	pflag.Int("buffer-size", 100, "max buffer size")
	pflag.Int("buffer-delay", 100, "max buffer delay (milliseconds)")
	pflag.Int("autorotate", 86400, "Interval for internal rotation (seconds)")
	pflag.Parse()
	viper.SetEnvPrefix("IF")
	f := pflag.CommandLine
	normalizeFunc := f.GetNormalizeFunc()
	f.SetNormalizeFunc(func(fs *pflag.FlagSet, name string) pflag.NormalizedName {
		result := normalizeFunc(fs, name)
		name = strings.ReplaceAll(string(result), "-", "_")
		return pflag.NormalizedName(name)
	})
	viper.BindPFlags(f)
	viper.AutomaticEnv()
}

func main() {
	bind_string := fmt.Sprintf("%s:%d", viper.GetString("address"), viper.GetInt("port"))
	log.Printf("Starting on %s", bind_string)
	idb := immuConnection{}
	idb.cfg_init()
	idb.connect(context.Background())
	size := viper.GetInt("buffer_size")
	delay := time.Duration(viper.GetInt("buffer_delay")) * time.Millisecond
	buffer := delaybuffer.NewDelayBuffer[logMsg](size, delay, idb.pushmsg)
	autorotate(idb)
	http.HandleFunc("/ping", ping)
	http.HandleFunc("/log", logHandler(idb, buffer.Push))
	http.HandleFunc("/rotate", rotator(idb))
	http.Handle("/metrics", promhttp.Handler())
	err := http.ListenAndServe(bind_string, nil)
	log.Printf("Exiting: %s\n", err.Error())
}
