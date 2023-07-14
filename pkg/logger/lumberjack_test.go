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
	"compress/gzip"
	"fmt"
	"github.com/megaease/easegress/pkg/util/codectool"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// !!!NOTE!!!
//
// Running these tests in parallel will almost certainly cause sporadic (or even
// regular) failures, because they're all messing with the same global variable
// that controls the logic's mocked time.Now.  So... don't do that.

// Since all the tests uses the time to determine filenames etc, we need to
// control the wall clock as much as possible, which means having a wall clock
// that doesn't change unless we want it to.
var fakeCurrentTime = time.Now()
var lock = &sync.Mutex{}

func fakeTime() time.Time {
	lock.Lock()
	defer lock.Unlock()
	return fakeCurrentTime
}

func TestSpecValidate(t *testing.T) {
	data := `
fileName: foo
maxSize: 5
maxAge: 10
maxBackups: 3
localTime: true
compress: true
`
	spec := &Spec{}
	codectool.MustUnmarshal([]byte(data), spec)
	at := assert.New(t)
	at.NoError(spec.Validate())

	data = `
fileName: foo
maxSize: 5
maxAge: 10
maxBackups: 3
localTime: true
compress: true
perm: '0600'
`
	spec = &Spec{}
	codectool.MustUnmarshal([]byte(data), spec)
	at.NoError(spec.Validate())

	data = `
fileName: foo
maxSize: 5
maxAge: 10
maxBackups: 3
localTime: true
compress: true
perm: '0o600'
`
	spec = &Spec{}
	codectool.MustUnmarshal([]byte(data), spec)
	at.NoError(spec.Validate())

	data = `
fileName: foo
maxSize: 5
maxAge: 10
maxBackups: 3
localTime: true
compress: true
perm: '600'
`

	spec = &Spec{}
	codectool.MustUnmarshal([]byte(data), spec)
	at.Error(spec.Validate())

	data = `
fileName: foo
maxSize: 5
maxAge: 10
maxBackups: 3
localTime: true
compress: true
perm: '0x600'
`
	spec = &Spec{}
	codectool.MustUnmarshal([]byte(data), spec)
	at.Error(spec.Validate())
}

func TestNewFile(t *testing.T) {
	currentTime = fakeTime

	dir := makeTempDir("TestNewFile", t)
	defer os.RemoveAll(dir)
	l := newLogger(&Spec{
		FileName: logFilePath(dir),
	})
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)

	at := assert.New(t)
	at.NoError(err)
	at.Equal(len(b), n)
	existsWithContent(logFilePath(dir), b, t)
	fileCount(dir, 1, t)
}

func TestOpenExisting(t *testing.T) {
	currentTime = fakeTime
	dir := makeTempDir("TestOpenExisting", t)
	defer os.RemoveAll(dir)

	filename := logFilePath(dir)
	data := []byte("foo!")
	err := os.WriteFile(filename, data, 0644)
	at := assert.New(t)
	at.NoError(err)
	existsWithContent(filename, data, t)

	l := newLogger(&Spec{
		FileName: logFilePath(dir),
	})
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	at.NoError(err)
	at.Equal(len(b), n)

	// make sure the file got appended
	existsWithContent(filename, append(data, b...), t)

	// make sure no other files were created
	fileCount(dir, 1, t)
}

func TestWriteTooLong(t *testing.T) {
	currentTime = fakeTime
	megabyte = 1
	dir := makeTempDir("TestWriteTooLong", t)
	defer os.RemoveAll(dir)
	l := newLogger(&Spec{
		FileName: logFilePath(dir),
		MaxSize:  5,
	})
	defer l.Close()
	b := []byte("booooooooooooooo!")
	n, err := l.Write(b)

	at := assert.New(t)
	at.Error(err)
	at.Equal(0, n)
	at.Equal(err.Error(),
		fmt.Sprintf("write length %d exceeds maximum file size %d", len(b), l.spec.MaxSize))
	_, err = os.Stat(logFilePath(dir))
	at.True(os.IsNotExist(err), "File exists, but should not have been created")
}

