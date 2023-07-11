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
	"compress/gzip"
	"errors"
	"fmt"
	"github.com/megaease/easegress/pkg/util/stringtool"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	backupTimeFormat = "2006-01-02T15-04-05.000"
	compressSuffix   = ".gz"
	defaultMaxSize   = 100
	defaultFileMode  = os.FileMode(0o644)
)

// ensure we always implement io.WriteCloser
var _ io.WriteCloser = (*Logger)(nil)

type (
	// Logger is base on lumberjack.Logger v2.2.1 with minor modifications.
	Logger struct {
		spec       *Spec
		perm       os.FileMode
		closeEvent chan struct{}

		size int64
		file *os.File
		mu   sync.Mutex

		millCh    chan bool
		startMill sync.Once
	}

	Spec struct {
		// FileName is the file to write logs to.  Backup log files will be retained
		// in the same directory.  It uses <processname>-lumberjack.log in
		// os.TempDir() if empty.
		FileName string `json:"fileName" jsonschema:"required"`

		// MaxSize is the maximum size in megabytes of the log file before it gets
		// rotated. It defaults to 100 megabytes.
		MaxSize int `json:"maxSize" jsonschema:"omitempty"`

		// MaxAge is the maximum number of days to retain old log files based on the
		// timestamp encoded in their filename.  Note that a day is defined as 24
		// hours and may not exactly correspond to calendar days due to daylight
		// savings, leap seconds, etc. The default is not to remove old log files
		// based on age.
		MaxAge int `json:"maxAge" jsonschema:"omitempty"`

		// MaxBackups is the maximum number of old log files to retain.  The default
		// is to retain all old log files (though MaxAge may still cause them to get
		// deleted.)
		MaxBackups int `json:"maxBackups" jsonschema:"omitempty"`

		// LocalTime determines if the time used for formatting the timestamps in
		// backup files is the computer's local time.  The default is to use UTC
		// time.
		LocalTime bool `json:"localTime" jsonschema:"omitempty"`

		// Compress determines if the rotated log files should be compressed
		// using gzip. The default is not to perform compression.
		Compress bool `json:"compress" jsonschema:"omitempty"`

		// File represents a file's mode and permission bits. default 0644/0o644
		Perm string `json:"perm" jsonschema:"omitempty"`
	}
)

var (
	// currentTime exists so it can be mocked out by tests.
	currentTime = time.Now

	// os_Stat exists so it can be mocked out by tests.
	osStat = os.Stat

	// megabyte is the conversion factor between MaxSize and bytes.  It is a
	// variable so tests can mock it out and not need to write megabytes of data
	// to disk.
	megabyte = 1024 * 1024
)

func newLogger(spec *Spec) *Logger {
	if err := spec.Validate(); err != nil {
		panic(err)
	}
	perm := defaultFileMode
	if spec.Perm != "" {
		parse, _ := strconv.ParseUint(spec.Perm, 8, 32)
		perm = os.FileMode(parse)
	}

	logger := &Logger{
		spec:       spec,
		perm:       perm,
		closeEvent: make(chan struct{}),
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGHUP)
	go func() {
		for {
			select {
			case <-signalChan:
				stderrLogger.Infof("notify SIGHUP signal and rotate %s file", logger.filename())
				err := logger.Rotate()
				if err != nil {
					stderrLogger.Warnf("log file rotate fail %s", err)
				}
			case <-logger.closeEvent:
				return
			}
		}
	}()
	return logger
}

func (s *Spec) Validate() error {
	if s.Perm != "" {
		// Look for octal
		if s.Perm[0] != '0' || (len(s.Perm) >= 3 && (stringtool.LowerChar(s.Perm[1]) == 'b' || stringtool.LowerChar(s.Perm[1]) == 'x')) {
			return fmt.Errorf("file mode %s invalid, it must be in octal", s.Perm)
		}
	}
	return nil
}

// Write implements io.Writer.  If a write would cause the log file to be larger
// than MaxSize, the file is closed, renamed to include a timestamp of the
// current time, and a new log file is created using the original log file name.
// If the length of the write is greater than MaxSize, an error is returned.
func (l *Logger) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	writeLen := int64(len(p))
	if writeLen > l.max() {
		return 0, fmt.Errorf(
			"write length %d exceeds maximum file size %d", writeLen, l.max(),
		)
	}

	if l.file == nil {
		if err = l.openExistingOrNew(len(p)); err != nil {
			return 0, err
		}
	}

	if l.size+writeLen > l.max() {
		if err := l.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = l.file.Write(p)
	l.size += int64(n)

	return n, err
}

