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

package logger

import (
	"bytes"
	"fmt"
	"time"
)

const (
	logChanSize = 10240

	cacheTimeout = 2 * time.Second
)

type (
	// LogFile add features upon the regular file:
	// 1. Reopen the file after receiving SIGHUP, for log rotate.
	// 2. Reduce execution time of callers by asynchronous log(return after only memory copy).
	// 3. Batch write logs by cache them with timeout.
	LogFile struct {
		logger *Logger

		logChan   chan []byte
		eventChan chan interface{}

		cacheCount    uint32
		maxCacheCount uint32
		cache         *bytes.Buffer
	}

	syncEvent struct {
		resultChan chan error
	}
	closeEvent struct{ done chan struct{} }
)

// NewLogFile can not open /dev/stderr, it will cause dead lock.
func NewLogFile(spec *Spec, maxCacheCount uint32) (*LogFile, error) {
	lf := &LogFile{
		logChan:       make(chan []byte, logChanSize),
		eventChan:     make(chan interface{}),
		maxCacheCount: maxCacheCount,
		cache:         bytes.NewBuffer(nil),
		logger:        newLogger(spec),
	}

	go lf.run()

	return lf, nil
}

func (lf *LogFile) run() {
	ticker := time.NewTimer(cacheTimeout)
	defer ticker.Stop()
	for {
		select {
		case p := <-lf.logChan:
			lf.writeLog(p)
		case event := <-lf.eventChan:
			switch event.(type) {
			case *syncEvent:
				err := lf.flush()
				if err != nil {
					event.(*syncEvent).resultChan <- err
				} else {
					event.(*syncEvent).resultChan <- lf.logger.file.Sync()
				}
			case *closeEvent:
				close(event.(*closeEvent).done)
				return
			}
		case <-ticker.C:
			lf.flush()
		}
	}
}

// Write writes log asynchronously, it always returns successful result.
func (lf *LogFile) Write(p []byte) (int, error) {
	// NOTE: The memory of p may be corrupted after Write returned
	// So it's necessary to do copy.
	buff := make([]byte, len(p))
	copy(buff, p)
	lf.logChan <- buff
	return len(p), nil
}

// Sync flushes all cache to file with os-level flush.
func (lf *LogFile) Sync() error {
	event := &syncEvent{
		resultChan: make(chan error, 1),
	}
	lf.eventChan <- event

	return <-event.resultChan
}

func (lf *LogFile) writeLog(p []byte) {
	// No need to copy twice for non-cacheable log file.
	if lf.maxCacheCount == 0 {
		_, err := lf.logger.Write(p)
		if err != nil {
			stderrLogger.Errorf("%v", err)
		}
		return
	}

	n, err := lf.cache.Write(p)
	if err != nil || len(p) != n {
		stderrLogger.Errorf("write %s to cache failed: %v", p, err)
	}
	lf.cacheCount++

	if lf.cacheCount < lf.maxCacheCount {
		return
	}

	err = lf.flush()
	if err != nil {
		stderrLogger.Errorf("%v", err)
	}
}

// flush flushes all cache to file without os-level flush.
func (lf *LogFile) flush() error {
	if lf.cache.Len() == 0 {
		return nil
	}

	// NOTE: Discard all buffer regardless of it succeed or failed.
	defer func() {
		lf.cache.Reset()
		lf.cacheCount = 0
	}()

	n, err := lf.logger.Write(lf.cache.Bytes())
	if err != nil || n != lf.cache.Len() {
		return fmt.Errorf("write buffer to %s failed: %d, %v", lf.logger.filename(), n, err)
	}

	return nil
}

func (lf *LogFile) Close() {
	lf.Sync()

	closed := &closeEvent{done: make(chan struct{})}
	lf.eventChan <- closed
	<-closed.done
	close(lf.eventChan)

	lf.logger.Close()
}
