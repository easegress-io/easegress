/*
 * Copyright (c) 2017, MegaEase
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

	"github.com/fsnotify/fsnotify"
	"github.com/megaease/easegress/v2/pkg/logger"
)

func (s *Server) logsAPIEntries() []*Entry {
	return []*Entry{
		{
			Path:    "/logs",
			Method:  "GET",
			Handler: s.getLogs,
		},
	}
}

func (s *Server) getLogs(w http.ResponseWriter, r *http.Request) {
	flusher := w.(http.Flusher)
	var err error
	defer func() {
		if err != nil {
			HandleAPIError(w, r, http.StatusInternalServerError, err)
		}
	}()

	tail, follow, err := parseLogQueries(r)
	if err != nil {
		return
	}
	logPath := logger.GetLogPath()
	if logPath == "" {
		err = errors.New("log path not found")
		return
	}
	file, err := os.Open(logPath)
	if err != nil {
		return
	}
	defer file.Close()

	end, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return
	}

	err = readLogFileWithTail(w, file, tail, end)
	if err != nil {
		return
	}
	flusher.Flush()

	// watch log file from the end
	if !follow {
		return
	}
	writeCh, closeFn, err := watchLogFile(logPath, end)
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

func readLogFileWithTail(w http.ResponseWriter, file *os.File, tail int, end int64) error {
	var index int64
	var err error
	if tail == -1 {
		index = 0
	} else {
		index, err = findLastNLineIndex(file, tail, end)
		if err != nil {
			return err
		}
	}
	for index < end {
		data := make([]byte, 1024)
		length, err := file.ReadAt(data, index)
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

func watchLogFile(filePath string, offset int64) (<-chan string, func(), error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, nil, err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, err
	}

	watchCh := make(chan string)
	closeCh := make(chan struct{})
	go func() {
		index := offset
		defer func() {
			logger.Infof("close watcher for %s", filePath)
			close(watchCh)
		}()

		for {
			select {
			case <-closeCh:
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) {
					for {
						data := make([]byte, 1024)
						length, err := file.ReadAt(data, index)
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
	}()

	err = watcher.Add(filePath)
	if err != nil {
		return nil, nil, err
	}
	return watchCh, func() { close(closeCh) }, nil
}

// findLastNLineIndex returns the byte position (index) of the start of the last n lines
// in the file. If the file has fewer than n lines, it returns 0.
func findLastNLineIndex(file *os.File, n int, end int64) (int64, error) {
	buf := make([]byte, 1)

	lineCount := 0
	end--
	for end > 0 && lineCount < n {
		end--
		_, err := file.ReadAt(buf, end)
		if err != nil {
			return 0, err
		}

		// Count the newline characters
		if buf[0] == '\n' {
			lineCount++
			if lineCount == n {
				// Move past the newline character to start of the line
				end++
				break
			}
		}
	}
	// If the file has fewer than n lines, return 0
	if lineCount < n {
		return 0, nil
	}
	return end, nil
}
