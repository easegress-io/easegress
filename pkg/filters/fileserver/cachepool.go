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

package fileserver

import (
	"container/list"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/exp/mmap"
)

type (
	FileEntry struct {
		data     []byte
		size     int64
		expireAt time.Time
	}

	cacheItem struct {
		key   string
		entry *FileEntry
	}

	mmapReadSeeker struct {
		r    *mmap.ReaderAt
		size int64
		pos  int64
	}

	BufferPool struct {
		mu          sync.Mutex
		cache       map[string]*list.Element
		ll          *list.List
		currentSize int64
		maxSize     int64
		maxFileSize int64
		ttl         time.Duration
	}
)

func NewBufferPool(maxSize, maxFileSize int64, ttl time.Duration) *BufferPool {
	bp := &BufferPool{
		cache:       make(map[string]*list.Element),
		ll:          list.New(),
		currentSize: 0,
		maxSize:     maxSize,
		maxFileSize: maxFileSize,
		ttl:         ttl,
	}
	go bp.cleanup()
	return bp
}

func (bp *BufferPool) cleanup() {
	if bp.ttl <= 0 {
		return
	}
	ticker := time.NewTicker(bp.ttl / 2)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		bp.mu.Lock()
		for e := bp.ll.Back(); e != nil; {
			prev := e.Prev()
			item := e.Value.(*cacheItem)
			if now.After(item.entry.expireAt) {
				bp.removeElement(e)
			}
			e = prev
		}
		bp.mu.Unlock()
	}
}

func (bp *BufferPool) removeElement(e *list.Element) {
	if e == nil {
		return
	}
	item := e.Value.(*cacheItem)
	delete(bp.cache, item.key)
	bp.currentSize -= item.entry.size
	bp.ll.Remove(e)
}

func (bp *BufferPool) removeOldest() {
	e := bp.ll.Back()
	if e != nil {
		bp.removeElement(e)
	}
}

func (bp *BufferPool) GetFile(path string) (data []byte, cached bool, err error) {
	bp.mu.Lock()
	if ele, ok := bp.cache[path]; ok {
		item := ele.Value.(*cacheItem)
		if time.Now().Before(item.entry.expireAt) {
			bp.ll.MoveToFront(ele)
			d := item.entry.data
			bp.mu.Unlock()
			return d, true, nil
		}
		bp.removeElement(ele)
	}
	bp.mu.Unlock()

	info, err := os.Stat(path)
	if err != nil {
		return nil, false, err
	}

	if info.Size() > bp.maxFileSize {
		return nil, false, nil
	}

	bs, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	entry := &FileEntry{
		data:     bs,
		size:     info.Size(),
		expireAt: time.Now().Add(bp.ttl),
	}

	bp.mu.Lock()
	defer bp.mu.Unlock()

	if ele, ok := bp.cache[path]; ok {
		item := ele.Value.(*cacheItem)
		if time.Now().Before(item.entry.expireAt) {
			bp.ll.MoveToFront(ele)
			return item.entry.data, true, nil
		}
		bp.removeElement(ele)
	}

	for bp.currentSize+entry.size > bp.maxSize {
		if bp.ll.Len() == 0 {
			break
		}
		bp.removeOldest()
	}

	if entry.size > bp.maxSize {
		return bs, false, nil
	}

	ele := bp.ll.PushFront(&cacheItem{key: path, entry: entry})
	bp.cache[path] = ele
	bp.currentSize += entry.size

	return entry.data, true, nil
}

func (m *mmapReadSeeker) Read(p []byte) (int, error) {
	if m.pos >= m.size {
		return 0, io.EOF
	}
	n, err := m.r.ReadAt(p, m.pos)
	m.pos += int64(n)
	return n, err
}

func (m *mmapReadSeeker) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0:
		m.pos = offset
	case 1:
		m.pos += offset
	case 2:
		m.pos = m.size + offset
	}
	if m.pos < 0 {
		m.pos = 0
	}
	if m.pos > m.size {
		m.pos = m.size
	}
	return m.pos, nil
}

func fileHandler(w http.ResponseWriter, r *http.Request, pool *BufferPool) {
	path := filepath.Clean("." + r.URL.Path)

	data, _, err := pool.GetFile(path)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	if data != nil {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		_, _ = w.Write(data)
		return
	}

	rdr, err := mmap.Open(path)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	defer rdr.Close()

	info, err := os.Stat(path)
	if err != nil {
		http.Error(w, "stat error", http.StatusInternalServerError)
		return
	}
	rs := &mmapReadSeeker{r: rdr, size: info.Size(), pos: 0}
	http.ServeContent(w, r, info.Name(), info.ModTime(), rs)
}
