package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/miekg/dns"
)

type handler struct {
	db *sql.DB
}

func (this *handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	msg := dns.Msg{}
	msg.SetReply(r)
	defer w.WriteMsg(&msg)

	msg.Authoritative = true
	msg.RecursionAvailable = false
	msg.Rcode = dns.RcodeNameError

	domain := r.Question[0].Name
	fmt.Println(domain)

	if !strings.HasSuffix(domain, ".spottedpanther.fun.") {
		return
	}

	parts := strings.Split(domain, ".")
	fmt.Println(parts)
	isJson := false
	if parts[0] == "_json_" {
		isJson = true
		parts = parts[1:]
	}
	if len(parts) < 4 {
		return
	}
	key := parts[0 : len(parts)-4]
	id := parts[len(parts)-4]

	if id == "test" {
		msg.Rcode = dns.RcodeSuccess
		msg.Answer = append(msg.Answer, &dns.TXT{
			Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 60},
			Txt: []string{"asdf", "qwer"},
		})
		return
	}

	fmt.Println(id, key)
	switch r.Question[0].Qtype {
	case dns.TypeTXT:
		for idx, value := range key {
			if value == "_" {
				key[idx] = "%"
			}
		}
		pattern := strings.Join(key, ".")
		wildcard := strings.Join(append([]string{"%"}, key...), ".")
		fmt.Println(pattern, wildcard)
		rows, err := this.db.Query("select path, key, value from entries where id = ? and path like ? or path like ?", id, wildcard, pattern)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer rows.Close()
		entries := map[string]map[string]string{}
		for rows.Next() {
			key := ""
			value := ""
			path := ""
			err = rows.Scan(&path, &key, &value)
			if err != nil {
				fmt.Println(err)
				return
			}
			entry, ok := entries[path]
			if !ok {
				entry = map[string]string{}
			}
			entry[key] = value
			entries[path] = entry
		}
		if len(entries) == 0 {
			return
		}
		text := []string{}
		if isJson {

			data, err := json.Marshal(entries)
			if err != nil {
				fmt.Println(err)
				return
			}
			text = []string{string(data)}
		} else {
			for _, entry := range entries {
				for k, v := range entry {
					text = append(text, k+"="+v)
				}
			}
		}
		msg.Rcode = dns.RcodeSuccess
		msg.Answer = append(msg.Answer, &dns.TXT{
			Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 60},
			Txt: text,
		})
	}
}

func main() {
	address := flag.String("address", "127.0.0.1:9999", "address to listen on")
	debug := flag.Bool("debug", false, "enable debug mode")

	file := "/srv/botcheckup.io/sqlite/kv.db"
	if *debug {
		file = "kv.sqlite"
	}
	flag.Parse()

	srv := &dns.Server{
		Addr: *address,
		Net:  "udp",
	}
	db, err := sql.Open("sqlite3", file)
	if err != nil {
		panic(err)
	}
	db.Exec("CREATE TABLE entries (id string, path string, key string, value string)")
	defer db.Close()
	srv.Handler = &handler{db: db}

	fmt.Println("listening on", *address)
	if err := srv.ListenAndServe(); err != nil {
		panic(err)
	}
}
