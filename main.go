package main

import (
  "net/http"
  "sync"
  "os"
  "fmt"
  "strings"
  "log"
  "time"
)

type countHandler struct {
  mutex sync.Mutex
  n int
}

func (h *countHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  fmt.Println("starting count")
  h.mutex.Lock()
  defer h.mutex.Unlock()
  h.n++
  fmt.Fprintf(w, "count is %d\n", h.n)
}

type healthHandler struct {
}

func (h *healthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  fmt.Println("starting healh check")
  now := time.Now()
  fmt.Fprintf(w, "time is %d\n", now.Unix())
}


func main() {
  for _, e := range os.Environ() {
    pair := strings.SplitN(e, "=", 2)
    fmt.Println(pair[0], pair[1])
  }

  http.Handle("/count", new(countHandler))
  http.Handle("/bealth", new(healthHandler))

  log.Fatal(http.ListenAndServe(":8080", nil))
}
