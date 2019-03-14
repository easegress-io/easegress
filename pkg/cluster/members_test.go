package cluster

import (
	"math/rand"
	"os"
	"path/filepath"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Etcd Members", func() {
	var (
		members  = newMembers()
		filename = filepath.Join(os.TempDir(), "members.yaml")
	)

	BeforeSuite(func() {
		randPort := strconv.Itoa(rand.Intn(10000) + 1024)
		members.Members = append(members.Members, member{Name: "host1-127.0.0.1:" + randPort, PeerListener: "http://127.0.0.1:" + randPort})
		members.Members = append(members.Members, member{Name: "m2", PeerListener: "http://127.0.0.1:2390"})

		os.RemoveAll(filename)

	})

	It("Should be able to save to file.", func() {
		err := members.save2file(filename)
		Expect(err).To(BeNil())
		Expect(filename).To(BeAnExistingFile())
	})

	It("Should be able to loaded from the file again.", func() {
		reloaded := newMembers()
		err := reloaded.loadFromfile(filename)

		Expect(err).To(BeNil())
		Expect(reloaded).To(Equal(members), "The loaded data from %s, should be the same as the original.", filename)
	})

})
