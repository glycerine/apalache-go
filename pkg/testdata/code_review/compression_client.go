package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

func probe(url, encoding string) (map[string]any, error) {
	id := strings.Repeat("audit", 1024)
	plain, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": id, "method": "health", "params": map[string]any{},
	})
	if err != nil {
		return nil, err
	}
	var wire []byte
	if encoding == "gzip" {
		var buffer bytes.Buffer
		writer := gzip.NewWriter(&buffer)
		if _, err = writer.Write(plain); err != nil {
			return nil, err
		}
		if err = writer.Close(); err != nil {
			return nil, err
		}
		wire = buffer.Bytes()
	} else {
		writer, err := zstd.NewWriter(nil)
		if err != nil {
			return nil, err
		}
		wire = writer.EncodeAll(plain, nil)
		writer.Close()
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(wire))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Content-Encoding", encoding)
	request.Header.Set("Accept-Encoding", encoding)
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var reader io.Reader = response.Body
	switch response.Header.Get("Content-Encoding") {
	case "gzip":
		decoded, err := gzip.NewReader(reader)
		if err != nil {
			return nil, err
		}
		defer decoded.Close()
		reader = decoded
	case "zstd":
		decoded, err := zstd.NewReader(reader)
		if err != nil {
			return nil, err
		}
		defer decoded.Close()
		reader = decoded
	}
	var result map[string]any
	if err = json.NewDecoder(reader).Decode(&result); err != nil {
		return nil, err
	}
	return map[string]any{
		"http_status":       response.StatusCode,
		"response_encoding": response.Header.Get("Content-Encoding"),
		"id_round_trip":     result["id"] == id,
		"result":            result["result"], "error": result["error"],
	}, nil
}

func main() {
	results := make(map[string]any)
	for _, encoding := range []string{"gzip", "zstd"} {
		result, err := probe(os.Args[1], encoding)
		if err != nil {
			results[encoding] = map[string]any{"error": err.Error()}
		} else {
			results[encoding] = result
		}
	}
	encoded, err := json.Marshal(results)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(encoded))
}
