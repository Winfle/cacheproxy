package cacheproxy

import (
	"net/http"
	"fmt"
)


func ProcessResponse(hash string, resp *http.Request, statusCode, bytesWritten int) {
	
	fmt.Printf("%s %s -> %d Bytes written: %d\n", resp.Method, resp.URL.Path, statusCode, bytesWritten)
	
	fmt.Println(resp.Header.Get("Content-Encoding"))
	rsBody, _ := DecompressGzip(reqBuffer)

	cache.Set(hash, rsBody)
	fmt.Printf("%s -> redis\n", hash)
}