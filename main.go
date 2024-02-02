package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

func mkHandler(idb immuConnection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Method: %s", r.Method)
		if r.Method != "POST" {
			http.Error(w, "Invalid method\n", http.StatusBadRequest)
		}
		var msg []logMsg

		err := json.NewDecoder(r.Body).Decode(&msg)
		if err != nil {
			log.Printf("Error decoding msg: %s", err.Error())
			http.Error(w, "Error decoding json\n", http.StatusBadRequest)
			return
		}
		log.Printf("Message: %+v", msg)
		err = idb.pushmsg(msg)
		if err != nil {
			log.Printf("Error pushing to immudb: %s", err.Error())
			http.Error(w, "Invalid pushing to immudb\n", http.StatusInternalServerError)
			return
		}
		log.Printf("Completed!")
		fmt.Fprintf(w, "OK")
		return
	}
}

func ping(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "PONG\n")
}

func main() {
	bind_address := get_env_default("IF_BIND", ":8080")
	idb := immuConnection{}
	idb.cfg_init()
	idb.connect(context.Background())
	http.HandleFunc("/ping", ping)
	http.HandleFunc("/log", mkHandler(idb))
	err := http.ListenAndServe(bind_address, nil)
	log.Printf("Exiting: %s\n", err.Error())
}
