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
	project = flag.String("project", "plated-dryad-148318", "Project name")
	workers = flag.Int("workers", 100, "Number of workers")
	wait    = flag.Int("wait", 100, "Wait time")
)

func main() {
	flag.Parse()

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
				break meanWhile
			}
		}
	}

	fmt.Println("scan complete")
	docName := time.Now().Format(app.TimeLayout)
	app.SaveManifestToFireStore("lanscan", docName)
}