// Close implements io.Closer, and closes the current logfile.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	close(l.closeEvent)
	return l.close()
}

// close closes the file if it is open.
func (l *Logger) close() error {
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}

// Rotate causes Logger to close the existing log file and immediately create a
// new one.  This is a helper function for applications that want to initiate
// rotations outside of the normal rotation rules, such as in response to
// SIGHUP.  After rotating, this initiates compression and removal of old log
// files according to the configuration.
func (l *Logger) Rotate() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.rotate()
}

// rotate closes the current file, moves it aside with a timestamp in the name,
// (if it exists), opens a new file with the original filename, and then runs
// post-rotation processing and removal.
func (l *Logger) rotate() error {
	if err := l.close(); err != nil {
		return err
	}
	if err := l.openNew(); err != nil {
		return err
	}
	l.mill()
	return nil
}

// openNew opens a new log file for writing, moving any old log file out of the
// way.  This methods assumes the file has already been closed.
func (l *Logger) openNew() error {
	err := os.MkdirAll(l.dir(), 0755)
	if err != nil {
		return fmt.Errorf("can't make directories for new logfile: %s", err)
	}

	name := l.filename()
	mode := os.FileMode(l.perm)
	_, err = osStat(name)
	if err == nil {
		// move the existing file
		newName := backupName(name, l.spec.LocalTime)
		if err := os.Rename(name, newName); err != nil {
			return fmt.Errorf("can't rename log file: %s", err)
		}
	}

	// we use truncate here because this should only get called when we've moved
	// the file ourselves. if someone else creates the file in the meantime,
	// just wipe out the contents.
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("can't open new logfile: %s", err)
	}
	l.file = f
	l.size = 0
	return nil
}

// backupName creates a new filename from the given name, inserting a timestamp
// between the filename and the extension, using the local time if requested
// (otherwise UTC).
func backupName(name string, local bool) string {
	dir := filepath.Dir(name)
	filename := filepath.Base(name)
	ext := filepath.Ext(filename)
	prefix := filename[:len(filename)-len(ext)]
	t := currentTime()
	if !local {
		t = t.UTC()
	}

	timestamp := t.Format(backupTimeFormat)
	return filepath.Join(dir, fmt.Sprintf("%s-%s%s", prefix, timestamp, ext))
}

// openExistingOrNew opens the logfile if it exists and if the current write
// would not put it over MaxSize.  If there is no such file or the write would
// put it over the MaxSize, a new file is created.
func (l *Logger) openExistingOrNew(writeLen int) error {
	l.mill()

	filename := l.filename()
	info, err := osStat(filename)
	if os.IsNotExist(err) {
		return l.openNew()
	}
	if err != nil {
		return fmt.Errorf("error getting log file info: %s", err)
	}

	if info.Size()+int64(writeLen) >= l.max() {
		return l.rotate()
	}

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		// if we fail to open the old log file for some reason, just ignore
		// it and open a new log file.
		return l.openNew()
	}
	l.file = file
	l.size = info.Size()
	return nil
}

// filename generates the name of the logfile from the current time.
func (l *Logger) filename() string {
	if l.spec.FileName != "" {
		return l.spec.FileName
	}
	name := filepath.Base(os.Args[0]) + "-lumberjack.log"
	return filepath.Join(os.TempDir(), name)
}

// millRunOnce performs compression and removal of stale log files.
// Log files are compressed if enabled via configuration and old log
// files are removed, keeping at most l.MaxBackups files, as long as
// none of them are older than MaxAge.
func (l *Logger) millRunOnce() error {
	if l.spec.MaxBackups == 0 && l.spec.MaxAge == 0 && !l.spec.Compress {
		return nil
	}

	files, err := l.oldLogFiles()
	if err != nil {
		return err
	}

	var compress, remove []logInfo

	if l.spec.MaxBackups > 0 && l.spec.MaxBackups < len(files) {
		preserved := make(map[string]bool)
		var remaining []logInfo
		for _, f := range files {
			// Only count the uncompressed log file or the
			// compressed log file, not both.
			fn := f.Name()
			if strings.HasSuffix(fn, compressSuffix) {
				fn = fn[:len(fn)-len(compressSuffix)]
			}
			preserved[fn] = true

			if len(preserved) > l.spec.MaxBackups {
				remove = append(remove, f)
			} else {
				remaining = append(remaining, f)
			}
		}
		files = remaining
	}
	if l.spec.MaxAge > 0 {
		diff := time.Duration(int64(24*time.Hour) * int64(l.spec.MaxAge))
		cutoff := currentTime().Add(-1 * diff)

		var remaining []logInfo
		for _, f := range files {
			if f.timestamp.Before(cutoff) {
				remove = append(remove, f)
			} else {
				remaining = append(remaining, f)
			}
		}
		files = remaining
	}

	if l.spec.Compress {
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), compressSuffix) {
				compress = append(compress, f)
			}
		}
	}

	for _, f := range remove {
		errRemove := os.Remove(filepath.Join(l.dir(), f.Name()))
		if err == nil && errRemove != nil {
			err = errRemove
		}
	}
	for _, f := range compress {
		fn := filepath.Join(l.dir(), f.Name())
		errCompress := compressLogFile(fn, fn+compressSuffix, l.perm)
		if err == nil && errCompress != nil {
			err = errCompress
		}
	}

	return err
}

