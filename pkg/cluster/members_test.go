package cluster

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Etcd Members", func() {
	var (
		members  = newMembers()
		filename = filepath.Join(os.TempDir(), "members.yaml")
	)

	BeforeEach(func() {
		randPort := strconv.Itoa(rand.Intn(10000) + 1024)
		members.Members = append(members.Members, member{Name: "host1-127.0.0.1:" + randPort, PeerListener: "http://127.0.0.1:" + randPort})
		members.Members = append(members.Members, member{Name: "m2", PeerListener: "http://127.0.0.1:2390"})

		os.RemoveAll(filename)

	})

	FSpecify("With members", func() {
		By("Save the members to a file", func() {
			err := members.save2file(filename)
			Expect(err).To(BeNil())
			Expect(filename).To(BeAnExistingFile(), "Members should be save to file %s", filename)
		})

		By("Load from the file immediately", func() {
			reloaded := newMembers()
			err := reloaded.loadFromFile(filename)
			Expect(err).To(BeNil())
			x := reloaded == members
			fmt.Println(x)
			Expect(reloaded.Md5()).To(Equal(members.Md5()), "The loaded data from %s, should be the same as the original.", filename)
		})

		By("Add a member", func() {
			members.Members = append(members.Members, member{Name: "m3", PeerListener: "http://127.0.0.1:2390"})
			err := members.save2file(filename)
			Expect(err).To(BeNil())

			info, err := ioutil.ReadDir(os.TempDir())
			Expect(err).To(BeNil())

			foundBackupFile := false
			for _, f := range info {
				foundBackupFile, err = regexp.Match("members\\.yaml.*\\.bak", []byte(f.Name()))
				if foundBackupFile {
					break
				}
			}

			Expect(foundBackupFile).To(BeTrue(), "A backup file should be generated")

		})
	})
})
