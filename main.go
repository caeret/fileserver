package main

import (
	"flag"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

// ResponseWriter wraps the http.ResponseWriter.
type ResponseWriter struct {
	inner  http.ResponseWriter
	status int
}

func (w *ResponseWriter) Header() http.Header {
	return w.inner.Header()
}

func (w *ResponseWriter) WriteHeader(code int) {
	w.status = code
	w.inner.WriteHeader(code)
}

func (w *ResponseWriter) Write(b []byte) (int, error) {
	return w.inner.Write(b)
}

var (
	dir    string
	isFile bool
	port   int
	eth    string
)

func init() {
	mime.AddExtensionType(".go", "text/plain")

	flag.StringVar(&dir, "d", "", "file or directory")
	flag.StringVar(&eth, "i", "en0", "network interface")
	flag.IntVar(&port, "p", 8000, "http server port")
	flag.Parse()
}

func main() {
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
			os.Exit(10)
		}
	}()
	if len(dir) > 0 {
		fi, err := os.Stat(dir)
		if err != nil {
			panic(err)
		}
		isFile = !fi.IsDir()
	} else {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			panic(err)
		}
	}
	addr := getIP() + ":" + strconv.Itoa(port)
	log.Printf("=> start with http://%s at \"%s\".", addr, dir)
	var handler http.Handler
	if isFile {
		handler = http.HandlerFunc(handleFile)
	} else {
		handler = http.FileServer(http.Dir(dir))
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		wrapper := &ResponseWriter{inner: w, status: 200}
		handler.ServeHTTP(wrapper, r)
		log.Printf("=> [%s] %s %d \"%s\"", r.RemoteAddr, r.RequestURI, wrapper.status, getHeader(r, "User-Agent", "-"))
	})
	panic(http.ListenAndServe(":8000", nil))
}

func getHeader(r *http.Request, key string, defs ...string) string {
	value := r.Header.Get(key)
	if len(value) == 0 && len(defs) > 0 {
		value = defs[0]
	}
	return value
}

func getIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}

	for _, intf := range interfaces {
		if intf.Name == eth {
			addrs, err := intf.Addrs()
			if err != nil {
				panic(err)
			}
			for _, addr := range addrs {
				if ipAddr, ok := addr.(*net.IPNet); ok {
					if ip := ipAddr.IP.To4(); ip != nil {
						return ip.String()
					}
				}
			}
		}
	}
	return "0.0.0.0"
}

func handleFile(w http.ResponseWriter, r *http.Request) {
	// redirect to the filename based uri.
	filename := filepath.Base(dir)
	if "/"+filename != r.URL.Path {
		w.Header().Set("Location", "/"+filename)
		w.WriteHeader(302)
		return
	}
	file, err := os.Open(dir)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer file.Close()
	contentType := mime.TypeByExtension(filepath.Ext(dir))
	if len(contentType) > 0 {
		w.Header().Set("Content-Type", contentType)
	} else {
		w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(dir)+"\"")
	}
	_, err = io.Copy(w, file)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}
