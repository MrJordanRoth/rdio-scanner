import { HttpClient } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { firstValueFrom } from 'rxjs';

export interface AdminUser {
    id?: number;
    username: string;
    email: string;
    isSuspended?: boolean;
    roles?: number[];
}

export interface AdminRole {
    id?: number;
    name: string;
    description?: string;
    delaySeconds?: number;
    connectionLimit?: number;
    managers?: number[];
    systems?: number[];
    talkgroups?: number[];
}

export interface AdminActiveSession {
    userId: number;
    username: string;
    ipAddress?: string;
    connectionSeconds: number;
}

export interface AdminRoleScopeSystem {
    id: number;
    label: string;
}

export interface AdminRoleScopeTalkgroup {
    id: number;
    label?: string;
    name?: string;
    systemId: number;
    systemLabel?: string;
}

export interface AdminRoleScopesResponse {
    systems: AdminRoleScopeSystem[];
    talkgroups: AdminRoleScopeTalkgroup[];
}

export interface AdminInvite {
    id?: number;
    inviteCode: string;
    isUsed: boolean;
    roleId?: number;
    expirationDate?: number;
}

export interface AdminAccessCode {
    id: number;
    userId: number;
    code: string;
    label: string;
    createdAt: string;
}

@Injectable({ providedIn: 'root' })
export class UserManagementService {
    constructor(private ngHttpClient: HttpClient) {}

    async listUsers(): Promise<AdminUser[]> {
        try {
            return await firstValueFrom(this.ngHttpClient.get<AdminUser[]>('/api/admin/users', { withCredentials: true }));
        } catch {
            return [];
        }
    }

    async createUser(payload: { username: string; email: string; password: string; roles: number[]; isSuspended?: boolean }): Promise<void> {
        try {
            await firstValueFrom(this.ngHttpClient.post('/api/admin/users', payload, { withCredentials: true }));
        } catch {
            await firstValueFrom(this.ngHttpClient.post('/api/admin/user-add', payload, { withCredentials: true }));
        }
    }

    async updateUser(id: number, payload: { username?: string; email?: string; roles?: number[]; isSuspended?: boolean; password?: string }): Promise<void> {
        await firstValueFrom(this.ngHttpClient.put(`/api/admin/users/${id}`, payload, { withCredentials: true }));
    }

    async deleteUser(id: number): Promise<void> {
        try {
            await firstValueFrom(this.ngHttpClient.delete(`/api/admin/users/${id}`, { withCredentials: true }));
        } catch {
            await firstValueFrom(this.ngHttpClient.post('/api/admin/user-remove', { id }, { withCredentials: true }));
        }
    }

    async listRoles(): Promise<AdminRole[]> {
        try {
            return await firstValueFrom(this.ngHttpClient.get<AdminRole[]>('/api/admin/roles', { withCredentials: true }));
        } catch {
            return [];
        }
    }

    async createRole(payload: { name: string; description: string; delaySeconds: number; connectionLimit: number; managers: number[]; systems: number[]; talkgroups: number[] }): Promise<void> {
        await firstValueFrom(this.ngHttpClient.post('/api/admin/roles', payload, { withCredentials: true }));
    }

    async updateRole(id: number, payload: { name: string; description: string; delaySeconds: number; connectionLimit: number; managers: number[]; systems: number[]; talkgroups: number[] }): Promise<void> {
        await firstValueFrom(this.ngHttpClient.put(`/api/admin/roles/${id}`, payload, { withCredentials: true }));
    }

    async listActiveSessions(): Promise<AdminActiveSession[]> {
        try {
            return await firstValueFrom(this.ngHttpClient.get<AdminActiveSession[]>('/api/admin/active-sessions', { withCredentials: true }));
        } catch {
            return [];
        }
    }

    async kickUser(userId: number): Promise<void> {
        await firstValueFrom(this.ngHttpClient.post(`/api/admin/kick/${userId}`, null, { withCredentials: true }));
    }

    async listRoleScopes(): Promise<AdminRoleScopesResponse> {
        try {
            return await firstValueFrom(this.ngHttpClient.get<AdminRoleScopesResponse>('/api/admin/role-scopes', { withCredentials: true }));
        } catch {
            return { systems: [], talkgroups: [] };
        }
    }

    async deleteRole(id: number): Promise<void> {
        await firstValueFrom(this.ngHttpClient.delete(`/api/admin/roles/${id}`, { withCredentials: true }));
    }

    async listInvites(): Promise<AdminInvite[]> {
        try {
            return await firstValueFrom(this.ngHttpClient.get<AdminInvite[]>('/api/admin/invites', { withCredentials: true }));
        } catch {
            return [];
        }
    }

    async createInvite(payload: { inviteCode: string; expirationDate?: number; roleId: number }): Promise<void> {
        await firstValueFrom(this.ngHttpClient.post('/api/admin/invites', payload, { withCredentials: true }));
    }

    async updateInvite(id: number, payload: { inviteCode: string; isUsed: boolean; expirationDate?: number; roleId: number }): Promise<void> {
        await firstValueFrom(this.ngHttpClient.put(`/api/admin/invites/${id}`, payload, { withCredentials: true }));
    }

    async deleteInvite(id: number): Promise<void> {
        await firstValueFrom(this.ngHttpClient.delete(`/api/admin/invites/${id}`, { withCredentials: true }));
    }

    async listUserAccessCodes(userId: number): Promise<AdminAccessCode[]> {
        return firstValueFrom(this.ngHttpClient.get<AdminAccessCode[]>(`/api/admin/users/${userId}/codes`, { withCredentials: true }));
    }

    async createUserAccessCode(userId: number, label: string): Promise<AdminAccessCode> {
        return firstValueFrom(this.ngHttpClient.post<AdminAccessCode>(
            `/api/admin/users/${userId}/codes`,
            { label },
            { withCredentials: true },
        ));
    }

    async deleteUserAccessCode(userId: number, codeId: number): Promise<void> {
        await firstValueFrom(this.ngHttpClient.delete(`/api/admin/users/${userId}/codes/${codeId}`, { withCredentials: true }));
    }
}
