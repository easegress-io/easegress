package logger

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type fileReopener struct {
	sync.Mutex
	filename string
	file     *os.File
}

// Can't open stderr, it will cause dead lock.
func newFileReopener(filename string) (*fileReopener, error) {
	openfile := func(filename string) (*os.File, error) {
		return os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	}

	file, err := openfile(filename)
	if err != nil {
		return nil, err
	}

	fr := &fileReopener{
		filename: filename,
		file:     file,
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	go func() {
		for {
			<-ch
			fr.Lock()
			err := fr.file.Close()
			if err != nil {
				stderrLogger.Errorf("close %s failed: %v", fr.filename, err)
			}
			fr.file, err = openfile(fr.filename)
			if err != nil {
				stderrLogger.Errorf("open %s failed: %v", fr.filename, err)
			}
			fr.Unlock()
		}
	}()

	return fr, nil
}

func (fr *fileReopener) Write(b []byte) (int, error) {
	fr.Lock()
	defer fr.Unlock()

	return fr.file.Write(b)
}
