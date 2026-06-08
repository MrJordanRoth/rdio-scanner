package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type adminAccessContext struct {
	userId         uint64
	isAdmin        bool
	managedRoleIds map[uint64]bool
}

func (auth *Auth) getAdminAccessContext(r *http.Request) (*adminAccessContext, bool) {
	userId, ok := auth.getSessionUserId(r)
	if !ok {
		return nil, false
	}

	isAdmin, err := auth.userHasRole(userId, "Admin")
	if err != nil {
		auth.logError("admin.access", err)
		return nil, false
	}

	managedSet := map[uint64]bool{}
	if !isAdmin {
		managed, managedErr := auth.getManagedRoleIds(userId)
		if managedErr != nil {
			auth.logError("admin.access", managedErr)
			return nil, false
		}
		for _, roleId := range managed {
			if roleId > 0 {
				managedSet[roleId] = true
			}
		}
	}

	return &adminAccessContext{
		userId:         userId,
		isAdmin:        isAdmin,
		managedRoleIds: managedSet,
	}, true
}

func (ctx *adminAccessContext) canManageRole(roleId uint64) bool {
	if ctx.isAdmin {
		return true
	}

	if roleId == 0 {
		return false
	}

	return ctx.managedRoleIds[roleId]
}

func (ctx *adminAccessContext) canManageAllRoles(roleIds []uint64) bool {
	if ctx.isAdmin {
		return true
	}

	if len(roleIds) == 0 {
		return false
	}

	hasManagedRole := false
	for _, roleId := range roleIds {
		if !ctx.canManageRole(roleId) {
			return false
		}
		hasManagedRole = true
	}

	return hasManagedRole
}

