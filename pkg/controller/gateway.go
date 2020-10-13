package controller

import (
	"compress/gzip"
	"context"
	"gin-demo/pkg/util/jsonlib"
	"gin-demo/pkg/util/springcloud"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

var (
	httpRequestTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "http_request_total",
		Help: "The total number of request received",
	})

	httpRequestForwardSuccess = promauto.NewCounter(prometheus.CounterOpts{
		Name: "http_request_forward_success_total",
		Help: "The total number of successfully forwarded request",
	})

	httpRequestForwardFail = promauto.NewCounter(prometheus.CounterOpts{
		Name: "http_request_forward_fail_total",
		Help: "The total number of unsuccessfully forwarded request",
	})

	httpRequestPanic = promauto.NewCounter(prometheus.CounterOpts{
		Name: "http_request_panic_total",
		Help: "The total number of requests encountering panic",
	})
)

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
			defer func() {
				if err := recover(); err != nil {
					httpRequestPanic.Inc()
				}
			}()

			httpRequestTotal.Inc()
			appId := c.Param("appId")
			uri := c.Param("uri")
			instance, exist := controller.ribbon.GetApplicationInstance(appId)
			if !exist {
				c.JSON(200, &gatewayResponse{
					Code: -1,
					Msg:  "service not found",
				})
				httpRequestForwardFail.Inc()
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
				httpRequestForwardFail.Inc()
				return
			}

			request.Header = c.Request.Header

			response, err := controller.httpClient.Do(request)
			if err != nil {
				c.JSON(200, &gatewayResponse{
					Code: -1,
					Msg:  "failed to access service:" + err.Error(),
				})
				httpRequestForwardFail.Inc()
				return
			}

			httpRequestForwardSuccess.Inc()

			if preserveBody := response.Header.Get("x-preserve-body"); preserveBody != "" {
				extraHeader := make(map[string]string)
				for headerName, headerValues := range response.Header {
					if len(headerValues) > 0 {
						extraHeader[headerName] = headerValues[0]
					}
				}
				c.DataFromReader(response.StatusCode, response.ContentLength, response.Header.Get("content-length"), response.Body, extraHeader)
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
	if err = jsonlib.Unmarshal(bodyBytes, &upstreamResponse); err != nil {
		return &gatewayResponse{
			Code:    -1,
			Msg:     "failed to unmarshal upstream response: " + err.Error(),
			SubCode: -1,
			SubMsg:  string(bodyBytes),
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
