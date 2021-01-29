package main

import (
  "net/http"
  "sync"
  "os"
  "fmt"
  "strings"
  "log"
  "time"
  "path/filepath"
)

type ServerState int

const (
  Up ServerState = iota
  Down
)

type Service struct {
  State ServerState
  RequestCount int
}

func NewService() Service {
  return Service {
    State: Up,
  }
}

type CountHandler struct {
  Service *Service
  mutex sync.Mutex
}

func NewCountHandler(service *Service) CountHandler {
  return CountHandler {
    Service: service,
  }
}

func (h *CountHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  fmt.Println("starting count")

  h.mutex.Lock()
  defer h.mutex.Unlock()

  h.Service.RequestCount++
  fmt.Fprintf(w, "count is %d\n", h.Service.RequestCount)
}

type ListFileSystemHandler struct { }

func (h *ListFileSystemHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  var files []string

  err := r.ParseForm()
  if err != nil {
      w.WriteHeader(http.StatusBadRequest)
      fmt.Fprint(w, "Invalid Url")
      return
  }

  root := r.FormValue("root")
  if root == "" {
      w.WriteHeader(http.StatusBadRequest)
      fmt.Fprint(w, "Invalid url no root provided")
      return
  }

  err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
    files = append(files, path)
    return nil
  })

  if err != nil {
      w.WriteHeader(http.StatusBadRequest)
      fmt.Fprint(w, "Error walking")
      return
  }

  for _, file := range files {
    fmt.Fprintf(w, "%s\n", file)
  }
}


type HealthHandler struct {
  Service *Service
  mutex sync.Mutex
}

func NewHealthHandler(service *Service) HealthHandler {
  return HealthHandler {
    Service: service,
  }
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  switch r.Method {
  case "GET":
    switch h.Service.State {
      case Up:
        now := time.Now()
        w.WriteHeader(http.StatusOK)
        fmt.Fprintf(w, "Service is up\ntime is %d\n", now.Unix())
      case Down:
        w.WriteHeader(http.StatusServiceUnavailable)
        fmt.Fprintf(w, "Service is down")
    }
  case "POST":
    err := r.ParseForm()

    if err != nil {
      w.WriteHeader(http.StatusBadRequest)
      fmt.Fprint(w, "Invalid post form\n")
      // lr := io.LimitReader(r.Body, 100)

      // body, err := lr.Read()

      // if err != nil {
      //   log.Printf("Invalid post could not read body")
      //   return
      // }

      // log.Printf("Invalid post body: %s", body)
      return
    }

    switch r.PostFormValue("state") {
    case "up":
      w.WriteHeader(http.StatusOK)
      fmt.Fprintf(w, "Uping service\n")
      log.Print("Health Up\n")

      h.mutex.Lock()
      defer h.mutex.Unlock()

      h.Service.State = Up
    case "down":
      w.WriteHeader(http.StatusOK)
      fmt.Fprintf(w, "Downing service\n")
      log.Print("Health Down\n")

      h.mutex.Lock()
      defer h.mutex.Unlock()

      h.Service.State = Down
    default:
      w.WriteHeader(http.StatusBadRequest)
      fmt.Fprintf(w, "Invalid post form: state not found\n")
    }
  }
}



func main() {
  for _, e := range os.Environ() {
    pair := strings.SplitN(e, "=", 2)
    fmt.Println(pair[0], pair[1])
  }

  service := NewService()
  countHandler := NewCountHandler(&service)
  healthHandler := NewHealthHandler(&service)

  http.Handle("/count", &countHandler)
  http.Handle("/health", &healthHandler)
  http.Handle("/files", new(ListFileSystemHandler))

  log.Fatal(http.ListenAndServe(":8080", nil))
}
