package main

import (
	"fmt"
	"net"
	"time"
)

type ScannerConfig struct {
	Wait       int
	Address    string
	PortChan   chan int
	ResultChan chan int
}

func Scanner(cfg *ScannerConfig) {
	for {
		select {
		case p := <-cfg.PortChan:
			address := fmt.Sprintf("%s:%d", cfg.Address, p)
			res, err := net.DialTimeout("tcp", address, time.Duration(cfg.Wait)*time.Millisecond)
			if err != nil {
				// fmt.Println("Scanner", err)
				continue
			}
			res.Close()
			cfg.ResultChan <- p
		}
	}
}
