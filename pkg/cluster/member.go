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

package cluster

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/option"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

const (
	membersFilename       = "members.json"
	membersBackupFilename = "members.bak.json"
)

type (
	members struct {
		sync.RWMutex `json:"-"`

		opt        *option.Options
		file       string
		backupFile string
		lastBuff   []byte

		selfIDChanged bool

		ClusterMembers *membersSlice `json:"clusterMembers"`
		KnownMembers   *membersSlice `json:"knownMembers"`
	}

	// membersSlice carries unique members whose PeerURL is the primary id.
	membersSlice []*member

	member struct {
		ID      uint64 `json:"id"`
		Name    string `json:"name"`
		PeerURL string `json:"peerURL"`
	}
)

func newMembers(opt *option.Options) (*members, error) {
	m := &members{
		opt:        opt,
		file:       filepath.Join(opt.AbsMemberDir, membersFilename),
		backupFile: filepath.Join(opt.AbsMemberDir, membersBackupFilename),

		ClusterMembers: newMemberSlices(),
		KnownMembers:   newMemberSlices(),
	}

	err := m.load()
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (m *members) fileExist() bool {
	_, err := os.Stat(m.file)
	return !os.IsNotExist(err)
}

func (m *members) load() error {
	if !m.fileExist() {
		return nil
	}

	buff, err := os.ReadFile(m.file)
	if err != nil {
		return err
	}

	membersToLoad := &members{}
	err = codectool.Unmarshal(buff, membersToLoad)
	if err != nil {
		return err
	}

	m.ClusterMembers.update(*membersToLoad.ClusterMembers)
	m.KnownMembers.update(*membersToLoad.KnownMembers)

	return nil
}

// store protected by callers.
func (m *members) store() {
	buff, err := codectool.MarshalJSON(m)
	if err != nil {
		logger.Errorf("BUG: get json of %#v failed: %v", m.KnownMembers, err)
	}
	if bytes.Equal(m.lastBuff, buff) {
		return
	}

	if m.fileExist() {
		err := os.Rename(m.file, m.backupFile)
		if err != nil {
			logger.Errorf("rename %s to %s failed: %v",
				m.file, m.backupFile, err)
			return
		}
	}

	err = os.WriteFile(m.file, buff, 0o644)
	if err != nil {
		logger.Errorf("write file %s failed: %v", m.file, err)
	} else {
		m.lastBuff = buff
		logger.Infof("store clusterMembers: %s", m.ClusterMembers)
		logger.Infof("store knownMembers  : %s", m.KnownMembers)
	}
}

func (m *members) self() *member {
	m.RLock()
	defer m.RUnlock()
	return m.unsafeSelf()
}

func (m *members) unsafeSelf() *member {
	// NOTE: use clusterMembers before KnownMembers
	// owing to getting real-time ID if possible.
	s := m.ClusterMembers.getByName(m.opt.Name)
	if s != nil {
		return s
	}

	s = m.KnownMembers.getByName(m.opt.Name)
	if s != nil {
		return s
	}

	if m.opt.ClusterRole == "primary" {
		logger.Errorf("BUG: can't get self from cluster members: %s "+
			"knownMembers: %s", m.ClusterMembers, m.KnownMembers)
	}

	peerURL := ""
	if len(m.opt.Cluster.InitialAdvertisePeerURLs) != 0 {
		peerURL = m.opt.Cluster.InitialAdvertisePeerURLs[0]
	}

	return &member{
		Name:    m.opt.Name,
		PeerURL: peerURL,
	}
}

func (m *members) updateClusterMembers(pbMembers []*pb.Member) {
	m.Lock()
	defer m.Unlock()

	self := m.unsafeSelf()
	olderSelfID := self.ID

	ms := pbMembersToMembersSlice(pbMembers)
	// NOTE: The member list of result of MemberAdd carrys empty name
	// of the adding member which is myself.
	self.ID = 0
	ms.update(membersSlice{self})

	m.ClusterMembers.replace(ms)

	selfID := m.unsafeSelf().ID
	if selfID != olderSelfID {
		logger.Infof("self ID changed from %x to %x", olderSelfID, selfID)
		m.selfIDChanged = true
	}

	// NOTE: KnownMembers store members as many as possible
	m.KnownMembers.update(*m.ClusterMembers)

	// When cluster is initialized member by member, persist KnownMembers and ClusterMembers
	// to disk for failure recovery. If a member fails for any reason before it has been added
	// to cluster, the persisted file can be used to continue initialization.
	m.store()
}

func (m *members) knownPeerURLs() []string {
	m.RLock()
	defer m.RUnlock()

	return m.KnownMembers.peerURLs()
}

func pbMembersToMembersSlice(pbMembers []*pb.Member) membersSlice {
	ms := make(membersSlice, 0, len(pbMembers))
	for _, pbMember := range pbMembers {
		var peerURL string
		if len(pbMember.PeerURLs) > 0 {
			peerURL = pbMember.PeerURLs[0]
		}
		ms = append(ms, &member{
			ID:      pbMember.ID,
			Name:    pbMember.Name,
			PeerURL: peerURL,
		})
	}
	return ms
}

func newMemberSlices() *membersSlice {
	return &membersSlice{}
}

func (ms membersSlice) Len() int           { return len(ms) }
func (ms membersSlice) Swap(i, j int)      { ms[i], ms[j] = ms[j], ms[i] }
func (ms membersSlice) Less(i, j int) bool { return ms[i].Name < ms[j].Name }

func (ms membersSlice) String() string {
	ss := make([]string, 0)
	for _, member := range ms {
		name := "<emptyName>"
		if member.Name != "" {
			name = member.Name
		}
		ss = append(ss, fmt.Sprintf("%s(%x)=%s", name, member.ID, member.PeerURL))
	}

	return strings.Join(ss, ",")
}

func (ms membersSlice) peerURLs() []string {
	ss := make([]string, 0)
	for _, m := range ms {
		ss = append(ss, m.PeerURL)
	}
	return ss
}

// update adds the member if there is not the member.
// updates the Name(not empty) of the member with the same PeerURL.
func (ms *membersSlice) update(updateMembers membersSlice) {
	for _, updateMember := range updateMembers {
		if updateMember.PeerURL == "" {
			continue
		}
		found := false
		for _, m := range *ms {
			if m.PeerURL == updateMember.PeerURL {
				found = true
				if updateMember.Name != "" {
					m.Name = updateMember.Name
				}
				if updateMember.ID != 0 {
					m.ID = updateMember.ID
				}
			}
		}
		if !found {
			*ms = append(*ms, updateMember)
		}
	}

	sort.Sort(*ms)
}

// replace replaces membersSlice with the replaceMembers.
func (ms *membersSlice) replace(replaceMembers membersSlice) {
	*ms = replaceMembers
}

func (ms *membersSlice) getByName(name string) *member {
	if name == "" {
		return nil
	}
	for _, m := range *ms {
		if m.Name == name {
			return &member{
				ID:      m.ID,
				Name:    m.Name,
				PeerURL: m.PeerURL,
			}
		}
	}

	return nil
}

func (ms *membersSlice) getByPeerURL(peerURL string) *member {
	if peerURL == "" {
		return nil
	}
	for _, m := range *ms {
		if m.PeerURL == peerURL {
			return &member{
				ID:      m.ID,
				Name:    m.Name,
				PeerURL: m.PeerURL,
			}
		}
	}

	return nil
}

func (ms *membersSlice) deleteByName(name string) {
	msDeleted := make(membersSlice, 0)
	for _, m := range *ms {
		if m.Name == name {
			continue
		}
		msDeleted = append(msDeleted, m)
	}
	*ms = msDeleted
}

func (ms *membersSlice) deleteByPeerURL(peerURL string) {
	msDeleted := make(membersSlice, 0)
	for _, m := range *ms {
		if m.PeerURL == peerURL {
			continue
		}
		msDeleted = append(msDeleted, m)
	}
	*ms = msDeleted
}
