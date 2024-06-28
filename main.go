package main

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

func main() {
	target, _ := url.Parse(getTarget())
	log.Printf("Forwarding %s -> %s\n", getListenAddress(), target)

	proxy := httputil.NewSingleHostReverseProxy(target)

	proxy.Transport = DebugTransport{}

	d := proxy.Director
	proxy.Director = func(r *http.Request) {
		d(r) // call default director

		r.Host = target.Host // set Host header as expected by target
	}

	if err := http.ListenAndServe(getListenAddress(), proxy); err != nil {
		panic(err)
	}
}

func getTarget() string {
	target := getEnv("TARGET", "http://example.com")
	return target
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getListenAddress() string {
	port := getEnv("PORT", "1338")
	return ":" + port
}

var reqCounter int32

type DebugTransport struct{}

func (DebugTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	counter := atomic.AddInt32(&reqCounter, 1)

	requestDump, err := httputil.DumpRequestOut(r, true)
	if err != nil {
		return nil, err
	}

	fmt.Printf("\n-------------------------------- %s --------------------------------\n# REQUEST %d\n\n%s\n\n", time.Now(), counter, string(requestDump))

	t := time.Now()

	response, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		return nil, err
	}

	took := time.Since(t)

	responseDump, err := DumpResponse(response, true)
	if err != nil {
		// copying the response body did not work
		return nil, err
	}

	fmt.Printf("\n# RESPONSE %d\n\nRoundtrip: %s\n%s\n\n-------------------------------- %s --------------------------------\n\n", counter, took, string(responseDump), time.Now())

	return response, err
}

// DumpResponse comes from httputil, but decompresses gzip responses and only
// prints the head and tail of longer responses.
func DumpResponse(resp *http.Response, body bool) ([]byte, error) {
	var b bytes.Buffer
	var err error
	save := resp.Body
	savecl := resp.ContentLength

	if !body {
		// For content length of zero. Make sure the body is an empty
		// reader, instead of returning error through failureToReadBody{}.
		if resp.ContentLength == 0 {
			resp.Body = emptyBody
		} else {
			resp.Body = failureToReadBody{}
		}
	} else if resp.Body == nil {
		resp.Body = emptyBody
	} else {
		save, resp.Body, resp.ContentLength, err = drainBody(resp)
		if err != nil {
			return nil, err
		}
	}

	err = resp.Write(&b)
	if err == errNoBody {
		err = nil
	}
	resp.Body = save
	resp.ContentLength = savecl
	if err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

var emptyBody = io.NopCloser(strings.NewReader(""))
var errNoBody = errors.New("sentinel error value")

type failureToReadBody struct{}

func (failureToReadBody) Read([]byte) (int, error) { return 0, errNoBody }
func (failureToReadBody) Close() error             { return nil }

func drainBody(resp *http.Response) (r1, r2 io.ReadCloser, contentLength int64, err error) {
	b := resp.Body
	if b == nil || b == http.NoBody {
		// No copying needed. Preserve the magic sentinel meaning of NoBody.
		return http.NoBody, http.NoBody, contentLength, nil
	}

	var buf bytes.Buffer
	if _, err = buf.ReadFrom(b); err != nil {
		return nil, b, contentLength, err
	}
	if err = b.Close(); err != nil {
		return nil, b, contentLength, err
	}

	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(bytes.NewReader(buf.Bytes()))
		if err != nil {
			return nil, b, contentLength, err
		}

		var gzipBuf bytes.Buffer

		if _, err = gzipBuf.ReadFrom(gzipReader); err != nil {
			return nil, b, contentLength, err
		}
		if err = gzipReader.Close(); err != nil {
			return nil, b, contentLength, err
		}

		displayBuf, contentLength := headTail(gzipBuf)

		return io.NopCloser(&buf), io.NopCloser(displayBuf), contentLength, nil
	}

	displayBuf, contentLength := headTail(buf)

	return io.NopCloser(&buf), io.NopCloser(displayBuf), contentLength, nil
}

func headTail(buf bytes.Buffer) (*bytes.Reader, int64) {
	b := buf.Bytes()
	contentLength := int64(len(b))

	if contentLength > 800 {
		headTail := make([]byte, 803)
		copy(headTail, b[0:400])
		headTail = append(headTail, []byte("...")...)
		headTail = append(headTail, b[contentLength-401:]...)
		contentLength = int64(len(headTail))
		b = headTail
	}

	return bytes.NewReader(b), contentLength
}
