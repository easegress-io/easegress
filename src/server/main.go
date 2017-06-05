package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"engine"
	"logger"
	"rest"
)

func main() {
	gateway, err := engine.NewGateway()
	if err != nil {
		logger.Errorf("[initialize gateway engine failed: %v.]", err)
		os.Exit(1)
	}

	api, err := rest.NewReset(gateway)
	if err != nil {
		logger.Errorf("[initialize rest interface failed: %v]", err)
		os.Exit(2)
	}

	setupSignalHandler(gateway)

	done1, err := gateway.Run()
	if err != nil {
		logger.Errorf("[start gateway engine failed: %v]", err)
		os.Exit(3)
	} else {
		logger.Infof("[gateway engine started]")
	}

	done2, listenAddr, err := api.Start()
	if err != nil {
		logger.Errorf("[start rest interface at %s failed: %s]", listenAddr, err)
		os.Exit(4)
	} else {
		logger.Infof("[rest interface started at %s]", listenAddr)
	}

	var msg string
	select {
	case err = <-done1:
		msg = "gateway engine"
	case err = <-done2:
		msg = "api server"
	}

	if err != nil {
		msg = fmt.Sprintf("[exit from %s due to error: %v]", msg, err)
		logger.Warnf(msg)
	} else {
		msg = fmt.Sprintf("[exit from %s without error]", msg)
		logger.Infof(msg)
	}

	// interrupt by signal
	gateway.Close()
	api.Close()

	logger.Infof("[gateway exited normally]")
	os.Exit(0)
}

func setupSignalHandler(gateway *engine.Gateway) {
	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		for times := 0; sigChannel != nil; times++ {
			sig := <-sigChannel
			if sig == nil {
				return // channel closed by normal exit process
			}

			switch times {
			case 0:
				go func() {
					logger.Infof("[%s signal received, shutting down gateway]", sig)
					gateway.Stop()
					close(sigChannel)
					sigChannel = nil
				}()
			case 1:
				logger.Infof("[%s signal received, terminating gateway immediately]", sig)
				close(sigChannel)
				sigChannel = nil
				os.Exit(255)
			}
		}
	}()
}
