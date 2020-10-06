package springcloud

import (
	"strings"
	"sync"
)

type Ribbon struct {
	eureka       *Eureka
	rwLock       *sync.RWMutex
	instanceInfo map[string]*instanceChooser
}

type ApplicationInstance struct {
	InstanceId string
	HostName   string
	App        string
	IpAddr     string
	Port       int
}

type instanceChooser struct {
	index     int
	instances []ApplicationInstance
}

func (chooser *instanceChooser) next() *ApplicationInstance {
	defer func() {
		if chooser.index == len(chooser.instances)-1 {
			chooser.index = 0
		} else {
			chooser.index++
		}
	}()

	return &chooser.instances[chooser.index]
}

func NewRibbon(serverUrl string, applicationName string, registryFetchIntervalSeconds int, registerWithEureka bool, preferIpAddress bool) *Ribbon {
	eureka := NewEureka(serverUrl, applicationName, registryFetchIntervalSeconds, registerWithEureka, preferIpAddress)
	ribbon := &Ribbon{
		eureka: eureka,
		rwLock: new(sync.RWMutex),
	}
	eureka.updateSubscriber = ribbon.onApplicationsUpdate
	return ribbon
}

func (r *Ribbon) GetApplicationInstance(applicationName string) (*ApplicationInstance, bool) {
	r.rwLock.RLock()
	defer r.rwLock.RUnlock()

	for appId, chooser := range r.instanceInfo {
		if strings.EqualFold(appId, applicationName) {
			return chooser.next(), true
		}
	}

	return nil, false
}

func (r *Ribbon) Start() error {
	return r.eureka.Start()
}

func (r *Ribbon) Stop() {
	r.eureka.Stop()
}

func (r *Ribbon) onApplicationsUpdate(applications ApplicationType) {
	instanceInfo := make(map[string]*instanceChooser)
	for applicationName, applicationInfo := range applications {
		instances := make([]ApplicationInstance, 0, len(applicationInfo))
		for _, instanceDto := range applicationInfo {
			instances = append(instances, ApplicationInstance{
				InstanceId: instanceDto.InstanceId,
				HostName:   instanceDto.HostName,
				App:        instanceDto.App,
				IpAddr:     instanceDto.IpAddr,
				Port:       instanceDto.Port.Value,
			})
		}
		instanceInfo[applicationName] = &instanceChooser{
			index:     0,
			instances: instances,
		}
	}

	r.rwLock.Lock()
	defer r.rwLock.Unlock()
	r.instanceInfo = instanceInfo
}
