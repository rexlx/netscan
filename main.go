package main

import (
	"context"
	"flag"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	"google.golang.org/api/option"
)

type Program struct {
	LastPort        int
	Project         string
	AddressManifest map[netip.Addr][]int
	Firestore       *firestore.Client
	Mux             *sync.RWMutex
	PortChan        chan int
	ResultChan      chan int
}

type Stat struct {
	Value []float64     `json:"value"`
	Time  time.Time     `json:"time"`
	ID    string        `json:"id"`
	Extra []interface{} `json:"extra"`
}

var (
	project = flag.String("project", "plated-dryad-148318", "Project name")
	workers = flag.Int("workers", 100, "Number of workers")
	wait    = flag.Int("wait", 100, "Wait time")
)

func main() {
	flag.Parse()
	app := NewProgram(*project)
	ctx := context.Background()
	sa := option.WithCredentialsFile("/Users/rxlx/bin/data/fbase.json")
	fb, err := firebase.NewApp(ctx, &firebase.Config{
		ProjectID: *project,
	}, sa)

	if err != nil {
		panic(err)
	}
	client, err := fb.Firestore(ctx)
	if err != nil {
		panic(err)
	}
	defer client.Close()
	app.Firestore = client

	app.GetRecordsFromFireStore()
	for addr, _ := range app.AddressManifest {
		fmt.Println(addr)
		var sc ScannerConfig
		sc.Wait = *wait
		sc.Address = addr.String()
		sc.PortChan = app.PortChan
		sc.ResultChan = app.ResultChan
		for i := 0; i < *workers; i++ {
			go Scanner(&sc)
		}
		go func() {
			for i := 1; i <= app.LastPort; i++ {
				sc.PortChan <- i
			}
		}()
		fmt.Println("Waiting for workers to finish")
		// close(sc.PortChan)
	meanWhile:
		for {
			select {
			case res := <-sc.ResultChan:
				fmt.Println("Port", res, "is open")
				app.Mux.Lock()
				app.AddressManifest[addr] = append(app.AddressManifest[addr], res)
				app.Mux.Unlock()
			case <-time.After(9 * time.Second):
				fmt.Println("Timeout")
				break meanWhile
			}
		}
	}

	fmt.Println("moving on")
	for addr, ports := range app.AddressManifest {
		if len(ports) > 0 {
			println(addr.String(), ports)
		}
	}
}

func NewProgram(project string) *Program {
	manifest := make(map[netip.Addr][]int)
	mux := sync.RWMutex{}
	portChan := make(chan int, 1000)
	resultChan := make(chan int, 1000)
	return &Program{
		Project:         project,
		AddressManifest: manifest,
		Mux:             &mux,
		PortChan:        portChan,
		ResultChan:      resultChan,
		LastPort:        65535,
	}
}

func (p *Program) AddAddress(addr netip.Addr) {
	p.Mux.Lock()
	defer p.Mux.Unlock()
	if _, ok := p.AddressManifest[addr]; !ok {
		p.AddressManifest[addr] = []int{}
	}

}

func (p *Program) GetRecordsFromFireStore() {
	ctx := context.Background()
	query := p.Firestore.Collection("lan-access.log").OrderBy("Time", firestore.Desc).Limit(25)
	iter := query.Documents(ctx)
	for {
		var stat Stat
		doc, err := iter.Next()
		if err != nil {
			break
		}
		doc.DataTo(&stat)
		ipString := stat.Extra[0].(string)
		addrs := strings.Split(ipString, " ")
		for _, addr := range addrs {
			a, err := netip.ParseAddr(addr)
			if err != nil {
				continue
			}
			p.AddAddress(a)
		}
	}
}