func TestMakeLogDir(t *testing.T) {
	currentTime = fakeTime
	dir := time.Now().Format("TestMakeLogDir" + backupTimeFormat)
	dir = filepath.Join(os.TempDir(), dir)
	defer os.RemoveAll(dir)
	l := newLogger(&Spec{
		FileName: logFilePath(dir),
	})
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	at := assert.New(t)
	at.NoError(err)
	at.Equal(len(b), n)
	existsWithContent(logFilePath(dir), b, t)
	fileCount(dir, 1, t)
}

func TestDefaultFilename(t *testing.T) {
	currentTime = fakeTime
	dir := os.TempDir()
	filename := filepath.Join(dir, filepath.Base(os.Args[0])+"-lumberjack.log")
	defer os.Remove(filename)
	l := newLogger(&Spec{})
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	at := assert.New(t)
	at.NoError(err)
	at.Equal(len(b), n)
	existsWithContent(filename, b, t)
}

func TestAutoRotate(t *testing.T) {
	currentTime = fakeTime
	megabyte = 1

	dir := makeTempDir("TestAutoRotate", t)
	defer os.RemoveAll(dir)

	filename := logFilePath(dir)
	l := newLogger(&Spec{
		FileName: logFilePath(dir),
		MaxSize:  10,
	})
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)

	at := assert.New(t)
	at.NoError(err)
	at.Equal(len(b), n)

	existsWithContent(filename, b, t)
	fileCount(dir, 1, t)

	newFakeTime()

	b2 := []byte("foooooo!")
	n, err = l.Write(b2)
	at.NoError(err)
	at.Equal(len(b2), n)

	// the old logfile should be moved aside and the main logfile should have
	// only the last write in it.
	existsWithContent(filename, b2, t)

	// the backup file will use the current fake time and have the old contents.
	existsWithContent(backupFile(dir), b, t)

	fileCount(dir, 2, t)
}

func TestFirstWriteRotate(t *testing.T) {
	currentTime = fakeTime
	megabyte = 1
	dir := makeTempDir("TestFirstWriteRotate", t)
	defer os.RemoveAll(dir)

	filename := logFilePath(dir)
	l := newLogger(&Spec{
		FileName: logFilePath(dir),
		MaxSize:  10,
	})
	defer l.Close()

	start := []byte("boooooo!")
	err := os.WriteFile(filename, start, 0600)
	at := assert.New(t)

	at.NoError(err)

	newFakeTime()

	// this would make us rotate
	b := []byte("fooo!")
	n, err := l.Write(b)
	at.NoError(err)
	at.Equal(len(b), n)

	existsWithContent(filename, b, t)
	existsWithContent(backupFile(dir), start, t)

	fileCount(dir, 2, t)
}

