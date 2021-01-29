package main

import (
  "net/http"
  "io"
  "os"
  "net"
  "strings"
  "fmt"
  "log"
  "time"
  "sync"
  "path/filepath"
)

type PortState int

func (p PortState) String() string {
  switch p {
  case 0:
    return "PortOpen"
  case 1:
    return "PortUnchecked"
  case 2:
    return "PortClosed"
  case 3:
    return "PortTimeout"
  case 4:
    return "PortError"
  default:
    return "????"
  }
}

const (
  PortOpen PortState = iota
  PortUnchecked
  PortClosed
  PortTimeout
  PortErrorOpenFiles
)

type PortScanner struct {
  Ip string
  Ports []Port
}

type Port struct {
  Ip string
  Port int
  State PortState
}

func (p *Port) String() string {
  return fmt.Sprintf("%s:%d %s", p.Ip, p.Port, p.State.String())
}

func (ps *PortScanner) Start(start, end int, timeout time.Duration) {
  var wg sync.WaitGroup

  var ports = make(chan Port, 1)

  for i := start; i <= end; i++ {
    wg.Add(1)

    ports<- Port {
      Ip: ps.Ip,
      Port: i,
      State: PortUnchecked,
    }

    go func() {
      defer wg.Done()
      port := <-ports

      _ = ScanPort(&port, timeout, true)
      if port.State == PortOpen {
        ps.Ports = append(ps.Ports, port)
      }
    }()
  }

  wg.Wait()
}

func ScanPort(p *Port, timeout time.Duration, test bool) error {
    target := fmt.Sprintf("%s:%d", p.Ip, p.Port)
    conn, err := net.DialTimeout("tcp", target, timeout)

    if err != nil {
        if strings.Contains(err.Error(), "too many open files") {
          p.State = PortErrorOpenFiles
          return err
        }

        p.State = PortClosed
        return err
    }
    defer conn.Close()

    p.State = PortOpen
    if test {
      log.Print("TESTING")
      fmt.Fprintf(conn, "GET / HTTP/1.1\r\n\r\nHOST:%s", os.Getenv("RENDER_INTERNAL_HOSTNAME"))

      if err != nil {
        log.Print("Failed to write to conn: %s", p.String())
        return err
      }

      buff := make([]byte, 256)
      _, err := conn.Read(buff)
      if err != nil {
        if err != io.EOF {
          log.Print("Failed to read back from conn: %s", p.String())
          return err
        }
      }

      log.Printf("Read back: %s\n%s\n", p.String(), string(buff))

    }

    return nil
}

type ServerState int

const (
  Up ServerState = iota
  Down
)

type Service struct {
  State ServerState
  RequestCount int
  Username string
  Password string
}

func NewService(username string, password string) Service {
  return Service {
    State: Up,
    Username: username,
    Password: password,
  }
}


type ScanHandler struct {
  Service *Service
  mutex sync.Mutex
}

func NewScanHandler(service *Service) ScanHandler {
  return ScanHandler {
    Service: service,
  }
}

func (h *ScanHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  ip := r.FormValue("ip")
  if ip == "" {
    w.WriteHeader(http.StatusBadRequest)
    fmt.Fprintf(w, "no valid ip provided\n")
    return
  }

  log.Print("starting network scan\n")
  h.mutex.Lock()
  defer h.mutex.Unlock()

  ps := &PortScanner {
    Ip: ip,
  }

  ps.Start(1, 65535, 500*time.Millisecond)


  w.WriteHeader(http.StatusOK)
  for _, p := range ps.Ports {
    fmt.Fprintf(w, "%s:%d %s\n", p.Ip, p.Port, p.State.String())
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

type ListFileSystemHandler struct {
  Service *Service
}

func NewListFileSystemHandler(service *Service) ListFileSystemHandler {
  return ListFileSystemHandler {
    Service: service,
  }
}

func (h *ListFileSystemHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  var files []string

  username, password, ok := r.BasicAuth()

  if !ok || username != h.Service.Username || password != h.Service.Password {
      w.WriteHeader(http.StatusUnauthorized)
      fmt.Fprint(w, "Invalid Username or Password")
      return
  }


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
        w.WriteHeader(http.StatusOK)
        now := time.Now()
        fmt.Fprintf(w, "Service is up\ntime is %d\n", now.Unix())
      case Down:
        w.WriteHeader(http.StatusServiceUnavailable)
        now := time.Now()
        fmt.Fprintf(w, "Service is down\ntime is %d\n", now.Unix())
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
  // for _, e := range os.Environ() {
  //   pair := strings.SplitN(e, "=", 2)
  //   fmt.Println(pair[0], pair[1])
  // }

  username := os.Getenv("USERNAME")
  if username == "" {
    log.Fatal("username unset, must have some value")
  }

  password := os.Getenv("PASSWORD")
  if password == "" {
    log.Fatal("password unset, must have some value")
  }

  service := NewService(username, password)
  countHandler := NewCountHandler(&service)
  healthHandler := NewHealthHandler(&service)
  scanHandler := NewScanHandler(&service)

  http.Handle("/count", &countHandler)
  http.Handle("/health", &healthHandler)
  http.Handle("/files", new(ListFileSystemHandler))
  http.Handle("/scan", &scanHandler)

  log.Print("Starting Server on :8080\n")
  log.Fatal(http.ListenAndServe(":8080", nil))
}
