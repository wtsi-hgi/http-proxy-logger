# HTTP proxy logger

http proxy logger can be used in between a client and a server to log all their
requests and responses to the console.

This is based on https://github.com/stn1slv/http-proxy-logger/ but changes the
output to first decompress gzip'd responses, and to only show the head and tail
of longer responses. It also shows the roundtrip time.


## Build
```
go build -o proxylogger
```

Then copy proxylogger in to your path.

## Usage
If your client talks to server http://domain.com:1234, instead configure it to
talk to http://localhost:1338 and then:

```
TARGET=http://domain.com:1234 PORT=1338 proxylogger
```
