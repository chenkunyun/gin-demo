package springcloud

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type ApplicationType map[string][]ApplicationInstanceDto

const (
	StatusUp               = "UP"
	maxRetryTimesIfFailure = 2
)

type Eureka struct {
	serverUrls                   []string
	currentServerUrlIndex        int
	ApplicationName              string
	registryFetchIntervalSeconds int
	RegisterWithEureka           bool
	PreferIpAddress              bool
	Applications                 ApplicationType
	metaData                     map[string]string
	httpClient                   *http.Client
	rwLock                       *sync.RWMutex
	stop                         chan bool
	registryFetchTimer           *time.Timer
	updateSubscriber             func(applications ApplicationType)
}

type (
	Application struct {
		Applications applicationsDto `json:"applications"`
	}

	applicationsDto struct {
		VersionsDelta string           `json:"versions__delta"`
		AppsHashcode  string           `json:"apps__hashcode"`
		Application   []applicationDto `json:"application"`
	}

	applicationDto struct {
		Name     string                   `json:"name"`
		Instance []ApplicationInstanceDto `json:"instance"`
	}

	PortDto struct {
		Value   int    `json:"$"`
		Enabled string `json:"@enabled"` // true|false
	}

	ApplicationInstanceDto struct {
		InstanceId       string  `json:"instanceId,omitempty"`
		HostName         string  `json:"hostName"`
		App              string  `json:"app"`
		IpAddr           string  `json:"ipAddr"`
		Port             PortDto `json:"port"`
		Status           string  `json:"status"`
		Overriddenstatus string  `json:"overriddenstatus"`
	}
)

func NewEureka(serverUrl string, applicationName string, registryFetchIntervalSeconds int, registerWithEureka bool, preferIpAddress bool) *Eureka {
	serverUrls := make([]string, 0, 3)
	for _, url := range strings.Split(serverUrl, ",") {
		trimSpace := strings.TrimSpace(url)
		if trimSpace != "" {
			serverUrls = append(serverUrls, trimSpace)
		}
	}

	if len(serverUrls) == 0 {
		panic("no valid url provided")
	}

	if registryFetchIntervalSeconds < 5 {
		panic("registryFetchIntervalSeconds should be greater or equal than 5")
	}

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
			MaxConnsPerHost:     100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		},
		// use "context" instead
		// Timeout: 10 * time.Second,
	}

	e := &Eureka{
		serverUrls:                   serverUrls,
		currentServerUrlIndex:        0,
		ApplicationName:              applicationName,
		registryFetchIntervalSeconds: registryFetchIntervalSeconds,
		RegisterWithEureka:           registerWithEureka,
		PreferIpAddress:              preferIpAddress,
		metaData:                     map[string]string{},
		httpClient:                   httpClient,
		rwLock:                       new(sync.RWMutex),
		stop:                         make(chan bool),
	}

	return e
}

func (e *Eureka) Start() error {
	if _, err := e.GetApplications(); err != nil {
		return err
	}

	e.registryFetchTimer = time.NewTimer(time.Second)
	go func() {
		for {
			select {
			case <-e.stop:
				return
			case <-e.registryFetchTimer.C:
				_, _ = e.GetApplications()
				e.registryFetchTimer.Reset(time.Duration(e.registryFetchIntervalSeconds) * time.Second)
			default:

			}
		}
	}()

	return nil
}

func (e *Eureka) Stop() {
	e.stop <- true
	if e.registryFetchTimer != nil {
		if !e.registryFetchTimer.Stop() {
			_, _ = <-e.registryFetchTimer.C
		}
	}
}

func (e *Eureka) AddMetaData(key, value string) {
	e.metaData[key] = value
}

func (e *Eureka) GetApplications() (ApplicationType, error) {
	applications, err := e.doGetApplications(e.currentServer())
	if err == nil {
		e.tryNotifySubscriber(applications)
		return applications, nil
	}

	// retry
	for i := 0; i < maxRetryTimesIfFailure; i++ {
		if applications, err := e.doGetApplications(e.nextServer()); err == nil {
			e.tryNotifySubscriber(applications)
			return applications, nil
		}
	}

	return nil, errors.New("no available server")
}

func (e *Eureka) GetApplication(applicationName string) ([]ApplicationInstanceDto, bool) {
	e.rwLock.RLock()
	defer e.rwLock.RUnlock()

	for appName, instanceDtos := range e.Applications {
		if !strings.EqualFold(appName, applicationName) {
			continue
		}
		applications := make([]ApplicationInstanceDto, 0, len(instanceDtos))
		for _, instanceDto := range instanceDtos {
			if strings.EqualFold(instanceDto.App, applicationName) && instanceDto.Status == StatusUp {
				applications = append(applications, instanceDto)
			}
		}
		if len(applications) == 0 {
			return nil, false
		}
		return applications, true
	}

	return nil, false
}

func (e *Eureka) currentServer() string {
	if len(e.serverUrls) == 1 {
		return e.serverUrls[0]
	}

	e.rwLock.RLock()
	defer e.rwLock.RUnlock()
	return e.serverUrls[e.currentServerUrlIndex]
}

func (e *Eureka) nextServer() string {
	if len(e.serverUrls) == 1 {
		return e.serverUrls[0]
	}

	e.rwLock.Lock()
	defer e.rwLock.Unlock()
	if e.currentServerUrlIndex == len(e.serverUrls)-1 {
		e.currentServerUrlIndex = 0
	} else {
		e.currentServerUrlIndex++
	}

	return e.serverUrls[e.currentServerUrlIndex]
}

func (e *Eureka) doGetApplications(baseUrl string) (ApplicationType, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, "GET", baseUrl+"apps/", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Add("Accept", "application/json")
	request.Header.Add("Accept-Encoding", "gzip")
	request.Header.Add("Connection", "Keep-Alive")
	response, err := e.httpClient.Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, errors.Errorf("server response code:%d", response.StatusCode)
	}

	if response.Body == nil || response.Body == http.NoBody {
		return nil, errors.New("no response body")
	}

	var reader io.ReadCloser
	switch response.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(response.Body)
		if err != nil {
			response.Body.Close()
			return nil, errors.Errorf("failed to create gzip reader:%s", err)
		}
	default:
		reader = response.Body
	}
	defer reader.Close()

	var application Application
	bodyBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, errors.Errorf("error while reading body:%s", err.Error())
	}

	err = json.Unmarshal(bodyBytes, &application)
	if err != nil {
		return nil, errors.Errorf("error while unmarshalling body:%s", err.Error())
	}

	applications := make(ApplicationType)
	for _, app := range application.Applications.Application {
		applications[app.Name] = app.Instance
	}

	e.rwLock.Lock()
	e.Applications = applications
	e.rwLock.Unlock()

	return e.Applications, nil
}

func (e *Eureka) tryNotifySubscriber(applications ApplicationType) {
	if e.updateSubscriber != nil {
		e.updateSubscriber(applications)
	}
}