func TestMaxBackups(t *testing.T) {
	currentTime = fakeTime
	megabyte = 1
	dir := makeTempDir("TestMaxBackups", t)
	defer os.RemoveAll(dir)

	filename := logFilePath(dir)
	l := newLogger(&Spec{
		FileName:   logFilePath(dir),
		MaxSize:    10,
		MaxBackups: 1,
	})
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	at := assert.New(t)
	at.NoError(err, t)
	at.Equal(len(b), n)

	existsWithContent(filename, b, t)
	fileCount(dir, 1, t)

	newFakeTime()

	// this will put us over the max
	b2 := []byte("foooooo!")
	n, err = l.Write(b2)
	at.NoError(err)
	at.Equal(len(b2), n)

	// this will use the new fake time
	secondFilename := backupFile(dir)
	existsWithContent(secondFilename, b, t)

	// make sure the old file still exists with the same content.
	existsWithContent(filename, b2, t)

	fileCount(dir, 2, t)

	newFakeTime()

	// this will make us rotate again
	b3 := []byte("baaaaaar!")
	n, err = l.Write(b3)
	at.NoError(err)
	at.Equal(len(b3), n)

	// this will use the new fake time
	thirdFilename := backupFile(dir)
	existsWithContent(thirdFilename, b2, t)

	existsWithContent(filename, b3, t)

	// we need to wait a little bit since the files get deleted on a different
	// goroutine.
	<-time.After(time.Millisecond * 10)

	// should only have two files in the dir still
	fileCount(dir, 2, t)

	// second file name should still exist
	existsWithContent(thirdFilename, b2, t)

	// should have deleted the first backup
	notExist(secondFilename, t)

	// now test that we don't delete directories or non-logfile files

	newFakeTime()

	// create a file that is close to but different from the logfile name.
	// It shouldn't get caught by our deletion filters.
	notlogfile := logFilePath(dir) + ".foo"
	err = os.WriteFile(notlogfile, []byte("data"), 0644)
	at.NoError(err)

	// Make a directory that exactly matches our log file filters... it still
	// shouldn't get caught by the deletion filter since it's a directory.
	notlogfiledir := backupFile(dir)
	err = os.Mkdir(notlogfiledir, 0700)
	at.NoError(err)

	newFakeTime()

	// this will use the new fake time
	fourthFilename := backupFile(dir)

	// Create a log file that is/was being compressed - this should
	// not be counted since both the compressed and the uncompressed
	// log files still exist.
	compLogFile := fourthFilename + compressSuffix
	err = os.WriteFile(compLogFile, []byte("compress"), 0644)
	at.NoError(err)

	// this will make us rotate again
	b4 := []byte("baaaaaaz!")
	n, err = l.Write(b4)
	at.NoError(err)
	at.Equal(len(b4), n)

	existsWithContent(fourthFilename, b3, t)
	existsWithContent(fourthFilename+compressSuffix, []byte("compress"), t)

	// we need to wait a little bit since the files get deleted on a different
	// goroutine.
	<-time.After(time.Millisecond * 10)

	// We should have four things in the directory now - the 2 log files, the
	// not log file, and the directory
	fileCount(dir, 5, t)

	// third file name should still exist
	existsWithContent(filename, b4, t)

	existsWithContent(fourthFilename, b3, t)

	// should have deleted the first filename
	notExist(thirdFilename, t)

	// the not-a-logfile should still exist
	exists(notlogfile, t)

	// the directory
	exists(notlogfiledir, t)
}

func TestCleanupExistingBackups(t *testing.T) {
	// test that if we start with more backup files than we're supposed to have
	// in total, that extra ones get cleaned up when we rotate.

	currentTime = fakeTime
	megabyte = 1

	dir := makeTempDir("TestCleanupExistingBackups", t)
	defer os.RemoveAll(dir)

	// make 3 backup files

	data := []byte("data")
	backup := backupFile(dir)
	err := os.WriteFile(backup, data, 0644)

	at := assert.New(t)
	at.NoError(err)

	newFakeTime()

	backup = backupFile(dir)
	err = os.WriteFile(backup+compressSuffix, data, 0644)
	at.NoError(err)

	newFakeTime()

	backup = backupFile(dir)
	err = os.WriteFile(backup, data, 0644)
	at.NoError(err)

	// now create a primary log file with some data
	filename := logFilePath(dir)
	err = os.WriteFile(filename, data, 0644)
	at.NoError(err)

	l := newLogger(&Spec{
		FileName:   logFilePath(dir),
		MaxSize:    10,
		MaxBackups: 1,
	})
	defer l.Close()

	newFakeTime()

	b2 := []byte("foooooo!")
	n, err := l.Write(b2)
	at.NoError(err)
	at.Equal(len(b2), n)

	// we need to wait a little bit since the files get deleted on a different
	// goroutine.
	<-time.After(time.Millisecond * 10)

	// now we should only have 2 files left - the primary and one backup
	fileCount(dir, 2, t)
}

