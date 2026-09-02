package ctxkey

type key int

const (
	UserID key = iota
	OrgID
	WorkspaceID
	Role
	Email
)
