package cluster

type NodeStatus string

func (s NodeStatus) String() string {
	return string(s)
}

const (
	NodeAlive    NodeStatus = NodeStatus("NodeAlive")
	NodeLeaving  NodeStatus = NodeStatus("NodeLeaving")
	NodeLeft     NodeStatus = NodeStatus("NodeLeft")
	NodeShutdown NodeStatus = NodeStatus("NodeShutdown")
)

////

type MemberStatus string

func (s MemberStatus) String() string {
	return string(s)
}

const (
	MemberNone    MemberStatus = MemberStatus("MemberNone")
	MemberAlive   MemberStatus = MemberStatus("MemberAlive")
	MemberLeaving MemberStatus = MemberStatus("MemberLeaving")
	MemberLeft    MemberStatus = MemberStatus("MemberLeft")
	MemberFailed  MemberStatus = MemberStatus("MemberFailed")
)
