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
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	yaml "gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/option"
)

const (
	membersFilename       = "members.yaml"
	membersBackupFilename = "members.bak.yaml"
)

type (
	members struct {
		sync.RWMutex `yaml:"-"`

		opt        *option.Options
		file       string
		backupFile string
		lastBuff   []byte

		selfIDChanged bool

		ClusterMembers *membersSlice `yaml:"clusterMembers"`
		KnownMembers   *membersSlice `yaml:"knownMembers"`
	}

	// membersSlice carries unique members whose PeerURL is the primary id.
	membersSlice []*member

	member struct {
		ID      uint64 `yaml:"id"`
		Name    string `yaml:"name"`
		PeerURL string `yaml:"peerURL"`
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

	initMS := make(membersSlice, 0)
	if opt.ClusterRole == "writer" && len(opt.ClusterInitialAdvertisePeerURLs) != 0 {
		initMS = append(initMS, &member{
			Name:    opt.Name,
			PeerURL: opt.ClusterInitialAdvertisePeerURLs[0],
		})
	}
	m.ClusterMembers.update(initMS)

	if len(opt.ClusterJoinURLs) != 0 {
		for _, peerURL := range opt.ClusterJoinURLs {
			initMS = append(initMS, &member{
				PeerURL: peerURL,
			})
		}
	}
	m.KnownMembers.update(initMS)

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

	buff, err := ioutil.ReadFile(m.file)
	if err != nil {
		return err
	}

	membersToLoad := &members{}
	err = yaml.Unmarshal(buff, membersToLoad)
	if err != nil {
		return err
	}

	m.ClusterMembers.update(*membersToLoad.ClusterMembers)
	m.KnownMembers.update(*membersToLoad.KnownMembers)

	return nil
}

// store protected by callers.
func (m *members) store() {
	buff, err := yaml.Marshal(m)
	if err != nil {
		logger.Errorf("BUG: get yaml of %#v failed: %v", m.KnownMembers, err)
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

	err = ioutil.WriteFile(m.file, buff, 0o644)
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
	return m._self()
}

func (m *members) _self() *member {
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

	if m.opt.ClusterRole == "writer" {
		logger.Errorf("BUG: can't get self from cluster members: %s "+
			"knownMembers: %s", m.ClusterMembers, m.KnownMembers)
	}

	peerURL := ""
	if len(m.opt.ClusterInitialAdvertisePeerURLs) != 0 {
		peerURL = m.opt.ClusterInitialAdvertisePeerURLs[0]
	}

	return &member{
		Name:    m.opt.Name,
		PeerURL: peerURL,
	}
}

func (m *members) _selfWithoutID() *member {
	s := m._self()
	s.ID = 0
	return s
}

func (m *members) clusterMembersLen() int {
	m.RLock()
	defer m.RUnlock()
	return m.ClusterMembers.Len()
}

func (m *members) updateClusterMembers(pbMembers []*pb.Member) {
	m.Lock()
	defer m.Unlock()

	olderSelfID := m._self().ID

	ms := pbMembersToMembersSlice(pbMembers)
	// NOTE: The member list of result of MemberAdd carrys empty name
	// of the adding member which is myself.
	ms.update(membersSlice{m._selfWithoutID()})
	m.ClusterMembers.replace(ms)

	selfID := m._self().ID
	if selfID != olderSelfID {
		logger.Infof("self ID changed from %x to %x", olderSelfID, selfID)
		m.selfIDChanged = true
	}

	// NOTE: KnownMembers store members as many as possible
	m.KnownMembers.update(*m.ClusterMembers)

	m.store()
}

func (m *members) knownMembersLen() int {
	m.RLock()
	defer m.RUnlock()
	return m.KnownMembers.Len()
}

func (m *members) knownPeerURLs() []string {
	m.RLock()
	defer m.RUnlock()

	return m.KnownMembers.peerURLs()
}

func (m *members) initCluster() string {
	m.RLock()
	defer m.RUnlock()

	return m.ClusterMembers.initCluster()
}

func pbMembersToMembersSlice(pbMembers []*pb.Member) membersSlice {
	ms := make(membersSlice, 0)
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

func (ms membersSlice) copy() membersSlice {
	copied := make(membersSlice, len(ms))
	for i := 0; i < len(copied); i++ {
		member := *ms[i]
		copied[i] = &member
	}

	return copied
}

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

func (ms membersSlice) initCluster() string {
	ss := make([]string, 0)
	for _, m := range ms {
		if m.Name != "" {
			ss = append(ss, fmt.Sprintf("%s=%s", m.Name, m.PeerURL))
		}
	}
	return strings.Join(ss, ",")
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