func TestMaxAge(t *testing.T) {
	currentTime = fakeTime
	megabyte = 1

	dir := makeTempDir("TestMaxAge", t)
	defer os.RemoveAll(dir)

	filename := logFilePath(dir)
	l := newLogger(&Spec{
		FileName: logFilePath(dir),
		MaxSize:  10,
		MaxAge:   1,
	})
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)

	at := assert.New(t)
	at.NoError(err)
	at.Equal(len(b), n)

	existsWithContent(filename, b, t)
	fileCount(dir, 1, t)

	// two days later
	newFakeTime()

	b2 := []byte("foooooo!")
	n, err = l.Write(b2)
	at.NoError(err)
	at.Equal(len(b2), n)
	existsWithContent(backupFile(dir), b, t)

	// we need to wait a little bit since the files get deleted on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	// We should still have 2 log files, since the most recent backup was just
	// created.
	fileCount(dir, 2, t)

	existsWithContent(filename, b2, t)

	// we should have deleted the old file due to being too old
	existsWithContent(backupFile(dir), b, t)

	// two days later
	newFakeTime()

	b3 := []byte("baaaaar!")
	n, err = l.Write(b3)
	at.NoError(err)
	at.Equal(len(b3), n)
	existsWithContent(backupFile(dir), b2, t)

	// we need to wait a little bit since the files get deleted on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	// We should have 2 log files - the main log file, and the most recent
	// backup.  The earlier backup is past the cutoff and should be gone.
	fileCount(dir, 2, t)

	existsWithContent(filename, b3, t)

	// we should have deleted the old file due to being too old
	existsWithContent(backupFile(dir), b2, t)
}

func TestOldLogFiles(t *testing.T) {
	currentTime = fakeTime
	megabyte = 1

	dir := makeTempDir("TestOldLogFiles", t)
	defer os.RemoveAll(dir)

	filename := logFilePath(dir)
	data := []byte("data")
	err := os.WriteFile(filename, data, 07)
	at := assert.New(t)
	at.NoError(err)

	// This gives us a time with the same precision as the time we get from the
	// timestamp in the name.
	t1, err := time.Parse(backupTimeFormat, fakeTime().UTC().Format(backupTimeFormat))
	at.NoError(err)

	backup := backupFile(dir)
	err = os.WriteFile(backup, data, 07)
	at.NoError(err)

	newFakeTime()

	t2, err := time.Parse(backupTimeFormat, fakeTime().UTC().Format(backupTimeFormat))
	at.NoError(err)

	backup2 := backupFile(dir)
	err = os.WriteFile(backup2, data, 07)
	at.NoError(err)

	l := newLogger(&Spec{
		FileName:   logFilePath(dir),
		MaxSize:    10,
		MaxBackups: 1,
	})
	files, err := l.oldLogFiles()
	at.NoError(err)
	at.Equal(2, len(files))

	// should be sorted by newest file first, which would be t2
	at.Equal(t2, files[0].timestamp)
	at.Equal(t1, files[1].timestamp)
}

func TestTimeFromName(t *testing.T) {
	l := newLogger(&Spec{
		FileName: "foo.log",
	})

	prefix, ext := l.prefixAndExt()

	tests := []struct {
		filename string
		want     time.Time
		wantErr  bool
	}{
		{"foo-2014-05-04T14-44-33.555.log", time.Date(2014, 5, 4, 14, 44, 33, 555000000, time.UTC), false},
		{"foo-2014-05-04T14-44-33.555", time.Time{}, true},
		{"2014-05-04T14-44-33.555.log", time.Time{}, true},
		{"foo.log", time.Time{}, true},
	}
	at := assert.New(t)
	for _, test := range tests {
		got, err := l.timeFromName(test.filename, prefix, ext)
		at.Equal(got, test.want)
		at.Equal(err != nil, test.wantErr)
	}
}

