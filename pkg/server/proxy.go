package server

import (
	"github.com/Interstellarss/faas-share/pkg/k8s"
	"io"
	glog "k8s.io/klog"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/openfaas/faas-provider/httputil"
	"github.com/openfaas/faas-provider/types"

	clientset "github.com/Interstellarss/faas-share/pkg/client/clientset/versioned"
)

const (
	watchdogPort       = "8080"
	defaultContentType = "text/plain"

	maxidleconns        = 0
	maxidleConnsPerHost = 10
	idleConnTimeout     = 400 * time.Millisecond
)

// BaseURLResolver URL resolver for proxy requests
//
// The FaaS provider implementation is responsible for providing the resolver function implementation.
// BaseURLResolver.Resolve will receive the function name and should return the URL of the
// function service.
/*
type BaseURLResolver interface {
	//map[string]*SharePodInfo
	Resolve(functionName string, suffix string) (url.URL, string, error)
	Update(duration time.Duration, functionName string, podName string)
	deleteFunction(name string)
	GetSharePodInfo(sharepod string) *map[string]k8s.PodInfo
}
*/

// NewHandlerFunc creates a standard http.HandlerFunc to proxy function requests.
// The returned http.HandlerFunc will ensure:
//
// 	- proper proxy request timeouts
// 	- proxy requests for GET, POST, PATCH, PUT, and DELETE
// 	- path parsing including support for extracing the function name, sub-paths, and query paremeters
// 	- passing and setting the `X-Forwarded-Host` and `X-Forwarded-For` headers
// 	- logging errors and proxy request timing to stdout
//
// Note that this will panic if `resolver` is nil.

