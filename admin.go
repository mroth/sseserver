package sseserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"time"
)

type hubStatus struct {
	Node        string         `json:"node"`
	Status      string         `json:"status"`
	Reported    int64          `json:"reported_at"`
	StartupTime int64          `json:"startup_time"`
	SentMsgs    uint64         `json:"msgs_broadcast"`
	Connections connStatusList `json:"connections"`
}

// Status returns the status struct for a given server.
// This mirrors what is made available on the /admin/status.json endpoint.
//
// Mostly useful for reporting.
func (s *Server) Status() hubStatus {
	return s.hub.Status()
}

// implement sort.Interface for the connections
type connStatusList []connectionStatus

func (cl connStatusList) Len() int           { return len(cl) }
func (cl connStatusList) Swap(i, j int)      { cl[i], cl[j] = cl[j], cl[i] }
func (cl connStatusList) Less(i, j int) bool { return cl[i].Created < cl[j].Created }

// Status returns the status struct for a given connection hub.
// This hub is the real source of truth and Server is just a wrapper but people
// don't know that...
func (h *hub) Status() hubStatus {

	stat := hubStatus{
		Node:        fmt.Sprintf("%s-%s-%s", platform(), env(), dyno()),
		Status:      "OK",
		Reported:    time.Now().Unix(),
		StartupTime: h.startupTime.Unix(),
		SentMsgs:    h.sentMsgs,
	}

	stat.Connections = connStatusList{}
	for k := range h.connections {
		stat.Connections = append(stat.Connections, k.Status())
	}
	sort.Sort(stat.Connections)

	return stat
}

func platform() string {
	return "go"
}

func dyno() string {
	dyno := os.Getenv("DYNO")
	if dyno != "" {
		return dyno
	} else {
		return "dev.1"
	}
}

func env() string {
	env := os.Getenv("GO_ENV")
	if env != "" {
		return env
	} else {
		return "development"
	}
}

func adminStatusDataHandler(w http.ResponseWriter, r *http.Request, h *hub) {
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.MarshalIndent(h.Status(), "", "  ")
	fmt.Fprint(w, string(b))
	return
}
