package handler

import (
	"testing"

	"ChatRoom/internal/model"
)

func TestCanInviteMembers(t *testing.T) {
	tests := []struct {
		name string
		role int
		want bool
	}{
		{
			name: "member cannot invite",
			role: model.GroupRoleMember,
			want: false,
		},
		{
			name: "admin can invite",
			role: model.GroupRoleAdmin,
			want: true,
		},
		{
			name: "owner can invite",
			role: model.GroupRoleOwner,
			want: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := canInviteMembers(test.role)
			if got != test.want {
				t.Fatalf(
					"canInviteMembers(%d) = %v, want %v",
					test.role,
					got,
					test.want,
				)
			}
		})
	}
}

func TestCanRemoveMember(t *testing.T) {
	const (
		ownerID  = uint(1)
		adminID  = uint(2)
		memberID = uint(3)
		otherID  = uint(4)
	)

	tests := []struct {
		name       string
		actorID    uint
		actorRole  int
		targetID   uint
		targetRole int
		want       bool
	}{
		{
			name:       "owner removes admin",
			actorID:    ownerID,
			actorRole:  model.GroupRoleOwner,
			targetID:   adminID,
			targetRole: model.GroupRoleAdmin,
			want:       true,
		},
		{
			name:       "owner removes member",
			actorID:    ownerID,
			actorRole:  model.GroupRoleOwner,
			targetID:   memberID,
			targetRole: model.GroupRoleMember,
			want:       true,
		},
		{
			name:       "admin removes member",
			actorID:    adminID,
			actorRole:  model.GroupRoleAdmin,
			targetID:   memberID,
			targetRole: model.GroupRoleMember,
			want:       true,
		},
		{
			name:       "admin cannot remove admin",
			actorID:    adminID,
			actorRole:  model.GroupRoleAdmin,
			targetID:   otherID,
			targetRole: model.GroupRoleAdmin,
			want:       false,
		},
		{
			name:       "admin cannot remove owner",
			actorID:    adminID,
			actorRole:  model.GroupRoleAdmin,
			targetID:   ownerID,
			targetRole: model.GroupRoleOwner,
			want:       false,
		},
		{
			name:       "member cannot remove member",
			actorID:    memberID,
			actorRole:  model.GroupRoleMember,
			targetID:   otherID,
			targetRole: model.GroupRoleMember,
			want:       false,
		},
		{
			name:       "owner cannot remove self",
			actorID:    ownerID,
			actorRole:  model.GroupRoleOwner,
			targetID:   ownerID,
			targetRole: model.GroupRoleOwner,
			want:       false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := canRemoveMember(
				ownerID,
				test.actorID,
				test.actorRole,
				test.targetID,
				test.targetRole,
			)

			if got != test.want {
				t.Fatalf(
					"canRemoveMember() = %v, want %v",
					got,
					test.want,
				)
			}
		})
	}
}
