package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kit/kit/log"
	"golang.org/x/net/context"

	"github.com/obazavil/openstack-workload-transcoding/database"
	"github.com/obazavil/openstack-workload-transcoding/wtcommon"
)

// test:  go run database/cmd/main.go
func main() {
	var err error

	var (
		httpAddr = ":" + wtcommon.DATABASE_PORT
	)

	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stdout)
		logger = log.NewContext(logger).With("ts", log.DefaultTimestampUTC)
		logger = log.NewContext(logger).With("caller", log.DefaultCaller)
	}
	httpLogger := log.NewContext(logger).With("component", "http")

	var ctx context.Context
	{
		ctx = context.Background()
	}

	var ds database.Service
	{
		ds, err = database.NewService()
		if err != nil {
			logger.Log("error", "Cannot create service: "+err.Error())
			os.Exit(1)
		}
	}

	mux := http.NewServeMux()

	mux.Handle("/", database.MakeHandler(ctx, ds, httpLogger))

	http.Handle("/", wtcommon.AccessControl(mux))

	errs := make(chan error, 2)

	go func() {
		logger.Log("transport", "http", "address", httpAddr, "msg", "listening")
		errs <- http.ListenAndServeTLS(httpAddr, "certs/server.pem", "certs/server.key", nil)
	}()
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT)
		errs <- fmt.Errorf("%s", <-c)
	}()

	logger.Log("terminated", <-errs)
}