func (auth *Auth) AdminUsersHandler(w http.ResponseWriter, r *http.Request) {
	ctx, ok := auth.getAdminAccessContext(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		if err := auth.Controller.Users.Read(auth.Controller.Database); err != nil {
			auth.logError("admin.users.list", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		users := []map[string]any{}
		for _, user := range auth.Controller.Users.List {
			if !ctx.isAdmin && !ctx.canManageAllRoles(user.Groups) {
				continue
			}

			users = append(users, map[string]any{
				"id":       user.Id,
				"username": user.Username,
				"email":    user.Email,
				"isSuspended": user.IsSuspended,
				"roles":    user.Groups,
			})
		}

		auth.writeJSON(w, http.StatusOK, users)

	case http.MethodPost:
		m := map[string]any{}
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		username, _ := m["username"].(string)
		email, _ := m["email"].(string)
		password, _ := m["password"].(string)

		username = strings.TrimSpace(username)
		email = strings.TrimSpace(email)

		if username == "" || email == "" || password == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(password), authBcryptCost)
		if err != nil {
			auth.logError("admin.users.create", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		user := NewUser()
		user.Username = username
		user.Email = email
		user.PasswordHash = string(hash)
		user.IsSuspended = false
		user.Groups = userIdsFromAny(m["roles"])
		if len(user.Groups) == 0 {
			user.Groups = userIdsFromAny(m["groups"])
		}

		if !ctx.canManageAllRoles(user.Groups) {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		auth.Controller.Users.Add(user)
		if err = auth.Controller.Users.Write(auth.Controller.Database); err != nil {
			auth.logError("admin.users.create", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		if err = auth.Controller.Users.Read(auth.Controller.Database); err != nil {
			auth.logError("admin.users.create", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		auth.writeJSON(w, http.StatusCreated, map[string]any{"ok": true})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (auth *Auth) AdminUserByIDHandler(w http.ResponseWriter, r *http.Request) {
	ctx, ok := auth.getAdminAccessContext(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	raw := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/users/"), "/")
	segments := strings.Split(raw, "/")
	if len(segments) == 0 || strings.TrimSpace(segments[0]) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(segments[0], 10, 64)
	if err != nil || id == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if len(segments) >= 2 && strings.EqualFold(segments[1], "codes") {
		auth.adminUserCodesHandler(w, r, ctx, id, segments[2:])
		return
	}

	if len(segments) > 1 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodPut:
		m := map[string]any{}
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := auth.Controller.Users.Read(auth.Controller.Database); err != nil {
			auth.logError("admin.users.update", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		user, found := auth.Controller.Users.GetUserById(id)
		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if !ctx.isAdmin && !ctx.canManageAllRoles(user.Groups) {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		if username, ok := m["username"].(string); ok {
			user.Username = strings.TrimSpace(username)
		}
		if email, ok := m["email"].(string); ok {
			user.Email = strings.TrimSpace(email)
		}
		if isSuspended, ok := m["isSuspended"].(bool); ok {
			user.IsSuspended = isSuspended
		}
		password := ""
		if rawPassword, ok := m["password"].(string); ok {
			password = rawPassword
		} else if rawPassword, ok := m["Password"].(string); ok {
			password = rawPassword
		}

		if strings.TrimSpace(password) != "" {
			hash, err := bcrypt.GenerateFromPassword([]byte(password), authBcryptCost)
			if err != nil {
				auth.logError("admin.users.update", err)
				w.WriteHeader(http.StatusExpectationFailed)
				return
			}
			user.PasswordHash = string(hash)
		}

		roles := userIdsFromAny(m["roles"])
		if len(roles) == 0 {
			roles = userIdsFromAny(m["groups"])
		}
		if len(roles) > 0 {
			user.Groups = roles
		}

		if !ctx.canManageAllRoles(user.Groups) {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		if err := auth.Controller.Users.Write(auth.Controller.Database); err != nil {
			auth.logError("admin.users.update", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		if user.IsSuspended {
			auth.revokeSessionsByUser(user.Id)
		}

		auth.writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	case http.MethodDelete:
		if err := auth.Controller.Users.Read(auth.Controller.Database); err != nil {
			auth.logError("admin.users.delete", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		user, found := auth.Controller.Users.GetUserById(id)
		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if !ctx.isAdmin && !ctx.canManageAllRoles(user.Groups) {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		auth.Controller.Users.Remove(user)
		if err := auth.Controller.Users.Write(auth.Controller.Database); err != nil {
			auth.logError("admin.users.delete", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		auth.writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (auth *Auth) adminUserCodesHandler(w http.ResponseWriter, r *http.Request, ctx *adminAccessContext, userId uint64, tail []string) {
	if err := auth.Controller.Users.Read(auth.Controller.Database); err != nil {
		auth.logError("admin.user.codes", err)
		w.WriteHeader(http.StatusExpectationFailed)
		return
	}

	user, found := auth.Controller.Users.GetUserById(userId)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if !ctx.isAdmin && !ctx.canManageAllRoles(user.Groups) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if len(tail) == 0 || (len(tail) == 1 && strings.TrimSpace(tail[0]) == "") {
		switch r.Method {
		case http.MethodGet:
			if err := auth.Controller.AccessCodes.Read(auth.Controller.Database); err != nil {
				auth.logError("admin.user.codes.list", err)
				w.WriteHeader(http.StatusExpectationFailed)
				return
			}

			auth.writeJSON(w, http.StatusOK, auth.Controller.AccessCodes.ListByUser(userId))

		case http.MethodPost:
			m := map[string]any{}
			if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			label, _ := m["label"].(string)
			code, err := auth.newSecureToken(mobileTokenRandomBytes)
			if err != nil {
				auth.logError("admin.user.codes.create", err)
				w.WriteHeader(http.StatusExpectationFailed)
				return
			}

			if err := auth.Controller.AccessCodes.Read(auth.Controller.Database); err != nil {
				auth.logError("admin.user.codes.create", err)
				w.WriteHeader(http.StatusExpectationFailed)
				return
			}

			accessCode := &AccessCode{
				UserId:    userId,
				Code:      code,
				Label:     normalizeAccessCodeLabel(label),
				CreatedAt: uint64(time.Now().Unix()),
			}

			auth.Controller.AccessCodes.Add(accessCode)
			if err := auth.Controller.AccessCodes.Write(auth.Controller.Database); err != nil {
				auth.logError("admin.user.codes.create", err)
				w.WriteHeader(http.StatusExpectationFailed)
				return
			}

			auth.writeJSON(w, http.StatusCreated, accessCode)

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}

		return
	}

	if len(tail) != 1 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	codeId, err := strconv.ParseUint(strings.TrimSpace(tail[0]), 10, 64)
	if err != nil || codeId == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := auth.Controller.AccessCodes.Read(auth.Controller.Database); err != nil {
		auth.logError("admin.user.codes.delete", err)
		w.WriteHeader(http.StatusExpectationFailed)
		return
	}

	if !auth.Controller.AccessCodes.RemoveByIdAndUser(codeId, userId) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err := auth.Controller.AccessCodes.Write(auth.Controller.Database); err != nil {
		auth.logError("admin.user.codes.delete", err)
		w.WriteHeader(http.StatusExpectationFailed)
		return
	}

	auth.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (auth *Auth) AdminUserAddCompatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	auth.AdminUsersHandler(w, r)
}

func (auth *Auth) AdminUserRemoveCompatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	m := map[string]any{}
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ids := userIdsFromAny([]any{m["id"]})
	if len(ids) == 0 {
		if idf, ok := m["id"].(float64); ok {
			ids = []uint64{uint64(idf)}
		}
	}

	if len(ids) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := auth.Controller.Users.Read(auth.Controller.Database); err != nil {
		auth.logError("admin.users.delete.compat", err)
		w.WriteHeader(http.StatusExpectationFailed)
		return
	}

	user, found := auth.Controller.Users.GetUserById(ids[0])
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	ctx, ok := auth.getAdminAccessContext(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if !ctx.isAdmin && !ctx.canManageAllRoles(user.Groups) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	auth.Controller.Users.Remove(user)
	if err := auth.Controller.Users.Write(auth.Controller.Database); err != nil {
		auth.logError("admin.users.delete.compat", err)
		w.WriteHeader(http.StatusExpectationFailed)
		return
	}

	auth.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (auth *Auth) AdminRolesHandler(w http.ResponseWriter, r *http.Request) {
	ctx, ok := auth.getAdminAccessContext(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		if err := auth.Controller.Roles.Read(auth.Controller.Database); err != nil {
			auth.logError("admin.roles.list", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		if !ctx.isAdmin {
			roles := []*Role{}
			for _, role := range auth.Controller.Roles.List {
				if ctx.canManageRole(role.Id) {
					roles = append(roles, role)
				}
			}

			auth.writeJSON(w, http.StatusOK, roles)
			return
		}

		auth.writeJSON(w, http.StatusOK, auth.Controller.Roles.List)

	case http.MethodPost:
		if !ctx.isAdmin {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		m := map[string]any{}
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		name, _ := m["name"].(string)
		if strings.TrimSpace(name) == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		role := NewRole().FromMap(m)
		role.Name = strings.TrimSpace(role.Name)
		role.DelaySeconds = normalizeRoleDelaySeconds(role.DelaySeconds)

		auth.Controller.Roles.Add(role)
		if err := auth.Controller.Roles.Write(auth.Controller.Database); err != nil {
			auth.logError("admin.roles.create", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		if err := auth.Controller.Roles.Read(auth.Controller.Database); err != nil {
			auth.logError("admin.roles.create", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		auth.writeJSON(w, http.StatusCreated, map[string]any{"ok": true})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (auth *Auth) AdminRoleScopesHandler(w http.ResponseWriter, r *http.Request) {
	ctx, ok := auth.getAdminAccessContext(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if !ctx.isAdmin {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := auth.Controller.Systems.Read(auth.Controller.Database); err != nil {
		auth.logError("admin.roles.scopes", err)
		w.WriteHeader(http.StatusExpectationFailed)
		return
	}

	talkgroups := []map[string]any{}
	systems := []map[string]any{}

	for _, system := range auth.Controller.Systems.List {
		systems = append(systems, map[string]any{
			"id":    system.Id,
			"label": system.Label,
		})

		for _, talkgroup := range system.Talkgroups.List {
			talkgroups = append(talkgroups, map[string]any{
				"id":          talkgroup.Id,
				"label":       talkgroup.Label,
				"name":        talkgroup.Name,
				"systemId":    system.Id,
				"systemLabel": system.Label,
			})
		}
	}

	auth.writeJSON(w, http.StatusOK, map[string]any{
		"systems":    systems,
		"talkgroups": talkgroups,
	})
}

func (auth *Auth) AdminActiveSessionsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, ok := auth.getAdminAccessContext(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if !ctx.isAdmin && len(ctx.managedRoleIds) == 0 {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := auth.Controller.Users.Read(auth.Controller.Database); err != nil {
		auth.logError("admin.active-sessions", err)
		w.WriteHeader(http.StatusExpectationFailed)
		return
	}

	sessions := []map[string]any{}
	for _, session := range auth.Controller.Clients.GetActiveSessions() {
		if !ctx.isAdmin {
			user, found := auth.Controller.Users.GetUserById(session.UserId)
			if !found || !ctx.canManageAllRoles(user.Groups) {
				continue
			}
		}

		sessions = append(sessions, map[string]any{
			"userId":            session.UserId,
			"username":          session.Username,
			"ipAddress":         session.IpAddress,
			"connectionSeconds": session.ConnectionSeconds,
		})
	}

	sort.Slice(sessions, func(i int, j int) bool {
		left, _ := sessions[i]["username"].(string)
		right, _ := sessions[j]["username"].(string)
		if strings.EqualFold(left, right) {
			leftID, _ := sessions[i]["userId"].(uint64)
			rightID, _ := sessions[j]["userId"].(uint64)
			return leftID < rightID
		}
		return strings.ToLower(left) < strings.ToLower(right)
	})

	auth.writeJSON(w, http.StatusOK, sessions)
}

func (auth *Auth) AdminKickByUserIDHandler(w http.ResponseWriter, r *http.Request) {
	ctx, ok := auth.getAdminAccessContext(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if !ctx.isAdmin && len(ctx.managedRoleIds) == 0 {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	userId, ok := parseEntityID(r.URL.Path, "/api/admin/kick/")
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := auth.Controller.Users.Read(auth.Controller.Database); err != nil {
		auth.logError("admin.kick", err)
		w.WriteHeader(http.StatusExpectationFailed)
		return
	}

	user, found := auth.Controller.Users.GetUserById(userId)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if !ctx.isAdmin && !ctx.canManageAllRoles(user.Groups) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	kicked := auth.Controller.Clients.KickUser(userId)
	auth.writeJSON(w, http.StatusOK, map[string]any{"ok": true, "kicked": kicked})
}

func (auth *Auth) AdminRoleByIDHandler(w http.ResponseWriter, r *http.Request) {
	ctx, ok := auth.getAdminAccessContext(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if !ctx.isAdmin {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	id, ok := parseEntityID(r.URL.Path, "/api/admin/roles/")
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		m := map[string]any{}
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := auth.Controller.Roles.Read(auth.Controller.Database); err != nil {
			auth.logError("admin.roles.update", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		role, found := auth.Controller.Roles.GetRoleById(id)
		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if name, ok := m["name"].(string); ok {
			role.Name = strings.TrimSpace(name)
		}
		if description, ok := m["description"].(string); ok {
			role.Description = description
		}
		if delaySeconds, ok := m["delaySeconds"].(float64); ok {
			role.DelaySeconds = normalizeRoleDelaySeconds(uint(delaySeconds))
		}
		if connectionLimit, ok := m["connectionLimit"].(float64); ok {
			if connectionLimit < 0 {
				role.ConnectionLimit = 0
			} else {
				role.ConnectionLimit = uint(connectionLimit)
			}
		}

		systems := userIdsFromAny(m["systems"])
		if len(systems) == 0 {
			systems = userIdsFromAny(m["systemIds"])
		}
		role.Systems = systems

		talkgroups := userIdsFromAny(m["talkgroups"])
		if len(talkgroups) == 0 {
			talkgroups = userIdsFromAny(m["talkgroupIds"])
		}
		role.Talkgroups = talkgroups

		managers := userIdsFromAny(m["managers"])
		if len(managers) == 0 {
			managers = userIdsFromAny(m["managerIds"])
		}
		role.Managers = managers

		if err := auth.Controller.Roles.Write(auth.Controller.Database); err != nil {
			auth.logError("admin.roles.update", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		auth.writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	case http.MethodDelete:
		if err := auth.Controller.Roles.Read(auth.Controller.Database); err != nil {
			auth.logError("admin.roles.delete", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		role, found := auth.Controller.Roles.GetRoleById(id)
		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		auth.Controller.Roles.Remove(role)
		if err := auth.Controller.Roles.Write(auth.Controller.Database); err != nil {
			auth.logError("admin.roles.delete", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		auth.writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (auth *Auth) AdminInvitesHandler(w http.ResponseWriter, r *http.Request) {
	ctx, ok := auth.getAdminAccessContext(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		if err := auth.Controller.Invites.Read(auth.Controller.Database); err != nil {
			auth.logError("admin.invites.list", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		if ctx.isAdmin {
			auth.writeJSON(w, http.StatusOK, auth.Controller.Invites.List)
			return
		}

		invites := []*Invite{}
		for _, invite := range auth.Controller.Invites.List {
			if ctx.canManageRole(invite.RoleId) {
				invites = append(invites, invite)
			}
		}

		auth.writeJSON(w, http.StatusOK, invites)

	case http.MethodPost:
		m := map[string]any{}
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		inviteCode, _ := m["inviteCode"].(string)
		if strings.TrimSpace(inviteCode) == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		invite := NewInvite().FromMap(m)
		invite.InviteCode = strings.TrimSpace(invite.InviteCode)

		if !ctx.canManageRole(invite.RoleId) {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		auth.Controller.Invites.Add(invite)
		if err := auth.Controller.Invites.Write(auth.Controller.Database); err != nil {
			auth.logError("admin.invites.create", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		if err := auth.Controller.Invites.Read(auth.Controller.Database); err != nil {
			auth.logError("admin.invites.create", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		auth.writeJSON(w, http.StatusCreated, map[string]any{"ok": true})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (auth *Auth) AdminInviteByIDHandler(w http.ResponseWriter, r *http.Request) {
	ctx, ok := auth.getAdminAccessContext(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	id, ok := parseEntityID(r.URL.Path, "/api/admin/invites/")
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		m := map[string]any{}
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := auth.Controller.Invites.Read(auth.Controller.Database); err != nil {
			auth.logError("admin.invites.update", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		invite, found := auth.Controller.Invites.GetInviteById(id)
		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if !ctx.canManageRole(invite.RoleId) {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		if inviteCode, ok := m["inviteCode"].(string); ok {
			invite.InviteCode = strings.TrimSpace(inviteCode)
		}
		if isUsed, ok := m["isUsed"].(bool); ok {
			invite.IsUsed = isUsed
		}
		if expirationDate, ok := m["expirationDate"].(float64); ok {
			invite.ExpirationDate = uint64(expirationDate)
		}
		if roleId, ok := m["roleId"].(float64); ok {
			invite.RoleId = uint64(roleId)
		}

		if !ctx.canManageRole(invite.RoleId) {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		if err := auth.Controller.Invites.Write(auth.Controller.Database); err != nil {
			auth.logError("admin.invites.update", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		auth.writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	case http.MethodDelete:
		if err := auth.Controller.Invites.Read(auth.Controller.Database); err != nil {
			auth.logError("admin.invites.delete", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		invite, found := auth.Controller.Invites.GetInviteById(id)
		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if !ctx.canManageRole(invite.RoleId) {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		auth.Controller.Invites.Remove(invite)
		if err := auth.Controller.Invites.Write(auth.Controller.Database); err != nil {
			auth.logError("admin.invites.delete", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		auth.writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func parseEntityID(path string, prefix string) (uint64, bool) {
	raw := strings.TrimPrefix(path, prefix)
	raw = strings.Trim(raw, "/")
	if raw == "" {
		return 0, false
	}

	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, false
	}

	return id, true
}

func (auth *Auth) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if b, err := json.Marshal(payload); err == nil {
		w.Write(b)
	}
}
