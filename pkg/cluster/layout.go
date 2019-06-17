package cluster

import "fmt"

// Cluster store tree layout.
// Status means dynamic, different in every member.
// Config means static, same in every member.
const (
	leaseFormat              = "/leases/%s" //+memberName
	statusMemberPrefix       = "/status/members/"
	statusMemberFormat       = "/status/members/%s"    // +memberName
	statusObjectPrefixFormat = "/status/objects/%s/"   // +objectName
	statusObjectFormat       = "/status/objects/%s/%s" // +objectName +memberName
	configObjectPrefix       = "/config/objects/"
	configObjectFormat       = "/config/objects/%s" // +objectName
)

type (
	// Layout represents storage tree layout.
	Layout struct {
		memberName      string
		statusMemberKey string
	}
)

func (c *cluster) initLayout() {
	c.layout = &Layout{
		memberName: c.opt.Name,
	}
}

// Layout returns cluster layout.
func (c *cluster) Layout() *Layout {
	return c.layout
}

// Lease returns the key of own member lease.
func (l *Layout) Lease() string {
	return fmt.Sprintf(leaseFormat, l.memberName)
}

// OtherLease returns the key of the given member lease.
func (l *Layout) OtherLease(memberName string) string {
	return fmt.Sprintf(leaseFormat, memberName)
}

// StatusMemberPrefix returns the prefix of member status.
func (l *Layout) StatusMemberPrefix() string {
	return statusMemberPrefix
}

// StatusMemberKey returns the key of own member status.
func (l *Layout) StatusMemberKey() string {
	return fmt.Sprintf(statusMemberFormat, l.memberName)
}

// OtherStatusMemberKey returns the key of given member status.
func (l *Layout) OtherStatusMemberKey(memberName string) string {
	return fmt.Sprintf(statusMemberFormat, memberName)
}

// StatusObjectPrefix returns the prefix of object status.
func (l *Layout) StatusObjectPrefix(name string) string {
	return fmt.Sprintf(statusObjectPrefixFormat, name)
}

// StatusObjectKey returns the key of object status.
func (l *Layout) StatusObjectKey(name string) string {
	return fmt.Sprintf(statusObjectFormat, name, l.memberName)
}

// ConfigObjectPrefix returns the prefix of object config.
func (l *Layout) ConfigObjectPrefix() string {
	return configObjectPrefix
}

// ConfigObjectKey returns the key of object config.
func (l *Layout) ConfigObjectKey(name string) string {
	return fmt.Sprintf(configObjectFormat, name)
}
