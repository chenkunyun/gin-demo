package springcloud

import "testing"

func TestRibbon_GetApplicationInstance(t *testing.T) {
	ribbon := NewRibbon("http://localhost:1111/eureka/", "gin-demo", 30, true, true)
	if err := ribbon.Start(); err != nil {
		t.Fatal("ribbon start failed: ", err)
	}

	instance, exist := ribbon.GetApplicationInstance("demo-v1")
	if exist {
		t.Log("ip: ", instance.IpAddr, ", port: ", instance.Port)
	} else {
		t.Error("application not found")
	}

	instance, exist = ribbon.GetApplicationInstance("demo-v1")
	if exist {
		t.Log("ip: ", instance.IpAddr, ", port: ", instance.Port)
	} else {
		t.Error("application not found")
	}

	instance, exist = ribbon.GetApplicationInstance("demo-v1")
	if exist {
		t.Log("ip: ", instance.IpAddr, ", port: ", instance.Port)
	} else {
		t.Error("application not found")
	}

	instance, exist = ribbon.GetApplicationInstance("demo-v1")
	if exist {
		t.Log("ip: ", instance.IpAddr, ", port: ", instance.Port)
	} else {
		t.Error("application not found")
	}

	instance, exist = ribbon.GetApplicationInstance("demo-v1")
	if exist {
		t.Log("ip: ", instance.IpAddr, ", port: ", instance.Port)
	} else {
		t.Error("application not found")
	}

	instance, exist = ribbon.GetApplicationInstance("demo-v1")
	if exist {
		t.Log("ip: ", instance.IpAddr, ", port: ", instance.Port)
	} else {
		t.Error("application not found")
	}
}
