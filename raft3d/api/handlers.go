package api

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "raft3d/models"
    "raft3d/raft"

    hraft "github.com/hashicorp/raft"
)

type JoinRequest struct {
    NodeID   string `json:"node_id"`
    RaftAddr string `json:"raft_addr"`
}

func RegisterHandlers(mux *http.ServeMux, rn *raft.RaftNode) {
    mux.HandleFunc("/api/v1/printers", func(w http.ResponseWriter, r *http.Request) {
        switch r.Method {
        case "POST":
            var p models.Printer
            json.NewDecoder(r.Body).Decode(&p)
            cmd := raft.Command{Op: "add_printer", Value: json.RawMessage(marshal(p))}
            rn.Apply(cmd)
            json.NewEncoder(w).Encode(p)
        case "GET":
            json.NewEncoder(w).Encode(rn.FSM.Printers)
        }
    })

    mux.HandleFunc("/api/v1/filaments", func(w http.ResponseWriter, r *http.Request) {
        switch r.Method {
        case "POST":
            var f models.Filament
            json.NewDecoder(r.Body).Decode(&f)
            cmd := raft.Command{Op: "add_filament", Value: json.RawMessage(marshal(f))}
            rn.Apply(cmd)
            json.NewEncoder(w).Encode(f)
        case "GET":
            json.NewEncoder(w).Encode(rn.FSM.Filaments)
        }
    })

    mux.HandleFunc("/api/v1/print_jobs", func(w http.ResponseWriter, r *http.Request) {
        switch r.Method {
        case "POST":
            var pj models.PrintJob
            json.NewDecoder(r.Body).Decode(&pj)
            pj.Status = "Queued"
            cmd := raft.Command{Op: "add_print_job", Value: json.RawMessage(marshal(pj))}
            rn.Apply(cmd)
            json.NewEncoder(w).Encode(pj)
        case "GET":
            json.NewEncoder(w).Encode(rn.FSM.PrintJobs)
        }
    })

    mux.HandleFunc("/api/v1/print_jobs/", func(w http.ResponseWriter, r *http.Request) {
        if r.Method == "POST" {
            id := r.URL.Path[len("/api/v1/print_jobs/"):]
            status := r.URL.Query().Get("status")
            if pj, ok := rn.FSM.PrintJobs[id]; ok {
                pj.Status = status
                cmd := raft.Command{Op: "update_print_job", Value: json.RawMessage(marshal(pj))}
                rn.Apply(cmd)
                json.NewEncoder(w).Encode(pj)
            } else {
                http.Error(w, "Print job not found", http.StatusNotFound)
            }
        }
    })

    mux.HandleFunc("/join", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "POST" {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }
        var jr JoinRequest
        if err := json.NewDecoder(r.Body).Decode(&jr); err != nil {
            http.Error(w, "Invalid request payload", http.StatusBadRequest)
            return
        }
        
        // Validate the Raft address format
        if jr.RaftAddr == "" {
            http.Error(w, "Raft address is required", http.StatusBadRequest)
            return
        }
        
        // Add the voter to the Raft cluster
        future := rn.Raft.AddVoter(hraft.ServerID(jr.NodeID), hraft.ServerAddress(jr.RaftAddr), 0, 0)
        if err := future.Error(); err != nil {
            http.Error(w, fmt.Sprintf("Failed to add voter: %v", err), http.StatusInternalServerError)
            return
        }
        io.WriteString(w, "Join successful")
    })
}

func marshal(v interface{}) []byte {
    data, _ := json.Marshal(v)
    return data
}
