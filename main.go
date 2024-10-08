package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

var config Config
var balance_Mutex sync.Mutex=sync.Mutex{}
var last_Balancer int
var buckets []*Bucket=make([]*Bucket, 0)
var buckets_Mutex sync.Mutex=sync.Mutex{}
var last_Bucket int
var ports_Assigned []int
var requests       int
var cache          map[string]string=make(map[string]string)

type Config struct {
	Balancers                    []string
	Multi_Balancer               bool
	Program_Url                  string
	Max_Local_Program_Instances  int
	Port                         int
	Max_Request_Per_Bucket       int
	Min_Request_Per_Bucket       int
	Scaling_Interval             int
	Database                     bool
	Time_To_Start_Bucket         int
}

type Bucket struct {
	Id                           string
	Port                         int
	Cmd                          *exec.Cmd
	Mutex                        *sync.Mutex
}

type Response struct {
	Value                        string
	Ok                           bool
}

type String_Array_Response struct {
	Value                        []string
	Ok                           bool
}

func Proxy(url string, response http.ResponseWriter, request *http.Request) {
	fmt.Println(url)
	pulled_Request,err:=http.NewRequest(request.Method, url, request.Body)
	if err!=nil {
		http.Error(response, "Internal Error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	for header:=range request.Header {
		pulled_Request.Header.Add(header, request.Header.Get(header))
	}
	for _,cookie:=range request.Cookies() {
		pulled_Request.AddCookie(cookie)
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
	for _,cookie:=range pulled_Response.Cookies() {
		http.SetCookie(response, cookie)
	}
	response.Write(body)
}

func Unzip(source, dest string) error {
	read, err := zip.OpenReader(source)
	if err != nil { return err }
	defer read.Close()
	for _, file := range read.File {
		if file.Mode().IsDir() { continue }
		open, err := file.Open()
		if err != nil { return err }
		name := path.Join(dest, file.Name)
		os.MkdirAll(path.Dir(name), os.ModeDir)
		create, err := os.Create(name)
		if err != nil { return err }
		defer create.Close()
		create.ReadFrom(open)
	}
	return nil
}

func Upscale() (Bucket, error) {
	bucket:=Bucket{Mutex: &sync.Mutex{}}
	byted_Id:=[]byte{}
	digest:=sha256.Sum256([]byte(strconv.FormatInt(time.Now().UnixMilli(), 10)))
	for _,b:=range digest {
		byted_Id = append(byted_Id, b)
	}
	bucket.Id=hex.EncodeToString(byted_Id)
	file,err:=os.Create("buckets/"+bucket.Id+".zip")
	if err!=nil {
		fmt.Println(err)
		return bucket, err
	}
	pulled_Request,err:=http.Get(config.Program_Url)
	if err!=nil {
		fmt.Println(err)
		return bucket, err
	}
	io.Copy(file, pulled_Request.Body)
	file.Close()
	err=Unzip("buckets/"+bucket.Id+".zip", "buckets/"+bucket.Id)
	if err!=nil {
		fmt.Println(err)
		return bucket, err
	}
	os.Remove("buckets/"+bucket.Id+".zip")
	bucket.Port=100
	for {
		to_Use:=true
		for i:=0; i<len(ports_Assigned); i++ {
			if ports_Assigned[i]==bucket.Port {
				to_Use=false
				break
			}
		}
		if to_Use {
			break
		}
		bucket.Port+=1
	}
	os.WriteFile("buckets/"+bucket.Id+"/PORT", []byte(strconv.FormatInt(int64(bucket.Port), 10)), 0644)
	shell_Script,err:=os.ReadFile("buckets/"+bucket.Id+"/start.sh")
	if err!=nil {
		fmt.Println(err)
		return bucket, err
	}
	shell:=strings.Split(strings.Replace(string(shell_Script), "{PORT}", strconv.FormatInt(int64(bucket.Port), 10), -1), "\n")[0]
	bucket.Cmd=exec.Command(strings.Split(shell, " ")[0], strings.Split(shell, " ")[1:]...)
	bucket.Cmd.Dir="buckets/"+bucket.Id
	bucket.Cmd.Start()
	time.Sleep(time.Second * time.Duration(config.Time_To_Start_Bucket))
	buckets_Mutex.Lock()
	ports_Assigned = append(ports_Assigned, bucket.Port)
	buckets = append(buckets, &bucket)
	buckets_Mutex.Unlock()
	return bucket, nil
}

func Downscale(bucket Bucket) {
	buckets_Mutex.Lock()
	bucket.Mutex.Lock()
	if bucket.Cmd!=nil && bucket.Cmd.Process!=nil {
		bucket.Cmd.Process.Kill()
	}
	new_Buckets:=make([]*Bucket, 0)
	for i:=0; i<len(buckets); i++ {
		if buckets[i].Id!=bucket.Id {
			new_Buckets = append(new_Buckets, buckets[i])
		}
	}
	buckets=new_Buckets
	new_Ports:=make([]int, 0)
	for i:=0; i<len(ports_Assigned); i++ {
		if ports_Assigned[i]!=bucket.Port {
			new_Ports = append(new_Ports, ports_Assigned[i])
		}
	}
	ports_Assigned=new_Ports
	buckets_Mutex.Unlock()
	os.RemoveAll("buckets/"+bucket.Id)
	bucket.Mutex.Unlock()
}

func Request_Monitor() {
	cycle_Time:=0
	for {
		time.Sleep(time.Second * time.Duration(config.Scaling_Interval + cycle_Time))
		cycle_Time=0
		buckets_Mutex.Lock()
		rpbs:=requests/(len(buckets)*config.Scaling_Interval)
		requests=0
		buckets_Mutex.Unlock()
		if rpbs>config.Max_Request_Per_Bucket {
			if len(buckets)<config.Max_Local_Program_Instances {
				start_Time:=time.Now().Unix()
				Upscale()
				cycle_Time=int(time.Now().Unix()-start_Time)
			}
		}
		if rpbs<config.Min_Request_Per_Bucket && len(buckets)>1 {
			Downscale(*buckets[0])
		}
		DeleteEmptyDirs("buckets")
	}
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
	fmt.Println("Time to start buckets:", config.Time_To_Start_Bucket)
	server_Mux:=http.NewServeMux()
	if config.Multi_Balancer  {
		server_Mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			url:=r.URL.RawPath
			if r.URL.RawQuery!="" {
				url+="?"+r.URL.RawQuery
			}
			balance_Mutex.Lock()
			last_Balancer=(last_Balancer+1)%len(config.Balancers)
			balance_Mutex.Unlock()
			balancer:=config.Balancers[last_Balancer]
			Proxy(balancer+url, w, r)
		})
	}
	if !config.Multi_Balancer && !config.Database {
		os.Mkdir("buckets", 0644)
		Upscale()
		server_Mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			url:=r.URL.Path
			if r.URL.RawQuery!="" {
				url+="?"+r.URL.RawQuery
			}
			buckets_Mutex.Lock()
			requests+=1
			last_Bucket=(last_Bucket+1)%len(buckets)
			bucket:=buckets[last_Bucket]
			buckets_Mutex.Unlock()
			bucket.Mutex.Lock()
			Proxy("http://127.0.0.1:"+strconv.FormatInt(int64(bucket.Port), 10)+url, w, r)
			bucket.Mutex.Unlock()
		})
		go Request_Monitor()
	}
	if config.Database {
		db := getConn("main.db")
		db_Mutex:=sync.Mutex{}
		defer db.Close()
		server_Mux.HandleFunc("/set", func(w http.ResponseWriter, r *http.Request) {
			key:=r.URL.Query().Get("key")
			if key=="" {
				w.Write([]byte("error"))
				return
			}
			val:=r.URL.Query().Get("val")
			if val=="" {
				w.Write([]byte("error"))
				return
			}
			db_Mutex.Lock()
			defer db_Mutex.Unlock()
			Set(db, key, val)
			cache[key]=val
			if len(cache)>100 {
				for key:=range cache {
					delete(cache, key)
					break
				}
			}
			w.Write([]byte("true"))
		})
		server_Mux.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
			key:=r.URL.Query().Get("key")
			if key=="" {
				return
			}
			db_Mutex.Lock()
			defer db_Mutex.Unlock()
			value,ok:=cache[key]
			if ok {
				out,_:=json.Marshal(Response{Ok: true, Value: value})
				w.Write(out)
			} else {
				value,ok:=Get(db, key)
				if ok {
					cache[key]=value
				}
				out,_:=json.Marshal(Response{Ok: ok, Value: value})
				w.Write(out)
			}
		})
		server_Mux.HandleFunc("/get_all", func(w http.ResponseWriter, r *http.Request) {
			key:=r.URL.Query().Get("key")
			if key=="" {
				return
			}
			db_Mutex.Lock()
			defer db_Mutex.Unlock()
			value,ok:=Get_All(db, key)
			out_string,_:=json.Marshal(value)
			out,_:=json.Marshal(Response{Ok: ok, Value: string(out_string)})
			w.Write(out)
		})
		server_Mux.HandleFunc("/delete", func(w http.ResponseWriter, r *http.Request) {
			key:=r.URL.Query().Get("key")
			if key=="" {
				return
			}
			db_Mutex.Lock()
			defer db_Mutex.Unlock()
			ok:=Delete(db, key)
			out,_:=json.Marshal(Response{Ok: ok, Value: ""})
			w.Write(out)
		})
	}
	http.ListenAndServe(":"+strconv.FormatInt(int64(config.Port), 10), server_Mux)
}