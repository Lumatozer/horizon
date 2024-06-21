package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
	"crypto/sha256"
)

var config Config
var balance_Mutex sync.Mutex=sync.Mutex{}
var last_Balancer int

type Config struct {
	Balancers                    []string
	Multi_Balancer               bool
	Program_Url                  string
	Max_Local_Program_Instances  int
	Port                         int
	Max_Request_Per_Bucket       int
	Min_Request_Per_Bucket       int
	Scaling_Interval             int
}

type Bucket struct {
	Id                           string
	Port                         int
	Cmd                          exec.Cmd
}

func Proxy(url string, response http.ResponseWriter, request *http.Request) {
	pulled_Request,err:=http.NewRequest(request.Method, url, request.Body)
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

func Upscale() Bucket {
	bucket:=Bucket{}
	byted_Id:=[]byte{}
	digest:=sha256.Sum256([]byte(strconv.FormatInt(time.Now().Unix(), 10)))
	for _,b:=range digest {
		byted_Id = append(byted_Id, b)
	}
	bucket.Id=string(byted_Id)
	return bucket
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
	if config.Multi_Balancer  {
		server_Mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			balance_Mutex.Lock()
			last_Balancer=(last_Balancer+1)%len(config.Balancers)
			balance_Mutex.Unlock()
			balancer:=config.Balancers[last_Balancer]
			Proxy(balancer+r.URL.RawPath, w, r)
		})
	}
	if !config.Multi_Balancer {

	}
	http.ListenAndServe(":"+strconv.FormatInt(int64(config.Port), 10), server_Mux)
}