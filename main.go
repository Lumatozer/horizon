package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
)

var config Config
var balance_Mutex sync.Mutex=sync.Mutex{}
var last_Balancer int

type Config struct {
	Balancers                    []string
	Balance_Server               bool
	Program_Url                  string
	Max_Local_Program_Instances  int
	Port                         int
}

func Balance_Server(response http.ResponseWriter, request *http.Request) {
	balance_Mutex.Lock()
	last_Balancer=(last_Balancer+1)%len(config.Balancers)
	balance_Mutex.Unlock()
	balancer:=config.Balancers[last_Balancer]
	pulled_Request,err:=http.NewRequest(request.Method, balancer+request.URL.RawPath, request.Body)
	if err!=nil {
		http.Error(response, "Internal Error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	for header:=range request.Header {
		pulled_Request.Header.Add(header, request.Header.Get(header))
	}
	client:=&http.Client{}
	pulled_Response,err:=client.Do(pulled_Request)
	if err!=nil {
		http.Error(response, "Internal Error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	body,err:=io.ReadAll(pulled_Response.Body)
	if err!=nil {
		http.Error(response, "Internal Error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	response_Headers:=response.Header()
	for header:=range pulled_Response.Header {
		response_Headers.Add(header, pulled_Response.Header.Get(header))
	}
	response.Write(body)
}

func main() {
	data,err:=os.ReadFile("config.json")
	if err!=nil {
		fmt.Println(err)
		return
	}
	config=Config{}
	err=json.Unmarshal(data, &config)
	if err!=nil {
		fmt.Println(err)
		return
	}
	fmt.Println(config)
	server_Mux:=http.NewServeMux()
	if config.Balance_Server {
		server_Mux.HandleFunc("/", Balance_Server)
	}
	http.ListenAndServe(":"+strconv.FormatInt(int64(config.Port), 10), server_Mux)
}