func TestLocalTime(t *testing.T) {
	currentTime = fakeTime
	megabyte = 1

	dir := makeTempDir("TestLocalTime", t)
	defer os.RemoveAll(dir)

	l := newLogger(&Spec{
		FileName:  logFilePath(dir),
		MaxSize:   10,
		LocalTime: true,
	})
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)

	at := assert.New(t)
	at.NoError(err)
	at.Equal(len(b), n)

	b2 := []byte("fooooooo!")
	n2, err := l.Write(b2)
	at.NoError(err)
	at.Equal(len(b2), n2)

	existsWithContent(logFilePath(dir), b2, t)
	existsWithContent(backupFileLocal(dir), b, t)
}

func TestRotate(t *testing.T) {
	currentTime = fakeTime
	dir := makeTempDir("TestRotate", t)
	defer os.RemoveAll(dir)

	filename := logFilePath(dir)

	l := newLogger(&Spec{
		FileName:   logFilePath(dir),
		MaxSize:    100,
		MaxBackups: 1,
	})
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)

	at := assert.New(t)
	at.NoError(err)
	at.Equal(len(b), n)

	existsWithContent(filename, b, t)
	fileCount(dir, 1, t)

	newFakeTime()

	err = l.Rotate()
	at.NoError(err)

	// we need to wait a little bit since the files get deleted on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	filename2 := backupFile(dir)
	existsWithContent(filename2, b, t)
	existsWithContent(filename, []byte{}, t)
	fileCount(dir, 2, t)
	newFakeTime()

	err = l.Rotate()
	at.NoError(err)

	// we need to wait a little bit since the files get deleted on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	filename3 := backupFile(dir)
	existsWithContent(filename3, []byte{}, t)
	existsWithContent(filename, []byte{}, t)
	fileCount(dir, 2, t)

	b2 := []byte("foooooo!")
	n, err = l.Write(b2)
	at.NoError(err)
	at.Equal(len(b2), n)

	// this will use the new fake time
	existsWithContent(filename, b2, t)
}

func TestCompressOnRotate(t *testing.T) {
	currentTime = fakeTime
	megabyte = 1

	dir := makeTempDir("TestCompressOnRotate", t)
	defer os.RemoveAll(dir)

	filename := logFilePath(dir)
	l := newLogger(&Spec{
		FileName: filename,
		MaxSize:  10,
		Compress: true,
	})
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)

	at := assert.New(t)
	at.NoError(err)
	at.Equal(len(b), n)

	existsWithContent(filename, b, t)
	fileCount(dir, 1, t)

	newFakeTime()

	err = l.Rotate()
	at.NoError(err)

	// the old logfile should be moved aside and the main logfile should have
	// nothing in it.
	existsWithContent(filename, []byte{}, t)

	// we need to wait a little bit since the files get compressed on a different
	// goroutine.
	<-time.After(300 * time.Millisecond)

	// a compressed version of the log file should now exist and the original
	// should have been removed.
	bc := new(bytes.Buffer)
	gz := gzip.NewWriter(bc)
	_, err = gz.Write(b)
	at.NoError(err)
	err = gz.Close()
	at.NoError(err)
	existsWithContent(backupFile(dir)+compressSuffix, bc.Bytes(), t)
	notExist(backupFile(dir), t)

	fileCount(dir, 2, t)
}

