package sseserver

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	rice "github.com/GeertJohan/go.rice"
)

// ReportingStatus is snapshot of metadata about the status of a Server
//
// It can be serialized to JSON and is what gets reported to admin API endpoint.
type ReportingStatus struct {
	Node        string         `json:"node"`
	Status      string         `json:"status"`
	Reported    int64          `json:"reported_at"`
	StartupTime int64          `json:"startup_time"`
	SentMsgs    uint64         `json:"msgs_broadcast"`
	Connections connStatusList `json:"connections"`
}

// implements sort.Interface to enable []connectionStatus to be sorted by age
type connStatusList []connectionStatus

func (cl connStatusList) Len() int           { return len(cl) }
func (cl connStatusList) Swap(i, j int)      { cl[i], cl[j] = cl[j], cl[i] }
func (cl connStatusList) Less(i, j int) bool { return cl[i].Created < cl[j].Created }

// Status returns the ReportingStatus for a given server.
//
// Primarily intended for logging and reporting.
func (s *Server) Status() ReportingStatus {

	stats := ReportingStatus{
		Node:        fmt.Sprintf("%s-%s-%s", platform(), env(), nodeName()),
		Status:      "OK",
		Reported:    time.Now().Unix(),
		StartupTime: s.hub.startupTime.Unix(),
		SentMsgs:    s.hub.sentMsgs,
	}

	stats.Connections = connStatusList{}
	for k := range s.hub.connections {
		stats.Connections = append(stats.Connections, k.Status())
	}
	sort.Sort(stats.Connections)

	return stats
}

// The name of the platform we are running on. For now this is just "go", and is
// more or less a legacy from when there was also a Ruby and NodeJS version of
// this server.
func platform() string {
	return "go"
}

// Attempts to intelligently get the name of the node we are running on.
//
// First checks for a Heroku $DYNO variable (e.g. `web.2` etc), if that isn't
// found will default to the local hostname.
func nodeName() string {
	if dyno := os.Getenv("DYNO"); dyno != "" {
		return dyno
	}
	if host, err := os.Hostname(); err == nil && host != "" {
		return host
	}
	return "unknown.X"
}

// A string representing the environment (dev/staging/prod), for reporting.
func env() string {
	if env := os.Getenv("GO_ENV"); env != "" {
		return env
	}
	return "development"
}

// Handles serving the static HTML page
func adminStatusHTMLHandler(w http.ResponseWriter, r *http.Request) {
	// kinda ridiculous workaround for serving a single static file, sigh.
	box, err := rice.FindBox("views")
	if err != nil {
		log.Fatalf("error opening rice.Box: %s\n", err)
	}

	file, err := box.Open("admin.html")
	if err != nil {
		log.Fatalf("could not open file: %s\n", err)
	}

	fstat, err := file.Stat()
	if err != nil {
		log.Fatalf("could not stat file: %s\n", err)
	}

	http.ServeContent(w, r, fstat.Name(), fstat.ModTime(), file)
}

// Handles serving the JSON status data, effectively the admin API endpoint
func adminStatusDataHandler(w http.ResponseWriter, r *http.Request, s *Server) {
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.MarshalIndent(s.Status(), "", "  ")
	fmt.Fprint(w, string(b))
}

func adminHandler(s *Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Options.DisableAdminEndpoints {
			http.Error(w, "403 admin endpoint disabled", http.StatusForbidden)
			return
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/admin/", adminStatusHTMLHandler)
		mux.HandleFunc("/admin/status.json", func(w http.ResponseWriter, r *http.Request) {
			adminStatusDataHandler(w, r, s)
		})
		mux.ServeHTTP(w, r)
	})
}