// millRun runs in a goroutine to manage post-rotation compression and removal
// of old log files.
func (l *Logger) millRun() {
	for range l.millCh {
		// what am I going to do, log this?
		_ = l.millRunOnce()
	}
}

// mill performs post-rotation compression and removal of stale log files,
// starting the mill goroutine if necessary.
func (l *Logger) mill() {
	l.startMill.Do(func() {
		l.millCh = make(chan bool, 1)
		go l.millRun()
	})
	select {
	case l.millCh <- true:
	default:
	}
}

// oldLogFiles returns the list of backup log files stored in the same
// directory as the current log file, sorted by ModTime
func (l *Logger) oldLogFiles() ([]logInfo, error) {
	files, err := ioutil.ReadDir(l.dir())
	if err != nil {
		return nil, fmt.Errorf("can't read log file directory: %s", err)
	}
	logFiles := []logInfo{}

	prefix, ext := l.prefixAndExt()

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if t, err := l.timeFromName(f.Name(), prefix, ext); err == nil {
			logFiles = append(logFiles, logInfo{t, f})
			continue
		}
		if t, err := l.timeFromName(f.Name(), prefix, ext+compressSuffix); err == nil {
			logFiles = append(logFiles, logInfo{t, f})
			continue
		}
		// error parsing means that the suffix at the end was not generated
		// by lumberjack, and therefore it's not a backup file.
	}

	sort.Sort(byFormatTime(logFiles))

	return logFiles, nil
}

// timeFromName extracts the formatted time from the filename by stripping off
// the filename's prefix and extension. This prevents someone's filename from
// confusing time.parse.
func (l *Logger) timeFromName(filename, prefix, ext string) (time.Time, error) {
	if !strings.HasPrefix(filename, prefix) {
		return time.Time{}, errors.New("mismatched prefix")
	}
	if !strings.HasSuffix(filename, ext) {
		return time.Time{}, errors.New("mismatched extension")
	}
	ts := filename[len(prefix) : len(filename)-len(ext)]
	return time.Parse(backupTimeFormat, ts)
}

// max returns the maximum size in bytes of log files before rolling.
func (l *Logger) max() int64 {
	if l.spec.MaxSize == 0 {
		return int64(defaultMaxSize * megabyte)
	}
	return int64(l.spec.MaxSize) * int64(megabyte)
}

// dir returns the directory for the current filename.
func (l *Logger) dir() string {
	return filepath.Dir(l.filename())
}

// prefixAndExt returns the filename part and extension part from the Logger's
// filename.
func (l *Logger) prefixAndExt() (prefix, ext string) {
	filename := filepath.Base(l.filename())
	ext = filepath.Ext(filename)
	prefix = filename[:len(filename)-len(ext)] + "-"
	return prefix, ext
}

// compressLogFile compresses the given log file, removing the
// uncompressed log file if successful.
func compressLogFile(src, dst string, perm os.FileMode) (err error) {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}
	defer f.Close()

	// If this file already exists, we presume it was created by
	// a previous attempt to compress the log file.
	gzf, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return fmt.Errorf("failed to open compressed log file: %v", err)
	}
	defer gzf.Close()

	gz := gzip.NewWriter(gzf)

	defer func() {
		if err != nil {
			os.Remove(dst)
			err = fmt.Errorf("failed to compress log file: %v", err)
		}
	}()

	if _, err := io.Copy(gz, f); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	if err := gzf.Close(); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Remove(src); err != nil {
		return err
	}

	return nil
}

// logInfo is a convenience struct to return the filename and its embedded
// timestamp.
type logInfo struct {
	timestamp time.Time
	os.FileInfo
}

// byFormatTime sorts by newest time formatted in the name.
type byFormatTime []logInfo

func (b byFormatTime) Less(i, j int) bool {
	return b[i].timestamp.After(b[j].timestamp)
}

func (b byFormatTime) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byFormatTime) Len() int {
	return len(b)
}
