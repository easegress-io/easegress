/*
 * Copyright (c) 2017, The Easegress Authors
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/go-chi/chi/v5"
	"github.com/megaease/easegress/v2/pkg/logger"
	"go.uber.org/zap"
)

func (s *Server) logsAPIEntries() []*Entry {
	return []*Entry{
		{
			Path:    "/logs",
			Method:  "GET",
			Handler: s.getLogs,
		},
		{
			Path:    "/logs/level/{level}",
			Method:  "PUT",
			Handler: s.setLogLevel,
		},
		{
			Path:    "/logs/level",
			Method:  "GET",
			Handler: s.getLogLevel,
		},
	}
}

type logFile struct {
	Path     string
	File     *os.File
	Tail     int
	Follow   bool
	EndIndex int64
}

// Close close the log file.
func (lf *logFile) Close() {
	lf.File.Close()
}

func newLogFile(r *http.Request, filePath string) (*logFile, error) {
	tail, follow, err := parseLogQueries(r)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	end, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		file.Close()
		return nil, err
	}

	return &logFile{
		Path:     filePath,
		File:     file,
		Tail:     tail,
		Follow:   follow,
		EndIndex: end,
	}, nil
}

func (s *Server) getLogLevel(w http.ResponseWriter, r *http.Request) {
	level := logger.GetLogLevel()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(level))
}

func (s *Server) setLogLevel(w http.ResponseWriter, r *http.Request) {
	level := chi.URLParam(r, "level")
	if level == "" {
		HandleAPIError(w, r, http.StatusBadRequest, errors.New("level is required"))
		return
	}
	level = strings.ToLower(level)
	if level == "debug" {
		logger.SetLogLevel(zap.DebugLevel)
	} else if level == "info" {
		logger.SetLogLevel(zap.InfoLevel)
	} else {
		HandleAPIError(w, r, http.StatusBadRequest, fmt.Errorf("invalid level %s, only support to set log level to info or debug", level))
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) getLogs(w http.ResponseWriter, r *http.Request) {
	flusher := w.(http.Flusher)
	var err error
	defer func() {
		if err != nil {
			HandleAPIError(w, r, http.StatusInternalServerError, err)
		}
	}()

	logPath := logger.GetLogPath()
	if logPath == "" {
		err = errors.New("log path not found")
		return
	}

	logFile, err := newLogFile(r, logPath)
	if err != nil {
		return
	}
	defer logFile.Close()

	err = logFile.ReadWithTail(w)
	if err != nil {
		return
	}
	flusher.Flush()

	// watch log file from the end
	if !logFile.Follow {
		return
	}
	writeCh, closeFn, err := logFile.Watch()
	if err != nil {
		return
	}
	defer closeFn()

	for {
		select {
		case str, ok := <-writeCh:
			if !ok {
				return // log file closed
			}
			_, err = w.Write([]byte(str))
			if err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// ReadWithTail read log file wilt tail. If tail is -1, read all.
func (lf *logFile) ReadWithTail(w http.ResponseWriter) error {
	var index int64
	var err error
	end := lf.EndIndex
	// tail -1 means read all, find index to start reading
	if lf.Tail == -1 {
		index = 0
	} else {
		index, err = lf.findLastNLineIndex()
		if err != nil {
			return err
		}
	}

	// reading from index to end
	for index < end {
		data := make([]byte, 1024)
		length, err := lf.File.ReadAt(data, index)
		index += int64(length)
		if index > end {
			length = int(end - index + int64(length))
		}
		_, err2 := w.Write(data[:length])
		if err2 != nil {
			return err2
		}
		if err != nil {
			if err != io.EOF {
				return err
			}
			return nil
		}
	}
	return nil
}

func parseLogQueries(r *http.Request) (int, bool, error) {
	tailValue := r.URL.Query().Get("tail")
	if tailValue == "" {
		tailValue = "500"
	}
	tail, err := strconv.Atoi(tailValue)
	if err != nil {
		return 0, false, fmt.Errorf("invalid tail %s, %v", tailValue, err)
	}
	if tail < -1 {
		return 0, false, fmt.Errorf("invalid tail %d, tail should not less than -1", tail)
	}

	followValue := r.URL.Query().Get("follow")
	follow, err := strconv.ParseBool(followValue)
	if err != nil {
		return 0, false, fmt.Errorf("invalid follow %s, %v", followValue, err)
	}
	return tail, follow, nil
}

// Watch watch the log file from the end index.
// It returns watchCh to receive the new log and closeFn to close the watcher.
func (lf *logFile) Watch() (<-chan string, func(), error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, nil, err
	}

	watchCh := make(chan string)
	closeCh := make(chan struct{})
	go func(lf *logFile) {
		index := lf.EndIndex
		defer func() {
			logger.Infof("close watcher for %s", lf.Path)
			close(watchCh)
		}()

		for {
			select {
			case <-closeCh:
				// caller close the watcher, return
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) {
					for {
						data := make([]byte, 1024)
						length, err := lf.File.ReadAt(data, index)
						if err != nil {
							if err != io.EOF {
								return
							}
						}
						index += int64(length)
						watchCh <- string(data[:length])
						if err == io.EOF || length < 1024 {
							break
						}
					}
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}(lf)

	err = watcher.Add(lf.Path)
	if err != nil {
		close(closeCh)
		return nil, nil, err
	}
	closeFn := func() {
		watcher.Close()
		close(closeCh)
	}
	return watchCh, closeFn, nil
}

// findLastNLineIndex find the index of the last n line
// from EndIndex of file. It return 0 if the file has
// less than n lines.
func (lf *logFile) findLastNLineIndex() (int64, error) {
	buf := make([]byte, 1)
	lineCount := 0
	end := lf.EndIndex
	n := lf.Tail

	for end > 0 && lineCount < n {
		end--
		_, err := lf.File.ReadAt(buf, end)
		if err != nil {
			return 0, err
		}

		if buf[0] == '\n' && end != lf.EndIndex-1 {
			lineCount++
			if lineCount == n {
				end++
				break
			}
		}
	}
	if lineCount < n {
		return 0, nil
	}
	return end, nil
}
