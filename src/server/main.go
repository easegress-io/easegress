package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"

	"common"
	"engine"
	"logger"
	"plugins"
	"rest"
	"version"
)

func main() {
	var exitCode int
	var err error

	logger.Infof("[ease gateway server: release=%s, commit=%s, repo=%s]",
		version.RELEASE, version.COMMIT, version.REPO)

	if common.ShowVersion {
		os.Exit(exitCode)
	}

	setupLogFileReopenSignalHandler()

	var cpuProfile *os.File
	if common.CpuProfileFile != "" {
		cpuProfile, err = os.Create(common.CpuProfileFile)
		if err != nil {
			logger.Errorf("[create cpu profile failed: %v]", err)
			exitCode = 1
			return
		}

		pprof.StartCPUProfile(cpuProfile)

		logger.Infof("[cpu profiling started, profile output to %s]", common.CpuProfileFile)
	}

	defer func() {
		if common.CpuProfileFile != "" {
			pprof.StopCPUProfile()

			if cpuProfile != nil {
				cpuProfile.Close()
			}
		}

		os.Exit(exitCode)
	}()

	if common.MemProfileFile != "" {
		// to include every allocated block in the profile
		runtime.MemProfileRate = 1

		setupHeapDumpSignalHandler()

		logger.Infof("[memory profiling enabled, heap dump to %s]", common.CpuProfileFile)
	}

	err = plugins.LoadOutTreePluginTypes()
	if err != nil {
		logger.Errorf("[initialize out-tree plugin type failed: %v]", err)
		exitCode = 2
		return
	}

	gateway, err := engine.NewGateway()
	if err != nil {
		logger.Errorf("[initialize gateway engine failed: %v]", err)
		exitCode = 3
		return
	}

	api, err := rest.NewRest(gateway)
	if err != nil {
		logger.Errorf("[initialize rest interface failed: %v]", err)
		exitCode = 4
		return
	}

	setupExitSignalHandler(gateway)

	done1, err := gateway.Run()
	if err != nil {
		logger.Errorf("[start gateway engine failed: %v]", err)
		exitCode = 5
		return
	} else {
		logger.Infof("[gateway engine started]")
	}

	done2, listenAddr, err := api.Start()
	if err != nil {
		logger.Errorf("[start rest interface at %s failed: %s]", listenAddr, err)
		exitCode = 6
		return
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
		msg = fmt.Sprintf("[exit from %s failed: %v]", msg, err)
		logger.Warnf(msg)
	} else {
		msg = fmt.Sprintf("[exited from %s]", msg)
		logger.Infof(msg)
	}

	// interrupt by signal
	gateway.Close()
	api.Close()

	logger.Infof("[gateway exited normally]")

	return
}

func setupExitSignalHandler(gateway *engine.Gateway) {
	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM)

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

func setupHeapDumpSignalHandler() {
	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		for {
			sig := <-sigChannel
			switch sig {
			case syscall.SIGINT:
				fallthrough
			case syscall.SIGTERM:
				return
			case syscall.SIGQUIT:
				if common.MemProfileFile != "" {
					func() {
						f, err := os.Create(common.MemProfileFile)
						if err != nil {
							logger.Errorf("[create heap dump file failed: %v]", err)
						}
						defer f.Close()

						logger.Debugf("[memory profiling started, heap dump to %s]",
							common.MemProfileFile)

						// get up-to-date statistics
						runtime.GC()

						pprof.WriteHeapProfile(f)

						logger.Infof("[memory profiling finished, heap dump to %s]",
							common.MemProfileFile)
					}()
				}
			}
		}
	}()
}

func setupLogFileReopenSignalHandler() {
	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		for {
			sig := <-sigChannel
			switch sig {
			case syscall.SIGINT:
				fallthrough
			case syscall.SIGTERM:
				return
			case syscall.SIGHUP:
				logger.Infof("[%s signal received, reopen log files]", syscall.SIGHUP)
				logger.ReOpenLogFiles()
			}
		}
	}()
}
