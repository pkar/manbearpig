package manbearpig

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

type JobNotificationList struct {
	Jobs []*Notification `json:"jobs"`
	Auth string          `json:"auth"`
}

type APIServer struct {
	Port           string
	ServiceManager *ServiceManager
}

var API *APIServer

func (a *APIServer) processJobs(jobs *JobNotificationList) {
	for _, job := range jobs.Jobs {
		log.Printf("%+v", job)
		// Send to worker, could add db here for fault tolerance.
		go func(jb *Notification) {
			log.Printf("Starting %v", jb)
			a.ServiceManager.Work(jb, jobs.Auth)
		}(job)
	}
	log.Printf("Finished adding jobs")
}

func (a *APIServer) JobsHandler(w http.ResponseWriter, req *http.Request) {
	if req.Body == nil {
		log.Printf("No Body In Request %+v", req)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Bad Request")
		return
	}
	defer req.Body.Close()

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Printf("%s %+v", err, req)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Bad Request")
		return
	}

	var jnl JobNotificationList
	err = json.Unmarshal(body, &jnl)
	if err != nil {
		log.Printf("%s %+v", err, req)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Bad Request")
		return
	}

	log.Printf("Request: %+v Job: %+v", req, jnl)
	go a.processJobs(&jnl)
	fmt.Fprint(w, "OK")
}

func (a *APIServer) Run() {
	http.HandleFunc("/jobs", a.JobsHandler)
	err := http.ListenAndServe(fmt.Sprintf(":%s", a.Port), nil)
	if err != nil {
		log.Printf("%v", err)
	}
}

func NewAPIServer(port string, sm *ServiceManager) (*APIServer, error) {
	log.Printf("Api Server")
	API = &APIServer{port, sm}
	return API, nil
}
