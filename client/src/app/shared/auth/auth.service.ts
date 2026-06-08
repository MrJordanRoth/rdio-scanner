/*
 * *****************************************************************************
 * Copyright (C) 2019-2026 Chrystian Huot <chrystian.huot@saubeo.solutions>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>
 * ****************************************************************************
 */

import { HttpClient } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { firstValueFrom } from 'rxjs';

export type ProfileAccessCode = {
    id: number;
    userId: number;
    code: string;
    label: string;
    createdAt: string;
};

const SESSION_STORAGE_KEY = 'rdio-authenticated';
const SESSION_ROLES_KEY = 'rdio-auth-roles';
const SESSION_USER_KEY = 'rdio-auth-username';
const SESSION_MANAGED_ROLE_IDS_KEY = 'rdio-auth-managed-role-ids';

@Injectable({ providedIn: 'root' })
export class AuthService {
    constructor(private ngHttpClient: HttpClient) {}

    get username(): string {
        return window?.sessionStorage?.getItem(SESSION_USER_KEY) || '';
    }

    get roles(): string[] {
        const raw = window?.sessionStorage?.getItem(SESSION_ROLES_KEY) || '[]';

        try {
            const roles = JSON.parse(raw);
            return Array.isArray(roles) ? roles.filter((role) => typeof role === 'string') : [];
        } catch {
            return [];
        }
    }

    get managedRoleIds(): number[] {
        const raw = window?.sessionStorage?.getItem(SESSION_MANAGED_ROLE_IDS_KEY) || '[]';

        try {
            const roleIds = JSON.parse(raw);
            return Array.isArray(roleIds)
                ? roleIds.filter((roleId) => typeof roleId === 'number' && Number.isFinite(roleId) && roleId > 0)
                : [];
        } catch {
            return [];
        }
    }

    isAuthenticated(): boolean {
        return window?.sessionStorage?.getItem(SESSION_STORAGE_KEY) === '1';
    }

    isAdmin(): boolean {
        return this.roles.some((role) => role.toLowerCase() === 'admin');
    }

    isRoleManager(): boolean {
        return this.managedRoleIds.length > 0;
    }

    canAccessAdminPortal(): boolean {
        return this.isAdmin() || this.isRoleManager();
    }

    async login(username: string, password: string): Promise<boolean> {
        const response = await firstValueFrom(this.ngHttpClient.post<{ roles?: string[]; managedRoleIds?: number[] }>(
            '/api/auth/login',
            { username, password },
            { withCredentials: true, responseType: 'json' },
        ));

        window?.sessionStorage?.setItem(SESSION_STORAGE_KEY, '1');
        window?.sessionStorage?.setItem(SESSION_USER_KEY, username);
        window?.sessionStorage?.setItem(SESSION_ROLES_KEY, JSON.stringify(response?.roles || []));
        window?.sessionStorage?.setItem(SESSION_MANAGED_ROLE_IDS_KEY, JSON.stringify(response?.managedRoleIds || []));

        return true;
    }

    async logout(): Promise<void> {
        try {
            await firstValueFrom(this.ngHttpClient.post(
                '/api/auth/logout',
                null,
                { withCredentials: true, responseType: 'json' },
            ));
        } catch {
            // We still clear local session state even if backend logout fails.
        }

        this.logoutLocal();
    }

    logoutLocal(): void {
        window?.sessionStorage?.removeItem(SESSION_STORAGE_KEY);
        window?.sessionStorage?.removeItem(SESSION_ROLES_KEY);
        window?.sessionStorage?.removeItem(SESSION_USER_KEY);
        window?.sessionStorage?.removeItem(SESSION_MANAGED_ROLE_IDS_KEY);
    }

    async register(username: string, password: string, inviteCode?: string): Promise<void> {
        await firstValueFrom(this.ngHttpClient.post(
            '/api/auth/register',
            {
                inviteCode: inviteCode || undefined,
                password,
                username,
            },
            { withCredentials: true, responseType: 'json' },
        ));
    }

    async requestMobileToken(): Promise<string> {
        const response = await firstValueFrom(this.ngHttpClient.post<{ token: string }>(
            '/api/auth/mobile-token',
            null,
            { withCredentials: true, responseType: 'json' },
        ));

        return response.token;
    }

    async listProfileAccessCodes(): Promise<ProfileAccessCode[]> {
        return firstValueFrom(this.ngHttpClient.get<ProfileAccessCode[]>(
            '/api/profile/codes',
            { withCredentials: true, responseType: 'json' },
        ));
    }

    async createProfileAccessCode(label: string): Promise<ProfileAccessCode> {
        return firstValueFrom(this.ngHttpClient.post<ProfileAccessCode>(
            '/api/profile/codes',
            { label },
            { withCredentials: true, responseType: 'json' },
        ));
    }

    async deleteProfileAccessCode(id: number): Promise<void> {
        await firstValueFrom(this.ngHttpClient.delete(
            `/api/profile/codes/${id}`,
            { withCredentials: true, responseType: 'json' },
        ));
    }
}
