package main

import (
	"flag"
	"log"
	"os"
	"os/signal"

	"manbearpig"
)

func main() {
	port := flag.String("port", "9999", "port to listen on")
	flag.Parse()

	var serviceManager *manbearpig.ServiceManager
	log.Println("Starting ServiceManager")
	var err error
	serviceManager, err = manbearpig.NewServiceManager()
	if err != nil {
		log.Fatalf("%s", err)
		os.Exit(1)
	}

	log.Println("Starting API server")
	apiServer, err := manbearpig.NewAPIServer(*port, serviceManager)
	if err != nil {
		log.Fatalf("%s", err)
		os.Exit(1)
	}
	go apiServer.Run()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	select {
	case sig := <-interrupt:
		log.Println(sig)
		serviceManager.Close()
	}
}
