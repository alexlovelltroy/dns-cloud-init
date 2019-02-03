package main

import (
	"fmt"
	// "encoding/json"
	"github.com/gorilla/mux"
	"log"
	"net"
	"net/http"
	"strings"
	"text/template"
)

type host struct {
	instance_id     string `json:"instance-id"`
	hostname        string `json:"hostname"`
	kubernetes_role string `json:"kube-role"`
	ipaddress       string `json:"ip-address"`
	macaddress      string `json:"mac-address"`
	root_public_key string `json:"root-public-key"`
}

const userdata_template = `
#cloud-config
hostname: {{.hostname}}
local-hostname: {{.hostname}}
fqdn: {{.hostname}}.localdomain
manage_etc_hosts: false
ssh_pwauth: True
ssh_authorized_keys:
    - {{.root_public_key}}
`

func (h *host) shortname() string {
	return strings.Split(h.hostname, ".")[0]
}

func remoteIP(r *http.Request) string {
	// This will only be defined when site is accessed via non-anonymous proxy
	// and takes precedence over RemoteAddr
	// Header.Get is case-insensitive
	forwardedIp := r.Header.Get("X-Forwarded-For")
	if forwardedIp != "" {
		return forwardedIp
	}
	actualIp, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		//return nil, fmt.Errorf("userip: %q is not IP:port", r.RemoteAddr)

		log.Println("userip: %q is not IP:port", r.RemoteAddr)
	}
	return actualIp
}

func GetInstanceID(w http.ResponseWriter, r *http.Request) {
	me, _ := GatherTruth(remoteIP(r))
	//fmt.Fprintf(w, "meta-data \n")
	fmt.Fprintf(w, "%s\n", me.instance_id)
}

func GetInstanceHostname(w http.ResponseWriter, r *http.Request) {
	me, _ := GatherTruth(remoteIP(r))
	//fmt.Fprintf(w, "meta-data \n")
	fmt.Fprintf(w, "%s\n", me.shortname())
}

func GetMetaData(w http.ResponseWriter, r *http.Request) {
	me, _ := GatherTruth(remoteIP(r))
	//fmt.Fprintf(w, "meta-data \n")
	fmt.Fprintf(w, "instance-id: %s; \n", me.instance_id)
	fmt.Fprintf(w, "local-hostname: %s \n", me.shortname())
	fmt.Fprintf(w, "hostname: %s \n", me.shortname())
}

func GetUserData(w http.ResponseWriter, r *http.Request) {
	me, _ := GatherTruth(remoteIP(r))
	userDataTemplate, _ := template.New("userdata").Parse(userdata_template)
	userDataTemplate.Execute(w, me)
}

func FetchTXTforIP(ip string) ([]string, error) {
	// Here's what DNS looks like for my cluster
	// Seems like putting tags in the TXT record makes some sense for small tags

	// node0.kubernetes        CNAME   walter
	// node0.kubernetes        TXT     "instance-id=hello0,kube-role=worker"
	// node1.kubernetes        CNAME   karl
	// node1.kubernetes        TXT     "instance-id=hello1,kube-role=master"
	// node2.kubernetes        CNAME   knox
	// node2.kubernetes        TXT     "instance-id=hello2,kube-role=master"
	// node3.kubernetes        CNAME   jesus
	// node3.kubernetes        TXT     "instance-id=hello3,kube-role=master"
	// node4.kubernetes        CNAME   bunny
	// node4.kubernetes        TXT     "instance-id=hello4,kube-role=worker"
	// node5.kubernetes        CNAME   donald
	// node5.kubernetes        TXT     "instance-id=hello5,kube-role=worker"

	var txt_list []string
	names, err := FetchNamesforIP(ip)
	if err != nil {
		return txt_list, err
	}
	for _, name := range names {
		log.Printf("%s - %s \n", ip, name)
		txts, err := net.LookupTXT(name)
		if err != nil {
			return txt_list, err
		}
		if len(txts) == 0 {
			log.Printf("no records for %s \n", name)
		}
		for _, txt := range txts {
			//dig +short gmail.com txt
			log.Printf("%s \n", txt)
			txt_list = append(txt_list, txt)
		}
	}
	return txt_list, nil
}

func FetchNamesforIP(ip string) ([]string, error) {
	var empty []string
	names, err := net.LookupAddr(ip)
	if err != nil {
		log.Println(err)
		return empty, err
	}
	if len(names) == 0 {
		log.Println("no record")
		return empty, fmt.Errorf("No names correspond to the ip: %s", ip)
	}
	log.Printf("returning %d names \n", len(names))
	return names, nil
}

func GatherTruth(ip string) (host, error) {
	me := host{}
	names, err := FetchNamesforIP(ip)
	if err != nil {
		log.Println(err)
		return me, err
	}
	me.hostname = names[0]
	// I'd like to keep identity information in txt records in DNS
	txts, err := FetchTXTforIP(ip)
	if err != nil {
		return me, err
	}
	for _, txt := range txts {
		//dig +short gmail.com txt
		log.Printf("%s", txt)
		items := strings.Split(txt, "=")
		if items[0] == "instance-id" {
			me.instance_id = items[1]
		}
		if items[0] == "kube-role" {
			me.kubernetes_role = items[1]
		}
	}
	return me, nil
}

// our main function
func main() {
	router := mux.NewRouter()
	// Several meta-data urls that I've seen in use
	router.HandleFunc("/meta-data", GetMetaData).Methods("GET")
	router.HandleFunc("/meta-data/", GetMetaData).Methods("GET")
	router.HandleFunc("/meta-data/instance-id", GetInstanceID).Methods("GET")
	router.HandleFunc("/meta-data/hostname", GetInstanceHostname).Methods("GET")
	router.HandleFunc("/meta-data/public-keys", GetInstanceID).Methods("GET")
	router.HandleFunc("/meta-data/public-keys/", GetInstanceID).Methods("GET")
	// The only user-data url I've seen in use
	router.HandleFunc("/user-data", GetUserData).Methods("GET")
	log.Fatal(http.ListenAndServe(":8000", router))
}
