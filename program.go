package main

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
)

type Program struct {
	TimeLayout      string
	LastPort        int
	Project         string
	AddressManifest map[string][]int
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

func NewProgram(project string) *Program {
	layout := "2006-01-02T15"
	manifest := make(map[string][]int)
	mux := sync.RWMutex{}
	portChan := make(chan int, 1000)
	resultChan := make(chan int, 1000)
	return &Program{
		TimeLayout:      layout,
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
	if _, ok := p.AddressManifest[addr.String()]; !ok {
		p.AddressManifest[addr.String()] = []int{}
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

func (p *Program) SaveManifestToFireStore(col string, doc string) {
	fmt.Println("Saving to firestore")
	ctx := context.Background()
	_, err := p.Firestore.Collection(col).Doc(doc).Set(ctx, p.AddressManifest)
	if err != nil {
		panic(err)
	}
}
