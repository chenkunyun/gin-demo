package springcloud

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestGetApplications(t *testing.T) {
	eureka := NewEureka("http://localhost:1111/eureka/", "gin-demo", 30, true, true)
	err := eureka.Start()
	if err != nil {
		t.Fatal("eureka start failed: ", err)
	}
	defer eureka.Stop()

	applications, err := eureka.GetApplications()
	if err != nil {
		t.Fatal("failed to get applications: ", err)
	}
	if len(applications) == 0 {
		t.Fatal("no applications found")
	}

	for appId, app := range applications {
		fmt.Println("application:", appId)
		fmt.Println("----------")
		for _, a := range app {
			fmt.Printf("instanceId:%s, ipAddr:%s, port:%d, status:%s\n", a.InstanceId, a.IpAddr, a.Port.Value, a.Status)
		}
		fmt.Println()
	}
}

func TestCurrentServer(t *testing.T) {
	eureka := NewEureka("http://localhost:1111/eureka/,http://localhost:1112/eureka/", "gin-demo", 30, true, true)
	err := eureka.Start()
	if err != nil {
		t.Fatal("eureka start failed:", err)
	}
	defer eureka.Stop()

	currentServer := eureka.currentServer()
	if currentServer != "http://localhost:1111/eureka/" {
		t.Fatalf("wrong current server, expected:%s, actual:%s", "http://localhost:1111/eureka/", currentServer)
	}

	nextServer := eureka.nextServer()
	if nextServer != "http://localhost:1112/eureka/" {
		t.Fatalf("wrong next server, expected:%s, actual:%s", "http://localhost:1112/eureka/", nextServer)
	}

	currentServer = eureka.currentServer()
	if currentServer != "http://localhost:1112/eureka/" {
		t.Fatalf("wrong current server, expected:%s, actual:%s", "http://localhost:1112/eureka/", currentServer)
	}
}

func TestEureka_GetApplication(t *testing.T) {
	eureka := NewEureka("http://localhost:1112/eureka/,http://localhost:1111/eureka/,http://localhost:1113/eureka/", "gin-demo", 5, true, true)
	err := eureka.Start()
	if err != nil {
		t.Fatal("eureka start failed", err)
	}
	defer eureka.Stop()

	applicationName := "gateway-zuul"
	applications, exist := eureka.GetApplication(applicationName)
	if !exist {
		t.Log("application not exist: ", applicationName)
	} else {
		for _, application := range applications {
			t.Log("instanceId:", application.InstanceId)
		}
	}

	applicationName = "demo-v1"
	applications, exist = eureka.GetApplication(applicationName)
	if !exist {
		t.Log("application not exist: ", applicationName)
	} else {
		for _, application := range applications {
			t.Log("instanceId:", application.InstanceId)
		}
	}
}

func TestHttpClient(t *testing.T) {
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				// the time spent establishing a TCP connection (if a new one is needed).
				Timeout:   4 * time.Second,
				KeepAlive: 15 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: 4 * time.Second,

			// the time spent reading the headers of the response.
			ResponseHeaderTimeout: 10 * time.Second,

			// limits the time the client will wait between sending the request headers
			// when including an Expect: 100-continue and receiving the go-ahead to send the body
			ExpectContinueTimeout: 2 * time.Second,

			// connection pool limit
			MaxConnsPerHost:     2,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     30 * time.Second,
		},
		Timeout: 5 * time.Second,
	}

	waitGroup := &sync.WaitGroup{}
	requestCount := 5
	waitGroup.Add(requestCount)
	for i := 0; i < requestCount; i++ {
		go func(index int) {
			defer waitGroup.Done()
			fmt.Println("begin request: ", index)

			response, err := httpClient.Get("http://localhost:8765/health")
			if err != nil {
				fmt.Println("request: ", index, " failed with: ", err)
				return
			}
			bodyBytes, err := ioutil.ReadAll(response.Body)
			if err != nil {
				fmt.Println("request: ", index, ", failed with: ", err)
				return
			}

			fmt.Println("request: ", index, ", response: "+string(bodyBytes))
		}(i)
	}

	waitGroup.Wait()
}
