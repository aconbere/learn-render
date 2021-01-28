package main

import (
  "net/http"
  "sync"
  "os"
  "fmt"
  "strings"
  "log"
)

type countHandler struct {
  mutex sync.Mutex
  n int
}

func (h *countHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  h.mutex.Lock()
  defer h.mutex.Unlock()
  h.n++
  fmt.Fprintf(w, "count is %d\n", h.n)
}

func main() {
  for _, e := range os.Environ() {
    pair := strings.SplitN(e, "=", 2)
    fmt.Println(pair[0], pair[1])
  }

  http.Handle("/count", new(countHandler))

  log.Fatal(http.ListenAndServe(":8080", nil))
}