func TestCompressOnResume(t *testing.T) {
	currentTime = fakeTime
	megabyte = 1

	dir := makeTempDir("TestCompressOnResume", t)
	defer os.RemoveAll(dir)

	filename := logFilePath(dir)
	l := newLogger(&Spec{
		FileName: filename,
		MaxSize:  10,
		Compress: true,
	})
	defer l.Close()

	// Create a backup file and empty "compressed" file.
	filename2 := backupFile(dir)
	b := []byte("foo!")
	at := assert.New(t)
	err := os.WriteFile(filename2, b, 0644)
	at.NoError(err)
	err = os.WriteFile(filename2+compressSuffix, []byte{}, 0644)
	at.NoError(err)

	newFakeTime()

	b2 := []byte("boo!")
	n, err := l.Write(b2)
	at.NoError(err)
	at.Equal(len(b2), n)
	existsWithContent(filename, b2, t)

	// we need to wait a little bit since the files get compressed on a different
	// goroutine.
	<-time.After(300 * time.Millisecond)

	// The write should have started the compression - a compressed version of
	// the log file should now exist and the original should have been removed.
	bc := new(bytes.Buffer)
	gz := gzip.NewWriter(bc)
	_, err = gz.Write(b)
	at.NoError(err)
	err = gz.Close()
	at.NoError(err)
	existsWithContent(filename2+compressSuffix, bc.Bytes(), t)
	notExist(filename2, t)

	fileCount(dir, 2, t)
}

func TestJson(t *testing.T) {
	data := []byte(`
{
	"fileName": "foo",
	"maxSize": 5,
	"maxAge": 10,
	"maxBackups": 3,
	"localTime": true,
	"compress": true
}`[1:])
	spec := &Spec{}

	err := codectool.UnmarshalJSON(data, spec)
	at := assert.New(t)
	at.NoError(err)
	l := newLogger(spec)
	at.Equal("foo", l.spec.FileName)
	at.Equal(5, l.spec.MaxSize)
	at.Equal(10, l.spec.MaxAge)
	at.Equal(3, l.spec.MaxBackups)
	at.Equal(true, l.spec.LocalTime)
	at.Equal(true, l.spec.Compress)
}

// makeTempDir creates a file with a semi-unique name in the OS temp directory.
// It should be based on the name of the test, to keep parallel tests from
// colliding, and must be cleaned up after the test is finished.
func makeTempDir(name string, t testing.TB) string {
	dir := time.Now().Format(name + backupTimeFormat)
	dir = filepath.Join(os.TempDir(), dir)
	at := assert.New(t)
	at.NoError(os.Mkdir(dir, 0700))
	return dir
}

// existsWithContent checks that the given file exists and has the correct content.
func existsWithContent(path string, content []byte, t testing.TB) {
	info, err := os.Stat(path)
	at := assert.New(t)
	at.NoError(err)
	at.Equal(int64(len(content)), info.Size())

	b, err := os.ReadFile(path)
	at.NoError(err)
	at.Equal(content, b)
}

// logFilePath returns the log file name in the given directory for the current fake
// time.
func logFilePath(dir string) string {
	return filepath.Join(dir, "foobar.log")
}

func backupFile(dir string) string {
	return filepath.Join(dir, "foobar-"+fakeTime().UTC().Format(backupTimeFormat)+".log")
}

func backupFileLocal(dir string) string {
	return filepath.Join(dir, "foobar-"+fakeTime().Format(backupTimeFormat)+".log")
}

// fileCount checks that the number of files in the directory is exp.
func fileCount(dir string, exp int, t testing.TB) {
	files, err := os.ReadDir(dir)
	at := assert.New(t)
	at.NoError(err)
	// Make sure no other files were created.
	at.Equal(exp, len(files))
}

// newFakeTime sets the fake "current time" to two days later.
func newFakeTime() {
	lock.Lock()
	defer lock.Unlock()
	fakeCurrentTime = fakeCurrentTime.Add(time.Hour * 24 * 2)
}

func notExist(path string, t testing.TB) {
	_, err := os.Stat(path)
	at := assert.New(t)
	at.Truef(os.IsNotExist(err), "expected to get os.IsNotExist, but instead got %v", err)
}

func exists(path string, t testing.TB) {
	_, err := os.Stat(path)
	at := assert.New(t)
	at.Truef(err == nil, "expected file to exist, but got error from os.Stat: %v", err)
}
