package gracenet

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

const (
	envCountKey = "EG_LISTEN_FDS"
)

var (
	processDir, _ = os.Getwd()
	Gnet          = &Net{}
)

type Net struct {
	inherited   []net.Listener
	active      []net.Listener
	mutex       sync.Mutex
	inheritOnce sync.Once
	fdStart     int
}

func (n *Net) inherit() error {
	var retErr error
	n.inheritOnce.Do(func() {
		n.mutex.Lock()
		defer n.mutex.Unlock()
		countStr := os.Getenv(envCountKey)
		if countStr == "" {
			return
		}
		count, err := strconv.Atoi(countStr)
		if err != nil {
			retErr = fmt.Errorf("invalid fds count: %s=%s", envCountKey, countStr)
			return
		}
		fdStart := n.fdStart
		if fdStart == 0 {
			fdStart = 3 // for stdin,stdout,stderr
		}
		for i := fdStart; i < fdStart+count; i++ {
			file := os.NewFile(uintptr(i), "listener")
			l, err := net.FileListener(file)
			if err != nil {
				file.Close()
				retErr = fmt.Errorf("listen inherited sockfd %d failed:%s", i, err)
				return
			}
			if err := file.Close(); err != nil {
				retErr = fmt.Errorf("close inheritd sockfd %d failed:%s", i, err)
				return
			}
			n.inherited = append(n.inherited, l)
		}
	})
	return retErr
}

// Listen announces on the local network address laddr. The network net must be
// a stream-oriented network: "tcp", "tcp4", "tcp6", "unix" or "unixpacket". It
// returns an inherited net.Listener for the matching network and address, or
// creates a new one using net.Listen.
func (n *Net) Listen(network, address string) (net.Listener, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
		addr, err := net.ResolveTCPAddr(network, address)
		if err != nil {
			return nil, err
		}
		return n.ListenTCP(network, addr)
	case "unix", "unixpacket":
		addr, err := net.ResolveUnixAddr(network, address)
		if err != nil {
			return nil, err
		}
		return n.ListenUnix(network, addr)
	default:
		return nil, net.UnknownNetworkError(network)
	}
}

// ListenTCP announces on the local network address laddr. The network net must
// be: "tcp", "tcp4" or "tcp6". It returns an inherited net.Listener for the
// matching network and address, or creates a new one using net.ListenTCP.
func (n *Net) ListenTCP(network string, address *net.TCPAddr) (*net.TCPListener, error) {
	if err := n.inherit(); err != nil {
		return nil, err
	}

	n.mutex.Lock()
	defer n.mutex.Unlock()

	// look for an inherited listener
	for i, l := range n.inherited {
		if l == nil { // we nil used inherited listeners
			continue
		}
		if isSameAddr(l.Addr(), address) {
			n.inherited[i] = nil
			n.active = append(n.active, l)
			return l.(*net.TCPListener), nil
		}
	}

	// make a fresh listener
	l, err := net.ListenTCP(network, address)
	if err != nil {
		return nil, err
	}
	n.active = append(n.active, l)
	return l, nil
}

// ListenUnix announces on the local network address laddr. The network net
// must be a: "unix" or "unixpacket". It returns an inherited net.Listener for
// the matching network and address, or creates a new one using net.ListenUnix.
func (n *Net) ListenUnix(network string, address *net.UnixAddr) (*net.UnixListener, error) {
	if err := n.inherit(); err != nil {
		return nil, err
	}

	n.mutex.Lock()
	defer n.mutex.Unlock()

	// look for an inherited listener
	for i, l := range n.inherited {
		if l == nil { // we nil used inherited listeners
			continue
		}
		if isSameAddr(l.Addr(), address) {
			n.inherited[i] = nil
			n.active = append(n.active, l)
			return l.(*net.UnixListener), nil
		}
	}

	// make a fresh listener
	l, err := net.ListenUnix(network, address)
	if err != nil {
		return nil, err
	}
	n.active = append(n.active, l)
	return l, nil
}

func isSameAddr(a1, a2 net.Addr) bool {
	if a1.Network() != a2.Network() {
		return false
	}
	a1s := a1.String()
	a2s := a2.String()
	if a1s == a2s {
		return true
	}

	// This allows for ipv6 vs ipv4 local addresses to compare as equal. This
	// scenario is common when listening on localhost.
	const ipv6prefix = "[::]"
	a1s = strings.TrimPrefix(a1s, ipv6prefix)
	a2s = strings.TrimPrefix(a2s, ipv6prefix)
	const ipv4prefix = "0.0.0.0"
	a1s = strings.TrimPrefix(a1s, ipv4prefix)
	a2s = strings.TrimPrefix(a2s, ipv4prefix)
	return a1s == a2s
}

func (n *Net) activeListeners() ([]net.Listener, error) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	ls := make([]net.Listener, len(n.active))
	copy(ls, n.active)
	return ls, nil
}

func (n *Net) StartProcess() (*os.Process, error) {
	listeners, err := n.activeListeners()
	if err != nil {
		return nil, err
	}

	//set file[] from listeners
	files := make([]*os.File, len(listeners))
	for i, l := range listeners {
		files[i], err = l.(filter).File()
		if err != nil {
			return nil, err
		}
	}

	argv0, err := exec.LookPath(os.Args[0])
	if err != nil {
		return nil, err
	}

	var env []string
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, envCountKey) {
			env = append(env, v)
		}
	}

	//set sockfd count to env
	env = append(env, fmt.Sprintf("%s=%d", envCountKey, len(listeners)))

	allFiles := append([]*os.File{os.Stdin, os.Stdout, os.Stderr}, files...)
	process, err := os.StartProcess(argv0, os.Args, &os.ProcAttr{
		Dir:   processDir,
		Env:   env,
		Files: allFiles,
	})
	if err != nil {
		return nil, err
	}
	return process, nil
}

type filter interface {
	File() (*os.File, error)
}