func NewHandlerFunc(config types.FaaSConfig, resolver *k8s.FunctionLookup, kube clientset.Interface) http.HandlerFunc {
	if resolver == nil {
		panic("NewHandlerFunc: empty proxy handler resolver, cannot be nil")
	}

	proxyClient := NewProxyClientFromConfig(config)

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			defer r.Body.Close()
		}

		switch r.Method {
		case http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodGet,
			http.MethodOptions,
			http.MethodHead:
			proxyRequest(w, r, proxyClient, resolver, kube)

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

// NewProxyClientFromConfig creates a new http.Client designed for proxying requests and enforcing
// certain minimum configuration values.
func NewProxyClientFromConfig(config types.FaaSConfig) *http.Client {
	return NewProxyClient(config.GetReadTimeout(), config.GetMaxIdleConns(), config.GetMaxIdleConnsPerHost())
}

// NewProxyClient creates a new http.Client designed for proxying requests, this is exposed as a
// convenience method for internal or advanced uses. Most people should use NewProxyClientFromConfig.
func NewProxyClient(timeout time.Duration, maxIdleConns int, maxIdleConnsPerHost int) *http.Client {
	return &http.Client{
		// these Transport values ensure that the http Client will eventually timeout and prevents
		// infinite retries. The default http.Client configure these timeouts.  The specific
		// values tuned via performance testing/benchmarking
		//
		// Additional context can be found at
		// - https://medium.com/@nate510/don-t-use-go-s-default-http-client-4804cb19f779
		// - https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
		//
		// Additionally, these overrides for the default client enable re-use of connections and prevent
		// CoreDNS from rate limiting under high traffic
		//
		// See also two similar projects where this value was updated:
		// https://github.com/prometheus/prometheus/pull/3592
		// https://github.com/minio/minio/pull/5860
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   timeout,
				KeepAlive: 1 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          maxidleconns,
			MaxIdleConnsPerHost:   maxidleConnsPerHost,
			IdleConnTimeout:       idleConnTimeout,
			TLSHandshakeTimeout:   5 * time.Second,
			ExpectContinueTimeout: 1500 * time.Millisecond,
		},
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// proxyRequest handles the actual resolution of and then request to the function service.
func proxyRequest(w http.ResponseWriter, originalReq *http.Request, proxyClient *http.Client, resolver *k8s.FunctionLookup,
	kube clientset.Interface) {
	ctx := originalReq.Context()

	pathVars := mux.Vars(originalReq)
	functionName := pathVars["name"]
	if functionName == "" {
		httputil.Errorf(w, http.StatusBadRequest, "Provide function name in the request path")
		return
	}
	log.Printf("originalReq: %s", originalReq.URL.String())
	slice := strings.Split(originalReq.URL.String(), "/")
	suffix := ""
	log.Printf("after slice: %s", slice)
	if len(slice) > 3 {
		suffix = slice[3]
	}
	log.Printf("request suffix with: %s", suffix)

	functionAddr, podName, resolveErr := resolver.Resolve(functionName, suffix)
	if resolveErr != nil {
		// TODO: Should record the 404/not found error in Prometheus.
		log.Printf("resolver error: no endpoints for %s: %s\n", functionName, resolveErr.Error())
		httputil.Errorf(w, http.StatusServiceUnavailable, "No endpoints available for: %s.", functionName)
		return
	}

	proxyReq, err := buildProxyRequest(originalReq, functionAddr, pathVars["params"])
	if err != nil {
		httputil.Errorf(w, http.StatusInternalServerError, "Failed to resolve service: %s.", functionName)
		return
	}

	if proxyReq.Body != nil {
		defer proxyReq.Body.Close()
	}
	var possi bool = false
	var timeout *time.Timer = time.NewTimer(600 * time.Millisecond)

	if shrinfo, ok := resolver.ShareInfos[functionName]; ok {
		if podinfo, ok := shrinfo.PodInfos[podName]; ok {
			if podinfo.AvgResponseTime.Milliseconds() > 0 && podinfo.AvgResponseTime.Milliseconds() < 300 && len(shrinfo.PodInfos) > 2 {
				timeout = time.NewTimer(podinfo.AvgResponseTime * 2)
			}
		}
	}
	go func() {
		<-timeout.C
		glog.Warningf("possible time out of 500ms or 2 times avg time of shr %s pod %s", functionName, podName)
		possi = true
		go resolver.UpdatePossiTimeOut(true, functionName, podName)
	}()
	start := time.Now()
	response, err := proxyClient.Do(proxyReq.WithContext(ctx))
	seconds := time.Since(start)

	if err != nil {
		log.Printf("error with proxy request to: %s, %s\n", proxyReq.URL.String(), err.Error())

		httputil.Errorf(w, http.StatusInternalServerError, "Can't reach service for: %s.", functionName)
		return
	}

	if timeout.Stop() || response.StatusCode == 200 {
		possi = false
	} else {
		possi = true
	}

	defer func() {
		if response.StatusCode == 200 || response.StatusCode == http.StatusRequestTimeout || response.StatusCode == http.StatusGatewayTimeout {
			go resolver.Update(seconds, functionName, podName, kube, possi)
		} else {
			go resolver.Update(5, functionName, podName, kube, possi)
		}

	}()
	//resolver.
	if response.Body != nil {
		defer response.Body.Close()
	}

	log.Printf("%s took %f seconds\n", functionName, seconds.Seconds())

	clientHeader := w.Header()
	copyHeaders(clientHeader, &response.Header)
	w.Header().Set("Content-Type", getContentType(originalReq.Header, response.Header))

	w.WriteHeader(response.StatusCode)
	if response.Body != nil {
		io.Copy(w, response.Body)
	}

}

// buildProxyRequest creates a request object for the proxy request, it will ensure that
// the original request headers are preserved as well as setting openfaas system headers
func buildProxyRequest(originalReq *http.Request, baseURL url.URL, extraPath string) (*http.Request, error) {

	host := baseURL.Host
	if baseURL.Port() == "" {
		host = baseURL.Host + ":" + watchdogPort
	}

	url := url.URL{
		Scheme:   baseURL.Scheme,
		Host:     host,
		Path:     extraPath,
		RawQuery: originalReq.URL.RawQuery,
	}

	upstreamReq, err := http.NewRequest(originalReq.Method, url.String(), nil)
	if err != nil {
		return nil, err
	}
	copyHeaders(upstreamReq.Header, &originalReq.Header)

	if len(originalReq.Host) > 0 && upstreamReq.Header.Get("X-Forwarded-Host") == "" {
		upstreamReq.Header["X-Forwarded-Host"] = []string{originalReq.Host}
	}
	if upstreamReq.Header.Get("X-Forwarded-For") == "" {
		upstreamReq.Header["X-Forwarded-For"] = []string{originalReq.RemoteAddr}
	}

	if originalReq.Body != nil {
		upstreamReq.Body = originalReq.Body
	}

	return upstreamReq, nil
}

// copyHeaders clones the header values from the source into the destination.
func copyHeaders(destination http.Header, source *http.Header) {
	for k, v := range *source {
		vClone := make([]string, len(v))
		copy(vClone, v)
		destination[k] = vClone
	}
}

// getContentType resolves the correct Content-Type for a proxied function.
func getContentType(request http.Header, proxyResponse http.Header) (headerContentType string) {
	responseHeader := proxyResponse.Get("Content-Type")
	requestHeader := request.Get("Content-Type")

	if len(responseHeader) > 0 {
		headerContentType = responseHeader
	} else if len(requestHeader) > 0 {
		headerContentType = requestHeader
	} else {
		headerContentType = defaultContentType
	}

	return headerContentType
}
