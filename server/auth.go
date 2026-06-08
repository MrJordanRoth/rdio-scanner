// Copyright (C) 2019-2026 Chrystian Huot <chrystian.huot@saubeo.solutions>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>
//
// WebSocket API Access Policy:
// This WebSocket API is reserved exclusively for Saubeo Solutions and its native applications.
// Unauthorized access is strictly prohibited.
// See API_ACCESS_POLICY.md for full terms.

package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	authBcryptCost         = 12
	authCookieName         = "rdio_auth_session"
	authSessionDuration    = 24 * time.Hour
	mobileTokenMinLength   = 32
	mobileTokenRandomBytes = 32
	rootAdminUserId        = uint64(1)
)

type Auth struct {
	Controller *Controller
	Sessions   map[string]*AuthSession
	mutex      sync.Mutex
}

var authDebugLogger = log.New(os.Stdout, "auth.debug: ", log.LstdFlags)

type AuthSession struct {
	UserId    uint64
	ExpiresAt time.Time
}

func NewAuth(controller *Controller) *Auth {
	return &Auth{
		Controller: controller,
		Sessions:   map[string]*AuthSession{},
		mutex:      sync.Mutex{},
	}
}

func (auth *Auth) LoginHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		m := map[string]any{}
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var (
			password string
			username string
		)

		switch v := m["username"].(type) {
		case string:
			username = strings.TrimSpace(v)
		}

		switch v := m["password"].(type) {
		case string:
			password = v
		}

		if username == "" || password == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := auth.Controller.Users.Read(auth.Controller.Database); err != nil {
			auth.logError("login", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		user, ok := auth.Controller.Users.GetUserByUsername(username)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if user.IsSuspended {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		roles, err := auth.getUserRoleNames(user)
		if err != nil {
			auth.logError("login", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		sessionToken, err := auth.newSecureToken(48)
		if err != nil {
			auth.logError("login", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		expiresAt := time.Now().Add(authSessionDuration)
		auth.setSession(sessionToken, user.Id, expiresAt)

		http.SetCookie(w, &http.Cookie{
			Name:     authCookieName,
			Value:    sessionToken,
			Path:     "/",
			HttpOnly: true,
			Secure:   auth.shouldUseSecureCookie(r),
			SameSite: http.SameSiteLaxMode,
			Expires:  expiresAt,
			MaxAge:   int(authSessionDuration.Seconds()),
		})

		managedRoleIds, err := auth.getManagedRoleIds(user.Id)
		if err != nil {
			auth.logError("login", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		if b, err := json.Marshal(map[string]any{
			"ok":       true,
			"roles":    roles,
			"managedRoleIds": managedRoleIds,
			"userId":   user.Id,
			"username": user.Username,
		}); err == nil {
			w.Write(b)
		} else {
			w.WriteHeader(http.StatusExpectationFailed)
		}

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (auth *Auth) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		cookie, err := r.Cookie(authCookieName)
		if err == nil && cookie != nil && cookie.Value != "" {
			auth.clearSession(cookie.Value)
		}

		http.SetCookie(w, &http.Cookie{
			Name:     authCookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   auth.shouldUseSecureCookie(r),
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
		})

		auth.writeJSON(w, http.StatusOK, map[string]any{"ok": true})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (auth *Auth) MobileTokenHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		userId, ok := auth.getSessionUserId(r)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		tokenString, err := auth.newSecureToken(mobileTokenRandomBytes)
		if err != nil {
			auth.logError("mobiletoken", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		if len(tokenString) < mobileTokenMinLength {
			auth.logError("mobiletoken", fmt.Errorf("generated token too short"))
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		if err := auth.Controller.MobileTokens.Read(auth.Controller.Database); err != nil {
			auth.logError("mobiletoken", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		if err := auth.Controller.AccessCodes.Read(auth.Controller.Database); err != nil {
			auth.logError("mobiletoken", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		auth.Controller.MobileTokens.Add(&MobileToken{
			TokenString: tokenString,
			UserId:      userId,
			CreatedAt:   uint64(time.Now().Unix()),
		})

		auth.Controller.AccessCodes.Add(&AccessCode{
			UserId:    userId,
			Code:      tokenString,
			Label:     "Legacy Mobile App",
			CreatedAt: uint64(time.Now().Unix()),
		})

		if err := auth.Controller.MobileTokens.Write(auth.Controller.Database); err != nil {
			auth.logError("mobiletoken", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		if err := auth.Controller.AccessCodes.Write(auth.Controller.Database); err != nil {
			auth.logError("mobiletoken", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		if err := auth.Controller.MobileTokens.Read(auth.Controller.Database); err != nil {
			auth.logError("mobiletoken", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		if b, err := json.Marshal(map[string]any{
			"token": tokenString,
		}); err == nil {
			w.Write(b)
		} else {
			w.WriteHeader(http.StatusExpectationFailed)
		}

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (auth *Auth) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		m := map[string]any{}
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var (
			inviteCode string
			password   string
			username   string
		)

		switch v := m["username"].(type) {
		case string:
			username = strings.TrimSpace(v)
		}

		switch v := m["password"].(type) {
		case string:
			password = v
		}

		switch v := m["inviteCode"].(type) {
		case string:
			inviteCode = strings.TrimSpace(v)
		}

		if username == "" || password == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := auth.Controller.Options.Read(auth.Controller.Database); err != nil {
			auth.logError("register", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		var invite *Invite
		if inviteCode != "" {
			if err := auth.Controller.Invites.Read(auth.Controller.Database); err != nil {
				auth.logError("register", err)
				w.WriteHeader(http.StatusExpectationFailed)
				return
			}

			resolvedInvite, ok := auth.Controller.Invites.GetInvite(inviteCode)
			if !ok || resolvedInvite.IsUsed || resolvedInvite.hasExpired() {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if resolvedInvite.RoleId == 0 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if err := auth.Controller.Roles.Read(auth.Controller.Database); err != nil {
				auth.logError("register", err)
				w.WriteHeader(http.StatusExpectationFailed)
				return
			}

			if _, ok := auth.Controller.Roles.GetRoleById(resolvedInvite.RoleId); !ok {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			invite = resolvedInvite
		} else {
			if !auth.Controller.Options.EnablePublicRegistration {
				auth.writeJSON(w, http.StatusForbidden, map[string]any{
					"error": "Registration is currently invite-only",
				})
				return
			}

			if auth.Controller.Options.DefaultRoleId == 0 {
				auth.writeJSON(w, http.StatusForbidden, map[string]any{
					"error": "Public registration is not configured",
				})
				return
			}

			if err := auth.Controller.Roles.Read(auth.Controller.Database); err != nil {
				auth.logError("register", err)
				w.WriteHeader(http.StatusExpectationFailed)
				return
			}

			if _, ok := auth.Controller.Roles.GetRoleById(auth.Controller.Options.DefaultRoleId); !ok {
				auth.writeJSON(w, http.StatusForbidden, map[string]any{
					"error": "Public registration default role is invalid",
				})
				return
			}
		}

		if err := auth.Controller.Users.Read(auth.Controller.Database); err != nil {
			auth.logError("register", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		if _, ok := auth.Controller.Users.GetUserByUsername(username); ok {
			w.WriteHeader(http.StatusConflict)
			return
		}

		email := strings.TrimSpace(fmt.Sprintf("%v", m["email"]))
		for _, user := range auth.Controller.Users.List {
			if email != "" && strings.EqualFold(user.Email, email) {
				w.WriteHeader(http.StatusConflict)
				return
			}
		}

		passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), authBcryptCost)
		if err != nil {
			auth.logError("register", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		groups := []uint64{}
		if invite != nil {
			groups = append(groups, invite.RoleId)
		} else {
			groups = append(groups, auth.Controller.Options.DefaultRoleId)
		}

		auth.Controller.Users.Add(&User{
			Username:     username,
			PasswordHash: string(passwordHash),
			Email:        email,
			Groups:       groups,
		})

		if err := auth.Controller.Users.Write(auth.Controller.Database); err != nil {
			auth.logError("register", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		if err := auth.Controller.Users.Read(auth.Controller.Database); err != nil {
			auth.logError("register", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		if invite != nil {
			invite.IsUsed = true

			if err := auth.Controller.Invites.Write(auth.Controller.Database); err != nil {
				auth.logError("register", err)
				w.WriteHeader(http.StatusExpectationFailed)
				return
			}

			if err := auth.Controller.Invites.Read(auth.Controller.Database); err != nil {
				auth.logError("register", err)
				w.WriteHeader(http.StatusExpectationFailed)
				return
			}
		}

		if b, err := json.Marshal(map[string]any{"ok": true}); err == nil {
			w.WriteHeader(http.StatusCreated)
			w.Write(b)
		} else {
			w.WriteHeader(http.StatusExpectationFailed)
		}

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (auth *Auth) PublicConfigHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		b, err := json.Marshal(map[string]any{
			"anonymousListening": auth.Controller.Options.AnonymousListening,
		})
		if err != nil {
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (auth *Auth) ProfileCodesHandler(w http.ResponseWriter, r *http.Request) {
	userId, ok := auth.getSessionUserId(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		if err := auth.Controller.AccessCodes.Read(auth.Controller.Database); err != nil {
			auth.logError("profile.codes.list", err)
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
			auth.logError("profile.codes.create", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		if err := auth.Controller.AccessCodes.Read(auth.Controller.Database); err != nil {
			auth.logError("profile.codes.create", err)
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
			auth.logError("profile.codes.create", err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		auth.writeJSON(w, http.StatusCreated, accessCode)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (auth *Auth) ProfileCodeByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	userId, ok := auth.getSessionUserId(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/profile/codes/")
	id, err := strconv.ParseUint(strings.TrimSpace(idStr), 10, 64)
	if err != nil || id == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := auth.Controller.AccessCodes.Read(auth.Controller.Database); err != nil {
		auth.logError("profile.codes.delete", err)
		w.WriteHeader(http.StatusExpectationFailed)
		return
	}

	if !auth.Controller.AccessCodes.RemoveByIdAndUser(id, userId) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err := auth.Controller.AccessCodes.Write(auth.Controller.Database); err != nil {
		auth.logError("profile.codes.delete", err)
		w.WriteHeader(http.StatusExpectationFailed)
		return
	}

	auth.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (auth *Auth) getSessionUserId(r *http.Request) (userId uint64, ok bool) {
	cookie, err := r.Cookie(authCookieName)
	if err != nil || cookie == nil || cookie.Value == "" {
		authDebugLogger.Printf("session denied: missing auth cookie path=%s method=%s", r.URL.Path, r.Method)
		return 0, false
	}

	auth.mutex.Lock()
	defer auth.mutex.Unlock()

	now := time.Now()
	for token, session := range auth.Sessions {
		if session.ExpiresAt.Before(now) {
			delete(auth.Sessions, token)
		}
	}

	session, found := auth.Sessions[cookie.Value]
	if !found || session.ExpiresAt.Before(now) {
		authDebugLogger.Printf("session denied: token missing-or-expired path=%s method=%s", r.URL.Path, r.Method)
		return 0, false
	}

	return session.UserId, true
}

func (auth *Auth) getSessionUser(r *http.Request) (*User, bool, error) {
	userId, ok := auth.getSessionUserId(r)
	if !ok {
		return nil, false, nil
	}

	if err := auth.Controller.Users.Read(auth.Controller.Database); err != nil {
		return nil, false, err
	}

	user, found := auth.Controller.Users.GetUserById(userId)
	if !found || user.IsSuspended {
		return nil, false, nil
	}

	return user, true, nil
}

func (auth *Auth) logError(context string, err error) {
	auth.Controller.Logs.LogEvent(LogLevelError, fmt.Sprintf("auth.%s: %s", context, err.Error()))
}

func (invite *Invite) hasExpired() bool {
	if invite.ExpirationDate == 0 {
		return false
	}
	return time.Now().After(time.Unix(int64(invite.ExpirationDate), 0))
}

func (auth *Auth) newSecureToken(size int) (string, error) {
	if size < mobileTokenMinLength {
		size = mobileTokenMinLength
	}

	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (auth *Auth) setSession(token string, userId uint64, expiresAt time.Time) {
	auth.mutex.Lock()
	defer auth.mutex.Unlock()

	now := time.Now()
	for t, session := range auth.Sessions {
		if session.ExpiresAt.Before(now) {
			delete(auth.Sessions, t)
		}
	}

	auth.Sessions[token] = &AuthSession{
		UserId:    userId,
		ExpiresAt: expiresAt,
	}
}

func (auth *Auth) clearSession(token string) {
	auth.mutex.Lock()
	defer auth.mutex.Unlock()

	delete(auth.Sessions, token)
}

func (auth *Auth) revokeSessionsByUser(userId uint64) {
	auth.mutex.Lock()
	defer auth.mutex.Unlock()

	for token, session := range auth.Sessions {
		if session.UserId == userId {
			delete(auth.Sessions, token)
		}
	}
}

func (auth *Auth) RequireAdminOrRoleManager(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("AUTH ATTEMPT to %s", r.URL.Path)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		userId, ok := auth.getSessionUserId(r)
		if !ok {
			auth.logSessionCookieData(r)
			authDebugLogger.Printf("rbac denied: unauthorized (no valid session) path=%s method=%s", r.URL.Path, r.Method)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if auth.isSuperAdminUser(userId) {
			next(w, r)
			return
		}

		isAdmin, err := auth.userHasRole(userId, "Admin")
		if err != nil {
			auth.logError("requireAdminOrRoleManager", err)
			authDebugLogger.Printf("rbac denied: role lookup error userId=%d path=%s method=%s err=%v", userId, r.URL.Path, r.Method, err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		if isAdmin {
			next(w, r)
			return
		}

		managedRoleIds, err := auth.getManagedRoleIds(userId)
		if err != nil {
			auth.logError("requireAdminOrRoleManager", err)
			authDebugLogger.Printf("rbac denied: managed-role lookup error userId=%d path=%s method=%s err=%v", userId, r.URL.Path, r.Method, err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		if len(managedRoleIds) == 0 {
			authDebugLogger.Printf("rbac denied: user is not admin and manages no roles userId=%d path=%s method=%s", userId, r.URL.Path, r.Method)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		next(w, r)
	}
}

func (auth *Auth) RequireAuthenticated(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		if _, ok := auth.getSessionUserId(r); !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

func (auth *Auth) RequireRole(roleName string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("AUTH ATTEMPT to %s", r.URL.Path)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		userId, ok := auth.getSessionUserId(r)
		if !ok {
			auth.logSessionCookieData(r)
			authDebugLogger.Printf("rbac denied: unauthorized (no valid session) requiredRole=%s path=%s method=%s", roleName, r.URL.Path, r.Method)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if auth.isSuperAdminUser(userId) {
			next(w, r)
			return
		}

		hasRole, err := auth.userHasRole(userId, roleName)
		if err != nil {
			auth.logError("requireRole", err)
			authDebugLogger.Printf("rbac denied: role lookup error userId=%d requiredRole=%s path=%s method=%s err=%v", userId, roleName, r.URL.Path, r.Method, err)
			w.WriteHeader(http.StatusExpectationFailed)
			return
		}

		if !hasRole {
			authDebugLogger.Printf("rbac denied: missing role userId=%d requiredRole=%s path=%s method=%s", userId, roleName, r.URL.Path, r.Method)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		next(w, r)
	}
}

func (auth *Auth) getUserRoleNames(user *User) ([]string, error) {
	if err := auth.Controller.Roles.Read(auth.Controller.Database); err != nil {
		return nil, err
	}

	roles := []string{}
	for _, roleId := range user.Groups {
		if role, ok := auth.Controller.Roles.GetRoleById(roleId); ok {
			roles = append(roles, role.Name)
		}
	}

	sort.Strings(roles)

	return roles, nil
}

func (auth *Auth) userHasRole(userId uint64, roleName string) (bool, error) {
	if err := auth.Controller.Users.Read(auth.Controller.Database); err != nil {
		return false, err
	}

	user, ok := auth.Controller.Users.GetUserById(userId)
	if !ok {
		return false, nil
	}

	roles, err := auth.getUserRoleNames(user)
	if err != nil {
		return false, err
	}

	for _, role := range roles {
		if strings.EqualFold(role, roleName) {
			return true, nil
		}
	}

	return false, nil
}

func (auth *Auth) getManagedRoleIds(userId uint64) ([]uint64, error) {
	if err := auth.Controller.Roles.Read(auth.Controller.Database); err != nil {
		return nil, err
	}

	managed := []uint64{}
	for _, role := range auth.Controller.Roles.List {
		for _, managerId := range role.Managers {
			if managerId == userId {
				managed = append(managed, role.Id)
				break
			}
		}
	}

	sort.Slice(managed, func(i int, j int) bool { return managed[i] < managed[j] })

	return managed, nil
}

func (auth *Auth) isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}

	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (auth *Auth) shouldUseSecureCookie(r *http.Request) bool {
	if !auth.isSecureRequest(r) {
		return false
	}

	host := strings.TrimSpace(r.Host)
	if forwardedHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwardedHost != "" {
		host = strings.TrimSpace(strings.Split(forwardedHost, ",")[0])
	}

	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}

	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") || host == "127.0.0.1" || host == "::1" {
		return false
	}

	return true
}

func (auth *Auth) logSessionCookieData(r *http.Request) {
	cookie, err := r.Cookie(authCookieName)

	data := map[string]any{
		"cookieName":      authCookieName,
		"cookiePresent":   err == nil && cookie != nil,
		"cookieValue":     "",
		"sessionFound":    false,
		"sessionUserId":   uint64(0),
		"sessionExpiresAt": "",
		"path":            r.URL.Path,
		"method":          r.Method,
	}

	if err != nil {
		data["cookieError"] = err.Error()
	}

	if cookie != nil {
		data["cookieValue"] = cookie.Value

		auth.mutex.Lock()
		session, found := auth.Sessions[cookie.Value]
		auth.mutex.Unlock()

		if found {
			data["sessionFound"] = true
			data["sessionUserId"] = session.UserId
			data["sessionExpiresAt"] = session.ExpiresAt.UTC().Format(time.RFC3339)
		}
	}

	log.Printf("Session Cookie Data: %v", data)
}

func (auth *Auth) isSuperAdminUser(userId uint64) bool {
	if userId == rootAdminUserId {
		return true
	}

	hasRole, err := auth.userHasRole(userId, "Super Admin")
	if err != nil {
		auth.logError("isSuperAdminUser", err)
		return false
	}

	return hasRole
}
