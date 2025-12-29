package api

import (
	"io"
	"net/http"
	"os"
	"strconv"
)

func getenv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func getenvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func readAllLimited(r *http.Request, max int64) ([]byte, error) {
	rr := http.MaxBytesReader(nil, r.Body, max)
	defer rr.Close()
	return io.ReadAll(rr)
}
