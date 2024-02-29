package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	firebase "firebase.google.com/go"
	"google.golang.org/api/option"
)

var (
	project  = flag.String("project", "tight-knit-666666", "firestore project id")
	workers  = flag.Int("workers", 100, "number of workers")
	wait     = flag.Int("wait", 100, "how long to wait for a port to respond in ms")
	credFile = flag.String("cred", "/Users/rxlx/bin/data/fbase.json", "credential file")
)

func main() {
	flag.Parse()

	ctx := context.Background()
	sa := option.WithCredentialsFile(*credFile)

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

	app := NewProgram(*project)
	app.Firestore = client

	app.GetRecordsFromFireStore()

	for addr, _ := range app.AddressManifest {
		var sc ScannerConfig
		sc.Wait = *wait
		sc.Address = addr
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

	meanWhile:
		for {
			select {
			case res := <-sc.ResultChan:
				fmt.Println("Port", res, "is open")
				app.Mux.Lock()
				app.AddressManifest[addr] = append(app.AddressManifest[addr], res)
				app.Mux.Unlock()
			case <-time.After(19 * time.Second):
				fmt.Println("Timeout")
				if len(app.AddressManifest[addr]) > 0 {
					app.Mux.Lock()
					scannedRes := ScannedResult{
						Time:    time.Now(),
						Address: addr,
						Ports:   app.AddressManifest[addr],
					}
					app.Mux.Unlock()
					fmt.Println("Saving to firestore")
					ctx := context.Background()
					_, err := client.Collection("lanscan").Doc(scannedRes.Address).Set(ctx, scannedRes)
					if err != nil {
						panic(err)
					}
				}
				break meanWhile
			}
		}
	}

	fmt.Println("scan complete")
	// docName := time.Now().Format(app.TimeLayout)
	// app.SaveManifestToFireStore("lanscan", docName)
}
