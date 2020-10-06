package controller

import (
	"compress/gzip"
	"context"
	"gin-demo/pkg/springcloud"
	"github.com/gin-gonic/gin"
	jsoniter "github.com/json-iterator/go"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type GatewayController struct {
	ribbon     *springcloud.Ribbon
	httpClient *http.Client
}

type gatewayResponse struct {
	Code    int64       `json:"code"`
	Msg     string      `json:"msg"`
	SubCode int64       `json:"sub_code,omitempty"`
	SubMsg  string      `json:"sub_msg,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type upstreamServiceResponse struct {
	Code int64       `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func NewGatewayController(serverUrl string, applicationName string) (*GatewayController, error) {
	ribbon := springcloud.NewRibbon(serverUrl, applicationName, 30, true, true)
	if err := ribbon.Start(); err != nil {
		return nil, err
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

	return &GatewayController{
		ribbon:     ribbon,
		httpClient: httpClient,
	}, nil
}

func (controller *GatewayController) Handle(r *gin.Engine) {
	gatewayGroup := r.Group("gateway")
	{
		gatewayGroup.Any("/:appId/:uri", func(c *gin.Context) {
			appId := c.Param("appId")
			uri := c.Param("uri")
			instance, exist := controller.ribbon.GetApplicationInstance(appId)
			if !exist {
				c.JSON(200, &gatewayResponse{
					Code: -1,
					Msg:  "service not found",
				})
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			u := &url.URL{
				Scheme:   "http",
				Host:     instance.IpAddr + ":" + strconv.Itoa(instance.Port),
				Path:     "/" + uri,
				RawQuery: c.Request.URL.RawQuery,
			}
			request, err := http.NewRequestWithContext(ctx, c.Request.Method, u.String(), c.Request.Body)
			if err != nil {
				c.JSON(200, &gatewayResponse{
					Code: -1,
					Msg:  "failed to create request:" + err.Error(),
				})
				return
			}

			response, err := controller.httpClient.Do(request)
			if err != nil {
				c.JSON(200, &gatewayResponse{
					Code: -1,
					Msg:  "failed to access service:" + err.Error(),
				})
				return
			}

			c.JSON(200, controller.parseUpstreamResponse(response))
		})
	}
}

func (controller *GatewayController) parseUpstreamResponse(response *http.Response) *gatewayResponse {
	if response.StatusCode == http.StatusNotFound {
		response.Body.Close()
		return &gatewayResponse{
			Code:    0,
			Msg:     "success",
			SubCode: 404,
			SubMsg:  "not found",
		}
	}

	var reader io.ReadCloser
	switch response.Header.Get("Content-Encoding") {
	case "gzip":
		var err error
		reader, err = gzip.NewReader(response.Body)
		if err != nil {
			response.Body.Close()
			return &gatewayResponse{
				Code: -1,
				Msg:  "failed to unzip upstream response",
			}
		}
	default:
		reader = response.Body
	}
	defer reader.Close()

	bodyBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return &gatewayResponse{
			Code: -1,
			Msg:  "failed to read response bytes",
		}
	}

	var upstreamResponse upstreamServiceResponse
	if err = json.Unmarshal(bodyBytes, &upstreamResponse); err != nil {
		return &gatewayResponse{
			Code: -1,
			Msg:  "failed to unmarshal upstream response: " + err.Error(),
		}
	}

	return &gatewayResponse{
		Code:    0,
		Msg:     "success",
		SubCode: upstreamResponse.Code,
		SubMsg:  upstreamResponse.Msg,
		Data:    upstreamResponse.Data,
	}
}
